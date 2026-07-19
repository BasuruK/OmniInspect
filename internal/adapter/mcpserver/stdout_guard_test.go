package mcpserver

import (
	"OmniView/internal/core/domain"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Stdout Discipline Guard
// ==========================================

// safeBuffer is a goroutine-safe bytes.Buffer with a Closer.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) Close() error { return nil }

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

// runServerOverPipe starts the MCP server with an IOTransport whose writer
// is the supplied safeBuffer and whose reader is the returned PipeReader.
// Closing the server context terminates the server.
func runServerOverPipe(t *testing.T, s *Server, out *safeBuffer) (*pipe, func()) {
	t.Helper()

	pr, pw := newPipe()

	serverTransport := &mcp.IOTransport{
		Reader: pr,
		Writer: out,
	}

	serverCtx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- s.ServeWithTransport(serverCtx, serverTransport)
	}()

	return &pipe{Writer: pw, Reader: pr}, func() {
		cancel()
		_ = pw.Close()
		_ = pr.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Log("server did not exit within 2s")
		}
	}
}

// newPipe returns an in-memory, goroutine-safe pipe pair.
func newPipe() (*pipeReader, *pipeWriter) {
	pr, pw := newMemPipe()
	return pr, pw
}

// minimal in-memory pipe using bytes.Buffer + sync.Cond
type memPipe struct {
	mu   sync.Mutex
	cond *sync.Cond
	buf  bytes.Buffer
	open bool
}

func newMemPipe() (*pipeReader, *pipeWriter) {
	mp := &memPipe{open: true}
	mp.cond = sync.NewCond(&mp.mu)
	r := &pipeReader{mp: mp}
	w := &pipeWriter{mp: mp}
	return r, w
}

type pipeReader struct {
	mp *memPipe
}

func (r *pipeReader) Read(p []byte) (int, error) {
	r.mp.mu.Lock()
	defer r.mp.mu.Unlock()
	for r.mp.buf.Len() == 0 && r.mp.open {
		r.mp.cond.Wait()
	}
	if r.mp.buf.Len() == 0 && !r.mp.open {
		return 0, fmt.Errorf("pipe closed")
	}
	return r.mp.buf.Read(p)
}

func (r *pipeReader) Close() error {
	r.mp.mu.Lock()
	defer r.mp.mu.Unlock()
	r.mp.open = false
	r.mp.cond.Broadcast()
	return nil
}

type pipeWriter struct {
	mp *memPipe
}

func (w *pipeWriter) Write(p []byte) (int, error) {
	w.mp.mu.Lock()
	defer w.mp.mu.Unlock()
	if !w.mp.open {
		return 0, fmt.Errorf("pipe closed")
	}
	return w.mp.buf.Write(p)
}

func (w *pipeWriter) Close() error {
	w.mp.mu.Lock()
	defer w.mp.mu.Unlock()
	w.mp.open = false
	w.mp.cond.Broadcast()
	return nil
}

// pipe bundles the two ends so tests don't have to track them separately.
type pipe struct {
	Writer *pipeWriter
	Reader *pipeReader
}

// ==========================================
// Test: every stdout byte is either empty or valid MCP JSON
// ==========================================

// TestStdoutDiscipline_NoForeignBytes drives one tool call through the
// server's stdio transport and inspects every byte that crossed stdout.
// Anything that isn't empty whitespace or a complete JSON line is a
// violation of the MCP framing contract.
func TestStdoutDiscipline_NoForeignBytes(t *testing.T) {
	deps, cleanup := testDeps(t)
	defer cleanup()

	// Seed at least one message so list_traces has something to emit —
	// makes the test cover both the empty case and a real payload case.
	ctx := context.Background()
	msg, err := domain.NewQueueMessage("m1", "proc-A", domain.LogLevelInfo, "hello", time.Now())
	if err != nil {
		t.Fatalf("NewQueueMessage: %v", err)
	}
	if err := deps.TraceAppender.Append(ctx, msg); err != nil {
		t.Fatalf("Append: %v", err)
	}

	srv := NewServer(deps)

	stdout := &safeBuffer{}
	p, stop := runServerOverPipe(t, srv, stdout)
	defer stop()

	// Drive the server as a client. We don't use the SDK's high-level
	// client here because we want raw control over the framing — exactly
	// what an MCP host (Claude Code, Claude Desktop) does.

	// 1. initialize
	if err := writeFramedMessage(p.Writer, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-client", "version": "test"},
		},
	}); err != nil {
		t.Fatalf("write initialize: %v", err)
	}
	if _, err := readFramedMessage(p.Reader); err != nil {
		t.Fatalf("read initialize response: %v", err)
	}

	// 2. notifications/initialized
	if err := writeFramedMessage(p.Writer, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}); err != nil {
		t.Fatalf("write initialized: %v", err)
	}

	// 3. tools/call list_traces
	if err := writeFramedMessage(p.Writer, map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      "list_traces",
			"arguments": map[string]any{"limit": 5},
		},
	}); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}
	if _, err := readFramedMessage(p.Reader); err != nil {
		t.Fatalf("read tools/call response: %v", err)
	}

	// Drain anything that may still be in flight so the assert is stable.
	time.Sleep(50 * time.Millisecond)

	raw := stdout.Bytes()
	if len(raw) == 0 {
		t.Fatal("expected at least one framed message on stdout")
	}

	// Every line must be a JSON object. Empty lines are tolerated (the
	// framing is newline-delimited).
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var anyMsg map[string]any
		if err := json.Unmarshal([]byte(trimmed), &anyMsg); err != nil {
			t.Fatalf("stdout line is not valid JSON: %q (err=%v)", trimmed, err)
		}
		if _, ok := anyMsg["jsonrpc"]; !ok {
			t.Fatalf("stdout JSON missing jsonrpc field: %q", trimmed)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan stdout: %v", err)
	}
}

// writeFramedMessage writes one newline-delimited JSON message.
func writeFramedMessage(w *pipeWriter, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

// readFramedMessage reads one newline-delimited JSON message. Returns the
// decoded map plus the raw line for debugging.
func readFramedMessage(r *pipeReader) (map[string]any, error) {
	var buf bytes.Buffer
	one := make([]byte, 1)
	for {
		n, err := r.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				break
			}
			buf.WriteByte(one[0])
		}
		if err != nil {
			return nil, err
		}
	}
	line := strings.TrimSpace(buf.String())
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		return nil, fmt.Errorf("decode %q: %w", line, err)
	}
	return out, nil
}

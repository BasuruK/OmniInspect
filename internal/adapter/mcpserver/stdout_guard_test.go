package mcpserver

import (
	"OmniView/internal/core/domain"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ==========================================
// Stdout Discipline Guard
// ==========================================

// safeBuffer is a goroutine-safe bytes.Buffer with a Closer. It also
// notifies a channel whenever a complete newline-terminated line has been
// written, so tests can deterministically wait for the server's response to
// land on stdout instead of sleeping for an arbitrary duration.
type safeBuffer struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	pending []byte
	lines   chan string
}

func newSafeBuffer() *safeBuffer {
	return &safeBuffer{lines: make(chan string, 16)}
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, err := b.buf.Write(p)
	b.pending = append(b.pending, p...)
	for {
		idx := bytes.IndexByte(b.pending, '\n')
		if idx < 0 {
			break
		}
		line := string(b.pending[:idx])
		b.pending = b.pending[idx+1:]
		select {
		case b.lines <- line:
		default:
			// Buffer full; the test isn't keeping up. Drop rather than
			// block the server's write path.
		}
	}
	return n, err
}

func (b *safeBuffer) Close() error { return nil }

// waitLine blocks until the next complete stdout line is available (or the
// timeout elapses) and decodes it as JSON.
func (b *safeBuffer) waitLine(t *testing.T, timeout time.Duration) map[string]any {
	t.Helper()
	select {
	case line := <-b.lines:
		var out map[string]any
		if err := json.Unmarshal([]byte(line), &out); err != nil {
			t.Fatalf("decode stdout line %q: %v", line, err)
		}
		return out
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for a response line on stdout")
		return nil
	}
}

func (b *safeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

// pipe bundles the two ends of an OS pipe so tests don't have to track them
// separately. Backed by a real kernel pipe buffer (os.Pipe) instead of a
// hand-rolled bytes.Buffer+sync.Cond: writes of these small JSON-RPC
// messages never block (well under the OS pipe buffer size), and reads
// block until data is available, exactly like real stdio.
type pipe struct {
	Writer *os.File
	Reader *os.File
}

// runServerOverPipe starts the MCP server with an IOTransport whose writer
// is the supplied safeBuffer and whose reader is the returned pipe's read
// end. Closing the server context terminates the server.
func runServerOverPipe(t *testing.T, s *Server, out *safeBuffer) (*pipe, func()) {
	t.Helper()

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

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

	stdout := newSafeBuffer()
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
	// Wait for the real response line on stdout — this is the channel the
	// server actually writes to; p.Reader only carries the request stream.
	if resp := stdout.waitLine(t, 2*time.Second); resp["id"] != float64(1) {
		t.Fatalf("expected response id=1, got %v", resp["id"])
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
	if resp := stdout.waitLine(t, 2*time.Second); resp["id"] != float64(2) {
		t.Fatalf("expected response id=2, got %v", resp["id"])
	}

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
func writeFramedMessage(w *os.File, payload map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

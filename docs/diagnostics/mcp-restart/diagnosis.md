# MCP restart diagnosis

Historical investigation. The user chose stateless HTTP without bearer authentication and explicit agent-collected setup fields instead of the bridge. See [accepted implementation](implementation.md). The archived probe targets the baseline source before this change.

Investigated 2026-10-13. Scope: current source, real HTTP server restart on the same address. No deployed IDE trace or machine reboot captured. Production code unchanged.

## Confirmed failure

The server uses the Go MCP SDK v1.6.1 default stateful Streamable HTTP handler. Session state lives in the handler process. Restart creates a new handler; requests carrying the previous Mcp-Session-Id receive HTTP 404. The same bearer token successfully initializes a fresh session (HTTP 200). This reproduces the need to reconnect without requiring token expiry.

Token loading in cmd/omniview/mcp.go reads a persisted BoltDB entry and generates a token only when absent. Authentication compares the supplied bearer against that value; there is no expiry clock. The HTTP idle timeout controls connections, not token validity.

The MCP indicator means the server started/listens, not that an IDE authenticated or has a usable session. This matches the user's interpretation.

## Evidence and feedback loop

Archive: restart_diagnostic_test.go.txt. Copy to internal/adapter/mcpserver/restart_diagnostic_test.go; run:

`go test ./internal/adapter/mcpserver -run TestDiagnosticRestartRetainsClientSession -count=2 -v`

Both runs failed in approximately 0.01 seconds of test execution:

`same bearer after restart: old session HTTP 404; fresh initialize HTTP 200`

`IDE session requires reinitialization after restart: HTTP 404`

Minimal trigger: initialize, stop server, restart at same address with same bearer, send ping with previous session ID. Removes Oracle, TUI, IDE, reboot and token generation from the reproduction.

Hypotheses ranked before probes: lost session; changed database/token identity; time expiry. Lost session confirmed. Time expiry absent in current authentication code. Changed identity remains a separate possible cause of genuine HTTP 401.

Temporary experimental change: StreamableHTTPOptions{Stateless: true, JSONResponse: true}. Restart probe passed: old-session-header ping HTTP 200; fresh initialize HTTP 200. Stateless initialization emits no session ID; the probe supplies a synthetic old session header in this variant. Existing handshake, wrong-token, foreign-origin and raw-token rejection tests all passed. Options restored after experiment; diagnostic archived outside Go test discovery.

## Fix options

### Small workaround: stateless HTTP

Use Stateless + JSONResponse for request/response tools. Retain bearer authentication, Origin validation and loopback binding. Eliminates stale session lookup. Does not make an IDE retry after it marks an unavailable endpoint disconnected.

Tradeoff: this SDK version rejects server-to-client requests in stateless mode and supplies default client capabilities. Database setup currently uses req.Session.Elicit; the HTTP route would need an explicit supported fallback through TUI setup or complete tool arguments. Verify real IDE behavior across brief downtime before shipping.

### Recommended durable alternative: IDE-facing bridge

Configure the IDE once to launch a lightweight stdio bridge, separate from the TUI. The bridge owns the IDE session and connects upstream to the authenticated loopback server. On connection loss, retry with bounded exponential backoff. On upstream session-not-found 404, discard upstream session and initialize again. On 401, reload the credential from a protected stable path once; if still rejected, report authentication failure without retrying forever.

Preserve tool schemas, map request IDs and cancellation, and forward elicitation across the two sessions. Do not blindly replay mutations after a dropped response: clear_traces, add_database and connect_database may already have executed. Retry only safe reads; report uncertain mutation outcomes. While the TUI is down, keep the bridge alive and return a clear unavailable result within a bounded timeout. After reboot, the IDE launches a fresh bridge and connects once OmniView becomes available. This requires implementation and real IDE verification; it was not tested here.

### Stabilize identity independently

omniview.bolt, omniview.key and omniview-mcp.token currently use working-directory-relative paths. Another launch directory selects another database and can generate another bearer, causing genuine HTTP 401 against an IDE's copied token. Deleting the database also clears the token. Store the MCP credential under a fixed per-user application-data directory, independently of database switching, with platform-appropriate access controls (Windows ACLs; Unix 0600). Migrate the existing credential once; provide explicit rotation/revocation. Avoid silent regeneration on read errors.

Rename the indicator to MCP listening. If client visibility is added, show authenticated/recent client activity separately; a listening flag cannot prove IDE connectivity.

## Acceptance checks for implementation

- Same credential across app restart, working-directory change and database switching.
- Rejected incorrect bearer and foreign Origin remain rejected.
- Existing IDE connection recovers across app stop/start without user reload.
- IDE opens before OmniView after reboot and recovers when OmniView starts.
- Elicitation, cancellation and mutation uncertainty remain correct.
- No tokens in logs; explicit revocation invalidates previous credentials.

## References

- [MCP 2025-11-25 transport session management](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports): stale sessions require reinitialization after 404.
- [Pinned Go SDK transport implementation](https://github.com/modelcontextprotocol/go-sdk/blob/v1.6.1/mcp/streamable.go): default stateful sessions and stateless restrictions.

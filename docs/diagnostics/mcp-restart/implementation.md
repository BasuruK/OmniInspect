# Accepted implementation

Baseline: `81b07023298d470231b9c22cab6e3b44077bf686`.

This supersedes the persistent bridge recommendation in the historical diagnosis and blast-radius visual. No extra process is introduced.

## Behavior

- In-process HTTP remains bound to `127.0.0.1:54332`; retain Origin allowlisting and SDK Host validation.
- Use stateless JSON HTTP responses. No bearer token generation, persistence API, token file writes, or authorization requirement. Ignore legacy Authorization and session headers.
- An existing client can call tools after an app/server restart without a new initialization. Actual IDE retry policy during downtime remains client-owned.
- The agent gathers all database fields in IDE conversation; no MCP elicitation form. Missing fields and invalid ports return structured feedback without saving.
- Extend `add_database` with optional `connect`, default false. Preserve existing save-only behavior and first-configuration selection.
- With `connect: true`, atomically insert without overwriting another configuration or selecting it prematurely, then reuse existing login/permission/package verification and database selection logic.
- Success returns saved/connected state and the database ID. Failed verification leaves settings saved, preserves previous selection, and returns the failure code, ID, saved=true and connected=false. Retry using `connect_database`.
- Duplicate IDs remain rejected, including simultaneous add calls. Do not blindly replay mutations after response loss.
- Do not echo passwords in connection errors, output or close-error logs. Existing connection verification closes its adapter after checks; no permanent connection is added.

## Validation

Regression tests use real MCP requests over HTTP and in-memory transports, real temporary BoltDB storage, and an Oracle adapter fake at the external database boundary.

Covered: no-token HTTP, same-client restart recovery, obsolete session/header compatibility, Origin and Host rejection, one-call save/connect, missing-field feedback without elicitation, saved-but-unconnected failure with/without an existing selection, password redaction, connection retry, save-only behavior and duplicate protection.

Validation passed: `make build`, `go test ./... -count=1`, and `go vet ./...`. Actual IDE restart/retry and a live Oracle login remain unverified.

Historical diagnosis/proposal HTML files document the decision process, not an active bridge or configuration requirement. The archived diagnostic `.go.txt` targets the baseline transport signature; it is historical evidence, not a current regression test.

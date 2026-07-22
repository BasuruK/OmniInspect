<!-- lean-ctx-owned: PROJECT-LEAN-CTX.md v1 -->
<!-- lean-ctx-rules -->
<!-- version: 8 -->

lean-ctx shadow mode: native file/search/shell calls auto-route to ctx_* — no tool-mapping needed.
Exclusive tools (no native trigger): ctx_compose (understand code, call first), ctx_search(action=symbol) (exact symbol), ctx_search(action=semantic) (by meaning), ctx_callgraph (callers), ctx_knowledge / ctx_session (memory).

OUTPUT: never echo tool output, no narration comments, show only changed code.
<!-- lean-ctx-compression -->
OUTPUT STYLE: dense
- Each statement = one atomic fact line
- Use abbreviations: fn, cfg, impl, deps, req, res, ctx, err, ret
- Diff lines only (+/-/~), never repeat unchanged code
- Symbols: → (causes), + (adds), − (removes), ~ (modifies), ∴ (therefore)
- No narration, no filler, no hedging
- BUDGET: ≤200 tokens per response unless code block required
<!-- /lean-ctx-compression -->
<!-- /lean-ctx-rules -->

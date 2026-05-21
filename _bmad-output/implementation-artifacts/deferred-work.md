# Deferred Work Log

## Deferred from: code review of story 4.1-broadcast-message-isolation (2026-05-21)

- No test coverage for broadcast mode — new feature tests out of scope for review
- `NewQueueMessage` hardcodes `mode: "Global"` — messages only arrive via UnmarshalJSON, default is correct
- `GetBroadcastMode` silently returns zero value when key absent — "Global" is correct default, intentional
- Missing `ConfigRepository` port interface entry — follows existing BoltAdapter direct-access pattern
- `IsBroadcast()` not case-insensitive — PL/SQL always produces exact case, latent defense gap

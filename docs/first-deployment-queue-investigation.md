# First-deployment queue initialization failure

Confirmed on 2026-10-21 using the saved **Master Dep** environment (Oracle 19c).

## Cause

`OMNI_TRACER_API.Initialize` creates a sharded queue, then immediately calls
`GET_QUEUE_PARAMETER('SHARD_NUM')` before configuring that queue. On this database,
the read can raise `ORA-00600 [Queue parameter not initialized]`. Execution never
reaches `SET_QUEUE_PARAMETER` or `START_QUEUE`.

The initial live startup reproduced the screenshot exactly:

```text
failed to initialize tracer package:
ORA-00600: internal error code, arguments: [Queue parameter not initialized]
ORA-06512: at "IFSAPP.OMNI_TRACER_API", line 37
OMNI_TRACER_QUEUE: enqueue=NO, dequeue=NO
```

The package and queue were absent before this run. Afterwards, the package was
valid and the queue existed, despite initialization rolling back.

Retrying **only Initialize**, without package deployment, enabled both queue
directions. Package specification/body deployment timestamps stayed unchanged.
This explains recovery after restart: the retry takes the existing-queue path.
Package redeployment is not necessary for that recovery.

## Isolation and probes

A temporary opt-in Go diagnostic loaded saved credentials read-only, kept the
application cache unchanged, and used separate queues for repeated experiments.
No credentials were printed. The minimized failing operation was queue creation
followed by a parameter read in the same PL/SQL call; no TUI, service, package
deployment, or autonomous transaction was needed.

| Probe | Result |
| --- | --- |
| Embedded fresh initializer before fix | Exact Oracle error, twice |
| Minimal create/read in one call | Exact Oracle error, twice |
| Create/read in separate calls | Passed twice |
| Set SHARD_NUM before reading | Passed twice |
| Commit before reading | One pass, one failure |
| One-second delay before reading | One failure, one pass |

## Correction and verification

`assets/sql/Omni_Tracer.sql` now skips the parameter read when this invocation
created the queue, then sets the existing default of four shards. Existing queues
still read their configuration, preserving custom tuning. Other errors propagate.

The shared Oracle integration fixture now covers a genuinely absent queue as well
as an existing queue configured with eight shards. Previously, it always
preconfigured the queue, bypassing the failure.

The new regression failed before the correction. After correction, live fresh
initialization passed **5/5**, including repeated initialization, stopped queue
directions, injected start races, and propagation of unrelated errors. The existing
eight-shard fixture also passed. Full startup deployment succeeded on Master Dep.
Final inspection: both directions enabled; zero diagnostic objects remain.

`make build` and `make test` passed: 370 top-level tests passed; four opt-in Oracle
entry points skipped in the default run. Live fixture results are recorded above.

Permanent regression entry point (requires the existing
`OMNI_RECOVERY_ORACLE_*` disposable-schema configuration):

```text
go test -v -count=1 -run 'Test(Fresh|Existing)QueueInitializationOracle' ./internal/adapter/storage/oracle
```

Live tests used the same fixture through a temporary saved-environment connector,
which was removed afterwards. Other database versions/patch levels were not tested.

API reference: [Oracle 19c DBMS_AQADM](https://docs.oracle.com/en/database/oracle/oracle-database/19/arpls/DBMS_AQADM.html).
The error behavior above is established by live probes, not an identified Oracle
bug number.

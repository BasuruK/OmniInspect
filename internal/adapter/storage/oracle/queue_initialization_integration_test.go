package oracle

import (
	"OmniView/assets"
	"context"
	"strings"
	"testing"
)

func TestExistingQueueInitializationOracle(t *testing.T) {
	testQueueInitialization(t, recoveryOracleConnection(t), true)
}

func TestFreshQueueInitializationOracle(t *testing.T) {
	testQueueInitialization(t, recoveryOracleConnection(t), false)
}

// Uses only its own objects; existing queues start with nondefault shard tuning.
func testQueueInitialization(t *testing.T, db *OracleAdapter, existingQueue bool) {
	t.Helper()
	ctx := context.Background()
	const packageName = "OMNI_QUEUE_STATE_TEST_API"
	const queueName = "OMNI_QUEUE_STATE_TEST_QUEUE"
	const payloadName = "OMNI_QUEUE_STATE_TEST_TYPE"
	rows, err := db.Fetch(ctx, `SELECT COUNT(*) FROM user_objects WHERE object_name IN (
		'OMNI_QUEUE_STATE_TEST_API', 'OMNI_QUEUE_STATE_TEST_QUEUE', 'OMNI_QUEUE_STATE_TEST_TYPE')`)
	if err != nil {
		t.Fatal(err)
	}
	count, err := parseCountResult(rows)
	if err != nil || count != 0 {
		t.Fatal("queue-state test objects must not already exist")
	}
	execute := func(statement string) {
		t.Helper()
		if err := db.ExecuteStatement(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	// Register cleanup before creation so failures also remove partial fixtures.
	t.Cleanup(func() {
		for _, flags := range []string{"TRUE, FALSE", "FALSE, TRUE"} {
			if err := db.ExecuteStatement(ctx, `BEGIN DBMS_AQADM.STOP_QUEUE('`+queueName+`', `+flags+`); END;`); err != nil &&
				!strings.Contains(err.Error(), "ORA-24210") && !strings.Contains(err.Error(), "ORA-24010") {
				t.Error(err)
			}
		}
		for _, statement := range []string{
			`BEGIN DBMS_AQADM.DROP_SHARDED_QUEUE('` + queueName + `'); END;`,
			`DROP PACKAGE ` + packageName,
			`DROP TYPE ` + payloadName,
		} {
			if err := db.ExecuteStatement(ctx, statement); err != nil &&
				!strings.Contains(err.Error(), "ORA-24010") && !strings.Contains(err.Error(), "ORA-04043") {
				t.Error(err)
			}
		}
	})
	execute(`CREATE NONEDITIONABLE TYPE ` + payloadName + ` AS OBJECT (JSON_DATA BLOB)`)
	wantShards := "4"
	if existingQueue {
		wantShards = "8"
		execute(`BEGIN
			DBMS_AQADM.CREATE_SHARDED_QUEUE(queue_name => '` + queueName + `',
				multiple_consumers => TRUE, queue_payload_type => '` + payloadName + `');
			DBMS_AQADM.SET_QUEUE_PARAMETER('` + queueName + `', 'SHARD_NUM', 8);
		END;`)
	}
	sql, err := assets.GetSQLFile("Omni_Tracer.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, body, found := strings.Cut(string(sql), "PROCEDURE Initialize IS")
	if !found {
		t.Fatal("embedded Initialize procedure missing")
	}
	body, _, found = strings.Cut(body, "END Initialize;")
	if !found {
		t.Fatal("embedded Initialize procedure terminator missing")
	}
	procedure := strings.ReplaceAll("PROCEDURE Initialize IS"+body+"END Initialize;", "OMNI_TRACER_PAYLOAD_TYPE", payloadName)
	execute(`CREATE PACKAGE ` + packageName + ` AS
		TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := '` + queueName + `';
		PROCEDURE Initialize; END;`)
	deployBody := func(procedure string) {
		t.Helper()
		execute(`CREATE OR REPLACE PACKAGE BODY ` + packageName + ` AS ` + procedure + ` END;`)
	}
	initialize := `BEGIN ` + packageName + `.Initialize; END;`
	assertReady := func() {
		t.Helper()
		execute(`DECLARE ready_ NUMBER; shards_ NUMBER; BEGIN
			SELECT COUNT(*) INTO ready_ FROM user_queues WHERE name = '` + queueName + `'
				AND TRIM(enqueue_enabled) = 'YES' AND TRIM(dequeue_enabled) = 'YES';
			DBMS_AQADM.GET_QUEUE_PARAMETER('` + queueName + `', 'SHARD_NUM', shards_);
			IF ready_ != 1 OR shards_ IS NULL OR shards_ != ` + wantShards + ` THEN
				RAISE_APPLICATION_ERROR(-20005, 'Queue readiness or shard tuning lost');
			END IF;
		END;`)
	}
	deployBody(procedure)
	// Initially both directions are stopped. Then cover each partially stopped state.
	for _, stopFlags := range []string{"", "TRUE, FALSE", "FALSE, TRUE", "TRUE, TRUE"} {
		if stopFlags != "" {
			execute(`BEGIN DBMS_AQADM.STOP_QUEUE('` + queueName + `', ` + stopFlags + `); END;`)
		}
		execute(initialize)
		execute(initialize)
		assertReady()
	}
	// Deterministically replay another initializer enabling just one direction
	// between our state read and START_QUEUE, leaving the other direction stopped.
	const marker = "-- Sharded queues reject starting a direction that is already enabled."
	if strings.Count(procedure, marker) != 1 {
		t.Fatal("queue-start race injection seam missing")
	}
	for _, direction := range []string{"enqueue", "dequeue", "both"} {
		flags := "TRUE, FALSE"
		condition := direction
		if direction == "dequeue" {
			flags = "FALSE, TRUE"
		} else if direction == "both" {
			flags = "TRUE, TRUE"
			condition = "enqueue"
		}
		probe := `IF ` + condition + `_enabled_ = 'NO' THEN
			DBMS_AQADM.START_QUEUE(TRACER_QUEUE_NAME, ` + flags + `);
		END IF;
		`
		deployBody(strings.Replace(procedure, marker, probe+marker, 1))
		execute(`BEGIN DBMS_AQADM.STOP_QUEUE('` + queueName + `'); END;`)
		execute(initialize)
		assertReady()
	}
	// A peer repeatedly switching directions must exhaust the retry bound,
	// not report success with one direction still disabled.
	probe := `IF enqueue_enabled_ = 'NO' THEN
		IF dequeue_enabled_ = 'YES' THEN
			DBMS_AQADM.STOP_QUEUE(TRACER_QUEUE_NAME, FALSE, TRUE);
		END IF;
		DBMS_AQADM.START_QUEUE(TRACER_QUEUE_NAME, TRUE, FALSE);
	ELSE
		DBMS_AQADM.STOP_QUEUE(TRACER_QUEUE_NAME, TRUE, FALSE);
		DBMS_AQADM.START_QUEUE(TRACER_QUEUE_NAME, FALSE, TRUE);
	END IF;
	`
	deployBody(strings.Replace(procedure, marker, probe+marker, 1))
	execute(`BEGIN DBMS_AQADM.STOP_QUEUE('` + queueName + `'); END;`)
	if err := db.ExecuteStatement(ctx, initialize); err == nil || !strings.Contains(err.Error(), "ORA-24210") {
		t.Fatalf("persistent direction race must fail after bounded retries: %v", err)
	}
	// Restore readiness so the next case can stop both directions normally.
	deployBody(procedure)
	execute(initialize)
	// Replace the START_QUEUE call itself so the production error handler runs.
	start := strings.Index(procedure, "DBMS_AQADM.START_QUEUE (")
	if start < 0 {
		t.Fatal("queue-start error injection seam missing")
	}
	end := strings.Index(procedure[start:], ");")
	if end < 0 {
		t.Fatal("queue-start call terminator missing")
	}
	deployBody(procedure[:start] + `RAISE_APPLICATION_ERROR(-20006, 'Injected unrelated failure');` + procedure[start+end+2:])
	execute(`BEGIN DBMS_AQADM.STOP_QUEUE('` + queueName + `'); END;`)
	if err := db.ExecuteStatement(ctx, initialize); err == nil || !strings.Contains(err.Error(), "ORA-20006") {
		t.Fatalf("unrelated initialization failure must propagate: %v", err)
	}
}

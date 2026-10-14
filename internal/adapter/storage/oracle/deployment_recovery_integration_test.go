package oracle

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
)

func recoveryOracleConnection(t *testing.T) *OracleAdapter {
	t.Helper()
	username := os.Getenv("OMNI_RECOVERY_ORACLE_USER")
	if username == "" {
		t.Skip("set OMNI_RECOVERY_ORACLE_* to use a disposable Oracle schema")
	}
	if !strings.HasPrefix(strings.ToUpper(username), "OMNI_RECOVERY_TEST_") {
		t.Fatal("test requires a disposable OMNI_RECOVERY_TEST_* schema")
	}
	portNumber := 1521
	if value := os.Getenv("OMNI_RECOVERY_ORACLE_PORT"); value != "" {
		var err error
		portNumber, err = strconv.Atoi(value)
		if err != nil {
			t.Fatal("invalid test port")
		}
	}
	port, err := domain.NewPort(portNumber)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := domain.NewDatabaseSettings("recovery-test", os.Getenv("OMNI_RECOVERY_ORACLE_SERVICE"),
		os.Getenv("OMNI_RECOVERY_ORACLE_HOST"), port, username, os.Getenv("OMNI_RECOVERY_ORACLE_PASSWORD"))
	if err != nil {
		t.Fatal("invalid test connection settings")
	}
	db := NewOracleAdapter(settings)
	ctx := context.Background()
	if err := db.Connect(ctx); err != nil {
		t.Fatal("test Oracle connection failed")
	}
	t.Cleanup(func() { _ = db.Close(ctx) })
	return db
}

// This opt-in test leaves its objects in an empty disposable OMNI_RECOVERY_TEST_*
// schema for inspection. It never uses saved application credentials.
func TestDeploymentRecoveryOracle(t *testing.T) {
	db := recoveryOracleConnection(t)
	ctx := context.Background()

	rows, err := db.Fetch(ctx, `SELECT COUNT(*) FROM user_objects WHERE object_name LIKE 'OMNI_%'`)
	if err != nil {
		t.Fatal(err)
	}
	count, err := parseCountResult(rows)
	if err != nil || count != 0 {
		t.Fatal("test schema must contain no existing OMNI_* objects")
	}
	execute := func(statement string) {
		t.Helper()
		if err := db.ExecuteStatement(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	assertPackage := func(want bool) {
		t.Helper()
		got, err := db.PackageExists(ctx, "OMNI_RECOVERY_TEST_API")
		if err != nil || got != want {
			t.Fatalf("package readiness = %t, want %t; error: %v", got, want, err)
		}
	}

	execute(`CREATE PACKAGE OMNI_RECOVERY_TEST_API AS PROCEDURE Ping; END;`)
	assertPackage(false)
	// Oracle can create an INVALID body while leaving the specification VALID.
	err = db.ExecuteStatement(ctx, `CREATE PACKAGE BODY OMNI_RECOVERY_TEST_API AS
		PROCEDURE Ping IS BEGIN Missing_Procedure; END; END;`)
	if err != nil && !strings.Contains(err.Error(), "ORA-24344") {
		t.Fatal(err)
	}
	assertPackage(false)
	execute(`CREATE OR REPLACE PACKAGE BODY OMNI_RECOVERY_TEST_API AS
		PROCEDURE Ping IS BEGIN NULL; END; END;`)
	assertPackage(true)

	sql, err := assets.GetSQLFile("Omni_Tracer.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeployFile(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	// Model an interruption after queue creation but before configuration/start.
	execute(`BEGIN DBMS_AQADM.CREATE_SHARDED_QUEUE(
		queue_name => 'OMNI_TRACER_QUEUE', multiple_consumers => TRUE,
		queue_payload_type => 'OMNI_TRACER_PAYLOAD_TYPE'); END;`)
	initialize, err := assets.GetInsFile("Omni_Initialize.ins")
	if err != nil {
		t.Fatal(err)
	}
	execute(string(initialize))
	execute(`DECLARE shards_ NUMBER; ready_ NUMBER; BEGIN
		DBMS_AQADM.GET_QUEUE_PARAMETER('OMNI_TRACER_QUEUE', 'SHARD_NUM', shards_);
		SELECT COUNT(*) INTO ready_ FROM user_queues WHERE name = 'OMNI_TRACER_QUEUE'
			AND TRIM(enqueue_enabled) = 'YES' AND TRIM(dequeue_enabled) = 'YES';
		IF shards_ IS NULL OR shards_ < 1 OR ready_ != 1 THEN
			RAISE_APPLICATION_ERROR(-20001, 'Queue readiness was not recovered');
		END IF;
	END;`)

	execute(`BEGIN
		DBMS_AQADM.SET_QUEUE_PARAMETER('OMNI_TRACER_QUEUE', 'SHARD_NUM', 8);
		OMNI_TRACER_API.Register_Subscriber('RECOVERY_TEST');
		OMNI_TRACER_API.Trace_Message('preserve through repair');
		DBMS_AQADM.STOP_QUEUE(queue_name => 'OMNI_TRACER_QUEUE');
	END;`)
	execute(string(initialize))
	execute(string(initialize))
	execute(`DECLARE
		messages_ OMNI_TRACER_PAYLOAD_ARRAY; ids_ OMNI_TRACER_RAW_ARRAY;
		count_ INTEGER; subscribers_ NUMBER; shards_ NUMBER;
	BEGIN
		DBMS_AQADM.GET_QUEUE_PARAMETER('OMNI_TRACER_QUEUE', 'SHARD_NUM', shards_);
		IF shards_ IS NULL OR shards_ != 8 THEN
			RAISE_APPLICATION_ERROR(-20004, 'Repair overwrote existing shard tuning');
		END IF;
		SELECT COUNT(*) INTO subscribers_ FROM user_queue_subscribers
			WHERE queue_name = 'OMNI_TRACER_QUEUE' AND consumer_name = 'RECOVERY_TEST';
		IF subscribers_ != 1 THEN
			RAISE_APPLICATION_ERROR(-20002, 'Repair lost an existing subscriber');
		END IF;
		OMNI_TRACER_API.Dequeue_Array_Events('RECOVERY_TEST', 1, DBMS_AQ.NO_WAIT,
			messages_, ids_, count_);
		IF count_ != 1 THEN
			RAISE_APPLICATION_ERROR(-20003, 'Repair lost a queued message or left dequeue disabled');
		END IF;
	END;`)
}

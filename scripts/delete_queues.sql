/*
OMNI TRACER - CLEANUP SCRIPT (Multi-Schema Version)
⚠️  WARNING: This will delete all trace messages and remove the queue infrastructure.
   
Copyright (c) 2025.
*/

SET SERVEROUTPUT ON;

DECLARE
    v_target_schema VARCHAR2(30) := 'IFSAPP';  -- Change this to target schema
    v_q_name        VARCHAR2(30) := 'OMNI_TRACER_QUEUE';
    v_full_q_name   VARCHAR2(61);
    v_count         NUMBER;
    
    PROCEDURE drop_queue_safe(p_schema VARCHAR2, p_queue_name VARCHAR2) IS
        v_qualified_name VARCHAR2(61) := p_schema || '.' || p_queue_name;
        v_queue_table VARCHAR2(61);
    BEGIN
        -- Check if queue exists in target schema
        SELECT COUNT(*) INTO v_count
        FROM dba_queues
        WHERE owner = p_schema AND name = p_queue_name;
        
        IF v_count = 0 THEN
            DBMS_OUTPUT.PUT_LINE('⚠️  Queue does not exist in ' || p_schema || ': ' || p_queue_name);
            RETURN;
        END IF;
        
        -- Get queue table name for later cleanup
        BEGIN
            SELECT queue_table INTO v_queue_table
            FROM dba_queues
            WHERE owner = p_schema AND name = p_queue_name;
            v_queue_table := p_schema || '.' || v_queue_table;
        EXCEPTION
            WHEN OTHERS THEN
                v_queue_table := NULL;
        END;
        
        -- Try to start queue first (in case it's in a partial state)
        BEGIN
            DBMS_AQADM.START_QUEUE(queue_name => v_qualified_name, enqueue => TRUE, dequeue => TRUE);
            DBMS_OUTPUT.PUT_LINE('✓ Queue started: ' || v_qualified_name);
        EXCEPTION
            WHEN OTHERS THEN
                IF SQLCODE = -24002 THEN -- Queue already enabled
                    DBMS_OUTPUT.PUT_LINE('⚠️  Queue already started');
                ELSE
                    DBMS_OUTPUT.PUT_LINE('⚠️  Could not start queue: ' || SQLERRM);
                END IF;
        END;
        
        -- Stop queue (both enqueue and dequeue)
        BEGIN
            DBMS_AQADM.STOP_QUEUE(queue_name => v_qualified_name, enqueue => TRUE, dequeue => TRUE, wait => TRUE);
            DBMS_OUTPUT.PUT_LINE('✓ Queue stopped: ' || v_qualified_name);
        EXCEPTION
            WHEN OTHERS THEN
                DBMS_OUTPUT.PUT_LINE('⚠️  Error stopping queue: ' || SQLERRM || ' (will try force drop)');
        END;
        
        -- Drop queue
        BEGIN
            DBMS_AQADM.DROP_QUEUE(queue_name => v_qualified_name);
            DBMS_OUTPUT.PUT_LINE('✓ Queue dropped: ' || v_qualified_name);
        EXCEPTION
            WHEN OTHERS THEN
                DBMS_OUTPUT.PUT_LINE('⚠️  Normal drop failed: ' || SQLERRM);
        END;
        
        -- Drop queue table (this will remove all AQ$ objects)
        IF v_queue_table IS NOT NULL THEN
            BEGIN
                DBMS_AQADM.DROP_QUEUE_TABLE(queue_table => v_queue_table, force => TRUE);
                DBMS_OUTPUT.PUT_LINE('✓ Queue table dropped: ' || v_queue_table);
            EXCEPTION
                WHEN OTHERS THEN
                    IF SQLCODE = -24002 THEN
                        DBMS_OUTPUT.PUT_LINE('⚠️  Queue table does not exist');
                    ELSE
                        DBMS_OUTPUT.PUT_LINE('⚠️  Error dropping queue table: ' || SQLERRM);
                    END IF;
            END;
        END IF;
    END drop_queue_safe;
    
BEGIN
    DBMS_OUTPUT.PUT_LINE('======================================');
    DBMS_OUTPUT.PUT_LINE('OMNI TRACER CLEANUP');
    DBMS_OUTPUT.PUT_LINE('Target Schema: ' || v_target_schema);
    DBMS_OUTPUT.PUT_LINE('======================================');
    
    v_full_q_name := v_target_schema || '.' || v_q_name;
    
    -- 1. Remove subscribers BEFORE dropping queue
    BEGIN
        FOR subscriber_rec IN (
            SELECT consumer_name
            FROM dba_queue_subscribers
            WHERE owner = v_target_schema
            AND queue_name = v_q_name
        ) LOOP
            BEGIN
                DBMS_AQADM.REMOVE_SUBSCRIBER(
                    queue_name => v_full_q_name,
                    subscriber => SYS.AQ$_AGENT(subscriber_rec.consumer_name, NULL, NULL)
                );
                DBMS_OUTPUT.PUT_LINE('✓ Subscriber removed: ' || subscriber_rec.consumer_name);
            EXCEPTION
                WHEN OTHERS THEN
                    DBMS_OUTPUT.PUT_LINE('⚠️  Could not remove subscriber: ' || subscriber_rec.consumer_name || ' - ' || SQLERRM);
            END;
        END LOOP;
    EXCEPTION
        WHEN OTHERS THEN
            DBMS_OUTPUT.PUT_LINE('⚠️  Error checking subscribers: ' || SQLERRM);
    END;
    
    -- 2. Drop Queue
    drop_queue_safe(v_target_schema, v_q_name);
    
    -- 3. Drop Package Body
    BEGIN
        EXECUTE IMMEDIATE 'DROP PACKAGE BODY ' || v_target_schema || '.OMNI_TRACER_API';
        DBMS_OUTPUT.PUT_LINE('✓ Package body dropped');
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -4043 THEN
                DBMS_OUTPUT.PUT_LINE('⚠️  Package body does not exist');
            ELSE
                DBMS_OUTPUT.PUT_LINE('❌ Error: ' || SQLERRM);
                RAISE;
            END IF;
    END;
    
    -- 4. Drop Package Specification
    BEGIN
        EXECUTE IMMEDIATE 'DROP PACKAGE ' || v_target_schema || '.OMNI_TRACER_API';
        DBMS_OUTPUT.PUT_LINE('✓ Package specification dropped');
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -4043 THEN
                DBMS_OUTPUT.PUT_LINE('⚠️  Package specification does not exist');
            ELSE
                DBMS_OUTPUT.PUT_LINE('❌ Error: ' || SQLERRM);
                RAISE;
            END IF;
    END;
    
    -- 5. Drop Sequence
    BEGIN
        EXECUTE IMMEDIATE 'DROP SEQUENCE ' || v_target_schema || '.OMNI_TRACER_ID_SEQ';
        DBMS_OUTPUT.PUT_LINE('✓ Sequence dropped');
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -2289 THEN
                DBMS_OUTPUT.PUT_LINE('⚠️  Sequence does not exist');
            ELSE
                DBMS_OUTPUT.PUT_LINE('❌ Error: ' || SQLERRM);
                RAISE;
            END IF;
    END;
    
    COMMIT;
    
    DBMS_OUTPUT.PUT_LINE('======================================');
    DBMS_OUTPUT.PUT_LINE('✅ CLEANUP COMPLETE');
    DBMS_OUTPUT.PUT_LINE('   You can now run Omni_Tracer.sql');
    DBMS_OUTPUT.PUT_LINE('======================================');
    
END;
/

-- Verify cleanup in target schema
SELECT 'Schema: IFSAPP' AS info FROM dual;

SELECT 'QUEUES' AS object_type, owner, name 
FROM dba_queues 
WHERE owner = 'IFSAPP' AND name LIKE 'OMNI%'
UNION ALL
SELECT 'PACKAGES', owner, object_name 
FROM dba_objects 
WHERE owner = 'IFSAPP' AND object_name LIKE 'OMNI%' AND object_type = 'PACKAGE'
UNION ALL
SELECT 'SEQUENCES', sequence_owner, sequence_name 
FROM dba_sequences 
WHERE sequence_owner = 'IFSAPP' AND sequence_name LIKE 'OMNI%';
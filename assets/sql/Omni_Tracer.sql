/*
OMNI TRACER API
This Package and its contents provides functionality for tracing OMNI jobs and processes within the database.
Do not modify this file directly unless you are certain of the implications.

Copyright (c) 2025.
*/

-- @SECTION: SEQUENCE_CREATION

DECLARE
    v_count NUMBER;
BEGIN
    -- Check if sequence exists
    SELECT COUNT(*)
    INTO v_count
    FROM user_sequences
    WHERE sequence_name = 'OMNI_TRACER_ID_SEQ';
    
    -- Create only if it doesn't exist
    IF v_count = 0 THEN
        EXECUTE IMMEDIATE 'CREATE SEQUENCE OMNI_TRACER_ID_SEQ START WITH 1 INCREMENT BY 1 NOCACHE';
    END IF;
END;
/

-- @END_SECTION: SEQUENCE_CREATION

-- @SECTION: PACKAGE_SPECIFICATION

CREATE OR REPLACE PACKAGE OMNI_TRACER_API AS 
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'OMNI_TRACER_QUEUE';

    -- Collection types for bulk operations
    TYPE clob_tab IS TABLE OF CLOB INDEX BY PLS_INTEGER;
    TYPE raw_tab IS TABLE OF RAW(16) INDEX BY PLS_INTEGER;

    -- Core Methods
    PROCEDURE Initialize;
    PROCEDURE Trace_Message(message_ IN VARCHAR2, log_level_ IN VARCHAR2 DEFAULT 'INFO');
 
    -- Subscriber Management
    PROCEDURE Register_Subscriber(subscriber_name_ IN VARCHAR2);
    --PROCEDURE Unregister_Subscriber(subscriber_name_ IN VARCHAR2);

    -- Enqueue/Dequeue Methods
    -- High Performance bulk Array Dequeue
    PROCEDURE Dequeue_Array_Events (
        subscriber_name_ IN VARCHAR2,
        batch_size_      IN INTEGER,
        wait_time_      IN NUMBER DEFAULT DBMS_AQ.NO_WAIT,
        messages_       OUT clob_tab,
        message_ids_    OUT raw_tab,
        msg_count_      OUT INTEGER);
END OMNI_TRACER_API;
/

-- @END_SECTION: PACKAGE_SPECIFICATION

-- @SECTION: PACKAGE_BODY

CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API AS

    PROCEDURE Initialize IS
        PRAGMA AUTONOMOUS_TRANSACTION;
        queue_exists_ NUMBER;
    BEGIN
        -- Check if queue exists
        SELECT COUNT(*)
        INTO queue_exists_
        FROM user_queues
        WHERE name = TRACER_QUEUE_NAME;
        
        IF queue_exists_ = 0 THEN
            DBMS_AQADM.CREATE_TRANSACTIONAL_EVENT_QUEUE (
                queue_name => TRACER_QUEUE_NAME,
                multiple_consumers => TRUE -- setting this value true here, cause it allows named subscribers
            );
            DBMS_AQADM.START_QUEUE (
                queue_name => TRACER_QUEUE_NAME
            );
            COMMIT;

        END IF;  
    EXCEPTION
    WHEN OTHERS THEN
        IF SQLCODE = -24001 THEN                
        -- Queue already exists (race condition), ignore
            NULL;
        ELSE
            RAISE;
        END IF;
    END Initialize;


    PROCEDURE Register_Subscriber(subscriber_name_ IN VARCHAR2) 
    IS
        PRAGMA AUTONOMOUS_TRANSACTION;
        sub_ SYS.AQ$_AGENT;
    BEGIN
        IF subscriber_name_ IS NULL OR LENGTH(subscriber_name_) = 0 THEN
            RAISE_APPLICATION_ERROR(-20001, 'Subscriber name cannot be NULL or empty');
        END IF;

        sub_ := SYS.AQ$_AGENT(subscriber_name_, NULL, NULL);
        DBMS_AQADM.ADD_SUBSCRIBER (
            queue_name      => TRACER_QUEUE_NAME,
            subscriber      => sub_
        );
        COMMIT;
    EXCEPTION
    WHEN OTHERS THEN
        IF SQLCODE = -24034 THEN -- Subscriber already exists
            NULL;
        ELSE
            RAISE;
        END IF;
    END Register_Subscriber;


    PROCEDURE Enqueue_Event___ (
        process_name_   IN VARCHAR2,
        log_level_      IN VARCHAR2,
        payload         IN CLOB )
    IS
        message_            JSON_OBJECT_T;
        enqueue_options_    DBMS_AQ.ENQUEUE_OPTIONS_T;
        message_properties_ DBMS_AQ.MESSAGE_PROPERTIES_T;
        message_handle_     RAW(16);
        json_payload_       CLOB;
        jms_message_        SYS.AQ$_JMS_TEXT_MESSAGE;
    BEGIN
        enqueue_options_.visibility := DBMS_AQ.IMMEDIATE; -- Message is visible immediately, impervious to rollbacks, and runs an internal commit.

        message_ := JSON_OBJECT_T();
        message_.PUT('MESSAGE_ID', TO_CHAR(OMNI_tracer_id_seq.NEXTVAL));
        message_.PUT('PROCESS_NAME', process_name_);
        message_.PUT('LOG_LEVEL', log_level_);
        message_.PUT('PAYLOAD', payload);
        message_.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));

        json_payload_ := message_.TO_CLOB();
        jms_message_ := SYS.AQ$_JMS_TEXT_MESSAGE.CONSTRUCT();
        jms_message_.set_text(json_payload_);

        DBMS_AQ.ENQUEUE (
            queue_name          => TRACER_QUEUE_NAME,
            enqueue_options     => enqueue_options_,
            message_properties  => message_properties_,
            payload             => jms_message_,
            msgid               => message_handle_
        );
    END Enqueue_Event___;


    PROCEDURE Dequeue_Array_Events (
        subscriber_name_ IN VARCHAR2,
        batch_size_      IN INTEGER,
        wait_time_       IN NUMBER DEFAULT DBMS_AQ.NO_WAIT,
        messages_        OUT clob_tab,
        message_ids_     OUT raw_tab,
        msg_count_       OUT INTEGER)
    IS
        dequeue_options_     DBMS_AQ.DEQUEUE_OPTIONS_T;
        message_props_array_ DBMS_AQ.MESSAGE_PROPERTIES_ARRAY_T;
        payload_array_       SYS.AQ$_JMS_TEXT_MESSAGES;
        msg_id_array_        DBMS_AQ.MSGID_ARRAY_T;
        count_               NUMBER;
        temp_clob_           CLOB;
    BEGIN
        IF batch_size_ IS NULL OR batch_size_ <= 0 THEN
            RAISE_APPLICATION_ERROR(-20003, 'Batch size must be a positive integer');
        END IF;

        -- Async Listening
        dequeue_options_.consumer_name := subscriber_name_;
        dequeue_options_.wait := wait_time_;
        dequeue_options_.navigation := DBMS_AQ.FIRST_MESSAGE;
        dequeue_options_.visibility := DBMS_AQ.IMMEDIATE;

        -- Initialize output collections
        payload_array_ := SYS.AQ$_JMS_TEXT_MESSAGES();

        count_ := DBMS_AQ.DEQUEUE_ARRAY (
            queue_name                => TRACER_QUEUE_NAME,
            dequeue_options           => dequeue_options_,
            array_size                => batch_size_,
            message_properties_array  => message_props_array_,
            payload_array             => payload_array_,
            msgid_array               => msg_id_array_
        );

        msg_count_ := count_;

        FOR i_ IN 1 .. count_ LOOP
            payload_array_(i_).get_text(temp_clob_);
            messages_(i_) := temp_clob_;
            message_ids_(i_) := msg_id_array_(i_);
        END LOOP;
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -25228 THEN
                -- No message available
                msg_count_ := 0;
            ELSE
                RAISE;
            END IF;
    END Dequeue_Array_Events;

    PROCEDURE Trace_Message (
        message_    IN VARCHAR2,
        log_level_  IN VARCHAR2 DEFAULT 'INFO') 
    IS
        calling_process_ VARCHAR2(100);
    BEGIN
        calling_process_ := SUBSTR(DBMS_UTILITY.FORMAT_CALL_STACK, 1, 100);
        Enqueue_Event___(
            process_name_   => calling_process_,
            log_level_      => log_level_,
            payload         => message_
        );
    END Trace_Message;

END OMNI_TRACER_API;
/

-- @END_SECTION: PACKAGE_BODY
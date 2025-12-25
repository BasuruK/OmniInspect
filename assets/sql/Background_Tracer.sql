/*
Background Tracer API
This Package and its contents provides functionality for tracing background jobs and processes within the database.
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
    WHERE sequence_name = 'BACKGROUND_TRACER_ID_SEQ';
    
    -- Create only if it doesn't exist
    IF v_count = 0 THEN
        EXECUTE IMMEDIATE 'CREATE SEQUENCE background_tracer_id_seq START WITH 1 INCREMENT BY 1 NOCACHE';
    END IF;
END;
/

-- @END_SECTION: SEQUENCE_CREATION

-- @SECTION: PACKAGE_SPECIFICATION

CREATE OR REPLACE PACKAGE BACKGROUND_TRACER_API AS 
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'BACKGROUND_TRACER_QUEUE';

    PROCEDURE Initialize;
    PROCEDURE Trace_Message(message_ IN VARCHAR2);
    FUNCTION Fetch_Next_Trace_Message RETURN CLOB;
END BACKGROUND_TRACER_API;
/

-- @END_SECTION: PACKAGE_SPECIFICATION

-- @SECTION: PACKAGE_BODY

CREATE OR REPLACE PACKAGE BODY BACKGROUND_TRACER_API AS
    PROCEDURE Initialize IS
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
                multiple_consumers => FALSE
            );
            DBMS_AQADM.START_QUEUE (
                queue_name => TRACER_QUEUE_NAME
            );
        END IF;
        
        COMMIT;
    EXCEPTION
        WHEN OTHERS THEN
            DBMS_OUTPUT.PUT_LINE('FATAL ERROR in Initialize: ' || SQLERRM);
            RAISE;
    END Initialize;

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
        message_.PUT('MESSAGE_ID', TO_CHAR(background_tracer_id_seq.NEXTVAL));
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

    PROCEDURE Dequeue_Event___ (
        wait_time_  IN NUMBER DEFAULT DBMS_AQ.FOREVER,
        message_    OUT CLOB,
        message_id_ OUT RAW )
    IS
        dequeue_options_    DBMS_AQ.DEQUEUE_OPTIONS_T;
        message_properties_ DBMS_AQ.MESSAGE_PROPERTIES_T;
        payload_            SYS.AQ$_JMS_TEXT_MESSAGE;
    BEGIN
        -- Async Listening
        dequeue_options_.wait := wait_time_;
        dequeue_options_.navigation := DBMS_AQ.FIRST_MESSAGE;
        dequeue_options_.visibility := DBMS_AQ.IMMEDIATE;

        DBMS_AQ.DEQUEUE (
            queue_name          => TRACER_QUEUE_NAME,
            dequeue_options     => dequeue_options_,
            message_properties  => message_properties_,
            payload             => payload_,
            msgid               => message_id_
        );

        payload_.get_text(message_);
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -25228 THEN
                -- No message available
                message_ := NULL;
                message_id_ := NULL;
            ELSE
                RAISE;
            END IF;
    END Dequeue_Event___;

    PROCEDURE Trace_Message (
        message_ IN VARCHAR2 ) 
    IS
        calling_process_ VARCHAR2(100);
    BEGIN
        calling_process_ := SUBSTR(DBMS_UTILITY.FORMAT_CALL_STACK, 1, 100);

        Enqueue_Event___(
            process_name_   => calling_process_,
            log_level_      => 'INFO',
            payload         => message_
        );
    END Trace_Message;

    FUNCTION Fetch_Next_Trace_Message RETURN CLOB 
    IS
        message_    CLOB;
        message_id_ RAW(16);
    BEGIN
        Dequeue_Event___(
            wait_time_  => 100,
            message_    => message_,
            message_id_ => message_id_
        );
        RETURN message_;
    END Fetch_Next_Trace_Message;

END BACKGROUND_TRACER_API;
/

-- @END_SECTION: PACKAGE_BODY
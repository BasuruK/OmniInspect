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

-- @SECTION: TYPE_CREATION

DECLARE
    is_editioned_ VARCHAR2(5);
    type_in_use EXCEPTION;
    PRAGMA EXCEPTION_INIT(type_in_use, -2303);
BEGIN
    -- Check if the DB is running in editioned mode
    BEGIN
        SELECT CASE 
         WHEN EXISTS (SELECT 1 FROM user_editioned_types) THEN 'TRUE' 
         ELSE 'FALSE' 
       END
    INTO is_editioned_
    FROM dual;
    EXCEPTION
        WHEN OTHERS THEN
            is_editioned_ := 'FALSE';
    END;

    IF is_editioned_ = 'TRUE' THEN
        -- Try to create/replace with NONEDITIONABLE
        -- This ensures types are correct for AQ Sharded Queue (which requires non-editioned types)
        -- Editioning is not needed for these types as they are only used internally by the queue, and these packages will not be shipped across editions.
        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE NONEDITIONABLE TYPE OMNI_TRACER_PAYLOAD_TYPE AS OBJECT (JSON_DATA BLOB)';
        EXCEPTION
            WHEN type_in_use THEN NULL; -- Ignore if type has dependents (e.g. Queue exists)
        END;

        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE NONEDITIONABLE TYPE OMNI_TRACER_PAYLOAD_ARRAY AS TABLE OF OMNI_TRACER_PAYLOAD_TYPE';
        EXCEPTION
            WHEN type_in_use THEN NULL;
        END;

        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE NONEDITIONABLE TYPE OMNI_TRACER_RAW_ARRAY AS TABLE OF RAW(16)';
        EXCEPTION
            WHEN type_in_use THEN NULL;
        END;
    ELSE
        -- Non-editioned DB, create normally
         -- Non-editioned database - use regular types
        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE TYPE OMNI_TRACER_PAYLOAD_TYPE AS OBJECT (JSON_DATA BLOB)';
        EXCEPTION
            WHEN type_in_use THEN NULL;
        END;

        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE TYPE OMNI_TRACER_PAYLOAD_ARRAY AS TABLE OF OMNI_TRACER_PAYLOAD_TYPE';
        EXCEPTION
            WHEN type_in_use THEN NULL;
        END;

        BEGIN
            EXECUTE IMMEDIATE 'CREATE OR REPLACE TYPE OMNI_TRACER_RAW_ARRAY AS TABLE OF RAW(16)';
        EXCEPTION
            WHEN type_in_use THEN NULL;
        END;
    END IF;
END;
/

-- @END_SECTION: TYPE_CREATION

-- @SECTION: PACKAGE_SPECIFICATION

CREATE OR REPLACE PACKAGE OMNI_TRACER_API AS 
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'OMNI_TRACER_QUEUE';

    -- Core Methods
    PROCEDURE Initialize;
    PROCEDURE Trace_Message(message_ IN CLOB, log_level_ IN VARCHAR2 DEFAULT 'INFO');
    PROCEDURE Trace_Message_To_Webhook(message_ IN CLOB, log_level_ IN VARCHAR2 DEFAULT 'INFO');
    PROCEDURE Dequeue_Array_Events(
        subscriber_name_ IN  VARCHAR2,
        batch_size_      IN  INTEGER,
        wait_time_       IN  NUMBER DEFAULT DBMS_AQ.NO_WAIT,
        messages_        OUT OMNI_TRACER_PAYLOAD_ARRAY,
        message_ids_     OUT OMNI_TRACER_RAW_ARRAY,
        msg_count_       OUT INTEGER
    );
    
    -- Subscriber Management
    PROCEDURE Register_Subscriber(subscriber_name_ IN VARCHAR2);
    --PROCEDURE Unregister_Subscriber(subscriber_name_ IN VARCHAR2);

END OMNI_TRACER_API;
/

-- @END_SECTION: PACKAGE_SPECIFICATION

-- @SECTION: PACKAGE_BODY

CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API AS

    -- Internal type for RAW array fetching
    TYPE raw_payload_tab IS TABLE OF RAW(32767) INDEX BY PLS_INTEGER;

    -- Forward declarations for private functions
    FUNCTION Clob_To_Blob___(input_ IN CLOB) RETURN BLOB;

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
            -- 1. Create the Sharded Queue
            DBMS_AQADM.CREATE_SHARDED_QUEUE (
                queue_name => TRACER_QUEUE_NAME,
                multiple_consumers => TRUE, -- setting this value true here, cause it allows named subscribers
                queue_payload_type => 'OMNI_TRACER_PAYLOAD_TYPE'
            );
            -- 2. Set Shard count to 4. This can be adjusted based on expected load.
            DBMS_AQADM.SET_QUEUE_PARAMETER(
                queue_name => TRACER_QUEUE_NAME,
                param_name => 'SHARD_NUM',
                param_value => 4
            );

            -- 3. Start the Queue with explicit enqueue/dequeue enabled
            DBMS_AQADM.START_QUEUE (
                queue_name => TRACER_QUEUE_NAME,
                enqueue    => TRUE,
                dequeue    => TRUE
            );
        END IF;
        
        COMMIT;
    EXCEPTION
    WHEN OTHERS THEN
        IF SQLCODE = -24001 THEN                
            -- Queue already exists (race condition), ignore
            COMMIT; -- Must commit autonomous transaction even on expected exception
        ELSE
            ROLLBACK;
            RAISE;
        END IF;
    END Initialize;


    PROCEDURE Register_Subscriber(subscriber_name_ IN VARCHAR2) 
    IS
        PRAGMA AUTONOMOUS_TRANSACTION;
        sub_ SYS.AQ$_AGENT;
    BEGIN
        IF subscriber_name_ IS NULL THEN
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
            COMMIT; -- Must commit autonomous transaction even on expected exception
        ELSE
            ROLLBACK;
            RAISE;
        END IF;
    END Register_Subscriber;


    PROCEDURE Enqueue_Event___ (
        process_name_       IN VARCHAR2,
        log_level_          IN VARCHAR2,
        payload             IN CLOB,
        additional_props_   IN CLOB DEFAULT NULL )
    IS
        message_            JSON_OBJECT_T;
        additional_props_obj_ JSON_OBJECT_T;
        additional_prop_keys_ JSON_KEY_LIST;
        enqueue_options_    DBMS_AQ.ENQUEUE_OPTIONS_T;
        message_properties_ DBMS_AQ.MESSAGE_PROPERTIES_T;
        message_handle_     RAW(16);
        json_payload_       CLOB;
        temp_blob_          BLOB;
        payload_object_     OMNI_TRACER_PAYLOAD_TYPE;
    BEGIN
        enqueue_options_.visibility := DBMS_AQ.IMMEDIATE; -- Message visible immediately without waiting for commit

        message_ := JSON_OBJECT_T();
        message_.PUT('MESSAGE_ID', TO_CHAR(OMNI_tracer_id_seq.NEXTVAL));
        message_.PUT('PROCESS_NAME', process_name_);
        message_.PUT('LOG_LEVEL', log_level_);
        message_.PUT('PAYLOAD', payload);
        message_.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));

        -- Merge additional properties if provided (for extensibility)
        IF additional_props_ IS NOT NULL AND DBMS_LOB.GETLENGTH(additional_props_) > 0 THEN
            additional_props_obj_ := JSON_OBJECT_T.parse(additional_props_);
            additional_prop_keys_ := additional_props_obj_.get_keys;

            IF additional_prop_keys_.COUNT = 2
               AND additional_props_obj_.has('key')
               AND additional_props_obj_.has('value') THEN
                message_.PUT(
                    additional_props_obj_.get_string('key'),
                    additional_props_obj_.get('value')
                );
            ELSE
                FOR i_ IN 1 .. additional_prop_keys_.COUNT LOOP
                    message_.PUT(
                        additional_prop_keys_(i_),
                        additional_props_obj_.get(additional_prop_keys_(i_))
                    );
                END LOOP;
            END IF;
        END IF;

        json_payload_ := message_.TO_CLOB();
        temp_blob_ := Clob_To_Blob___(json_payload_);
        payload_object_ := OMNI_TRACER_PAYLOAD_TYPE(temp_blob_);

        DBMS_AQ.ENQUEUE (
            queue_name          => TRACER_QUEUE_NAME,
            enqueue_options     => enqueue_options_,
            message_properties  => message_properties_,
            payload             => payload_object_,
            msgid               => message_handle_
        );

        IF temp_blob_ IS NOT NULL AND DBMS_LOB.ISTEMPORARY(temp_blob_) = 1 THEN
            DBMS_LOB.FREETEMPORARY(temp_blob_);
        END IF;
        
        IF json_payload_ IS NOT NULL AND DBMS_LOB.ISTEMPORARY(json_payload_) = 1 THEN
            DBMS_LOB.FREETEMPORARY(json_payload_);
        END IF;
    EXCEPTION
        WHEN OTHERS THEN
            IF temp_blob_ IS NOT NULL AND DBMS_LOB.ISTEMPORARY(temp_blob_) = 1 THEN
                DBMS_LOB.FREETEMPORARY(temp_blob_);
            END IF;

            IF json_payload_ IS NOT NULL AND DBMS_LOB.ISTEMPORARY(json_payload_) = 1 THEN
                DBMS_LOB.FREETEMPORARY(json_payload_);
            END IF;

            RAISE;
    END Enqueue_Event___;
    

    PROCEDURE Trace_Message (
        message_    IN CLOB,
        log_level_  IN VARCHAR2 DEFAULT 'INFO')
    IS
        calling_process_ VARCHAR2(100);
    BEGIN
        calling_process_ := 'OMNI_TRACER_API';
        Enqueue_Event___(
            process_name_       => calling_process_,
            log_level_          => log_level_,
            payload             => message_,
            additional_props_   => NULL
        );
    END Trace_Message;


    -- @DOC: Trace_Message_To_Webhook
    -- Traces a message and signals the Go client to forward it to the configured webhook URL.
    -- The webhook URL is stored in BoltDB on the Go client side.
    PROCEDURE Trace_Message_To_Webhook (
        message_    IN CLOB,
        log_level_  IN VARCHAR2 DEFAULT 'INFO')
    IS
        calling_process_ VARCHAR2(100);
    BEGIN
        calling_process_ := 'OMNI_TRACER_API';
        Enqueue_Event___(
            process_name_       => calling_process_,
            log_level_          => log_level_,
            payload             => message_,
            additional_props_   => '{"SEND_TO_WEBHOOK":"TRUE"}'
        );
    END Trace_Message_To_Webhook;


    PROCEDURE Dequeue_Array_Events(
        subscriber_name_ IN  VARCHAR2,
        batch_size_      IN  INTEGER,
        wait_time_       IN  NUMBER DEFAULT DBMS_AQ.NO_WAIT,
        messages_        OUT OMNI_TRACER_PAYLOAD_ARRAY,
        message_ids_     OUT OMNI_TRACER_RAW_ARRAY,
        msg_count_       OUT INTEGER)
    IS
        dequeue_options_     DBMS_AQ.DEQUEUE_OPTIONS_T;
        message_props_array_ DBMS_AQ.MESSAGE_PROPERTIES_ARRAY_T;
        payload_array_       OMNI_TRACER_PAYLOAD_ARRAY;
        msg_id_array_        DBMS_AQ.MSGID_ARRAY_T;
    BEGIN
        messages_ := OMNI_TRACER_PAYLOAD_ARRAY();
        message_ids_ := OMNI_TRACER_RAW_ARRAY();
        msg_count_ := 0;

        dequeue_options_.consumer_name := subscriber_name_;
        dequeue_options_.wait          := wait_time_;
        dequeue_options_.navigation    := DBMS_AQ.FIRST_MESSAGE;
        dequeue_options_.visibility    := DBMS_AQ.IMMEDIATE;

        msg_count_ := DBMS_AQ.DEQUEUE_ARRAY(
            queue_name                => TRACER_QUEUE_NAME,
            dequeue_options           => dequeue_options_,
            array_size                => batch_size_,
            message_properties_array  => message_props_array_,
            payload_array             => payload_array_,
            msgid_array               => msg_id_array_
        );
        
        messages_ := payload_array_;

        FOR i_ IN 1 .. msg_count_ LOOP
            message_ids_.EXTEND;
            message_ids_(i_) := msg_id_array_(i_);
        END LOOP;
    EXCEPTION
        WHEN OTHERS THEN
            IF SQLCODE = -25228 THEN
                -- No messages available
                messages_ := OMNI_TRACER_PAYLOAD_ARRAY();
                message_ids_ := OMNI_TRACER_RAW_ARRAY();
                msg_count_ := 0;
            ELSE
                RAISE;
            END IF;
    END Dequeue_Array_Events;


    FUNCTION Clob_To_Blob___(input_ IN CLOB) RETURN BLOB
    IS
        output_         BLOB;
        dest_offset_    INTEGER := 1;
        src_offset_     INTEGER := 1;
        lang_context_   INTEGER := DBMS_LOB.DEFAULT_LANG_CTX;
        warning_        INTEGER;
    BEGIN
        IF input_ IS NULL OR DBMS_LOB.GETLENGTH(input_) = 0 THEN
            RETURN NULL;
        END IF;
        
        DBMS_LOB.CREATETEMPORARY(output_, TRUE);
        DBMS_LOB.CONVERTTOBLOB(
            dest_lob     => output_,
            src_clob     => input_,
            amount       => DBMS_LOB.LOBMAXSIZE,
            dest_offset  => dest_offset_,
            src_offset   => src_offset_,
            blob_csid    => DBMS_LOB.DEFAULT_CSID,
            lang_context => lang_context_,
            warning      => warning_
        );
        RETURN output_;
    END Clob_To_Blob___;


END OMNI_TRACER_API;
/

-- @END_SECTION: PACKAGE_BODY
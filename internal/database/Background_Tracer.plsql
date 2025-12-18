/*
Background Tracer API
This Package and its contents provides functionality for tracing background jobs and processes within the database.
Do not modify this file directly unless you are certain of the implications.

Copyright (c) 2025.
*/
-- Date --|--- Author ---|------------------------- Description ------------------------
-- 251218 |    Basblk    | Initial creation of Background Tracer
---------------------------------------------------------------------------------------

-- Package Specification
CREATE OR REPLACE PACKAGE BACKGROUND_TRACER_API AS 
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'BACKGROUND_TRACER_QUEUE';

    PROCEDURE Initialize;
END BACKGROUND_TRACER_API;
/

-- Package Body
CREATE OR REPLACE PACKAGE BODY BACKGROUND_TRACER_API AS
    PROCEDURE Initialize IS
        v_queue_exists NUMBER;
    BEGIN
        BEGIN
            -- Create sequence
            EXECUTE IMMEDIATE 'CREATE SEQUENCE background_tracer_id_seq START WITH 1 INCREMENT BY 1 NOCACHE';
        EXCEPTION
            WHEN OTHERS THEN
                IF SQLCODE != -955 THEN
                    RAISE;
                END IF;
        END;

        SELECT COUNT(*)
        INTO v_queue_exists
        FROM user_queues
        WHERE name = TRACER_QUEUE_NAME;
        
        IF v_queue_exists = 0 THEN
            DBMS_AQADM.CREATE_TRANSACTIONAL_EVENT_QUEUE (
                queue_name => TRACER_QUEUE_NAME,
                multiple_consumers => FALSE
            );

            DBMS_AQADM.START_QUEUE (
                queue_name => TRACER_QUEUE_NAME
            );
        END IF;
    END Initialize;
END BACKGROUND_TRACER_API;
/
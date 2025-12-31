/*
Transactional Event Queue (TxEventQ) Permission Verification Package
This package provides boolean functions to check if a schema has all necessary 
privileges to implement the Background Tracer API with TxEventQ.

Can be called from:
1. PL/SQL directly
2. Go application via database connection

Copyright (c) 2025.
*/

-- @SECTION: PACKAGE_SPECIFICATION

CREATE OR REPLACE PACKAGE TXEVENTQ_PERMISSION_CHECK_API AS

    -- Master validation function - returns TRUE if all required privileges exist
    FUNCTION Validate_All_Permissions(p_schema IN VARCHAR2) RETURN BOOLEAN;
    
    -- Get detailed report as VARCHAR2 (for Go to parse)
    FUNCTION Get_Permission_Report(p_schema IN VARCHAR2) RETURN VARCHAR2;
    
END TXEVENTQ_PERMISSION_CHECK_API;
/

-- @END_SECTION: PACKAGE_SPECIFICATION

-- @SECTION: PACKAGE_BODY

CREATE OR REPLACE PACKAGE BODY TXEVENTQ_PERMISSION_CHECK_API AS

    FUNCTION Has_Create_Sequence_Priv(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_sys_privs
        WHERE privilege IN ('CREATE SEQUENCE', 'CREATE ANY SEQUENCE');

        RETURN count_ > 0;
    END Has_Create_Sequence_Priv;

    FUNCTION Has_Create_Procedure_Priv(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_sys_privs
        WHERE privilege IN ('CREATE PROCEDURE', 'CREATE ANY PROCEDURE');

        RETURN count_ > 0;
    END Has_Create_Procedure_Priv;

    FUNCTION Has_AQ_Admin_Role(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_role_privs
        WHERE granted_role = 'AQ_ADMINISTRATOR_ROLE';

        RETURN count_ > 0;
    END Has_AQ_Admin_Role;

    FUNCTION Has_AQ_User_Role(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_role_privs
        WHERE granted_role = 'AQ_USER_ROLE';

        RETURN count_ > 0;
    END Has_AQ_User_Role;

    FUNCTION Has_DBMS_AQADM_Exec(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_tab_privs
        WHERE table_name = 'DBMS_AQADM'
          AND privilege = 'EXECUTE';

        RETURN count_ > 0;
    END Has_DBMS_AQADM_Exec;

    FUNCTION Has_DBMS_AQ_Exec(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        count_ NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO count_
        FROM user_tab_privs
        WHERE table_name = 'DBMS_AQ'
          AND privilege = 'EXECUTE';

        RETURN count_ > 0;
    END Has_DBMS_AQ_Exec;

    FUNCTION Validate_All_Permissions(p_schema IN VARCHAR2) RETURN BOOLEAN IS
    BEGIN
        RETURN Has_Create_Sequence_Priv(p_schema)
           AND Has_Create_Procedure_Priv(p_schema)
           AND Has_AQ_Admin_Role(p_schema)
           AND Has_AQ_User_Role(p_schema)
           AND Has_DBMS_AQADM_Exec(p_schema)
           AND Has_DBMS_AQ_Exec(p_schema);
    END Validate_All_Permissions;

    FUNCTION Get_Permission_Report(p_schema IN VARCHAR2) RETURN VARCHAR2 IS
        report_         VARCHAR2(4000);
        create_seq_     BOOLEAN;
        create_proc_    BOOLEAN;
        aq_admin_       BOOLEAN;
        aq_user_        BOOLEAN;
        dbms_aqadm_     BOOLEAN;
        dbms_aq_        BOOLEAN;
        all_valid_      BOOLEAN;
    BEGIN
        -- Call each check once
        create_seq_     := Has_Create_Sequence_Priv(p_schema);
        create_proc_    := Has_Create_Procedure_Priv(p_schema);
        aq_admin_       := Has_AQ_Admin_Role(p_schema);
        aq_user_        := Has_AQ_User_Role(p_schema);
        dbms_aqadm_     := Has_DBMS_AQADM_Exec(p_schema);
        dbms_aq_        := Has_DBMS_AQ_Exec(p_schema);
        all_valid_      := create_seq_ AND create_proc_ AND aq_admin_ AND aq_user_ AND dbms_aqadm_ AND dbms_aq_;

        report_ := '{';
        report_ := report_ || '"Schema":"' || p_schema || '",';

        report_ := report_ || '"CreateSequence":' || 
            CASE WHEN create_seq_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"CreateProcedure":' || 
            CASE WHEN create_proc_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"AQAdministratorRole":' || 
            CASE WHEN aq_admin_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"AQUserRole":' || 
            CASE WHEN aq_user_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"DBMSAQADMExecute":' || 
            CASE WHEN dbms_aqadm_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"DBMSAQExecute":' || 
            CASE WHEN dbms_aq_ THEN 'true' ELSE 'false' END || ',';

        report_ := report_ || '"AllValid":' || 
            CASE WHEN all_valid_ THEN 'true' ELSE 'false' END;

        report_ := report_ || '}';

        RETURN report_;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE;
    END Get_Permission_Report;

END TXEVENTQ_PERMISSION_CHECK_API;
/

-- @END_SECTION: PACKAGE_BODY
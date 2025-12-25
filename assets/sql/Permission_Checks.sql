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
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_sys_privs
        WHERE grantee = UPPER(p_schema)
          AND privilege IN ('CREATE SEQUENCE', 'CREATE ANY SEQUENCE');
        
        RETURN v_count > 0;
    END Has_Create_Sequence_Priv;

    FUNCTION Has_Create_Procedure_Priv(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_sys_privs
        WHERE grantee = UPPER(p_schema)
          AND privilege IN ('CREATE PROCEDURE', 'CREATE ANY PROCEDURE');
        
        RETURN v_count > 0;
    END Has_Create_Procedure_Priv;

    FUNCTION Has_AQ_Admin_Role(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_role_privs
        WHERE grantee = UPPER(p_schema)
          AND granted_role = 'AQ_ADMINISTRATOR_ROLE';
        
        RETURN v_count > 0;
    END Has_AQ_Admin_Role;

    FUNCTION Has_AQ_User_Role(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_role_privs
        WHERE grantee = UPPER(p_schema)
          AND granted_role = 'AQ_USER_ROLE';
        
        RETURN v_count > 0;
    END Has_AQ_User_Role;

    FUNCTION Has_DBMS_AQADM_Exec(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_tab_privs
        WHERE grantee = UPPER(p_schema)
          AND table_name = 'DBMS_AQADM'
          AND privilege = 'EXECUTE';
        
        RETURN v_count > 0;
    END Has_DBMS_AQADM_Exec;

    FUNCTION Has_DBMS_AQ_Exec(p_schema IN VARCHAR2) RETURN BOOLEAN IS
        v_count NUMBER;
    BEGIN
        SELECT COUNT(*)
        INTO v_count
        FROM dba_tab_privs
        WHERE grantee = UPPER(p_schema)
          AND table_name = 'DBMS_AQ'
          AND privilege = 'EXECUTE';
        
        RETURN v_count > 0;
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
        v_report VARCHAR2(4000);
    BEGIN
        v_report := '{';
        v_report := v_report || '"schema":"' || p_schema || '",';
        
        v_report := v_report || '"CREATE_SEQUENCE":' || 
            CASE WHEN Has_Create_Sequence_Priv(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"CREATE_PROCEDURE":' || 
            CASE WHEN Has_Create_Procedure_Priv(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"AQ_ADMINISTRATOR_ROLE":' || 
            CASE WHEN Has_AQ_Admin_Role(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"AQ_USER_ROLE":' || 
            CASE WHEN Has_AQ_User_Role(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"DBMS_AQADM_EXECUTE":' || 
            CASE WHEN Has_DBMS_AQADM_Exec(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"DBMS_AQ_EXECUTE":' || 
            CASE WHEN Has_DBMS_AQ_Exec(p_schema) THEN 'true' ELSE 'false' END || ',';
        
        v_report := v_report || '"ALL_VALID":' || 
            CASE WHEN Validate_All_Permissions(p_schema) THEN 'true' ELSE 'false' END;
        
        v_report := v_report || '}';
        
        RETURN v_report;
    END Get_Permission_Report;

END TXEVENTQ_PERMISSION_CHECK_API;
/

-- @END_SECTION: PACKAGE_BODY
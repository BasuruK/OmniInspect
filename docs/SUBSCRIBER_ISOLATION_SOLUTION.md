# Subscriber Isolation Solution
## Solving the Broadcast Problem in OmniInspect

| Field | Value |
|-------|-------|
| **Document Version** | 2.0 |
| **Created** | March 7, 2026 |
| **Last Updated** | March 7, 2026 |
| **Project** | OmniInspect (OmniView) |
| **Branch** | `bubbleteav2-ui-integration` |
| **Problem** | Messages broadcast to all subscribers instead of being delivered to the specific caller |

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [Scenarios Overview](#2-scenarios-overview)
3. [Solution Options](#3-solution-options)
4. [Testing Each Solution](#4-testing-each-solution)
5. [Implementation Guide](#5-implementation-guide)
6. [PL/SQL Changes](#6-plsql-changes)

---

## 1. Problem Statement

When multiple users connect to the same Oracle database using shared credentials (e.g., `ifsapp/ifsapp`), trace messages are broadcast to ALL subscribers instead of just the caller.

---

## 2. Scenarios Overview

| Scenario | Description | Identifiable By | Works? |
|----------|-------------|-----------------|--------|
| **A: Direct VPN** | User runs Trace_Message from their laptop via VPN | IP_ADDRESS | ✅ |
| **B: App Server** | Application executes Trace_Message (Azure K8s, etc.) | IP_ADDRESS | ❌ |
| **C: Mixed** | Some calls from app, some from direct users | Varies | ⚠️ |

---

## 3. Solution Options

### Solution 1: IP_ADDRESS Filtering (Client-Side)
- **Best for**: Direct VPN connections
- **Changes**: PL/SQL captures IP_ADDRESS, Go filters messages
- **Setup**: Low - just add metadata to messages

### Solution 2: Explicit Subscriber Parameter
- **Best for**: When you control PL/SQL code
- **Changes**: Add parameter to Trace_Message
- **Setup**: Medium - modify PL/SQL calls

### Solution 3: CLIENT_IDENTIFIER Pattern
- **Best for**: Application-centric scenarios
- **Changes**: App sets identifier once at connection
- **Setup**: Medium - requires app changes

### Solution 4: Session Binding Table
- **Best for**: Complex multi-tier architectures
- **Changes**: New table + PL/SQL logic
- **Setup**: Higher - requires table + app changes

### Solution 5: Hybrid (Recommended)
- **Best for**: All scenarios combined
- **Changes**: Combines all above
- **Setup**: Higher but comprehensive

---

## 4. Testing Each Solution

### PREREQUISITE: Setup Two OmniView Instances

Before testing, setup two OmniView instances to see isolation:

```sql
-- In Terminal 1: Start OmniView
-- Note your subscriber name (e.g., SUB_ABC123)
-- Let's say: SUB_6F3E2A1B4C5D6E7F

-- In Terminal 2: Start another OmniView
-- Note your subscriber name (e.g., SUB_DEF456)
-- Let's say: SUB_7G4H3I2J1K0L9M8N
```

Both should receive the same broadcast messages currently.

---

### Solution 1: IP_ADDRESS Filtering Test

**Purpose**: Test if IP_ADDRESS-based filtering works for direct connections.

#### Step 1: Update PL/SQL to Capture IP_ADDRESS

```sql
-- Run this in SQL*Plus or SQLcl as IFSAPP
-- This adds IP_ADDRESS to the message payload

-- First, check current package
SELECT text
FROM user_source
WHERE name = 'OMNI_TRACER_API'
AND type = 'PACKAGE BODY'
ORDER BY line;
```

#### Step 2: Test from Two Different Machines/Locations

```sql
-- From Laptop 1 (your VPN connection)
BEGIN
    OMNI_TRACER_API.Trace_Message('Message from Laptop 1', 'INFO');
END;

-- From Laptop 2 (colleague's connection)
BEGIN
    OMNI_TRACER_API.Trace_Message('Message from Laptop 2', 'INFO');
END;
```

#### Step 3: Check Message Payloads

```sql
-- View messages (you may need to query queue table)
-- The message JSON should contain IP_ADDRESS field
SELECT
    SYS_CONTEXT('USERENV', 'IP_ADDRESS') as my_ip,
    SYS_CONTEXT('USERENV', 'SESSIONID') as my_session
FROM dual;
```

**Expected Result**:
- Each message has different IP_ADDRESS
- Go client filters by matching IP_ADDRESS

---

### Solution 2: Explicit Subscriber Parameter Test

**Purpose**: Test explicit subscriber targeting.

#### Step 1: Modify Trace_Message (Add Parameter)

```sql
-- Create a test version first
CREATE OR REPLACE PROCEDURE OMNI_TRACER_API.Trace_Message_Test(
    message_     IN CLOB,
    log_level_   IN VARCHAR2 DEFAULT 'INFO',
    subscriber_  IN VARCHAR2 DEFAULT NULL
)
IS
    v_consumer_name VARCHAR2(100);
    enqueue_options    DBMS_AQ.ENQUEUE_OPTIONS_T;
    message_properties DBMS_AQ.MESSAGE_PROPERTIES_T;
    message_handle     RAW(16);
    payload_obj        OMNI_TRACER_PAYLOAD_TYPE;
    temp_blob          BLOB;
    json_msg           JSON_OBJECT_T;
BEGIN
    -- Determine consumer name
    v_consumer_name := subscriber_;  -- Use explicit subscriber or NULL for broadcast

    -- Set consumer for targeted delivery
    IF v_consumer_name IS NOT NULL THEN
        message_properties.consumer_name := v_consumer_name;
    END IF;

    -- Build message
    json_msg := JSON_OBJECT_T();
    json_msg.PUT('MESSAGE_ID', TO_CHAR(OMNI_TRACER_ID_SEQ.NEXTVAL));
    json_msg.PUT('PROCESS_NAME', 'TEST_APP');
    json_msg.PUT('LOG_LEVEL', log_level_);
    json_msg.PUT('PAYLOAD', message_);
    json_msg.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));
    json_msg.PUT('TARGET_SUBSCRIBER', CASE WHEN v_consumer_name IS NULL THEN 'BROADCAST' ELSE v_consumer_name END);

    -- Convert to blob
    temp_blob := json_msg.TOBLOB();
    payload_obj := OMNI_TRACER_PAYLOAD_TYPE(temp_blob);

    -- Enqueue
    enqueue_options.visibility := DBMS_AQ.IMMEDIATE;

    DBMS_AQ.ENQUEUE(
        queue_name         => 'OMNI_TRACER_QUEUE',
        enqueue_options    => enqueue_options,
        message_properties => message_properties,
        payload            => payload_obj,
        msgid              => message_handle
    );

    COMMIT;

    DBMS_LOB.FREETEMPORARY(temp_blob);
END Trace_Message_Test;
/
```

#### Step 2: Test Targeted Message

```sql
-- Send to specific subscriber (use actual subscriber name from your OmniView)
BEGIN
    OMNI_TRACER_API.Trace_Message_Test(
        'This should go to SUB_6F3E2A1B... only',
        'INFO',
        'SUB_6F3E2A1B4C5D6E7F'  -- Replace with actual subscriber name
    );
END;
```

#### Step 3: Test Broadcast (No Subscriber)

```sql
-- Send to all (broadcast)
BEGIN
    OMNI_TRACER_API.Trace_Message_Test(
        'This should go to EVERYONE',
        'INFO',
        NULL
    );
END;
```

**Expected Result**:
- With subscriber: Only that OmniView instance receives it
- With NULL: All OmniView instances receive it

---

### Solution 3: CLIENT_IDENTIFIER Test

**Purpose**: Test the CLIENT_IDENTIFIER approach for app servers.

#### Step 1: Set CLIENT_IDENTIFIER in a Session

```sql
-- In your application or manually in SQL*Plus
-- This simulates what the application would do

-- Set a CLIENT_IDENTIFIER (this propagates to all DB sessions from this connection)
DBMS_SESSION.SET_IDENTIFIER('SUB_6F3E2A1B4C5D6E7F');

-- Verify it was set
SELECT SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER') AS client_id FROM dual;
```

#### Step 2: Test Trace_Message with CLIENT_IDENTIFIER

```sql
-- Now call Trace_Message
BEGIN
    OMNI_TRACER_API.Trace_Message('Test with CLIENT_IDENTIFIER', 'INFO');
END;
```

#### Step 3: Check Message

The message JSON should contain:
```json
{
    "CLIENT_IDENTIFIER": "SUB_6F3E2A1B4C5D6E7F",
    ...
}
```

**Expected Result**:
- CLIENT_IDENTIFIER is captured in the message
- Can be used for filtering in Go client

---

### Solution 4: Session Binding Table Test

**Purpose**: Test the session binding approach for complex architectures.

#### Step 1: Create Session Binding Table

```sql
-- Run as IFSAPP
CREATE TABLE omni_session_subscriber (
    session_id       NUMBER PRIMARY KEY,
    subscriber_name  VARCHAR2(100),
    client_ip        VARCHAR2(64),
    client_host      VARCHAR2(256),
    registered_at    TIMESTAMP DEFAULT SYSTIMESTAMP,
    expires_at       TIMESTAMP
);

-- Create index for faster lookups
CREATE INDEX idx_omni_session_expire ON omni_session_subscriber(expires_at);
```

#### Step 2: Create Binding Procedure

```sql
CREATE OR REPLACE PROCEDURE OMNI_TRACER_API.Bind_Session(
    subscriber_name_ IN VARCHAR2,
    expires_in_minutes_ IN NUMBER DEFAULT 60
)
IS
    v_session_id NUMBER;
    PRAGMA AUTONOMOUS_TRANSACTION;
BEGIN
    v_session_id := SYS_CONTEXT('USERENV', 'SESSIONID');

    -- Delete any existing binding for this session
    DELETE FROM omni_session_subscriber WHERE session_id = v_session_id;

    -- Insert new binding
    INSERT INTO omni_session_subscriber (
        session_id,
        subscriber_name,
        client_ip,
        client_host,
        expires_at
    ) VALUES (
        v_session_id,
        subscriber_name_,
        SYS_CONTEXT('USERENV', 'IP_ADDRESS'),
        SYS_CONTEXT('USERENV', 'HOST'),
        SYSTIMESTAMP + NUMTODSINTERVAL(expires_in_minutes_, 'MINUTE')
    );

    COMMIT;
END Bind_Session;
/

-- Create function to get bound subscriber
CREATE OR REPLACE FUNCTION OMNI_TRACER_API.Get_Bound_Subscriber
RETURN VARCHAR2
IS
    v_subscriber VARCHAR2(100);
    v_session_id NUMBER;
BEGIN
    v_session_id := SYS_CONTEXT('USERENV', 'SESSIONID');

    SELECT subscriber_name
    INTO v_subscriber
    FROM omni_session_subscriber
    WHERE session_id = v_session_id
    AND (expires_at IS NULL OR expires_at > SYSTIMESTAMP);

    RETURN v_subscriber;
EXCEPTION
    WHEN NO_DATA_FOUND THEN
        RETURN NULL;
END Get_Bound_Subscriber;
/
```

#### Step 3: Test Binding

```sql
-- User's OmniView calls this (typically at startup)
BEGIN
    OMNI_TRACER_API.Bind_Session('SUB_6F3E2A1B4C5D6E7F');
END;

-- Application can now call Trace_Message without knowing subscriber
-- The procedure will look up the binding
BEGIN
    OMNI_TRACER_API.Trace_Message('Message with binding', 'INFO');
END;
```

**Expected Result**:
- After binding, messages from that session are automatically targeted
- No need to pass subscriber name in Trace_Message

---

### Solution 5: Hybrid Approach Test

**Purpose**: Test the combined approach.

#### Step 1: Create Hybrid Trace_Message

```sql
CREATE OR REPLACE PROCEDURE OMNI_TRACER_API.Trace_Message_Hybrid(
    message_     IN CLOB,
    log_level_   IN VARCHAR2 DEFAULT 'INFO',
    subscriber_  IN VARCHAR2 DEFAULT NULL
)
IS
    v_consumer_name VARCHAR2(100);
    enqueue_options    DBMS_AQ.ENQUEUE_OPTIONS_T;
    message_properties DBMS_AQ.MESSAGE_PROPERTIES_T;
    message_handle     RAW(16);
    payload_obj        OMNI_TRACER_PAYLOAD_TYPE;
    temp_blob          BLOB;
    json_msg           JSON_OBJECT_T;

    -- For session binding lookup
    v_bound_subscriber VARCHAR2(100);
BEGIN
    -- ==========================================
    -- RESOLUTION PRIORITY
    -- ==========================================

    -- Priority 1: Explicit subscriber parameter
    IF subscriber_ IS NOT NULL AND LENGTH(TRIM(subscriber_)) > 0 THEN
        v_consumer_name := subscriber_;
    -- Priority 2: CLIENT_IDENTIFIER (set by app)
    ELSIF SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER') IS NOT NULL
          AND SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER') LIKE 'SUB_%' THEN
        v_consumer_name := SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER');
    -- Priority 3: Session binding table
    ELSE
        BEGIN
            v_bound_subscriber := OMNI_TRACER_API.Get_Bound_Subscriber();
            IF v_bound_subscriber IS NOT NULL THEN
                v_consumer_name := v_bound_subscriber;
            END IF;
        EXCEPTION
            WHEN OTHERS THEN NULL;
        END;
    END IF;

    -- ==========================================
    -- BUILD MESSAGE WITH ALL METADATA
    -- ==========================================

    -- Set consumer for targeted delivery
    IF v_consumer_name IS NOT NULL THEN
        message_properties.consumer_name := v_consumer_name;
    END IF;

    -- Build comprehensive message
    json_msg := JSON_OBJECT_T();
    json_msg.PUT('MESSAGE_ID', TO_CHAR(OMNI_TRACER_ID_SEQ.NEXTVAL));
    json_msg.PUT('PROCESS_NAME', 'HYBRID_TEST');
    json_msg.PUT('LOG_LEVEL', log_level_);
    json_msg.PUT('PAYLOAD', message_);
    json_msg.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));

    -- Add all identification metadata
    json_msg.PUT('SESSION_ID', SYS_CONTEXT('USERENV', 'SESSIONID'));
    json_msg.PUT('IP_ADDRESS', SYS_CONTEXT('USERENV', 'IP_ADDRESS'));
    json_msg.PUT('SESSION_USER', SYS_CONTEXT('USERENV', 'SESSION_USER'));
    json_msg.PUT('HOST', SYS_CONTEXT('USERENV', 'HOST'));
    json_msg.PUT('TERMINAL', SYS_CONTEXT('USERENV', 'TERMINAL'));
    json_msg.PUT('MODULE', SYS_CONTEXT('USERENV', 'MODULE'));
    json_msg.PUT('CLIENT_IDENTIFIER', SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER'));
    json_msg.PUT('OS_USER', SYS_CONTEXT('USERENV', 'OS_USER'));

    -- Add targeting info
    IF v_consumer_name IS NOT NULL THEN
        json_msg.PUT('TARGET_SUBSCRIBER', v_consumer_name);
    ELSE
        json_msg.PUT('TARGET_SUBSCRIBER', 'BROADCAST');
    END IF;

    -- Convert to blob
    temp_blob := json_msg.TOBLOB();
    payload_obj := OMNI_TRACER_PAYLOAD_TYPE(temp_blob);

    -- Enqueue
    enqueue_options.visibility := DBMS_AQ.IMMEDIATE;

    DBMS_AQ.ENQUEUE(
        queue_name         => 'OMNI_TRACER_QUEUE',
        enqueue_options    => enqueue_options,
        message_properties => message_properties,
        payload            => payload_obj,
        msgid              => message_handle
    );

    COMMIT;

    DBMS_LOB.FREETEMPORARY(temp_blob);

EXCEPTION
    WHEN OTHERS THEN
        IF temp_blob IS NOT NULL AND DBMS_LOB.ISTEMPORARY(temp_blob) = 1 THEN
            DBMS_LOB.FREETEMPORARY(temp_blob);
        END IF;
        RAISE;
END Trace_Message_Hybrid;
/
```

#### Step 2: Test Each Scenario

**Scenario A: Explicit Subscriber**
```sql
BEGIN
    OMNI_TRACER_API.Trace_Message_Hybrid(
        'Direct to subscriber',
        'INFO',
        'SUB_6F3E2A1B4C5D6E7F'  -- Explicit
    );
END;
```

**Scenario B: CLIENT_IDENTIFIER**
```sql
-- First set CLIENT_IDENTIFIER
DBMS_SESSION.SET_IDENTIFIER('SUB_6F3E2A1B4C5D6E7F');

-- Then call (no subscriber parameter)
BEGIN
    OMNI_TRACER_API.Trace_Message_Hybrid('Via CLIENT_ID', 'INFO');
END;
```

**Scenario C: Session Binding**
```sql
-- First bind session
BEGIN
    OMNI_TRACER_API.Bind_Session('SUB_6F3E2A1B4C5D6E7F');
END;

-- Then call (no subscriber parameter)
BEGIN
    OMNI_TRACER_API.Trace_Message_Hybrid('Via binding', 'INFO');
END;
```

**Scenario D: Broadcast (nothing set)**
```sql
-- Just call without anything
BEGIN
    OMNI_TRACER_API.Trace_Message_Hybrid('Broadcast to all', 'INFO');
END;
```

---

## 5. Implementation Guide

### Quick Start: Try Solution 2 First (Easiest)

1. **Backup current package**:
```sql
CREATE OR REPLACE PACKAGE OMNI_TRACER_API_BAK AS
    -- Copy current package spec
END;
/

-- Backup body too
CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API_BAK AS
    -- Copy current package body
END;
/
```

2. **Test with simple procedure**:
```sql
-- Use the Trace_Message_Test procedure from Solution 2
-- Test targeted vs broadcast
```

3. **Verify with two OmniView instances**:
```sql
-- Terminal 1: OmniView 1 (SUB_A)
-- Terminal 2: OmniView 2 (SUB_B)

-- Test broadcast (both should see)
BEGIN
    OMNI_TRACER_API.Trace_Message_Test('Everyone sees this', 'INFO', NULL);
END;

-- Test targeted (only one should see)
BEGIN
    OMNI_TRACER_API.Trace_Message_Test('Only SUB_A sees', 'INFO', 'SUB_6F3E2A1B...');
END;
```

### If Solution 2 Works: Proceed to Full Implementation

Update the actual `OMNI_TRACER_API` package with the hybrid approach.

### If Solution 2 Doesn't Work: Try Solution 4 (Session Binding)

The session binding table approach doesn't require setting consumer_name at enqueue time. Instead, it relies on client-side filtering with additional metadata.

---

## 6. PL/SQL Changes

### Full Hybrid Implementation (For Production)

```sql
-- ==========================================
-- UPDATE PACKAGE SPECIFICATION
-- ==========================================

CREATE OR REPLACE PACKAGE OMNI_TRACER_API AS
    TRACER_QUEUE_NAME CONSTANT VARCHAR2(30) := 'OMNI_TRACER_QUEUE';

    -- Core Methods
    PROCEDURE Initialize;

    -- Original (for backward compatibility)
    PROCEDURE Trace_Message(message_ IN CLOB, log_level_ IN VARCHAR2 DEFAULT 'INFO');

    -- New: With explicit subscriber
    PROCEDURE Trace_Message(
        message_     IN CLOB,
        log_level_   IN VARCHAR2 DEFAULT 'INFO',
        subscriber_  IN VARCHAR2 DEFAULT NULL
    );

    -- Hybrid: Auto-resolve subscriber
    PROCEDURE Trace_Message_Hybrid(
        message_     IN CLOB,
        log_level_   IN VARCHAR2 DEFAULT 'INFO',
        subscriber_  IN VARCHAR2 DEFAULT NULL
    );

    -- Session binding for app scenarios
    PROCEDURE Bind_Session(subscriber_name_ IN VARCHAR2, expires_in_minutes_ IN NUMBER DEFAULT 60);
    FUNCTION Get_Bound_Subscriber RETURN VARCHAR2;

    -- Dequeue remains the same
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

END OMNI_TRACER_API;
/

-- ==========================================
-- UPDATE PACKAGE BODY (Key Changes)
-- ==========================================

CREATE OR REPLACE PACKAGE BODY OMNI_TRACER_API AS

    -- Add this function for subscriber resolution
    FUNCTION Resolve_Subscriber_(explicit_subscriber_ IN VARCHAR2)
    RETURN VARCHAR2
    IS
        v_subscriber VARCHAR2(100);
    BEGIN
        -- Priority 1: Explicit parameter
        IF explicit_subscriber_ IS NOT NULL AND LENGTH(TRIM(explicit_subscriber_)) > 0 THEN
            RETURN explicit_subscriber_;
        END IF;

        -- Priority 2: CLIENT_IDENTIFIER
        v_subscriber := SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER');
        IF v_subscriber IS NOT NULL AND v_subscriber LIKE 'SUB_%' THEN
            RETURN v_subscriber;
        END IF;

        -- Priority 3: Session binding
        BEGIN
            v_subscriber := Get_Bound_Subscriber();
            IF v_subscriber IS NOT NULL THEN
                RETURN v_subscriber;
            END IF;
        EXCEPTION
            WHEN OTHERS THEN NULL;
        END;

        -- Fallback: Broadcast (return NULL)
        RETURN NULL;
    END Resolve_Subscriber_;

    -- Modified Enqueue_Event___ to include subscriber resolution
    PROCEDURE Enqueue_Event___ (
        process_name_   IN VARCHAR2,
        log_level_      IN VARCHAR2,
        payload         IN CLOB,
        subscriber_     IN VARCHAR2 DEFAULT NULL )
    IS
        message_            JSON_OBJECT_T;
        enqueue_options_    DBMS_AQ.ENQUEUE_OPTIONS_T;
        message_properties_ DBMS_AQ.MESSAGE_PROPERTIES_T;
        message_handle_     RAW(16);
        json_payload_       CLOB;
        temp_blob_          BLOB;
        payload_object_     OMNI_TRACER_PAYLOAD_TYPE;
        v_consumer_name    VARCHAR2(100);
    BEGIN
        -- Resolve subscriber using priority
        v_consumer_name := Resolve_Subscriber_(subscriber_);

        -- Set consumer for targeted delivery
        IF v_consumer_name IS NOT NULL THEN
            message_properties_.consumer_name := v_consumer_name;
        END IF;

        enqueue_options_.visibility := DBMS_AQ.IMMEDIATE;

        -- Build message with comprehensive metadata
        message_ := JSON_OBJECT_T();
        message_.PUT('MESSAGE_ID', TO_CHAR(OMNI_tracer_id_seq.NEXTVAL));
        message_.PUT('PROCESS_NAME', process_name_);
        message_.PUT('LOG_LEVEL', log_level_);
        message_.PUT('PAYLOAD', payload);
        message_.PUT('TIMESTAMP', TO_CHAR(SYSTIMESTAMP, 'YYYY-MM-DD"T"HH24:MI:SS.FF3TZH:TZM'));

        -- Add all identification metadata
        message_.PUT('SESSION_ID', SYS_CONTEXT('USERENV', 'SESSIONID'));
        message_.PUT('IP_ADDRESS', SYS_CONTEXT('USERENV', 'IP_ADDRESS'));
        message_.PUT('SESSION_USER', SYS_CONTEXT('USERENV', 'SESSION_USER'));
        message_.PUT('HOST', SYS_CONTEXT('USERENV', 'HOST'));
        message_.PUT('TERMINAL', SYS_CONTEXT('USERENV', 'TERMINAL'));
        message_.PUT('MODULE', SYS_CONTEXT('USERENV', 'MODULE'));
        message_.PUT('ACTION', SYS_CONTEXT('USERENV', 'ACTION'));
        message_.PUT('CLIENT_IDENTIFIER', SYS_CONTEXT('USERENV', 'CLIENT_IDENTIFIER'));
        message_.PUT('OS_USER', SYS_CONTEXT('USERENV', 'OS_USER'));

        -- Add targeting info
        IF v_consumer_name IS NOT NULL THEN
            message_.PUT('TARGET_SUBSCRIBER', v_consumer_name);
        ELSE
            message_.PUT('TARGET_SUBSCRIBER', 'BROADCAST');
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

        -- Cleanup
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

    -- Rest of the package remains the same...

END OMNI_TRACER_API;
/
```

---

## Testing Checklist

- [ ] **Setup**: Two OmniView instances running with different subscribers
- [ ] **Test 1**: Broadcast message (all should see)
- [ ] **Test 2**: Targeted message to SUB_A only (SUB_B should not see)
- [ ] **Test 3**: Broadcast after targeted (both should see again)
- [ ] **Test 4**: From application server (different IP)
- [ ] **Test 5**: CLIENT_IDENTIFIER approach
- [ ] **Test 6**: Session binding approach

---

## Notes

1. **Backup First**: Always backup the package before making changes
2. **Test Incrementally**: Try solutions in order (2 → 4 → 5 hybrid)
3. **Verify with Two Clients**: You need two OmniView instances to verify isolation
4. **Check SYS_CONTEXT**: Use `SELECT SYS_CONTEXT('USERENV', 'ATTRIBUTE') FROM dual;` to verify values

---

*Document created for production testing.*

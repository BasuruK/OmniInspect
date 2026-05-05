package subscribers

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	procedureNamePrefix = "TRACE_MESSAGE_"
	packageName         = "OMNI_TRACER_API"
	baseSQLFile         = "Omni_Tracer.sql"
	packageDeployLock   = "OMNI_TRACER_API_DEPLOY_LOCK"
	lockTimeoutSeconds  = 30

	packageSpecStart = "-- @SECTION: PACKAGE_SPECIFICATION"
	packageSpecEnd   = "-- @END_SECTION: PACKAGE_SPECIFICATION"
	packageBodyStart = "-- @SECTION: PACKAGE_BODY"
	packageBodyEnd   = "-- @END_SECTION: PACKAGE_BODY"
)

type ProcedureGenerator struct {
	db ports.DatabaseRepository
}

func NewProcedureGenerator(db ports.DatabaseRepository) (*ProcedureGenerator, error) {
	if db == nil {
		return nil, domain.ErrNilDatabase
	}
	return &ProcedureGenerator{db: db}, nil
}

func (pg *ProcedureGenerator) ReserveFunnyName(ctx context.Context, subscriber *domain.Subscriber) (string, bool, error) {
	if subscriber == nil {
		return "", false, fmt.Errorf("ReserveFunnyName: %w", domain.ErrNilSubscriber)
	}

	if subscriber.FunnyName() != "" {
		if err := validateFunnyNameForProcedure(subscriber.FunnyName()); err != nil {
			return "", false, fmt.Errorf("ReserveFunnyName: %w", err)
		}
		if err := domain.DefaultFunnyNameGenerator().MarkAsUsed(subscriber.FunnyName()); err != nil {
			return "", false, fmt.Errorf("ReserveFunnyName: %w", err)
		}
		return subscriber.FunnyName(), false, nil
	}

	gen := domain.DefaultFunnyNameGenerator()
	attempts := gen.AvailableCount()
	for i := 0; i < attempts; i++ {
		funnyName, err := gen.GetRandomName()
		if err != nil {
			return "", false, fmt.Errorf("ReserveFunnyName: failed to get funny name: %w", err)
		}
		if err := validateFunnyNameForProcedure(funnyName); err != nil {
			_ = gen.MarkAsAvailable(funnyName)
			return "", false, fmt.Errorf("ReserveFunnyName: %w", err)
		}

		exists, err := pg.db.ProcedureExists(ctx, buildProcedureName(funnyName))
		if err != nil {
			_ = gen.MarkAsAvailable(funnyName)
			return "", false, fmt.Errorf("ReserveFunnyName: failed to check procedure existence: %w", err)
		}
		if !exists {
			return funnyName, true, nil
		}
	}

	return "", false, fmt.Errorf("ReserveFunnyName: %w", domain.ErrNoAvailableNames)
}

func (pg *ProcedureGenerator) ReleaseFunnyName(_ context.Context, funnyName string) {
	if funnyName == "" {
		return
	}
	_ = domain.DefaultFunnyNameGenerator().MarkAsAvailable(funnyName)
}

func (pg *ProcedureGenerator) GenerateSubscriberProcedure(ctx context.Context, subscriber *domain.Subscriber) error {
	if subscriber == nil {
		return fmt.Errorf("GenerateSubscriberProcedure: %w", domain.ErrNilSubscriber)
	}

	funnyName := subscriber.FunnyName()
	if err := validateFunnyNameForProcedure(funnyName); err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: %w", err)
	}

	procedureName := buildProcedureName(funnyName)
	exists, err := pg.db.ProcedureExists(ctx, procedureName)
	if err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: failed to check procedure existence: %w", err)
	}
	if exists {
		return nil
	}

	if err := pg.withPackageDeployLock(ctx, func() error {
		packageSpec, packageBody, err := pg.loadPackageSource(ctx)
		if err != nil {
			return fmt.Errorf("failed to load package source: %w", err)
		}

		packageSpec, err = injectProcedureDeclaration(packageSpec, funnyName)
		if err != nil {
			return fmt.Errorf("failed to update package spec: %w", err)
		}
		packageBody, err = injectProcedureBody(packageBody, funnyName)
		if err != nil {
			return fmt.Errorf("failed to update package body: %w", err)
		}

		if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
			return fmt.Errorf("failed to deploy package: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: %w", err)
	}

	return nil
}

func (pg *ProcedureGenerator) DropSubscriberProcedure(ctx context.Context, funnyName string) error {
	if err := validateFunnyNameForProcedure(funnyName); err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}

	procedureName := buildProcedureName(funnyName)
	exists, err := pg.db.ProcedureExists(ctx, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to check procedure existence: %w", err)
	}
	if !exists {
		return nil
	}

	if err := pg.withPackageDeployLock(ctx, func() error {
		packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
		if err != nil {
			return fmt.Errorf("failed to load package source: %w", err)
		}
		packageSpec, packageBody, err = normalizePackageSource(packageSpec, packageBody)
		if err != nil {
			return fmt.Errorf("failed to normalize package source: %w", err)
		}

		packageSpec, err = removeProcedureDeclaration(packageSpec, procedureName)
		if err != nil {
			return fmt.Errorf("failed to update package spec: %w", err)
		}
		packageBody, err = removeProcedureBody(packageBody, procedureName)
		if err != nil {
			return fmt.Errorf("failed to update package body: %w", err)
		}

		if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
			return fmt.Errorf("failed to deploy package: %w", err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}

	pg.ReleaseFunnyName(ctx, funnyName)
	return nil
}

func (pg *ProcedureGenerator) withPackageDeployLock(ctx context.Context, fn func() error) (err error) {
	if err := pg.acquirePackageDeployLock(ctx); err != nil {
		return fmt.Errorf("failed to acquire package deploy lock: %w", err)
	}

	defer func() {
		releaseErr := pg.releasePackageDeployLock(ctx)
		if releaseErr == nil {
			return
		}
		if err != nil {
			err = fmt.Errorf("%w; failed to release package deploy lock: %v", err, releaseErr)
			return
		}
		err = fmt.Errorf("failed to release package deploy lock: %w", releaseErr)
	}()

	err = fn()
	return err
}

func (pg *ProcedureGenerator) acquirePackageDeployLock(ctx context.Context) error {
	return pg.db.ExecuteStatement(ctx, fmt.Sprintf(`DECLARE
	lock_handle_ VARCHAR2(128);
	result_      INTEGER;
BEGIN
	DBMS_LOCK.ALLOCATE_UNIQUE(lockname => '%s', lockhandle => lock_handle_);
	result_ := DBMS_LOCK.REQUEST(
		lockhandle        => lock_handle_,
		lockmode          => DBMS_LOCK.X_MODE,
		timeout           => %d,
		release_on_commit => FALSE
	);
	IF result_ <> 0 THEN
		RAISE_APPLICATION_ERROR(-20001, 'failed to acquire package deploy lock: ' || result_);
	END IF;
END;`, packageDeployLock, lockTimeoutSeconds))
}

func (pg *ProcedureGenerator) releasePackageDeployLock(ctx context.Context) error {
	return pg.db.ExecuteStatement(ctx, fmt.Sprintf(`DECLARE
	lock_handle_ VARCHAR2(128);
	result_      INTEGER;
BEGIN
	DBMS_LOCK.ALLOCATE_UNIQUE(lockname => '%s', lockhandle => lock_handle_);
	result_ := DBMS_LOCK.RELEASE(lockhandle => lock_handle_);
	IF result_ <> 0 THEN
		RAISE_APPLICATION_ERROR(-20002, 'failed to release package deploy lock: ' || result_);
	END IF;
END;`, packageDeployLock))
}

func (pg *ProcedureGenerator) loadPackageSource(ctx context.Context) (string, string, error) {
	packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
	if err == nil {
		return normalizePackageSource(packageSpec, packageBody)
	}
	if errors.Is(err, domain.ErrPackageNotFound) {
		return loadBasePackageSource()
	}

	return "", "", fmt.Errorf("fetchCurrentPackageSource: %w", err)
}

func (pg *ProcedureGenerator) fetchCurrentPackageSource(ctx context.Context) (string, string, error) {
	packageSpecLines, err := pg.db.Fetch(ctx, fmt.Sprintf("SELECT text FROM user_source WHERE name = '%s' AND type = 'PACKAGE' ORDER BY line", packageName))
	if err != nil {
		return "", "", err
	}
	packageBodyLines, err := pg.db.Fetch(ctx, fmt.Sprintf("SELECT text FROM user_source WHERE name = '%s' AND type = 'PACKAGE BODY' ORDER BY line", packageName))
	if err != nil {
		return "", "", err
	}
	if len(packageSpecLines) == 0 || len(packageBodyLines) == 0 {
		return "", "", domain.ErrPackageNotFound
	}

	return strings.Join(packageSpecLines, "\n"), strings.Join(packageBodyLines, "\n"), nil
}

func loadBasePackageSource() (string, string, error) {
	sqlContent, err := assets.GetSQLFile(baseSQLFile)
	if err != nil {
		return "", "", err
	}

	packageSpec, err := extractSQLSection(string(sqlContent), packageSpecStart, packageSpecEnd)
	if err != nil {
		return "", "", err
	}
	packageBody, err := extractSQLSection(string(sqlContent), packageBodyStart, packageBodyEnd)
	if err != nil {
		return "", "", err
	}

	return strings.TrimSpace(packageSpec), strings.TrimSpace(packageBody), nil
}

func extractSQLSection(sqlContent string, startMarker string, endMarker string) (string, error) {
	startIdx := strings.Index(sqlContent, startMarker)
	endIdx := strings.Index(sqlContent, endMarker)
	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		return "", fmt.Errorf("section markers not found")
	}

	section := strings.TrimSpace(sqlContent[startIdx+len(startMarker) : endIdx])
	section = strings.TrimSpace(strings.TrimSuffix(section, "/"))
	if section == "" {
		return "", fmt.Errorf("section is empty")
	}

	return section, nil
}

func renderPackageDeploymentSQL(packageSpec string, packageBody string) string {
	return fmt.Sprintf(`%s

%s
/
%s

%s

%s
/
%s
`, packageSpecStart, strings.TrimSpace(packageSpec), packageSpecEnd, packageBodyStart, strings.TrimSpace(packageBody), packageBodyEnd)
}

func injectProcedureDeclaration(packageSpec string, funnyName string) (string, error) {
	procedureDeclaration := generateProcedureDeclaration(funnyName)
	if strings.Contains(strings.ToUpper(packageSpec), strings.ToUpper(procedureDeclaration)) {
		return packageSpec, nil
	}
	return insertBeforePackageEnd(packageSpec, procedureDeclaration)
}

func injectProcedureBody(packageBody string, funnyName string) (string, error) {
	procedureBody := generateProcedureBody(funnyName)
	if strings.Contains(strings.ToUpper(packageBody), strings.ToUpper(procedureBody)) {
		return packageBody, nil
	}
	return insertBeforePackageEnd(packageBody, procedureBody)
}

func normalizePackageSource(packageSpec string, packageBody string) (string, string, error) {
	var err error
	packageSpec, err = removeProcedureBlock(packageSpec, "PROCEDURE Enqueue_For_Subscriber(", ");")
	if err != nil {
		return "", "", err
	}
	packageBody, err = removeProcedureBlock(packageBody, "PROCEDURE Enqueue_For_Subscriber(", "END Enqueue_For_Subscriber;")
	if err != nil {
		return "", "", err
	}
	packageBody, err = replaceProcedureBlock(packageBody, "PROCEDURE Enqueue_Event___ (", "END Enqueue_Event___;", standardEnqueueEventBody())
	if err != nil {
		return "", "", err
	}
	return packageSpec, packageBody, nil
}

func removeProcedureDeclaration(packageSpec string, procedureName string) (string, error) {
	return removeProcedureBlock(packageSpec, fmt.Sprintf("PROCEDURE %s(", procedureName), ");")
}

func removeProcedureBody(packageBody string, procedureName string) (string, error) {
	return removeProcedureBlock(packageBody, fmt.Sprintf("PROCEDURE %s(", procedureName), fmt.Sprintf("END %s;", procedureName))
}

func insertBeforePackageEnd(packageSource string, block string) (string, error) {
	packageEndIdx := strings.LastIndex(strings.ToUpper(packageSource), fmt.Sprintf("END %s;", packageName))
	if packageEndIdx == -1 {
		return "", fmt.Errorf("package end marker not found")
	}

	prefix := strings.TrimRight(packageSource[:packageEndIdx], "\n")
	suffix := strings.TrimLeft(packageSource[packageEndIdx:], "\n")
	return prefix + "\n\n" + block + "\n\n" + suffix, nil
}

func removeProcedureBlock(packageSource string, startNeedle string, endNeedle string) (string, error) {
	upperSource := strings.ToUpper(packageSource)
	startIdx := strings.Index(upperSource, strings.ToUpper(startNeedle))
	if startIdx == -1 {
		return packageSource, nil
	}

	endRelIdx := strings.Index(upperSource[startIdx:], strings.ToUpper(endNeedle))
	if endRelIdx == -1 {
		return "", fmt.Errorf("procedure end marker not found")
	}
	endIdx := startIdx + endRelIdx + len(endNeedle)

	prefix := strings.TrimRight(packageSource[:startIdx], "\n")
	suffix := strings.TrimLeft(packageSource[endIdx:], "\n")
	if suffix == "" {
		return prefix + "\n", nil
	}
	return prefix + "\n\n" + suffix, nil
}

func replaceProcedureBlock(packageSource string, startNeedle string, endNeedle string, replacement string) (string, error) {
	upperSource := strings.ToUpper(packageSource)
	startIdx := strings.Index(upperSource, strings.ToUpper(startNeedle))
	if startIdx == -1 {
		return "", fmt.Errorf("procedure start marker not found")
	}

	endRelIdx := strings.Index(upperSource[startIdx:], strings.ToUpper(endNeedle))
	if endRelIdx == -1 {
		return "", fmt.Errorf("procedure end marker not found")
	}
	endIdx := startIdx + endRelIdx + len(endNeedle)

	prefix := strings.TrimRight(packageSource[:startIdx], "\n")
	suffix := strings.TrimLeft(packageSource[endIdx:], "\n")
	return prefix + "\n\n" + replacement + "\n\n" + suffix, nil
}

func validateFunnyNameForProcedure(name string) error {
	if name == "" {
		return domain.ErrInvalidFunnyName
	}
	if err := domain.ValidateFunnyNameFormat(name); err != nil {
		return err
	}
	if !domain.IsValidFunnyName(name) {
		return fmt.Errorf("validateFunnyNameForProcedure: %w", domain.ErrInvalidFunnyName)
	}
	return nil
}

func buildProcedureName(funnyName string) string {
	return procedureNamePrefix + strings.ToUpper(funnyName)
}

func generateProcedureDeclaration(funnyName string) string {
	procedureName := buildProcedureName(funnyName)
	return fmt.Sprintf(`    PROCEDURE %s(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO'
    );`, procedureName)
}

func generateProcedureBody(funnyName string) string {
	procedureName := buildProcedureName(funnyName)
	upperName := strings.ToUpper(funnyName)
	return fmt.Sprintf(`    PROCEDURE %s(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO'
    )
    IS
    BEGIN
        Enqueue_Event___(
			process_name_     => NULL,
            log_level_        => log_level_,
            payload           => message_,
            subscriber_name_  => '%s'
        );
    END %s;`, procedureName, upperName, procedureName)
}

func standardEnqueueEventBody() string {
	return `    PROCEDURE Enqueue_Event___ (
        process_name_       IN VARCHAR2,
        log_level_          IN VARCHAR2,
        payload             IN CLOB,
        additional_props_   IN CLOB DEFAULT NULL,
        subscriber_name_    IN VARCHAR2 DEFAULT NULL )
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
        resolved_process_   VARCHAR2(100);
    BEGIN
        enqueue_options_.visibility := DBMS_AQ.IMMEDIATE; -- Message visible immediately without waiting for commit

        resolved_process_ := process_name_;
        IF resolved_process_ IS NULL THEN
            resolved_process_ := SYS_CONTEXT('USERENV', 'MODULE');
            IF resolved_process_ IS NULL THEN
                resolved_process_ := 'OMNI_TRACER_API';
            END IF;
        END IF;

        IF subscriber_name_ IS NOT NULL THEN
            message_properties_.recipient_list := SYS.AQ$_RECIPIENT_LIST_T(
                SYS.AQ$_AGENT(subscriber_name_, NULL, NULL)
            );
        END IF;

        message_ := JSON_OBJECT_T();
        message_.PUT('MESSAGE_ID', TO_CHAR(OMNI_tracer_id_seq.NEXTVAL));
        message_.PUT('PROCESS_NAME', resolved_process_);
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

        IF subscriber_name_ IS NOT NULL THEN
            message_.PUT('SUBSCRIBER', subscriber_name_);
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
    END Enqueue_Event___;`
}

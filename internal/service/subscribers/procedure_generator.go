package subscribers

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	procedureNamePrefix = "TRACE_MESSAGE_"
	packageName         = "OMNI_TRACER_API"
	baseSQLFile         = "Omni_Tracer.sql"

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
		pg.ReleaseFunnyName(ctx, funnyName)
		return nil
	}

	packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}

	packageSpec, err = removeProcedureDeclaration(packageSpec, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}
	packageBody, err = removeProcedureBody(packageBody, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}

	if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}

	pg.ReleaseFunnyName(ctx, funnyName)
	return nil
}

func (pg *ProcedureGenerator) loadPackageSource(ctx context.Context) (string, string, error) {
	packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
	if err == nil {
		return packageSpec, packageBody, nil
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

// loadBasePackageSource loads the base OMNI_TRACER_API package source from the
// embedded Omni_Tracer.sql asset file. Used when the package does not yet exist
// in the database (e.g., first deploy).
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

// extractSQLSection extracts a section from SQL content between start and end
// markers, stripping trailing "/" and trimming whitespace.
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

// renderPackageDeploymentSQL builds a complete deployable SQL script from package
// spec and body sections, including section markers and delimiters.
// Oracle's user_source view omits the 'CREATE OR REPLACE' DDL prefix — this
// function restores it if absent so the rendered SQL is always executable.
func renderPackageDeploymentSQL(packageSpec string, packageBody string) string {
	spec := strings.TrimSpace(packageSpec)
	body := strings.TrimSpace(packageBody)

	// Restore CREATE OR REPLACE prefix stripped by user_source
	if !strings.HasPrefix(strings.ToUpper(spec), "CREATE") {
		spec = "CREATE OR REPLACE " + spec
	}
	if !strings.HasPrefix(strings.ToUpper(body), "CREATE") {
		body = "CREATE OR REPLACE " + body
	}

	return fmt.Sprintf(`%s

%s
/
%s

%s

%s
/
%s
`, packageSpecStart, spec, packageSpecEnd, packageBodyStart, body, packageBodyEnd)
}

// injectProcedureDeclaration adds a new procedure declaration to the package spec
// if it does not already exist. Uses regex search to detect duplicates.
func injectProcedureDeclaration(packageSpec string, funnyName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	pattern := regexp.MustCompile(`(?i)PROCEDURE\s+` + regexp.QuoteMeta(procedureName) + `\s*\(`)
	if pattern.MatchString(packageSpec) {
		return packageSpec, nil
	}
	return insertBeforePackageEnd(packageSpec, generateProcedureDeclaration(funnyName))
}

// injectProcedureBody adds a new procedure body to the package body if it does not
// already exist. Uses regex search to detect duplicates.
func injectProcedureBody(packageBody string, funnyName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	pattern := regexp.MustCompile(`(?i)PROCEDURE\s+` + regexp.QuoteMeta(procedureName) + `\s*\(`)
	if pattern.MatchString(packageBody) {
		return packageBody, nil
	}
	return insertBeforePackageEnd(packageBody, generateProcedureBody(funnyName))
}

// removeProcedureDeclaration removes a procedure declaration from the package spec.
func removeProcedureDeclaration(packageSpec string, procedureName string) (string, error) {
	return removeProcedureBlock(packageSpec, fmt.Sprintf("PROCEDURE %s(", procedureName), ");")
}

// removeProcedureBody removes a procedure body from the package body.
func removeProcedureBody(packageBody string, procedureName string) (string, error) {
	return removeProcedureBlock(packageBody, fmt.Sprintf("PROCEDURE %s(", procedureName), fmt.Sprintf("END %s;", procedureName))
}

// insertBeforePackageEnd inserts a block of text before the package END marker.
func insertBeforePackageEnd(packageSource string, block string) (string, error) {
	packageEndIdx := strings.LastIndex(strings.ToUpper(packageSource), fmt.Sprintf("END %s;", packageName))
	if packageEndIdx == -1 {
		return "", fmt.Errorf("package end marker not found")
	}

	prefix := strings.TrimRight(packageSource[:packageEndIdx], "\n")
	suffix := strings.TrimLeft(packageSource[packageEndIdx:], "\n")
	return prefix + "\n\n" + block + "\n\n" + suffix, nil
}

// removeProcedureBlock removes a procedure block from package source given start and
// end needle strings. Case-insensitive search. Returns source unchanged if not found.
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

// validateFunnyNameForProcedure checks that a funny name is valid for use in a
// generated procedure. It validates format (regex + length) and presence in the
// approved funny name list.
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

// buildProcedureName builds the full procedure name from a funny name (uppercases it).
func buildProcedureName(funnyName string) string {
	return procedureNamePrefix + strings.ToUpper(funnyName)
}

// generateProcedureDeclaration generates the PL/SQL declaration text for a
// subscriber procedure with message and log_level parameters.
func generateProcedureDeclaration(funnyName string) string {
	procedureName := buildProcedureName(funnyName)
	return fmt.Sprintf(`    PROCEDURE %s(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO',
		process_name_  IN VARCHAR2 DEFAULT NULL
    );`, procedureName)
}

// generateProcedureBody generates the PL/SQL body for a subscriber procedure. The
// procedure delegates to Enqueue_Event___ with the subscriber's funny name hardcoded
// for targeted message routing.
func generateProcedureBody(funnyName string) string {
	procedureName := buildProcedureName(funnyName)
	upperName := strings.ToUpper(funnyName)
	return fmt.Sprintf(`    PROCEDURE %s(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO',
		process_name_  IN VARCHAR2 DEFAULT NULL
    )
    IS
    BEGIN
        Enqueue_Event___(
			process_name_     => process_name_,
            log_level_        => log_level_,
            payload           => message_,
            subscriber_name_  => '%s'
        );
    END %s;`, procedureName, upperName, procedureName)
}

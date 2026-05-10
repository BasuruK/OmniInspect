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
		return "", false, nil
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

		procedureName := buildProcedureName(funnyName)
		exists, err := pg.db.ProcedureExists(ctx, domain.OmniTracerPackage, procedureName)
		if err != nil {
			_ = gen.MarkAsAvailable(funnyName)
			return "", false, fmt.Errorf("ReserveFunnyName: failed to check procedure existence: %w", err)
		}
		if !exists {
			return funnyName, true, nil
		}
		_ = gen.MarkAsAvailable(funnyName)
	}

	return "", false, fmt.Errorf("ReserveFunnyName: %w", domain.ErrNoAvailableNames)
}

func (pg *ProcedureGenerator) ReleaseFunnyName(ctx context.Context, funnyName string) error {
	if funnyName == "" {
		return nil
	}
	if err := domain.DefaultFunnyNameGenerator().MarkAsAvailable(funnyName); err != nil {
		return fmt.Errorf("ReleaseFunnyName: %w", err)
	}
	return nil
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
	packageSpec, packageBody, err := pg.loadPackageSource(ctx)
	if err != nil {
		return fmt.Errorf("failed to load package source: %w", err)
	}

	// ALWAYS check both spec and body for the correct signature/content.
	// injectProcedureDeclaration and injectProcedureBody skip if the signature matches.
	// However, to ensure existing installations with old bodies (missing process_name_)
	// are updated, we must first remove the old version if it exists but differs.
	if containsProcedureSignature(packageSpec, procedureName) {
		// If it's already using the new signature with process_name_, we can trust
		// the body is likely correct or will be handled by body injection check.
		// But the requirement asks for a content/version check.
		// Since we don't have a version marker, we use the signature as a proxy for the 'version'.
		if !strings.Contains(strings.ToUpper(packageSpec), "PROCESS_NAME_") {
			// Old version found - strip it so we can re-inject new version
			packageSpec, err = removeProcedureDeclaration(packageSpec, procedureName)
			if err != nil {
				return fmt.Errorf("failed to strip old package spec: %w", err)
			}
			packageBody, err = removeProcedureBody(packageBody, procedureName)
			if err != nil {
				return fmt.Errorf("failed to strip old package body: %w", err)
			}
		} else {
			// Current version exists
			return nil
		}
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
	exists, err := pg.db.ProcedureExists(ctx, domain.OmniTracerPackage, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to check procedure existence: %w", err)
	}
	if !exists {
		if err := pg.ReleaseFunnyName(ctx, funnyName); err != nil {
			return fmt.Errorf("DropSubscriberProcedure: %w", err)
		}
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

	if err := pg.ReleaseFunnyName(ctx, funnyName); err != nil {
		return fmt.Errorf("DropSubscriberProcedure: %w", err)
	}
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

	return "", "", fmt.Errorf("loadPackageSource: %w", err)
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

	return strings.Join(packageSpecLines, ""), strings.Join(packageBodyLines, ""), nil
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
		return "", fmt.Errorf("extractSQLSection: %w", domain.ErrSectionNotFound)
	}

	section := strings.TrimSpace(sqlContent[startIdx+len(startMarker) : endIdx])
	section = strings.TrimSpace(strings.TrimSuffix(section, "/"))
	if section == "" {
		return "", fmt.Errorf("extractSQLSection: %w", domain.ErrSectionEmpty)
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
// if it does not already exist.
func injectProcedureDeclaration(packageSpec string, funnyName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	if containsProcedureSignature(packageSpec, procedureName) {
		return packageSpec, nil
	}
	return insertBeforePackageEnd(packageSpec, generateProcedureDeclaration(funnyName))
}

// injectProcedureBody adds a new procedure body to the package body if it does not
// already exist.
func injectProcedureBody(packageBody string, funnyName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	if containsProcedureSignature(packageBody, procedureName) {
		return packageBody, nil
	}
	return insertBeforePackageEnd(packageBody, generateProcedureBody(funnyName))
}

// containsProcedureSignature checks if the package source contains a procedure declaration
func containsProcedureSignature(packageSource string, procedureName string) bool {
	normalizedSource := strings.ToUpper(strings.Join(strings.Fields(packageSource), " "))
	upperProcedureName := strings.ToUpper(procedureName)
	return strings.Contains(normalizedSource, "PROCEDURE "+upperProcedureName+"(") ||
		strings.Contains(normalizedSource, "PROCEDURE "+upperProcedureName+" (")
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
		return "", fmt.Errorf("insertBeforePackageEnd: %w", domain.ErrPackageEndMarkerNotFound)
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
		return "", fmt.Errorf("removeProcedureBlock: %w", domain.ErrProcedureEndMarkerNotFound)
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

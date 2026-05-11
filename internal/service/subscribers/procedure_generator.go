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
	procedureNamePrefix      = "TRACE_MESSAGE_"
	packageName              = "OMNI_TRACER_API"
	baseSQLFile              = "Omni_Tracer.sql"
	generatedMethodMarker    = "-- @SECTION: SUBSCRIBER_GENERATED_METHOD : "
	generatedMethodEndMarker = "-- @END_SECTION: SUBSCRIBER_GENERATED_METHOD : "

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

func (pg *ProcedureGenerator) EnsureOwnedFunnyName(ctx context.Context, subscriber *domain.Subscriber) (bool, error) {
	if subscriber == nil {
		return false, fmt.Errorf("EnsureOwnedFunnyName: %w", domain.ErrNilSubscriber)
	}
	if subscriber.FunnyName() == "" {
		funnyName, _, err := pg.ReserveFunnyName(ctx, subscriber)
		if err != nil {
			return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
		}
		if err := subscriber.AssignFunnyName(funnyName); err != nil {
			_ = pg.ReleaseFunnyName(ctx, funnyName)
			return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
		}
		return true, nil
	}
	if err := validateFunnyNameForProcedure(subscriber.FunnyName()); err != nil {
		return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
	}
	ownedByAnother, err := pg.procedureOwnedByAnother(ctx, subscriber)
	if err != nil {
		return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
	}
	if !ownedByAnother {
		return false, nil
	}

	if err := pg.ReleaseFunnyName(ctx, subscriber.FunnyName()); err != nil {
		return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
	}
	gen := domain.DefaultFunnyNameGenerator()
	attempts := gen.AvailableCount()
	for i := 0; i < attempts; i++ {
		funnyName, err := gen.GetRandomName()
		if err != nil {
			return false, fmt.Errorf("EnsureOwnedFunnyName: failed to get funny name: %w", err)
		}
		if err := subscriber.AssignFunnyName(funnyName); err != nil {
			_ = pg.ReleaseFunnyName(ctx, funnyName)
			return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
		}
		ownedByAnother, err := pg.procedureOwnedByAnother(ctx, subscriber)
		if err != nil {
			_ = pg.ReleaseFunnyName(ctx, funnyName)
			return false, fmt.Errorf("EnsureOwnedFunnyName: %w", err)
		}
		if !ownedByAnother {
			return true, nil
		}
		_ = pg.ReleaseFunnyName(ctx, funnyName)
	}
	return false, fmt.Errorf("EnsureOwnedFunnyName: %w", domain.ErrNoAvailableNames)
}

func (pg *ProcedureGenerator) procedureOwnedByAnother(ctx context.Context, subscriber *domain.Subscriber) (bool, error) {
	packageSpec, packageBody, err := pg.loadPackageSource(ctx)
	if err != nil {
		return false, err
	}
	procedureName := buildProcedureName(subscriber.FunnyName())
	declarationBlock, hasDeclaration, err := extractProcedureDeclaration(packageSpec, procedureName)
	if err != nil {
		return false, err
	}
	bodyBlock, hasBody, err := extractProcedureBody(packageBody, procedureName)
	if err != nil {
		return false, err
	}
	if !hasDeclaration && !hasBody {
		return false, nil
	}
	return (hasGeneratedMethodOwner(declarationBlock) && !procedureOwnedBy(declarationBlock, subscriber.Name())) ||
		(hasGeneratedMethodOwner(bodyBlock) && !procedureOwnedBy(bodyBlock, subscriber.Name())), nil
}

func (pg *ProcedureGenerator) EnsureSubscriberProcedure(ctx context.Context, subscriber *domain.Subscriber) error {
	if subscriber == nil {
		return fmt.Errorf("EnsureSubscriberProcedure: %w", domain.ErrNilSubscriber)
	}

	funnyName := subscriber.FunnyName()
	if err := validateFunnyNameForProcedure(funnyName); err != nil {
		return fmt.Errorf("EnsureSubscriberProcedure: %w", err)
	}

	procedureName := buildProcedureName(funnyName)
	packageSpec, packageBody, err := pg.loadPackageSource(ctx)
	if err != nil {
		return fmt.Errorf("failed to load package source: %w", err)
	}

	declarationBlock, hasDeclaration, err := extractProcedureDeclaration(packageSpec, procedureName)
	if err != nil {
		return fmt.Errorf("failed to inspect package spec: %w", err)
	}
	bodyBlock, hasBody, err := extractProcedureBody(packageBody, procedureName)
	if err != nil {
		return fmt.Errorf("failed to inspect package body: %w", err)
	}
	if hasDeclaration && hasBody {
		if procedureOwnedBy(declarationBlock, subscriber.Name()) && procedureOwnedBy(bodyBlock, subscriber.Name()) && hasExpectedGeneratedBody(bodyBlock, funnyName) {
			return nil
		}
		if hasGeneratedMethodOwner(declarationBlock) || hasGeneratedMethodOwner(bodyBlock) {
			return fmt.Errorf("EnsureSubscriberProcedure: %w", domain.ErrProcedureOwnershipConflict)
		}
		packageSpec, err = removeProcedureDeclaration(packageSpec, procedureName)
		if err != nil {
			return fmt.Errorf("failed to strip old package spec: %w", err)
		}
		packageBody, err = removeProcedureBody(packageBody, procedureName)
		if err != nil {
			return fmt.Errorf("failed to strip old package body: %w", err)
		}
	}

	packageSpec, err = injectProcedureDeclarationForSubscriber(packageSpec, funnyName, subscriber.Name())
	if err != nil {
		return fmt.Errorf("failed to update package spec: %w", err)
	}
	packageBody, err = injectProcedureBodyForSubscriber(packageBody, funnyName, subscriber.Name())
	if err != nil {
		return fmt.Errorf("failed to update package body: %w", err)
	}

	if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
		return fmt.Errorf("EnsureSubscriberProcedure: %w", err)
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

// injectProcedureDeclarationForSubscriber ensures the package spec contains a procedure declaration for the subscriber, injecting it if missing.
func injectProcedureDeclarationForSubscriber(packageSpec string, funnyName string, subscriberName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	if containsProcedureSignature(packageSpec, procedureName) {
		return packageSpec, nil
	}
	return insertBeforePackageEnd(packageSpec, generateProcedureDeclaration(funnyName, subscriberName))
}

// injectProcedureBodyForSubscriber checks if the procedure body already exists (e.g., from a previous deployment) and if not, injects the generated procedure body for the subscriber into the package body source.
func injectProcedureBodyForSubscriber(packageBody string, funnyName string, subscriberName string) (string, error) {
	procedureName := buildProcedureName(funnyName)
	if containsProcedureSignature(packageBody, procedureName) {
		return packageBody, nil
	}
	return insertBeforePackageEnd(packageBody, generateProcedureBody(funnyName, subscriberName))
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

func subscriberMethodStartMarker(subscriberName string) string {
	return generatedMethodMarker + strings.ToUpper(subscriberName)
}

func subscriberMethodEndMarker(subscriberName string) string {
	return generatedMethodEndMarker + strings.ToUpper(subscriberName)
}

func wrapSubscriberGeneratedMethod(subscriberName string, block string) string {
	if subscriberName == "" {
		return block
	}
	return subscriberMethodStartMarker(subscriberName) + "\n" + block + "\n" + subscriberMethodEndMarker(subscriberName)
}

func procedureOwnedBy(block string, subscriberName string) bool {
	return strings.Contains(strings.ToUpper(block), subscriberMethodStartMarker(subscriberName)) &&
		strings.Contains(strings.ToUpper(block), subscriberMethodEndMarker(subscriberName))
}

func hasGeneratedMethodOwner(block string) bool {
	return strings.Contains(strings.ToUpper(block), generatedMethodMarker)
}

func hasExpectedGeneratedBody(block string, funnyName string) bool {
	normalized := strings.ToUpper(strings.Join(strings.Fields(block), " "))
	return strings.Contains(normalized, "PROCESS_NAME_") &&
		strings.Contains(normalized, "ENQUEUE_EVENT___(") &&
		strings.Contains(normalized, "SUBSCRIBER_NAME_ => '"+strings.ToUpper(funnyName)+"'")
}

func extractProcedureDeclaration(packageSpec string, procedureName string) (string, bool, error) {
	return extractProcedureBlock(packageSpec, fmt.Sprintf("PROCEDURE %s(", procedureName), ");", generatedMethodMarker)
}

func extractProcedureBody(packageBody string, procedureName string) (string, bool, error) {
	return extractProcedureBlock(packageBody, fmt.Sprintf("PROCEDURE %s(", procedureName), fmt.Sprintf("END %s;", procedureName), generatedMethodMarker)
}

func extractProcedureBlock(packageSource string, startNeedle string, endNeedle string, ownerMarker string) (string, bool, error) {
	upperSource := strings.ToUpper(packageSource)
	startIdx := strings.Index(upperSource, strings.ToUpper(startNeedle))
	if startIdx == -1 {
		return "", false, nil
	}

	endRelIdx := strings.Index(upperSource[startIdx:], strings.ToUpper(endNeedle))
	if endRelIdx == -1 {
		return "", true, fmt.Errorf("extractProcedureBlock: %w", domain.ErrProcedureEndMarkerNotFound)
	}
	endIdx := startIdx + endRelIdx + len(endNeedle)
	markerIdx := strings.LastIndex(upperSource[:startIdx], strings.ToUpper(ownerMarker))
	if markerIdx != -1 {
		// Only expand when the ownership marker immediately precedes the procedure (only whitespace between them)
		markerLineEnd := strings.Index(upperSource[markerIdx:], "\n")
		if markerLineEnd != -1 {
			markerLineEndAbs := markerIdx + markerLineEnd + 1
			if strings.TrimSpace(packageSource[markerLineEndAbs:startIdx]) == "" {
				endMarkerIdx := strings.Index(upperSource[endIdx:], strings.ToUpper(generatedMethodEndMarker))
				if endMarkerIdx != -1 {
					endIdx = endIdx + endMarkerIdx + len(generatedMethodEndMarker)
					lineEndIdx := strings.Index(packageSource[endIdx:], "\n")
					if lineEndIdx != -1 {
						endIdx += lineEndIdx
					}
					startIdx = markerIdx
				}
			}
		}
	}
	return packageSource[startIdx:endIdx], true, nil
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
// Also strips adjacent ownership-marker lines when they immediately wrap the block.
func removeProcedureBlock(packageSource string, startNeedle string, endNeedle string) (string, error) {
	upperSource := strings.ToUpper(packageSource)
	startIdx := strings.Index(upperSource, strings.ToUpper(startNeedle))
	if startIdx == -1 {
		// Oracle may format the signature with a space before '(' — retry with that form
		startNeedleAlt := strings.Replace(startNeedle, "(", " (", 1)
		startIdx = strings.Index(upperSource, strings.ToUpper(startNeedleAlt))
		if startIdx == -1 {
			return packageSource, nil
		}
	}

	endRelIdx := strings.Index(upperSource[startIdx:], strings.ToUpper(endNeedle))
	if endRelIdx == -1 {
		return "", fmt.Errorf("removeProcedureBlock: %w", domain.ErrProcedureEndMarkerNotFound)
	}
	endIdx := startIdx + endRelIdx + len(endNeedle)

	// Expand removal range to include adjacent ownership markers when they immediately wrap the block.
	// Search backwards from the procedure start for a preceding @SECTION marker.
	markerIdx := strings.LastIndex(upperSource[:startIdx], strings.ToUpper(generatedMethodMarker))
	if markerIdx != -1 {
		// Find the end of the marker line so we can measure the gap between it and the procedure.
		markerLineEnd := strings.Index(upperSource[markerIdx:], "\n")
		if markerLineEnd != -1 {
			// Absolute position of the first character after the marker line.
			markerLineEndAbs := markerIdx + markerLineEnd + 1
			// Only treat the marker as "immediately preceding" when nothing but
			// whitespace sits between the end of the marker line and the procedure keyword.
			if strings.TrimSpace(packageSource[markerLineEndAbs:startIdx]) == "" {
				// The marker wraps this block — look for the matching @END_SECTION after the procedure.
				endMarkerIdx := strings.Index(upperSource[endIdx:], strings.ToUpper(generatedMethodEndMarker))
				if endMarkerIdx != -1 {
					// Convert the relative offset to an absolute position in the source.
					endMarkerAbsIdx := endIdx + endMarkerIdx
					// Advance past the full @END_SECTION line, including its newline.
					endMarkerLineEnd := strings.Index(upperSource[endMarkerAbsIdx:], "\n")
					if endMarkerLineEnd != -1 {
						endIdx = endMarkerAbsIdx + endMarkerLineEnd + 1
					} else {
						// No trailing newline — advance past the marker text itself.
						endIdx = endMarkerAbsIdx + len(generatedMethodEndMarker)
					}
					// Pull the removal start back to include the @SECTION line.
					startIdx = markerIdx
				}
			}
		}
	}

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
	if err := domain.ValidateFunnyNameForSQLInjection(name); err != nil {
		return err
	}
	return nil
}

// buildProcedureName builds the full procedure name from a funny name (uppercases it).
func buildProcedureName(funnyName string) string {
	return procedureNamePrefix + strings.ToUpper(funnyName)
}

// generateProcedureDeclaration generates the PL/SQL declaration text for a
// subscriber procedure with message and log_level parameters.
func generateProcedureDeclaration(funnyName string, subscriberName string) string {
	procedureName := buildProcedureName(funnyName)
	return wrapSubscriberGeneratedMethod(subscriberName, fmt.Sprintf(`    PROCEDURE %s(
        message_   IN CLOB,
        log_level_ IN VARCHAR2 DEFAULT 'INFO',
        process_name_  IN VARCHAR2 DEFAULT NULL
    );`, procedureName))
}

// generateProcedureBody generates the PL/SQL body for a subscriber procedure. The
// procedure delegates to Enqueue_Event___ with the subscriber's funny name hardcoded
// for targeted message routing.
func generateProcedureBody(funnyName string, subscriberName string) string {
	procedureName := buildProcedureName(funnyName)
	upperName := strings.ToUpper(funnyName)
	return wrapSubscriberGeneratedMethod(subscriberName, fmt.Sprintf(`    PROCEDURE %s(
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
    END %s;`, procedureName, upperName, procedureName))
}

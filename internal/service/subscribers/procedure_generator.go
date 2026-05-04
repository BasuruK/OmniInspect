package subscribers

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
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

func NewProcedureGenerator(db ports.DatabaseRepository) *ProcedureGenerator {
	return &ProcedureGenerator{db: db}
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

func (pg *ProcedureGenerator) ReleaseFunnyName(funnyName string) {
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
		return fmt.Errorf("GenerateSubscriberProcedure: failed to load package source: %w", err)
	}

	packageSpec, err = injectProcedureDeclaration(packageSpec, funnyName)
	if err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: failed to update package spec: %w", err)
	}
	packageBody, err = injectProcedureBody(packageBody, funnyName)
	if err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: failed to update package body: %w", err)
	}

	if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
		return fmt.Errorf("GenerateSubscriberProcedure: failed to deploy package: %w", err)
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

	packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to load package source: %w", err)
	}

	packageSpec, err = removeProcedureDeclaration(packageSpec, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to update package spec: %w", err)
	}
	packageBody, err = removeProcedureBody(packageBody, procedureName)
	if err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to update package body: %w", err)
	}

	if err := pg.db.DeployFile(ctx, renderPackageDeploymentSQL(packageSpec, packageBody)); err != nil {
		return fmt.Errorf("DropSubscriberProcedure: failed to deploy package: %w", err)
	}

	pg.ReleaseFunnyName(funnyName)
	return nil
}

func (pg *ProcedureGenerator) loadPackageSource(ctx context.Context) (string, string, error) {
	packageSpec, packageBody, err := pg.fetchCurrentPackageSource(ctx)
	if err == nil && packageHasEnqueueForSubscriber(packageSpec, packageBody) {
		return packageSpec, packageBody, nil
	}

	return loadBasePackageSource()
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
		return "", "", fmt.Errorf("package source not found")
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

func packageHasEnqueueForSubscriber(packageSpec string, packageBody string) bool {
	upperSpec := strings.ToUpper(packageSpec)
	upperBody := strings.ToUpper(packageBody)
	return strings.Contains(upperSpec, "PROCEDURE ENQUEUE_FOR_SUBSCRIBER(") && strings.Contains(upperBody, "PROCEDURE ENQUEUE_FOR_SUBSCRIBER(")
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
        OMNI_TRACER_API.Enqueue_For_Subscriber(
            subscriber_name_ => '%s',
            message_         => message_,
            log_level_       => log_level_
        );
    END %s;`, procedureName, upperName, procedureName)
}

package permissions

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"context"
	"encoding/json"
	"fmt"
)

// Service: Manages database permission checks and package deployments
// Uses dedicated PermissionsRepository for persistence
type PermissionService struct {
	db        ports.DatabaseRepository
	permsRepo ports.PermissionsRepository
	config    ports.ConfigRepository
}

// Constructor: NewPermissionService Constructor for PermissionService
func NewPermissionService(db ports.DatabaseRepository, permsRepo ports.PermissionsRepository, config ports.ConfigRepository) *PermissionService {
	return &PermissionService{
		db:        db,
		permsRepo: permsRepo,
		config:    config,
	}
}

// DeployAndCheck ensures the necessary permission checks package is deployed, checked and dropped
func (ps *PermissionService) DeployAndCheck(ctx context.Context, schema string) (bool, error) {
	// Check if permissions already exist for this schema in boltDB
	exists, err := ps.permsRepo.Exists(ctx, schema)
	if err != nil {
		return false, err
	}

	// If permissions don't exist for this schema, run the check
	if !exists {
		// Ensure the permission checks package is deployed
		if err := deployPermissionChecksPackage(ctx, ps); err != nil {
			_ = dropPermissionChecksPackage(ctx, ps)
			return false, err
		}
		// Check permissions using the deployed package
		perStatus, err := checkPermissions(ctx, ps, schema)
		if err != nil {
			_ = dropPermissionChecksPackage(ctx, ps)
			return false, err
		}
		// Save the permission status to BoltDB using new repository
		if err := ps.permsRepo.Save(ctx, perStatus); err != nil {
			_ = dropPermissionChecksPackage(ctx, ps)
			return false, err
		}
	}
	// Always retry cleanup/finalization for first-run flow recovery.
	if err := dropPermissionChecksPackage(ctx, ps); err != nil {
		return false, err
	}
	if err := ps.config.SetFirstRunCycleStatus(*ports.NewRunCycleStatus(false)); err != nil {
		return false, err
	}
	return true, nil
}

// DeployPermissionChecksPackage deploys the permission checks package to the database if not already present
func deployPermissionChecksPackage(ctx context.Context, ps *PermissionService) error {
	// Check if the permission checks package is already deployed
	exists, err := ps.db.PackageExists(ctx, "TXEVENTQ_PERMISSION_CHECK_API")
	if err != nil {
		return fmt.Errorf("failed to check package existence: %w", err)
	}

	if exists {
		// Package already exists, no need to deploy
		return nil
	}

	// Read the permission checks package file
	permissionChecksSQLPackage, err := assets.GetSQLFile("Permission_Checks.sql")
	if err != nil {
		return fmt.Errorf("failed to read permission checks package file: %w", err)
	}

	if err := ps.db.DeployFile(ctx, string(permissionChecksSQLPackage)); err != nil {
		return fmt.Errorf("failed to deploy permission checks package: %w", err)
	}

	return nil
}

func checkPermissions(ctx context.Context, ps *PermissionService, schema string) (*domain.DatabasePermissions, error) {
	// Execute permission check procedure
	query := `SELECT TXEVENTQ_PERMISSION_CHECK_API.Get_Permission_Report(:schema) FROM DUAL`
	results, err := ps.db.FetchWithParams(ctx, query, map[string]interface{}{
		"schema": schema,
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no results returned from permission check package")
	}

	jsonData := results[0]

	// Unmarshal the result into PermissionStatus
	var permsStatus domain.PermissionStatus
	if err := json.Unmarshal([]byte(jsonData), &permsStatus); err != nil {
		return nil, err
	}

	// Create the DatabasePermissions entity
	perStatus := domain.NewDatabasePermissions(schema, permsStatus)

	// Helper function to convert bool to status mark
	statusMark := func(b bool) string {
		if b {
			return "[OK]"
		}
		return "[NO]"
	}

	permStructTable := fmt.Sprintf("\nPermission Status Details:\n"+
		"┌───────────────────────────┬─────────┐\n"+
		"│ %-25s │ %-7s │\n"+
		"├───────────────────────────┼─────────┤\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"│ %-25s │ %-7s │\n"+
		"└───────────────────────────┴─────────┘",
		"Permission", "Status",
		"Create Sequence", statusMark(permsStatus.CreateSequence),
		"Create Procedure", statusMark(permsStatus.CreateProcedure),
		"Create Type", statusMark(permsStatus.CreateType),
		"AQ Administrator Role", statusMark(permsStatus.AQAdministratorRole),
		"AQ User Role", statusMark(permsStatus.AQUserRole),
		"Execute DBMS AQADM", statusMark(permsStatus.DBMSAQADMExecute),
		"Execute DBMS AQ", statusMark(permsStatus.DBMSAQExecute))

	// Evaluate if all permissions are valid
	if !perStatus.IsValid() {
		return nil, fmt.Errorf("permission checks failed for schema %s: %+v", schema, permStructTable)
	} else {
		fmt.Printf("All permission checks passed for schema %s %s\n", schema, permStructTable)
	}

	return perStatus, nil
}

// DropPermissionChecksPackage drops the permission checks package from the database
func dropPermissionChecksPackage(ctx context.Context, ps *PermissionService) error {
	dropQuery := `BEGIN
		EXECUTE IMMEDIATE 'DROP PACKAGE TXEVENTQ_PERMISSION_CHECK_API';
	EXCEPTION
		WHEN OTHERS THEN
			IF SQLCODE != -4043 THEN
				RAISE;
			END IF;
	END;`

	if err := ps.db.ExecuteStatement(ctx, dropQuery); err != nil {
		return fmt.Errorf("failed to drop permission checks package: %w", err)
	}

	return nil
}

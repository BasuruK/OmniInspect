package permissions

import (
	"OmniView/assets"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"encoding/json"
	"fmt"
)

// Service: Manages database permission checks and package deployments
// Injects a DatabaseRepository to interact with the database
type PermissionService struct {
	db   ports.DatabaseRepository
	bolt ports.ConfigRepository
}

// Constructor: NewPermissionService Constructor for PermissionService
func NewPermissionService(db ports.DatabaseRepository, bolt ports.ConfigRepository) *PermissionService {
	return &PermissionService{
		db:   db,
		bolt: bolt,
	}
}

// Check ensures the necessary permission checks package is deployed, checked and dropped
func (ps *PermissionService) Check(schema string) (bool, error) {
	// Check if permission checks have already been performed
	isFirstRun, err := ps.bolt.IsApplicationFirstRun()
	if err != nil {
		return false, err
	}
	if isFirstRun {
		var perStatus domain.DatabasePermissions
		// Ensure the permission checks package is deployed
		if err := deployPermissionChecksPackage(ps); err != nil {
			return false, err
		}
		// Check permissions using the deployed package
		_, err := checkPermissions(ps, schema, &perStatus)
		if err != nil {
			return false, err
		}
		// Save the permission status to BoltDB
		if err := savePermissionStatus(ps, &perStatus); err != nil {
			return false, err
		}
		// Set the first run cycle status to indicate that permission checks have been performed
		if err := ps.bolt.SetFirstRunCycleStatus(domain.RunCycleStatus{IsFirstRun: false}); err != nil {
			return false, err
		}
	}

	return true, nil
}

// DeployPermissionChecksPackage deploys the permission checks package to the database if not already present
func deployPermissionChecksPackage(ps *PermissionService) error {
	// Check if the permission checks package is already deployed
	exists, err := ps.db.PackageExists("TXEVENTQ_PERMISSION_CHECK_API")
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

	if err := ps.db.DeployFile(string(permissionChecksSQLPackage)); err != nil {
		return fmt.Errorf("failed to deploy permission checks package: %w", err)
	}

	return nil
}

func checkPermissions(ps *PermissionService, schema string, perStatus *domain.DatabasePermissions) (bool, error) {
	// Execute permission check procedure
	query := `SELECT TXEVENTQ_PERMISSION_CHECK_API.Get_Permission_Report(:schema) FROM DUAL`
	results, err := ps.db.FetchWithParams(query, map[string]interface{}{
		"schema": schema,
	})
	if err != nil {
		return false, err
	}
	if len(results) == 0 {
		return false, fmt.Errorf("no results returned from permission check package")
	}

	jsonData := results[0]
	// Unmarshall the result
	if err := json.Unmarshal([]byte(jsonData), &perStatus.Permissions); err != nil {
		return false, err
	}

	// Helper function to convert bool to status mark
	statusMark := func(b bool) string {
		if b {
			return "[OK]"
		}
		return "[NOT GRANTED]"
	}

	permStructTable := fmt.Sprintf("\nPermission Status Details:\n"+
		"┌─────────────────────────────────────┬─────────┐\n"+
		"│ %-35s │ %-7s │\n"+
		"├─────────────────────────────────────┼─────────┤\n"+
		"│ %-35s │ %-7s │\n"+
		"│ %-35s │ %-7s │\n"+
		"│ %-35s │ %-7s │\n"+
		"│ %-35s │ %-7s │\n"+
		"│ %-35s │ %-7s │\n"+
		"│ %-35s │ %-7s │\n"+
		"└─────────────────────────────────────┴─────────┘",
		"Permission", "Status",
		"Create Sequence", statusMark(perStatus.Permissions.CreateSequence),
		"Create Procedure", statusMark(perStatus.Permissions.CreateProcedure),
		"AQ Administrator Role", statusMark(perStatus.Permissions.AQAdministratorRole),
		"AQ User Role", statusMark(perStatus.Permissions.AQUserRole),
		"Execute DBMS AQADM", statusMark(perStatus.Permissions.DBMSAQADMExecute),
		"Execute DBMS AQ", statusMark(perStatus.Permissions.DBMSAQExecute))

	// Evaluate if all permissions are valid
	if !perStatus.Permissions.AllValid {
		return false, fmt.Errorf("permission checks failed for schema %s: %+v", schema, permStructTable)
	} else {
		fmt.Printf("All permission checks passed for schema %s %s\n", schema, permStructTable)
	}

	return true, nil
}

func savePermissionStatus(ps *PermissionService, status *domain.DatabasePermissions) error {
	if err := ps.bolt.SaveClientConfig(*status); err != nil {
		return err
	}
	return nil
}

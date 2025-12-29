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
	db ports.DatabaseRepository
}

// Constructor: NewPermissionService Constructor for PermissionService
func NewPermissionService(db ports.DatabaseRepository) *PermissionService {
	return &PermissionService{db: db}
}

// Check ensures the nessary permission checks package is deployed, checked and dropped
func (ps *PermissionService) Check(schema string) (bool, error) {
	// Ensure the permission checks package is deployed
	if err := deployPermissionChecksPackage(ps); err != nil {
		return false, err
	}
	// Check permissions using the deployed package
	_, err := checkPermissions(ps, schema)
	if err != nil {
		return false, err
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

func checkPermissions(ps *PermissionService, schema string) (bool, error) {
	var perStatus domain.PermissionStatus
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
	if err := json.Unmarshal([]byte(jsonData), &perStatus); err != nil {
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
		"Create Sequence", statusMark(perStatus.CreateSequence),
		"Create Procedure", statusMark(perStatus.CreateProcedure),
		"AQ Administrator Role", statusMark(perStatus.AQAdministratorRole),
		"AQ User Role", statusMark(perStatus.AQUserRole),
		"Execute DBMS AQADM", statusMark(perStatus.DBMSAQADMExecute),
		"Execute DBMS AQ", statusMark(perStatus.DBMSAQExecute))

	// Evaluate if all permissions are valid
	if !perStatus.AllValid {
		return false, fmt.Errorf("permission checks failed for schema %s: %+v", schema, permStructTable)
	} else {
		fmt.Printf("All permission checks passed for schema %s %s\n", schema, permStructTable)
	}

	return true, nil
}

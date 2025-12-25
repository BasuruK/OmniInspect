package permissions

import (
	"OmniView/assets"
	"OmniView/internal/core/ports"
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

func (ps *PermissionService) CheckAndDeploy() (bool, error) {
	// Ensure the permission checks package is deployed
	if err := ps.DeployPermissionChecksPackage(); err != nil {
		return false, err
	}
	return true, nil
}

func (ps *PermissionService) DeployPermissionChecksPackage() error {
	// Check if the permission checks package is already deployed
	if exists, err := ps.db.PackageExists("TXEVENTQ_PERMISSION_CHECK_API"); err == nil && exists == false {
		// Read the permission checks package file
		permissionChecksSQLPackage, err := assets.GetSQLFile("Permission_Checks.sql")
		if err != nil {
			return err
		}

		if err := ps.db.DeployFile(string(permissionChecksSQLPackage)); err != nil {
			return err
		}
	} else if exists {
		// Package already deployed, no action needed
		fmt.Println("Permission checks package already deployed.")
		return nil
	} else if err != nil {
		return fmt.Errorf("error with deploying permission package: %w", err)
	}

	return nil
}

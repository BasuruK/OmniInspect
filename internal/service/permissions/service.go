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

package permissions

import (
	"OmniView/internal/database"
	"os"
)

func deployPermissionChecksPackage() error {
	// Read the permission checks package file
	permissionChecksSQLPackage, err := os.ReadFile("internal/sql/permission_checks.plsql")
	if err != nil {
		return err
	}
	// Execute the package creation SQL
	sequences, err1 := database.ExtractSequenceBlocks(string(permissionChecksSQLPackage))
	packageSpec, err2 := database.ExtractPackageSpecBlocks(string(permissionChecksSQLPackage))
	packageBody, err3 := database.ExtractPackageBodyBlocks(string(permissionChecksSQLPackage))

	if err1 != nil || err2 != nil || err3 != nil {
		return err1
	}

	if err := database.DeployPackages(sequences, packageSpec, packageBody); err != nil {
		return err
	}

	return nil
}

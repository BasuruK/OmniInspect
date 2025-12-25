package ports

import "OmniView/internal/core/domain"

type ConfigLoader interface {
	LoadClientConfigurations() (*domain.AppConfigurations, error)
	LoadSystemConfigurations() (domain.RunCycleStatus, domain.DatabasePackageStatus, error)
}

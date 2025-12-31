package ports

import "OmniView/internal/core/domain"

type ConfigLoader interface {
	LoadClientConfigurations() (*domain.AppConfigurations, error)
}

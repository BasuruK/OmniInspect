package ports

import "OmniView/internal/core/domain"

// ClientSettings is an alias for domain.ClientSettings
type ClientSettings = domain.ClientSettings

// RunCycleStatus is an alias for domain.RunCycleStatus
type RunCycleStatus = domain.RunCycleStatus

// Helper constructors that delegate to domain
func NewClientSettings(enableUtf8 bool) *domain.ClientSettings {
	return domain.NewClientSettings(enableUtf8)
}

func DefaultClientSettings() *domain.ClientSettings {
	return domain.DefaultClientSettings()
}

func NewRunCycleStatus(isFirstRun bool) *domain.RunCycleStatus {
	return domain.NewRunCycleStatus(isFirstRun)
}

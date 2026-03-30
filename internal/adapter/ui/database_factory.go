package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
)

// DatabaseAdapterFactory creates a database adapter for a given settings object.
type DatabaseAdapterFactory func(*domain.DatabaseSettings) ports.DatabaseRepository

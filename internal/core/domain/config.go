package domain

// ==========================================
// Client Settings Value Object
// ==========================================

// ClientSettings represents client-specific settings
type ClientSettings struct {
	enableUtf8 bool
}

// NewClientSettings creates new client settings
func NewClientSettings(enableUtf8 bool) *ClientSettings {
	return &ClientSettings{enableUtf8: enableUtf8}
}

// DefaultClientSettings returns default client settings
func DefaultClientSettings() *ClientSettings {
	return &ClientSettings{enableUtf8: true}
}

func (c *ClientSettings) EnableUtf8() bool { return c.enableUtf8 }

// ==========================================
// Run Cycle Status Value Object
// ==========================================

// RunCycleStatus represents the status of the application's run cycle
type RunCycleStatus struct {
	isFirstRun bool
}

// NewRunCycleStatus creates a new run cycle status
func NewRunCycleStatus(isFirstRun bool) *RunCycleStatus {
	return &RunCycleStatus{isFirstRun: isFirstRun}
}

func (r *RunCycleStatus) IsFirstRun() bool { return r.isFirstRun }

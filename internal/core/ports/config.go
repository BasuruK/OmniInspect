package ports

// ClientSettings represents client-specific settings
type ClientSettings struct {
	enableUtf8 bool
}

func NewClientSettings(enableUtf8 bool) *ClientSettings {
	return &ClientSettings{enableUtf8: enableUtf8}
}

func DefaultClientSettings() *ClientSettings {
	return &ClientSettings{enableUtf8: true}
}

func (c *ClientSettings) EnableUtf8() bool { return c.enableUtf8 }

// RunCycleStatus represents the status of the application's run cycle
type RunCycleStatus struct {
	isFirstRun bool
}

func NewRunCycleStatus(isFirstRun bool) *RunCycleStatus {
	return &RunCycleStatus{isFirstRun: isFirstRun}
}

func (r *RunCycleStatus) IsFirstRun() bool { return r.isFirstRun }

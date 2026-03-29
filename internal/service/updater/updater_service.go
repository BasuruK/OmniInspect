package updater

import (
	"OmniView/internal/updater"
	upd "OmniView/internal/updater"
	"context"
)

// UpdaterService provides update checking and application functionality
// wrapped around the pure updater package functions.
type UpdaterService struct {
	currentVersion string
}

// NewUpdaterService creates a new UpdaterService with the given current version.
func NewUpdaterService(currentVersion string) *UpdaterService {
	return &UpdaterService{
		currentVersion: currentVersion,
	}
}

// CheckForUpdate checks the GitHub releases for a newer version.
// Returns (*updater.UpdateInfo, nil) when an update is available.
// Returns (nil, nil) when no update is needed or in development mode.
// Returns (nil, error) on failure.
func (s *UpdaterService) CheckForUpdate(ctx context.Context) (*upd.UpdateInfo, error) {
	return upd.CheckForUpdate(ctx, s.currentVersion)
}

// ApplyUpdate downloads, verifies, and applies the update.
// The progressFn callback is invoked at each stage with descriptive strings:
// "Downloading...", "Verifying checksum...", "Extracting...", "Restarting...".
func (s *UpdaterService) ApplyUpdate(ctx context.Context, info *updater.UpdateInfo, progressFn func(string)) error {
	return updater.DownloadAndApply(ctx, info, progressFn)
}

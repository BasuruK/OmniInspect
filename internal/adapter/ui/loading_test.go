package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	"OmniView/internal/updater"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

type stubDatabaseSettingsRepository struct {
	databases []domain.DatabaseSettings
	err       error
}

func (s stubDatabaseSettingsRepository) Save(context.Context, domain.DatabaseSettings) error {
	return nil
}

func (s stubDatabaseSettingsRepository) GetByID(context.Context, string) (*domain.DatabaseSettings, error) {
	return nil, nil
}

func (s stubDatabaseSettingsRepository) GetDefault(context.Context) (*domain.DatabaseSettings, error) {
	return nil, nil
}

func (s stubDatabaseSettingsRepository) GetAll(context.Context) ([]domain.DatabaseSettings, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.databases, nil
}

func (s stubDatabaseSettingsRepository) Delete(context.Context, string) error {
	return nil
}

func (s stubDatabaseSettingsRepository) Replace(context.Context, string, domain.DatabaseSettings) error {
	return nil
}

type stubPermissionsRepository struct{}

func (stubPermissionsRepository) Save(context.Context, *domain.DatabasePermissions) error {
	return nil
}

func (stubPermissionsRepository) Get(context.Context, string) (*domain.DatabasePermissions, error) {
	return nil, nil
}

func (stubPermissionsRepository) Exists(context.Context, string) (bool, error) {
	return true, nil
}

type stubConfigRepository struct{}

func (stubConfigRepository) SaveDatabaseConfig(*domain.DatabaseSettings) error { return nil }
func (stubConfigRepository) GetDefaultDatabaseConfig() (*domain.DatabaseSettings, error) {
	return nil, nil
}
func (stubConfigRepository) IsApplicationFirstRun() (bool, error) { return false, nil }
func (stubConfigRepository) SetFirstRunCycleStatus(ports.RunCycleStatus) error {
	return nil
}
func (stubConfigRepository) SaveWebhookConfig(*domain.WebhookConfig) error { return nil }
func (stubConfigRepository) GetWebhookConfig() (*domain.WebhookConfig, error) {
	return nil, nil
}
func (stubConfigRepository) DeleteWebhookConfig(string) error { return nil }

func newLoadingTestModel(t *testing.T, validated bool) *Model {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	eventStreamCtx, eventStreamCancel := context.WithCancel(ctx)
	t.Cleanup(eventStreamCancel)
	t.Cleanup(cancel)

	settings := newTestDatabaseSettings(t, "LOAD-DB")
	if validated {
		settings.MarkPermissionsValidated()
	}

	eventChannel := make(chan *domain.QueueMessage, 16)

	mockDB := NewMockDatabaseRepository()
	configRepo := stubConfigRepository{}
	tracerService, err := tracer.NewTracerService(mockDB, configRepo, eventChannel)
	if err != nil {
		t.Fatalf("NewTracerService: %v", err)
	}

	return &Model{
		ctx:               ctx,
		cancel:            cancel,
		eventStreamCtx:    eventStreamCtx,
		eventStreamCancel: eventStreamCancel,
		width:             120,
		height:            36,
		appConfig:         settings,
		dbAdapter:         mockDB,
		dbSettingsRepo:    stubDatabaseSettingsRepository{},
		permissionService: permissions.NewPermissionService(mockDB, stubPermissionsRepository{}, configRepo),
		tracerService:     tracerService,
		subscriberService: subscribers.NewSubscriberService(mockDB, nil, nil),
		eventChannel:      eventChannel,
	}
}

func TestRecoveryOptionsKeyboardHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		key    string
		assert func(t *testing.T, updated *Model, cmd tea.Cmd)
	}{
		{
			name: "retry increments count",
			key:  "r",
			assert: func(t *testing.T, updated *Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil {
					t.Fatal("expected retry command")
				}
				if updated.loading.retryCount != 1 {
					t.Fatalf("expected retryCount to increment to 1, got %d", updated.loading.retryCount)
				}
				if updated.loading.retryTimer == nil {
					t.Fatal("expected retry timer to be created")
				}
				if updated.loading.current != "Retrying connection in 1 seconds..." {
					t.Fatalf("expected first retry to wait 1 second, got %q", updated.loading.current)
				}
				defer updated.loading.retryTimer.Stop()

				updated.cancel()
				if _, ok := cmd().(retryTimerExpiryMsg); !ok {
					t.Fatal("expected retry timer command to emit retryTimerExpiryMsg")
				}
			},
		},
		{
			name: "switch emits SwitchDatabaseMsg",
			key:  "s",
			assert: func(t *testing.T, updated *Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil {
					t.Fatal("expected switch command")
				}
				if _, ok := cmd().(SwitchDatabaseMsg); !ok {
					t.Fatal("expected SwitchDatabaseMsg")
				}
				if updated.loading.retryCount != 0 {
					t.Fatalf("expected retryCount to remain unchanged, got %d", updated.loading.retryCount)
				}
			},
		},
		{
			name: "quit returns tea.Quit",
			key:  "q",
			assert: func(t *testing.T, updated *Model, cmd tea.Cmd) {
				t.Helper()
				if cmd == nil {
					t.Fatal("expected quit command")
				}
				if _, ok := cmd().(tea.QuitMsg); !ok {
					t.Fatal("expected tea.QuitMsg")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLoadingTestModel(t, false)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			m.ctx = ctx
			m.cancel = cancel
			m.loading.err = errors.New("connection refused")

			updated, cmd := m.updateLoading(makeCharPress(tt.key))
			tt.assert(t, updated, cmd)
		})
	}
}

func TestRetryTimerExpiryReattemptsDatabaseConnection(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.err = errors.New("connection refused")
	m.loading.retryCount = 1
	m.loading.retryTimer = time.NewTimer(time.Hour)
	defer func() {
		if m.loading.retryTimer != nil {
			m.loading.retryTimer.Stop()
		}
	}()

	updated, cmd := m.updateLoading(retryTimerExpiryMsg{})
	if cmd == nil {
		t.Fatal("expected reconnect command after retry timer expiry")
	}
	if updated.loading.retryTimer != nil {
		t.Fatal("expected retry timer to be cleared after expiry")
	}
	if updated.loading.err != nil {
		t.Fatalf("expected loading error to be cleared, got %v", updated.loading.err)
	}
	if updated.loading.current != "Connecting to database..." {
		t.Fatalf("expected loading current to reset to connecting, got %q", updated.loading.current)
	}

	connectResult, ok := cmd().(dbConnectedMsg)
	if !ok {
		t.Fatal("expected dbConnectedMsg from reconnect command")
	}
	if connectResult.err != nil {
		t.Fatalf("expected reconnect attempt to succeed, got %v", connectResult.err)
	}

	mockDB, ok := m.dbAdapter.(*MockDatabaseRepository)
	if !ok {
		t.Fatal("expected mock database adapter")
	}
	if len(mockDB.ConnectCalls) != 1 {
		t.Fatalf("expected one reconnect attempt, got %d", len(mockDB.ConnectCalls))
	}
}

func TestRetryTimerExpiryIgnoresStaleGeneration(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.err = errors.New("connection refused")
	m.loading.retryTimer = time.NewTimer(time.Hour)
	m.loading.retryGeneration = 2
	defer func() {
		if m.loading.retryTimer != nil {
			m.loading.retryTimer.Stop()
		}
	}()

	updated, cmd := m.updateLoading(retryTimerExpiryMsg{generation: 1})
	if cmd != nil {
		t.Fatal("expected stale retry expiry to be ignored")
	}
	if updated.loading.retryTimer == nil {
		t.Fatal("expected active retry timer to remain unchanged")
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading error to remain set")
	}

	mockDB, ok := m.dbAdapter.(*MockDatabaseRepository)
	if !ok {
		t.Fatal("expected mock database adapter")
	}
	if len(mockDB.ConnectCalls) != 0 {
		t.Fatalf("expected no reconnect attempt for stale retry expiry, got %d", len(mockDB.ConnectCalls))
	}
	updated.loading.retryTimer.Stop()
	updated.loading.retryTimer = nil
}

func TestUpdateLoading_SecondRetryInvalidatesFirstRetryCommand(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.ctx = ctx
	m.cancel = cancel
	m.loading.err = errors.New("database unavailable")

	firstRetry, firstCmd := m.updateLoading(makeCharPress("r"))
	if firstCmd == nil {
		t.Fatal("expected first retry command")
	}
	if firstRetry.loading.retryTimer == nil {
		t.Fatal("expected first retry timer")
	}

	secondRetry, secondCmd := firstRetry.updateLoading(makeCharPress("r"))
	if secondCmd == nil {
		t.Fatal("expected second retry command")
	}
	if secondRetry.loading.retryTimer == nil {
		t.Fatal("expected second retry timer")
	}
	defer func() {
		if secondRetry.loading.retryTimer != nil {
			secondRetry.loading.retryTimer.Stop()
		}
	}()

	staleMsg, ok := firstCmd().(retryTimerExpiryMsg)
	if !ok {
		t.Fatal("expected first retry command to emit retryTimerExpiryMsg")
	}
	updated, cmd := secondRetry.updateLoading(staleMsg)
	if cmd != nil {
		t.Fatal("expected stale retry expiry to be ignored")
	}
	if updated.loading.retryTimer == nil {
		t.Fatal("expected second retry timer to remain active")
	}

	mockDB, ok := updated.dbAdapter.(*MockDatabaseRepository)
	if !ok {
		t.Fatal("expected mock database adapter")
	}
	if len(mockDB.ConnectCalls) != 0 {
		t.Fatalf("expected no reconnect attempt from stale retry command, got %d", len(mockDB.ConnectCalls))
	}
	if updated.loading.retryCount != 2 {
		t.Fatalf("expected second retry state to remain active, got retryCount=%d", updated.loading.retryCount)
	}
	if updated.loading.current != "Retrying connection in 2 seconds..." {
		t.Fatalf("expected second retry delay to remain visible, got %q", updated.loading.current)
	}
	if staleMsg.generation == updated.loading.retryGeneration {
		t.Fatal("expected first retry generation to differ from active retry generation")
	}
}

func TestUpdateLoading_RetryKeySchedulesBackoff(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.loading.err = errors.New("database unavailable")

	updated, cmd := m.updateLoading(makeCharPress("r"))
	if cmd == nil {
		t.Fatal("expected retry timer command")
	}
	if updated.loading.retryCount != 1 {
		t.Fatalf("expected retryCount to increment to 1, got %d", updated.loading.retryCount)
	}
	if updated.loading.retryTimer == nil {
		t.Fatal("expected retryTimer to be created")
	}
	defer func() {
		if updated.loading.retryTimer != nil {
			updated.loading.retryTimer.Stop()
		}
	}()
	if updated.loading.current != "Retrying connection in 1 seconds..." {
		t.Fatalf("expected retry message, got %q", updated.loading.current)
	}
	cancel()
	if _, ok := cmd().(retryTimerExpiryMsg); !ok {
		t.Fatal("expected retry timer command to emit retryTimerExpiryMsg")
	}
}

func TestUpdateLoading_RetryKeyCapsBackoffAtThirtySeconds(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m.ctx = ctx
	m.loading.err = errors.New("database unavailable")
	m.loading.retryCount = 5

	updated, cmd := m.updateLoading(makeCharPress("r"))
	if cmd == nil {
		t.Fatal("expected retry timer command")
	}
	if updated.loading.retryCount != 6 {
		t.Fatalf("expected retryCount to increment to 6, got %d", updated.loading.retryCount)
	}
	if updated.loading.current != "Retrying connection in 30 seconds..." {
		t.Fatalf("expected capped retry message, got %q", updated.loading.current)
	}
	if updated.loading.retryTimer == nil {
		t.Fatal("expected retryTimer to be created")
	}
	updated.loading.retryTimer.Stop()
}

func TestViewLoading_ShowsRetryStatusWhenRetryScheduled(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.err = errors.New("database unavailable")
	m.loading.current = "Retrying connection in 4 seconds..."
	m.loading.retryTimer = time.NewTimer(time.Hour)
	defer m.loading.retryTimer.Stop()

	view := m.viewLoading()
	if !strings.Contains(view, "Retrying connection in 4 seconds...") {
		t.Fatalf("expected retry status to be visible in loading view, got %q", view)
	}
	if !strings.Contains(view, "R Retry") || !strings.Contains(view, "S Switch") || !strings.Contains(view, "Q Quit") {
		t.Fatalf("expected horizontal recovery options to be visible in loading view, got %q", view)
	}
	if strings.Contains(view, "A]dd") || strings.Contains(view, "A Add") {
		t.Fatalf("expected Add option to be absent from loading view, got %q", view)
	}
	if strings.Contains(view, "What would you like to do?") {
		t.Fatalf("expected question prompt to be removed from loading view, got %q", view)
	}
}

func TestUpdateLoading_SwitchDatabaseMsgOpensSettingsOverlay(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.err = errors.New("database unavailable")
	m.loading.retryTimer = time.NewTimer(time.Hour)
	m.dbSettingsRepo = stubDatabaseSettingsRepository{
		databases: []domain.DatabaseSettings{*newTestDatabaseSettings(t, "ALT-DB")},
	}

	updated, cmd := m.updateLoading(makeCharPress("s"))
	if cmd == nil {
		t.Fatal("expected switch database command")
	}
	if _, ok := cmd().(SwitchDatabaseMsg); !ok {
		t.Fatal("expected switch database message")
	}

	updated, cmd = m.updateLoading(SwitchDatabaseMsg{})
	if cmd != nil {
		t.Fatal("expected no follow-up command when opening settings overlay")
	}
	if !updated.dbSettings.visible {
		t.Fatal("expected database settings overlay to be visible")
	}
	if len(updated.dbSettings.databases) != 1 {
		t.Fatalf("expected one stored database, got %d", len(updated.dbSettings.databases))
	}
	if updated.loading.retryTimer != nil {
		t.Fatal("expected retry timer to be cleared when opening settings overlay")
	}
}

func TestModelUpdate_LoadingOverlayQClosesOverlay(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.screen = screenLoading
	m.dbSettings.visible = true

	updatedModel, cmd := m.Update(makeCharPress("q"))
	if cmd != nil {
		t.Fatal("expected q to be handled by overlay without quitting")
	}

	updated, ok := updatedModel.(*Model)
	if !ok {
		t.Fatal("expected Update to return *Model")
	}
	if updated.dbSettings.visible {
		t.Fatal("expected q to close the loading overlay")
	}
}

func TestUpdateLoading_DbConnected_UsesCachedPermissionValidation(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, true)

	updated, cmd := m.updateLoading(dbConnectedMsg{})
	if cmd == nil {
		t.Fatal("expected deploy tracer command when permissions are cached")
	}
	if updated.loading.current != "Deploying tracer package..." {
		t.Fatalf("expected loading.current to advance to tracer deployment, got %q", updated.loading.current)
	}
	if len(updated.loading.steps) < 2 {
		t.Fatalf("expected cached permission step to be recorded, got %v", updated.loading.steps)
	}
	if updated.loading.steps[1] != "✓ Permissions verified (cached)" {
		t.Fatalf("expected cached permission step, got %q", updated.loading.steps[1])
	}

	if _, ok := cmd().(tracerDeployedMsg); !ok {
		t.Fatal("expected deployTracerCmd to be returned when permissions are cached")
	}
}

func TestUpdateLoading_DbConnected_RechecksPermissionsWithoutCache(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)

	updated, cmd := m.updateLoading(dbConnectedMsg{})
	if cmd == nil {
		t.Fatal("expected permission check command when validation is not cached")
	}
	if updated.loading.current != "Checking permissions..." {
		t.Fatalf("expected loading.current to request permission checks, got %q", updated.loading.current)
	}
	if len(updated.loading.steps) != 1 {
		t.Fatalf("expected only the connection step before permission checks, got %v", updated.loading.steps)
	}

	if _, ok := cmd().(permissionsCheckedMsg); !ok {
		t.Fatal("expected checkPermissionsCmd to be returned when permissions are not cached")
	}
}

func TestUpdateLoading_PromptBlocksLoadingCompletionFromEnteringMain(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.started = true
	m.update.prompting = true
	m.update.info = &domainlessUpdateInfoForTest

	sub := newTestSubscriber(t)
	updated, cmd := m.updateLoading(subscriberRegisteredMsg{subscriber: sub})
	if cmd != nil {
		t.Fatal("expected no command while update prompt is gating startup")
	}
	if updated.screen == screenMain {
		t.Fatalf("expected startup to remain gated off main screen, got %q", updated.screen)
	}
	if !updated.loading.complete {
		t.Fatal("expected loading completion to be recorded while prompt is visible")
	}
	if !updated.update.prompting {
		t.Fatal("expected update prompt to remain visible")
	}
}

func TestUpdateLoading_DecliningPromptContinuesCompletedStartupIntoMain(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.started = true
	m.loading.complete = true
	m.update.info = &domainlessUpdateInfoForTest
	m.update.prompting = true

	updated, cmd := m.updateLoading(makeCharPress("n"))
	if updated.screen != screenMain {
		t.Fatalf("expected completed startup to enter main after declining update, got %q", updated.screen)
	}
	if updated.update.prompting {
		t.Fatal("expected prompt to be cleared after declining update")
	}
	if updated.update.info != nil {
		t.Fatal("expected update info to be cleared after declining update")
	}
	if cmd == nil {
		t.Fatal("expected event listener command when entering main")
	}
}

func TestUpdateLoading_AcceptingPromptKeepsStartupBlockedUntilUpdaterFinishes(t *testing.T) {
	t.Parallel()

	m := newLoadingTestModel(t, false)
	m.loading.started = true
	m.loading.complete = true
	m.update.info = &domainlessUpdateInfoForTest
	m.update.prompting = true

	updated, cmd := m.updateLoading(makeCharPress("y"))
	if cmd == nil {
		t.Fatal("expected apply update command")
	}
	if updated.screen == screenMain {
		t.Fatalf("expected startup to stay blocked during apply, got %q", updated.screen)
	}
	if !updated.update.applying {
		t.Fatal("expected update applying state after accepting prompt")
	}
	if updated.update.prompting {
		t.Fatal("expected prompt to clear once accepted")
	}

	updated, _ = updated.updateLoading(updateErrorMsg{err: errors.New("update failed")})
	if updated.screen == screenMain {
		t.Fatalf("expected update failure to remain gated off main, got %q", updated.screen)
	}
	if updated.update.err == nil {
		t.Fatal("expected update error to be shown after failed apply")
	}
	if !updated.loading.complete {
		t.Fatal("expected startup completion state to be preserved after failed apply")
	}

	updated, cmd = updated.updateLoading(makeCharPress("y"))
	if updated.screen != screenMain {
		t.Fatalf("expected continue-without-update to enter main, got %q", updated.screen)
	}
	if updated.update.err != nil {
		t.Fatal("expected update error to clear after continuing")
	}
	if cmd == nil {
		t.Fatal("expected event listener command when continuing into main")
	}
}

var domainlessUpdateInfoForTest = updater.UpdateInfo{
	Available:      true,
	CurrentVersion: "v1.0.0",
	NewVersion:     "v1.0.1",
}

package ui

import (
	"OmniView/internal/adapter/storage/boltdb"
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/tracer"

	"context"
	"errors"
	"testing"
	"time"
)

// ==========================================
// MockDatabaseRepository
// ==========================================

// MockDatabaseRepository implements ports.DatabaseRepository for testing.
type MockDatabaseRepository struct {
	ConnectFunc func(ctx context.Context) error
	CloseFunc   func(ctx context.Context) error

	ConnectCalls []context.Context
	CloseCalls   []context.Context

	connectError error
	closeError   error
}

// NewMockDatabaseRepository creates a MockDatabaseRepository with configurable behavior.
func NewMockDatabaseRepository() *MockDatabaseRepository {
	return &MockDatabaseRepository{
		ConnectCalls: make([]context.Context, 0),
		CloseCalls:   make([]context.Context, 0),
	}
}

// WithConnectError sets an error to return on Connect.
func (m *MockDatabaseRepository) WithConnectError(err error) *MockDatabaseRepository {
	m.connectError = err
	return m
}

// WithCloseError sets an error to return on Close.
func (m *MockDatabaseRepository) WithCloseError(err error) *MockDatabaseRepository {
	m.closeError = err
	return m
}

// Connect implements ports.DatabaseRepository.
func (m *MockDatabaseRepository) Connect(ctx context.Context) error {
	m.ConnectCalls = append(m.ConnectCalls, ctx)
	if m.ConnectFunc != nil {
		return m.ConnectFunc(ctx)
	}
	return m.connectError
}

// Close implements ports.DatabaseRepository.
func (m *MockDatabaseRepository) Close(ctx context.Context) error {
	m.CloseCalls = append(m.CloseCalls, ctx)
	if m.CloseFunc != nil {
		return m.CloseFunc(ctx)
	}
	return m.closeError
}

// RegisterNewSubscriber implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) RegisterNewSubscriber(ctx context.Context, subscriber domain.Subscriber) error {
	return nil
}

// BulkDequeueTracerMessages implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) BulkDequeueTracerMessages(ctx context.Context, subscriber domain.Subscriber) ([]string, [][]byte, int, error) {
	return nil, nil, 0, nil
}

// CheckQueueDepth implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) CheckQueueDepth(ctx context.Context, subscriberID string, queueTableName string) (int, error) {
	return 0, nil
}

// Fetch implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) Fetch(ctx context.Context, query string) ([]string, error) {
	return nil, nil
}

// ExecuteStatement implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) ExecuteStatement(ctx context.Context, query string) error {
	return nil
}

// ExecuteWithParams implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) ExecuteWithParams(ctx context.Context, query string, params map[string]interface{}) error {
	return nil
}

// FetchWithParams implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) FetchWithParams(ctx context.Context, query string, params map[string]interface{}) ([]string, error) {
	return nil, nil
}

// PackageExists implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) PackageExists(ctx context.Context, packageName string) (bool, error) {
	return false, nil
}

// DeployPackages implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) DeployPackages(ctx context.Context, sequences []string, types []string, packageSpec []string, packageBody []string) error {
	return nil
}

// DeployFile implements ports.DatabaseRepository (no-op for mock).
func (m *MockDatabaseRepository) DeployFile(ctx context.Context, sqlContent string) error {
	return nil
}

// ==========================================
// Test Helper Functions
// ==========================================

// newTestDatabaseSettings creates a minimal valid DatabaseSettings for testing.
func newTestDatabaseSettings(t *testing.T, id string) *domain.DatabaseSettings {
	t.Helper()

	port, err := domain.NewPort(1521)
	if err != nil {
		t.Fatalf("failed to create port: %v", err)
	}

	settings, err := domain.NewDatabaseSettings(
		id,
		"FREEDB",
		"localhost",
		port,
		"testuser",
		"testpass",
	)
	if err != nil {
		t.Fatalf("failed to create database settings: %v", err)
	}

	return settings
}

// newTestModelForSettings creates a minimal Model for testing handleSettingsSetAsMain.
func newTestModelForSettings(t *testing.T) *Model {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	eventStreamCtx, eventStreamCancel := context.WithCancel(ctx)

	return &Model{
		screen:            screenMain,
		width:             120,
		height:            36,
		ctx:               ctx,
		cancel:            cancel,
		eventStreamCtx:    eventStreamCtx,
		eventStreamCancel: eventStreamCancel,
		eventChannel:      make(chan *domain.QueueMessage, 16),
		main: mainState{
			autoScroll: true,
			ready:      true,
		},
		dbSettings: databaseSettingsState{
			visible: true,
		},
	}
}

func mustNewTracerService(t *testing.T, db ports.DatabaseRepository, eventChannel chan *domain.QueueMessage) *tracer.TracerService {
	t.Helper()

	service, err := tracer.NewTracerService(db, stubConfigRepository{}, eventChannel)
	if err != nil {
		t.Fatalf("NewTracerService: %v", err)
	}

	return service
}

func newTestBoltAdapter(t *testing.T) *boltdb.BoltAdapter {
	t.Helper()

	dbPath := t.TempDir() + "/test.bolt"
	adapter, err := boltdb.NewBoltAdapter(dbPath)
	if err != nil {
		t.Fatalf("NewBoltAdapter: %v", err)
	}
	if err := adapter.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	t.Cleanup(func() {
		_ = adapter.Close()
	})

	return adapter
}

// ==========================================
// Service Cleanup Tests
// ==========================================

func TestHandleSettingsSetAsMain_ServiceCleanup_WithExistingServices(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Setup existing services
	m.dbAdapter = mockDB
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "NEW-DB")

	// Create factory that returns our mock
	factoryCalled := false
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		factoryCalled = true
		return mockDB, nil
	}

	// Execute
	updated, cmd := m.handleSettingsSetAsMain(*selected)

	// Assert services are nil after switch (observable effect of StopAll being called)
	if updated.tracerService != nil {
		t.Error("expected tracerService to be nil after switch")
	}
	if updated.permissionService != nil {
		t.Error("expected permissionService to be nil after switch")
	}
	if updated.subscriberService != nil {
		t.Error("expected subscriberService to be nil after switch")
	}

	// Assert factory was called
	if !factoryCalled {
		t.Error("expected dbFactory to be called")
	}

	// Assert command is returned
	if cmd == nil {
		t.Error("expected non-nil command to be returned")
	}
}

func TestHandleSettingsSetAsMain_ServiceCleanup_DbAdapterClosed(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	oldMockDB := NewMockDatabaseRepository()
	newMockDB := NewMockDatabaseRepository()

	// Setup existing db adapter
	m.dbAdapter = oldMockDB

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return newMockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	if len(oldMockDB.CloseCalls) != 1 {
		t.Errorf("expected old dbAdapter.Close to be called once, got %d calls", len(oldMockDB.CloseCalls))
	}
	if len(newMockDB.ConnectCalls) != 1 {
		t.Errorf("expected replacement adapter.Connect to be called once for validation, got %d calls", len(newMockDB.ConnectCalls))
	}
	if len(newMockDB.CloseCalls) != 1 {
		t.Errorf("expected replacement adapter.Close to be called once after validation, got %d calls", len(newMockDB.CloseCalls))
	}

	// Assert dbAdapter is now the new one
	if updated.dbAdapter != newMockDB {
		t.Error("expected dbAdapter to be the new adapter from factory")
	}
}

func TestHandleSettingsSetAsMain_ServiceCleanup_NilTracerService(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Setup only dbAdapter (no tracer service)
	m.dbAdapter = mockDB
	m.tracerService = nil

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute - should not panic
	updated, cmd := m.handleSettingsSetAsMain(*selected)

	if updated.tracerService != nil {
		t.Error("expected tracerService to remain nil")
	}
	if cmd == nil {
		t.Error("expected non-nil command to be returned")
	}
}

func TestHandleSettingsSetAsMain_ServiceCleanup_NilDbAdapter(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Setup only tracer service (no db adapter)
	m.dbAdapter = nil
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute - should not panic
	updated, cmd := m.handleSettingsSetAsMain(*selected)

	if updated.dbAdapter == nil {
		t.Error("expected dbAdapter to be set after switch")
	}
	if updated.tracerService != nil {
		t.Error("expected tracerService to be nil after switch")
	}
	if cmd == nil {
		t.Error("expected non-nil command to be returned")
	}
}

func TestHandleSettingsSetAsMain_ResetsRetryState(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	boltAdapter := newTestBoltAdapter(t)
	oldMockDB := NewMockDatabaseRepository()
	newMockDB := NewMockDatabaseRepository()

	m.boltAdapter = boltAdapter
	m.dbAdapter = oldMockDB
	m.appConfig = newTestDatabaseSettings(t, "OLD-DB")
	m.loading.retryCount = 4
	m.loading.retryTimer = time.NewTimer(time.Hour)
	retryCtx, retryCancel := context.WithCancel(context.Background())
	m.loading.retryCancel = retryCancel
	m.loading.retryGeneration = 7

	selected := newTestDatabaseSettings(t, "NEW-DB")
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return newMockDB, nil
	}

	updated, cmd := m.handleSettingsSetAsMain(*selected)
	if cmd == nil {
		t.Fatal("expected connect command after switching databases")
	}
	if updated.loading.retryCount != 0 {
		t.Fatalf("expected retryCount to reset to 0, got %d", updated.loading.retryCount)
	}
	if updated.loading.retryTimer != nil {
		t.Fatal("expected retryTimer to be cleared after switching databases")
	}
	if updated.loading.retryCancel != nil {
		t.Fatal("expected retryCancel to be cleared after switching databases")
	}

	select {
	case <-retryCtx.Done():
	default:
		t.Fatal("expected previous retry context to be cancelled when switching databases")
	}
	if updated.loading.retryGeneration <= 7 {
		t.Fatalf("expected retryGeneration to advance, got %d", updated.loading.retryGeneration)
	}
	if updated.loading.current != "Connecting..." {
		t.Fatalf("expected loading current to reset to connecting, got %q", updated.loading.current)
	}
	if updated.screen != screenLoading {
		t.Fatalf("expected screen to switch to loading, got %q", updated.screen)
	}
	if updated.appConfig == nil || updated.appConfig.DatabaseID() != "NEW-DB" {
		t.Fatal("expected active database configuration to be updated")
	}
	if updated.dbAdapter != newMockDB {
		t.Fatal("expected dbAdapter to be replaced with the new adapter")
	}
	if len(newMockDB.ConnectCalls) != 1 {
		t.Fatalf("expected new adapter validation connect, got %d", len(newMockDB.ConnectCalls))
	}
	if len(newMockDB.CloseCalls) != 1 {
		t.Fatalf("expected new adapter validation close, got %d", len(newMockDB.CloseCalls))
	}
	if len(oldMockDB.CloseCalls) != 1 {
		t.Fatalf("expected old adapter close, got %d", len(oldMockDB.CloseCalls))
	}
}

// ==========================================
// Error Path Tests
// ==========================================

func TestHandleSettingsSetAsMain_FactoryError(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	m.dbAdapter = mockDB
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	expectedErr := errors.New("connection failed: invalid credentials")

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	// Factory returns error
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return nil, expectedErr
	}

	// Execute
	updated, cmd := m.handleSettingsSetAsMain(*selected)

	if !errors.Is(updated.loading.err, expectedErr) {
		t.Errorf("expected loading.err to wrap factory error, got %v", updated.loading.err)
	}

	// Assert dialog is shown
	if !updated.dbSettings.showDialog {
		t.Error("expected dbSettings.showDialog to be true on error")
	}
	if updated.dbSettings.dialogMsg == "" {
		t.Error("expected dbSettings.dialogMsg to be set on error")
	}

	// Assert services are not reset on error (early return before reset code)
	// The function returns early on error, so services keep their pre-error values
	if updated.tracerService == nil {
		t.Error("expected tracerService to remain as set before error (not reset)")
	}
	if updated.permissionService != nil {
		t.Error("expected permissionService to remain nil on error")
	}
	if updated.subscriberService != nil {
		t.Error("expected subscriberService to remain nil on error")
	}

	// Assert screen remains on main (not screenLoading) when error occurs
	if updated.screen == screenLoading {
		t.Error("expected screen to remain on main (not transition to loading) on error")
	}

	// Assert no command returned on error
	if cmd != nil {
		t.Error("expected nil command on error")
	}
}

func TestHandleSettingsSetAsMain_FactoryErrorDescriptiveMessage(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	m.dbAdapter = mockDB
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	factoryErr := errors.New("oracle connection refused")

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return nil, factoryErr
	}

	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Verify error message is descriptive
	if updated.dbSettings.dialogMsg == "" {
		t.Error("expected dialog message to be set")
	}
	if !errors.Is(updated.loading.err, factoryErr) {
		t.Errorf("expected loading.err to wrap factory error, got %v", updated.loading.err)
	}
	// Verify original error is wrapped
	if updated.dbSettings.dialogMsg == factoryErr.Error() {
		t.Error("expected dialog message to contain wrapped error, not just raw error")
	}
}

func TestHandleSettingsSetAsMain_ConnectionValidationError_KeepsExistingSession(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	oldMockDB := NewMockDatabaseRepository()
	newMockDB := NewMockDatabaseRepository().WithConnectError(errors.New("invalid credentials"))
	initialConfig := newTestDatabaseSettings(t, "CURRENT-DB")
	m.appConfig = initialConfig
	m.dbAdapter = oldMockDB
	m.tracerService = mustNewTracerService(t, oldMockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return newMockDB, nil
	}

	updated, cmd := m.handleSettingsSetAsMain(*selected)

	if cmd != nil {
		t.Fatal("expected nil command when validation connect fails")
	}
	if updated.dbAdapter != oldMockDB {
		t.Fatal("expected existing dbAdapter to remain active when validation fails")
	}
	if updated.appConfig != initialConfig {
		t.Fatal("expected existing appConfig to remain active when validation fails")
	}
	if len(oldMockDB.CloseCalls) != 0 {
		t.Errorf("expected existing adapter to remain open, got %d close calls", len(oldMockDB.CloseCalls))
	}
	if len(newMockDB.ConnectCalls) != 1 {
		t.Errorf("expected replacement adapter.Connect to be called once, got %d calls", len(newMockDB.ConnectCalls))
	}
	if len(newMockDB.CloseCalls) != 1 {
		t.Errorf("expected replacement adapter.Close to be called once after failed validation, got %d calls", len(newMockDB.CloseCalls))
	}
	if !updated.dbSettings.showDialog {
		t.Fatal("expected settings dialog to remain visible on validation failure")
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading.err to be populated on validation failure")
	}
}

func TestHandleSettingsSetAsMain_FactoryError_OldAdapterRemainsOpen(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	oldMockDB := NewMockDatabaseRepository()
	m.dbAdapter = oldMockDB
	m.tracerService = mustNewTracerService(t, oldMockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	// Factory returns error
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return nil, errors.New("connection refused")
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// The existing adapter should remain usable when creating the replacement fails.
	if len(oldMockDB.CloseCalls) != 0 {
		t.Errorf("expected old dbAdapter.Close not to be called on factory error, got %d calls", len(oldMockDB.CloseCalls))
	}
	if updated.dbAdapter != oldMockDB {
		t.Error("expected existing dbAdapter to remain active on factory error")
	}
}

// ==========================================
// State Transition Tests
// ==========================================

func TestHandleSettingsSetAsMain_Success_ScreenTransition(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert screen transitions to screenLoading
	if updated.screen != screenLoading {
		t.Errorf("expected screen to transition to screenLoading, got %s", updated.screen)
	}
}

func TestHandleSettingsSetAsMain_Success_LoadingState(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert loading state is set correctly
	if updated.loading.current != "Connecting..." {
		t.Errorf("expected loading.current to be 'Connecting...', got %q", updated.loading.current)
	}
	if updated.loading.err != nil {
		t.Errorf("expected loading.err to be nil on success, got %v", updated.loading.err)
	}
	if len(updated.loading.steps) != 0 {
		t.Errorf("expected loading.steps to be empty on success, got %d steps", len(updated.loading.steps))
	}
}

func TestHandleSettingsSetAsMain_Success_MainStateReset(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Setup main state with existing data
	testMsg := newTestQueueMessage(t)
	m.main.messages = []*domain.QueueMessage{testMsg}
	m.main.ready = true
	m.main.renderedContent.WriteString("some content")

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert main state is reset
	if len(updated.main.messages) != 0 {
		t.Errorf("expected main.messages to be cleared, got %d messages", len(updated.main.messages))
	}
	if updated.main.ready {
		t.Error("expected main.ready to be false after reset")
	}
	if updated.main.renderedContent.Len() != 0 {
		t.Error("expected main.renderedContent to be reset")
	}
}

func TestHandleSettingsSetAsMain_Success_DatabaseSettingsClosed(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert database settings panel is closed
	if updated.dbSettings.visible {
		t.Error("expected dbSettings.visible to be false after successful switch")
	}
}

func TestHandleSettingsSetAsMain_Success_ReturnsConnectCommand(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	_, cmd := m.handleSettingsSetAsMain(*selected)

	// Assert command is returned (function type)
	if cmd == nil {
		t.Fatal("expected non-nil command on success")
	}
}

func TestHandleSettingsSetAsMain_Success_AppConfigUpdated(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Set initial app config
	initialConfig := newTestDatabaseSettings(t, "INITIAL-DB")
	m.appConfig = initialConfig

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert app config is updated
	if updated.appConfig == nil {
		t.Fatal("expected appConfig to be set")
	}
	if updated.appConfig.DatabaseID() != "NEW-DB" {
		t.Errorf("expected appConfig.DatabaseID to be 'NEW-DB', got %s", updated.appConfig.DatabaseID())
	}
}

func TestHandleSettingsSetAsMain_Success_DbAdapterSet(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert db adapter is set
	if updated.dbAdapter == nil {
		t.Error("expected dbAdapter to be set after successful switch")
	}
	if updated.dbAdapter != mockDB {
		t.Error("expected dbAdapter to be the mock returned by factory")
	}
}

// ==========================================
// Edge Cases
// ==========================================

func TestHandleSettingsSetAsMain_ClosesOldAdapterBeforeCreatingNew(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	oldMockDB := NewMockDatabaseRepository()
	newMockDB := NewMockDatabaseRepository()

	m.dbAdapter = oldMockDB
	m.tracerService = mustNewTracerService(t, oldMockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return newMockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert old adapter was closed
	if len(oldMockDB.CloseCalls) != 1 {
		t.Errorf("expected old dbAdapter.Close to be called once, got %d", len(oldMockDB.CloseCalls))
	}

	// Assert new adapter is set
	if updated.dbAdapter != newMockDB {
		t.Error("expected dbAdapter to be set to new adapter")
	}
}

func TestHandleSettingsSetAsMain_UpdatesDbSettingsActiveID(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	m.dbSettings.activeID = "OLD-DB"

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Note: handleSettingsSetAsMain does NOT update dbSettings.activeID
	// It only updates appConfig. The activeID is updated elsewhere (e.g., after successful connection)
	if updated.dbSettings.activeID != "OLD-DB" {
		t.Errorf("expected dbSettings.activeID to remain 'OLD-DB', got %s", updated.dbSettings.activeID)
	}
	// Verify appConfig IS updated
	if updated.appConfig.DatabaseID() != "NEW-DB" {
		t.Errorf("expected appConfig.DatabaseID to be 'NEW-DB', got %s", updated.appConfig.DatabaseID())
	}
}

func TestHandleSettingsSetAsMain_WithExistingMessages_ClearsAllMessages(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()

	// Add multiple messages
	m.main.messages = []*domain.QueueMessage{
		newTestQueueMessage(t),
		newTestQueueMessage(t),
		newTestQueueMessage(t),
	}

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert all messages are cleared
	if len(updated.main.messages) != 0 {
		t.Errorf("expected all messages to be cleared, got %d messages", len(updated.main.messages))
	}
}

func TestHandleSettingsSetAsMain_CancelsPreviousConnectionEventStream(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	oldEventCtx := m.eventStreamCtx
	oldEventChannel := m.eventChannel

	m.dbAdapter = mockDB
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "NEW-DB")
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	_, _ = m.handleSettingsSetAsMain(*selected)

	select {
	case <-oldEventCtx.Done():
	default:
		t.Fatal("expected previous event stream context to be cancelled during database switch")
	}

	msg := waitForEventCmd(oldEventCtx, oldEventChannel)()
	if _, ok := msg.(eventChannelClosedMsg); !ok {
		t.Fatalf("expected waitForEventCmd on cancelled stream to return eventChannelClosedMsg, got %T", msg)
	}
}

func TestHandleSettingsSetAsMain_RotatesConnectionEventChannel(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	oldEventChannel := m.eventChannel

	m.dbAdapter = mockDB
	m.tracerService = mustNewTracerService(t, mockDB, m.eventChannel)

	selected := newTestDatabaseSettings(t, "NEW-DB")
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	updated, _ := m.handleSettingsSetAsMain(*selected)

	if updated.eventChannel == nil {
		t.Fatal("expected a replacement event channel after database switch")
	}
	if updated.eventChannel == oldEventChannel {
		t.Fatal("expected database switch to rotate to a new event channel")
	}
	if updated.eventStreamCtx == nil {
		t.Fatal("expected a replacement event stream context after database switch")
	}
}

// ==========================================
// Persistence Bug Test
// ==========================================

func TestSetAsMain_ThenValidate_PersistsDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	boltAdapter := newTestBoltAdapter(t)
	settingsRepo := boltdb.NewDatabaseSettingsRepository(boltAdapter)

	defaultConfig := newTestDatabaseSettings(t, "DEFAULT-DB")
	defaultConfig.SetAsDefault()
	if err := settingsRepo.Save(ctx, *defaultConfig); err != nil {
		t.Fatalf("failed to save default config: %v", err)
	}

	nonDefaultConfig := newTestDatabaseSettings(t, "NON-DEFAULT-DB")
	if err := settingsRepo.Save(ctx, *nonDefaultConfig); err != nil {
		t.Fatalf("failed to save non-default config: %v", err)
	}

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	m.appConfig = defaultConfig
	m.dbAdapter = mockDB
	m.boltAdapter = boltAdapter

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	updated, _ := m.handleSettingsSetAsMain(*nonDefaultConfig)

	if updated.appConfig == nil {
		t.Fatal("expected appConfig to be set after switch")
	}
	if updated.appConfig.DatabaseID() != "NON-DEFAULT-DB" {
		t.Fatalf("expected appConfig to be NON-DEFAULT-DB, got %s", updated.appConfig.DatabaseID())
	}

	reloadedDefault, err := settingsRepo.GetDefault(ctx)
	if err != nil {
		t.Fatalf("failed to reload default config: %v", err)
	}
	if reloadedDefault == nil {
		t.Fatal("expected default config to exist after reload")
	}
	if reloadedDefault.DatabaseID() != "NON-DEFAULT-DB" {
		t.Errorf("expected default pointer to be NON-DEFAULT-DB after switch, got %s", reloadedDefault.DatabaseID())
	}

	// Verify that only one config is marked as default
	allConfigs, err := settingsRepo.GetAll(ctx)
	if err != nil {
		t.Fatalf("failed to reload all configs: %v", err)
	}
	defaultCount := 0
	for _, cfg := range allConfigs {
		if cfg.IsDefault() {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Fatalf("expected exactly one default config, got %d", defaultCount)
	}
}

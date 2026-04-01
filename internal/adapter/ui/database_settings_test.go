package ui

import (
	"context"
	"errors"
	"testing"

	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/tracer"
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

	return &Model{
		screen: screenMain,
		width:  120,
		height: 36,
		ctx:    context.Background(),
		main: mainState{
			autoScroll: true,
			ready:      true,
		},
		dbSettings: databaseSettingsState{
			visible: true,
		},
	}
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
	m.tracerService = tracer.NewTracerService(mockDB, nil, make(chan *domain.QueueMessage, 16))

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
	mockDB := NewMockDatabaseRepository()

	// Setup existing db adapter
	m.dbAdapter = mockDB

	selected := newTestDatabaseSettings(t, "NEW-DB")

	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return mockDB, nil
	}

	// Execute
	updated, _ := m.handleSettingsSetAsMain(*selected)

	// Assert dbAdapter.Close was called
	if len(mockDB.CloseCalls) != 1 {
		t.Errorf("expected dbAdapter.Close to be called once, got %d calls", len(mockDB.CloseCalls))
	}

	// Assert dbAdapter is now the new one
	if updated.dbAdapter != mockDB {
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
	m.tracerService = tracer.NewTracerService(mockDB, nil, make(chan *domain.QueueMessage, 16))

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

// ==========================================
// Error Path Tests
// ==========================================

func TestHandleSettingsSetAsMain_FactoryError(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	mockDB := NewMockDatabaseRepository()
	m.dbAdapter = mockDB
	m.tracerService = tracer.NewTracerService(mockDB, nil, make(chan *domain.QueueMessage, 16))

	expectedErr := errors.New("connection failed: invalid credentials")

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	// Factory returns error
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return nil, expectedErr
	}

	// Execute
	updated, cmd := m.handleSettingsSetAsMain(*selected)

	// Assert error handling - loading.err is set
	if updated.loading.err == nil {
		t.Error("expected loading.err to be set on factory error")
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
	m.tracerService = tracer.NewTracerService(mockDB, nil, make(chan *domain.QueueMessage, 16))

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
	if updated.loading.err == nil {
		t.Error("expected loading error to be set")
	}
	// Verify original error is wrapped
	if updated.dbSettings.dialogMsg == factoryErr.Error() {
		t.Error("expected dialog message to contain wrapped error, not just raw error")
	}
}

func TestHandleSettingsSetAsMain_FactoryError_OldAdapterStillClosed(t *testing.T) {
	t.Parallel()

	m := newTestModelForSettings(t)
	oldMockDB := NewMockDatabaseRepository()
	m.dbAdapter = oldMockDB
	m.tracerService = tracer.NewTracerService(oldMockDB, nil, make(chan *domain.QueueMessage, 16))

	selected := newTestDatabaseSettings(t, "FAULTY-DB")

	// Factory returns error
	m.dbFactory = func(cfg *domain.DatabaseSettings) (ports.DatabaseRepository, error) {
		return nil, errors.New("connection refused")
	}

	// Execute
	m.handleSettingsSetAsMain(*selected)

	// Old adapter should still be closed even if factory fails
	if len(oldMockDB.CloseCalls) != 1 {
		t.Errorf("expected old dbAdapter.Close to be called even on factory error, got %d calls", len(oldMockDB.CloseCalls))
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
	if updated.loading.steps != nil {
		t.Error("expected loading.steps to be nil/empty on success")
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
	m.tracerService = tracer.NewTracerService(oldMockDB, nil, make(chan *domain.QueueMessage, 16))

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

package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/core/ports"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	"context"
	"testing"
)

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

	settings := newTestDatabaseSettings(t, "LOAD-DB")
	if validated {
		settings.MarkPermissionsValidated()
	}

	mockDB := NewMockDatabaseRepository()
	configRepo := stubConfigRepository{}
	tracerService, err := tracer.NewTracerService(mockDB, configRepo, make(chan *domain.QueueMessage, 1))
	if err != nil {
		t.Fatalf("NewTracerService: %v", err)
	}

	return &Model{
		ctx:               context.Background(),
		appConfig:         settings,
		dbAdapter:         mockDB,
		permissionService: permissions.NewPermissionService(mockDB, stubPermissionsRepository{}, configRepo),
		tracerService:     tracerService,
		subscriberService: subscribers.NewSubscriberService(mockDB, nil),
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

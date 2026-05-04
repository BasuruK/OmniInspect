package ui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"OmniView/internal/core/domain"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"
	updaterSvc "OmniView/internal/service/updater"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func newTestModelForWebhookSettings(t *testing.T) *Model {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	eventStreamCtx, eventStreamCancel := context.WithCancel(ctx)
	t.Cleanup(eventStreamCancel)

	boltAdapter := newTestBoltAdapter(t)
	mockDB := NewMockDatabaseRepository()
	eventChannel := make(chan *domain.QueueMessage, 16)

	tracerService, err := tracer.NewTracerService(mockDB, boltAdapter, eventChannel)
	if err != nil {
		t.Fatalf("NewTracerService: %v", err)
	}

	return &Model{
		screen:             screenMain,
		width:              120,
		height:             36,
		ctx:                ctx,
		cancel:             cancel,
		eventStreamCtx:     eventStreamCtx,
		eventStreamCancel:  eventStreamCancel,
		boltAdapter:        boltAdapter,
		dbAdapter:          mockDB,
		permissionService:  permissions.NewPermissionService(mockDB, stubPermissionsRepository{}, boltAdapter),
		tracerService:      tracerService,
		subscriberService:  subscribers.NewSubscriberService(mockDB, nil, nil),
		updaterService:     updaterSvc.NewUpdaterService("test"),
		eventChannel:       eventChannel,
		updateEventChannel: make(chan tea.Msg, 16),
		main: mainState{
			autoScroll: true,
			ready:      true,
		},
	}
}

func TestUpdateMain_SKeyOpensWebhookSettings(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	config, err := domain.NewWebhookConfig(domain.DefaultWebhookID, "https://example.com/trace", true)
	if err != nil {
		t.Fatalf("NewWebhookConfig: %v", err)
	}
	if err := m.boltAdapter.SaveWebhookConfig(config); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}

	updated, cmd := m.updateMain(makeCharPress("s"))
	if cmd != nil {
		t.Fatal("expected no follow-up command when opening webhook settings")
	}
	if !updated.webhookSettings.visible {
		t.Fatal("expected webhook settings overlay to be visible")
	}
	if updated.webhookSettings.input != config.URL {
		t.Fatalf("expected webhook input %q, got %q", config.URL, updated.webhookSettings.input)
	}
}

func TestSaveWebhookSettingsCmd_PersistsURL(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.input = "https://example.com/webhook"

	msg := m.saveWebhookSettingsCmd()()
	saved, ok := msg.(webhookConfigSavedMsg)
	if !ok {
		t.Fatalf("expected webhookConfigSavedMsg, got %T", msg)
	}
	if saved.err != nil {
		t.Fatalf("expected save to succeed, got %v", saved.err)
	}

	updated, cmd := m.updateWebhookSettings(saved)
	if cmd != nil {
		t.Fatal("expected no follow-up command after save")
	}
	if updated.webhookSettings.visible {
		t.Fatal("expected webhook settings overlay to close after save")
	}

	config, err := m.boltAdapter.GetWebhookConfig()
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if config.URL != "https://example.com/webhook" {
		t.Fatalf("expected stored webhook URL to match, got %q", config.URL)
	}
}

func TestSaveWebhookSettingsCmd_EmptyURLClearsConfig(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	config, err := domain.NewWebhookConfig(domain.DefaultWebhookID, "https://example.com/trace", true)
	if err != nil {
		t.Fatalf("NewWebhookConfig: %v", err)
	}
	if err := m.boltAdapter.SaveWebhookConfig(config); err != nil {
		t.Fatalf("SaveWebhookConfig: %v", err)
	}

	m.initWebhookSettings(config)
	m.webhookSettings.input = ""

	msg := m.saveWebhookSettingsCmd()()
	saved, ok := msg.(webhookConfigSavedMsg)
	if !ok {
		t.Fatalf("expected webhookConfigSavedMsg, got %T", msg)
	}
	if !saved.deleted {
		t.Fatal("expected empty webhook input to clear the stored config")
	}
	if saved.err != nil {
		t.Fatalf("expected clear to succeed, got %v", saved.err)
	}

	updated, cmd := m.updateWebhookSettings(saved)
	if cmd != nil {
		t.Fatal("expected no follow-up command after clearing webhook")
	}
	if updated.webhookSettings.visible {
		t.Fatal("expected webhook settings overlay to close after clearing")
	}

	if _, err := m.boltAdapter.GetWebhookConfig(); !errors.Is(err, domain.ErrWebhookConfigNotFound) {
		t.Fatalf("expected ErrWebhookConfigNotFound after clearing, got %v", err)
	}
}

func TestSaveWebhookSettingsCmd_EmptyURLWithoutExistingIsIdempotent(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	if _, err := m.boltAdapter.GetWebhookConfig(); !errors.Is(err, domain.ErrWebhookConfigNotFound) {
		t.Fatalf("expected no webhook before save, got %v", err)
	}

	m.initWebhookSettings(nil)
	m.webhookSettings.input = ""

	msg := m.saveWebhookSettingsCmd()()
	saved, ok := msg.(webhookConfigSavedMsg)
	if !ok {
		t.Fatalf("expected webhookConfigSavedMsg, got %T", msg)
	}
	if !saved.deleted {
		t.Fatal("expected empty save with no prior config to report deleted (close overlay)")
	}
	if saved.err != nil {
		t.Fatalf("expected idempotent clear to succeed, got %v", saved.err)
	}

	if _, err := m.boltAdapter.GetWebhookConfig(); !errors.Is(err, domain.ErrWebhookConfigNotFound) {
		t.Fatalf("expected ErrWebhookConfigNotFound after idempotent clear, got %v", err)
	}
}

func TestSaveWebhookSettingsCmd_UpdatePreservesCreatedAt(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.input = "https://example.com/first"

	first := m.saveWebhookSettingsCmd()()
	savedFirst, ok := first.(webhookConfigSavedMsg)
	if !ok || savedFirst.err != nil || savedFirst.config == nil {
		t.Fatalf("expected first save to succeed, got %#v", savedFirst)
	}

	stored, err := m.boltAdapter.GetWebhookConfig()
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	createdAt := stored.CreatedAt
	if createdAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt after first save")
	}

	time.Sleep(2 * time.Millisecond)

	m.initWebhookSettings(stored)
	m.webhookSettings.input = "https://example.com/second"

	second := m.saveWebhookSettingsCmd()()
	savedSecond, ok := second.(webhookConfigSavedMsg)
	if !ok || savedSecond.err != nil || savedSecond.config == nil {
		t.Fatalf("expected second save to succeed, got %#v", savedSecond)
	}
	if !savedSecond.config.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected CreatedAt preserved on URL update, got %v want %v", savedSecond.config.CreatedAt, createdAt)
	}
	if !savedSecond.config.UpdatedAt.After(createdAt) {
		t.Fatalf("expected UpdatedAt after CreatedAt, got UpdatedAt=%v CreatedAt=%v", savedSecond.config.UpdatedAt, createdAt)
	}

	reloaded, err := m.boltAdapter.GetWebhookConfig()
	if err != nil {
		t.Fatalf("GetWebhookConfig: %v", err)
	}
	if reloaded.URL != "https://example.com/second" {
		t.Fatalf("expected updated URL, got %q", reloaded.URL)
	}
	if !reloaded.CreatedAt.Equal(createdAt) {
		t.Fatalf("expected persisted CreatedAt unchanged, got %v want %v", reloaded.CreatedAt, createdAt)
	}
}

func TestSaveWebhookSettingsCmd_InvalidURLShowsError(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.input = "localhost-only"

	msg := m.saveWebhookSettingsCmd()()
	saved, ok := msg.(webhookConfigSavedMsg)
	if !ok {
		t.Fatalf("expected webhookConfigSavedMsg, got %T", msg)
	}
	if saved.err == nil {
		t.Fatal("expected invalid webhook URL to return an error")
	}

	updated, cmd := m.updateWebhookSettings(saved)
	if cmd != nil {
		t.Fatal("expected no follow-up command after invalid webhook save")
	}
	if !updated.webhookSettings.visible {
		t.Fatal("expected webhook settings overlay to remain visible after invalid input")
	}
	if !updated.webhookSettings.showDialog {
		t.Fatal("expected invalid webhook input to show an error dialog")
	}
}

func TestModelUpdate_MainWebhookOverlayQClosesOverlay(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.cursor = webhookBtnCancel // q should close when not on URL field

	updatedModel, cmd := m.Update(makeCharPress("q"))
	if cmd != nil {
		t.Fatal("expected q to be handled by webhook settings overlay without quitting")
	}

	updated, ok := updatedModel.(*Model)
	if !ok {
		t.Fatalf("expected Update to return *Model, got %T", updatedModel)
	}
	if updated.webhookSettings.visible {
		t.Fatal("expected q to close the webhook settings overlay when cursor is on Cancel button")
	}
}

func TestUpdateWebhookSettings_QLetterInURLField(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.cursor = webhookFieldURL
	m.webhookSettings.input = "https://example."

	updated, cmd := m.updateWebhookSettings(makeCharPress("q"))
	if cmd != nil {
		t.Fatal("expected no follow-up command for character input")
	}
	if updated.webhookSettings.input != "https://example.q" {
		t.Fatalf("expected q to be appended to input, got %q", updated.webhookSettings.input)
	}
	if !updated.webhookSettings.visible {
		t.Fatal("expected webhook settings overlay to remain visible")
	}
}

func TestUpdateWebhookSettings_QKeyOnSaveButtonClosesOverlay(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.initWebhookSettings(nil)
	m.webhookSettings.cursor = webhookBtnSave

	updated, cmd := m.updateWebhookSettings(makeCharPress("q"))
	if cmd != nil {
		t.Fatal("expected no follow-up command for q key")
	}
	if updated.webhookSettings.visible {
		t.Fatal("expected q to close webhook settings when cursor is on Save button")
	}
}

func TestResizeWebhookSettings_RecomputesCompactLayout(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.width = 48
	m.height = 12
	m.initWebhookSettings(nil)

	m.resizeWebhookSettings(m.width, m.height)

	if m.webhookSettings.layout.panelWidth > m.width {
		t.Fatalf("expected panel width to fit terminal, got %d > %d", m.webhookSettings.layout.panelWidth, m.width)
	}
	if want := m.webhookSettings.layout.panelWidth - 4; m.webhookSettings.layout.innerWidth != want {
		t.Fatalf("expected inner width %d, got %d", want, m.webhookSettings.layout.innerWidth)
	}
	if !m.webhookSettings.layout.compact {
		t.Fatal("expected small terminal height to enable compact webhook layout")
	}
	if m.webhookSettings.layout.showCurrentStatus {
		t.Fatal("expected compact layout to hide the current webhook summary")
	}
}

func TestViewWebhookSettings_StaysWithinNarrowWindowWidth(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	m.width = 48
	m.height = 14
	m.initWebhookSettings(nil)

	rendered := m.viewWebhookSettings()
	for _, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > m.width {
			t.Fatalf("expected webhook settings line width <= %d, got %d\nline: %q", m.width, got, line)
		}
	}
}

func TestMainFooterText_IncludesSettingsShortcut(t *testing.T) {
	t.Parallel()

	m := newTestModelForWebhookSettings(t)
	if got := m.mainFooterText(); got == "" || !containsAll(got, "D Database Settings", "S Settings") {
		t.Fatalf("expected footer text to include database and settings shortcuts, got %q", got)
	}
}

func containsAll(text string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			return false
		}
	}
	return true
}

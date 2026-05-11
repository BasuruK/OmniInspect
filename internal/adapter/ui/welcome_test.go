package ui

import (
	"context"
	"errors"
	"testing"

	"OmniView/internal/adapter/ui/animations"
	"OmniView/internal/core/domain"
	"OmniView/internal/service/permissions"
	"OmniView/internal/service/subscribers"
	"OmniView/internal/service/tracer"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// ==========================================
// welcome test helpers
// ==========================================

// newWelcomeTestModel builds a minimal Model wired for welcome-screen tests.
// The animation model is initialised so IsPlaying()/handleWelcomeTick work.
func newWelcomeTestModel(t *testing.T) *Model {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	eventStreamCtx, eventStreamCancel := context.WithCancel(ctx)
	t.Cleanup(eventStreamCancel)
	t.Cleanup(cancel)

	eventChannel := make(chan *domain.QueueMessage, 16)

	mockDB := NewMockDatabaseRepository()
	configRepo := stubConfigRepository{}
	tracerSvc, err := tracer.NewTracerService(mockDB, configRepo, eventChannel)
	if err != nil {
		t.Fatalf("NewTracerService: %v", err)
	}

	return &Model{
		screen:            screenWelcome,
		width:             120,
		height:            36,
		ctx:               ctx,
		cancel:            cancel,
		eventStreamCtx:    eventStreamCtx,
		eventStreamCancel: eventStreamCancel,
		dbSettingsRepo:    stubDatabaseSettingsRepository{},
		dbAdapter:         mockDB,
		permissionService: permissions.NewPermissionService(mockDB, stubPermissionsRepository{}, configRepo),
		tracerService:     tracerSvc,
		subscriberService: subscribers.NewSubscriberService(mockDB, nil, nil),
		eventChannel:      eventChannel,
		loading: loadingState{
			spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		},
		welcome: welcomeState{
			animModel: animations.NewWithDefaults(),
		},
		main: mainState{autoScroll: true},
	}
}

// advanceAnimToEnd drives the animation model to completion by feeding tickMsgs.
// It stops after a safety limit to avoid infinite loops in tests.
func advanceAnimToEnd(m *Model) {
	const maxFrames = 5000
	for i := 0; i < maxFrames && m.welcome.animModel.IsPlaying(); i++ {
		updated, _ := m.welcome.animModel.Update(tickMsgForTest())
		m.welcome.animModel = updated.(animations.Model)
	}
}

// tickMsgForTest returns the internal tickMsg used by the animation.
// Because tickMsg is an unexported type alias for time.Time we pass a
// zero-value tea.Msg that matches the type via the animation's Update path.
// We call the real Update with a live tickMsg obtained from Init.
func tickMsgForTest() tea.Msg {
	// Use the animations package's internal tick by driving through Update.
	// We just need any value; the animation increments frameIndex on any tickMsg.
	// Since tickMsg is not exported, we obtain it by running Init and invoking
	// the returned Cmd.
	m := animations.NewWithDefaults()
	cmd := m.Init()
	if cmd == nil {
		return nil
	}
	return cmd()
}

// newProgressBarForTest creates a minimal progress bar for welcome state.
func newProgressBarForTest() progress.Model {
	return progress.New(
		progress.WithoutPercentage(),
		progress.WithFillCharacters('━', '─'),
	)
}

// ==========================================
// computeProgressBarWidth
// ==========================================

func TestComputeProgressBarWidth_LargeTerminal(t *testing.T) {
	t.Parallel()
	if got := computeProgressBarWidth(200); got != progressBarWidth {
		t.Fatalf("expected %d for large terminal, got %d", progressBarWidth, got)
	}
}

func TestComputeProgressBarWidth_ZeroTerminal(t *testing.T) {
	t.Parallel()
	if got := computeProgressBarWidth(0); got != progressBarWidth {
		t.Fatalf("expected %d for zero-width terminal, got %d", progressBarWidth, got)
	}
}

func TestComputeProgressBarWidth_SmallTerminal(t *testing.T) {
	t.Parallel()
	// terminal width 30: 30-4=26 < 80
	if got := computeProgressBarWidth(30); got != 26 {
		t.Fatalf("expected 26 for 30-wide terminal, got %d", got)
	}
}

func TestComputeProgressBarWidth_VerySmallTerminal(t *testing.T) {
	t.Parallel()
	// terminal width 10: 10-4=6 < 20 minimum → clamped to 20
	if got := computeProgressBarWidth(10); got != 20 {
		t.Fatalf("expected minimum 20 for very small terminal, got %d", got)
	}
}

// ==========================================
// handleDBReady
// ==========================================

func TestHandleDBReady_WithSettings_StartsParallelLoading(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	settings := newTestDatabaseSettings(t, "DB-PARALLEL")

	updated, cmd := m.handleDBReady(dbReadyMsg{settings: settings, err: nil})

	if !updated.welcome.dbReady {
		t.Fatal("expected dbReady to be true")
	}
	if !updated.welcome.loadingStarted {
		t.Fatal("expected loadingStarted to be true after settings arrive")
	}
	if updated.appConfig == nil {
		t.Fatal("expected appConfig to be set")
	}
	if cmd == nil {
		t.Fatal("expected connectDBCmd to be returned")
	}
	if updated.loading.current != "Connecting to database..." {
		t.Fatalf("expected connecting message, got %q", updated.loading.current)
	}
}

func TestHandleDBReady_WithError_SetsDbErr(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	dbErr := errors.New("bolt read failed")

	updated, cmd := m.handleDBReady(dbReadyMsg{settings: nil, err: dbErr})

	if updated.welcome.dbErr == nil {
		t.Fatal("expected dbErr to be set")
	}
	if !errors.Is(updated.welcome.dbErr, dbErr) {
		t.Fatalf("expected dbErr %v, got %v", dbErr, updated.welcome.dbErr)
	}
	if updated.welcome.loadingStarted {
		t.Fatal("expected loadingStarted to remain false on error")
	}
	_ = cmd // no command expected
}

func TestHandleDBReady_NoSettings_DoesNotStartLoading(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)

	updated, cmd := m.handleDBReady(dbReadyMsg{settings: nil, err: nil})

	if updated.welcome.loadingStarted {
		t.Fatal("expected loadingStarted to be false when no settings")
	}
	if cmd != nil {
		t.Fatal("expected no command when no settings")
	}
}

func TestHandleDBReady_AnimAlreadyComplete_CallsHandleWelcomeComplete(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	// Mark animation as already done.
	advanceAnimToEnd(m)
	m.welcome.complete = true

	settings := newTestDatabaseSettings(t, "DB-FAST")
	updated, _ := m.handleDBReady(dbReadyMsg{settings: settings, err: nil})

	// handleWelcomeComplete should have been called and transitioned screen.
	if updated.screen == screenWelcome {
		t.Fatal("expected screen to leave welcome after animation complete + settings ready")
	}
}

func TestHandleDBReady_ProgressBarWidthRespectsTerminal(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.width = 40 // narrow terminal: 40-4=36
	settings := newTestDatabaseSettings(t, "DB-NARROW")

	updated, _ := m.handleDBReady(dbReadyMsg{settings: settings, err: nil})

	if !updated.welcome.loadingStarted {
		t.Fatal("expected loadingStarted")
	}
	got := updated.welcome.progressBar.Width()
	want := computeProgressBarWidth(40)
	if got != want {
		t.Fatalf("expected progress bar width %d, got %d", want, got)
	}
}

// ==========================================
// handleWelcomeComplete
// ==========================================

func TestHandleWelcomeComplete_FastPath_GoesToLoadingScreenForUpdateGate(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingComplete = true
	m.welcome.loadingStarted = true
	m.welcome.complete = true

	updated, cmd := m.handleWelcomeComplete()

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading, got %q", updated.screen)
	}
	if !updated.loading.complete {
		t.Fatal("expected loading completion to be preserved")
	}
	if !updated.update.checking {
		t.Fatal("expected update check gate to start before main screen")
	}
	if cmd == nil {
		t.Fatal("expected loading startup commands")
	}
}

func TestHandleWelcomeComplete_MidProgress_GoesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.loadingComplete = false

	updated, cmd := m.handleWelcomeComplete()

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading, got %q", updated.screen)
	}
	if cmd == nil {
		t.Fatal("expected spinner tick command")
	}
}

func TestHandleWelcomeComplete_WithError_GoesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.dbErr = errors.New("bolt read failed")

	updated, _ := m.handleWelcomeComplete()

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading on error, got %q", updated.screen)
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading.err to be set from dbErr")
	}
}

func TestHandleWelcomeComplete_NoSettings_GoesToOnboarding(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.dbSettings = nil
	m.welcome.loadingStarted = false

	updated, _ := m.handleWelcomeComplete()

	if updated.screen != screenOnboarding {
		t.Fatalf("expected screenOnboarding, got %q", updated.screen)
	}
}

func TestHandleWelcomeComplete_WithSettings_GoesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.dbSettings = newTestDatabaseSettings(t, "DB-SEQ")
	m.welcome.loadingStarted = false

	updated, cmd := m.handleWelcomeComplete()

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading for sequential path, got %q", updated.screen)
	}
	if cmd == nil {
		t.Fatal("expected spinner+update batch command")
	}
}

// ==========================================
// handleWelcomeGlobal — window resize updates bar width
// ==========================================

func TestHandleWelcomeGlobal_WindowResize_UpdatesProgressBarWidth(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()
	m.welcome.progressBar.SetWidth(80)

	resizeMsg := tea.WindowSizeMsg{Width: 50, Height: 30}
	updated, _ := m.handleWelcomeGlobal(resizeMsg)

	if updated.width != 50 {
		t.Fatalf("expected width 50, got %d", updated.width)
	}
	want := computeProgressBarWidth(50)
	if got := updated.welcome.progressBar.Width(); got != want {
		t.Fatalf("expected bar width %d after resize, got %d", want, got)
	}
}

func TestHandleWelcomeGlobal_WindowResize_NoBarWhenLoadingNotStarted(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = false

	// Should not panic or mutate a zero-value progress bar.
	resizeMsg := tea.WindowSizeMsg{Width: 50, Height: 30}
	updated, _ := m.handleWelcomeGlobal(resizeMsg)

	if updated.width != 50 {
		t.Fatalf("expected width 50, got %d", updated.width)
	}
}

// ==========================================
// handleWelcomeLoadingMsg — step progression
// ==========================================

func TestHandleWelcomeLoadingMsg_DBConnected_AdvancesProgress(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	settings := newTestDatabaseSettings(t, "DB-CONN")
	m.appConfig = settings
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, cmd := m.handleWelcomeLoadingMsg(dbConnectedMsg{err: nil})

	if len(updated.loading.steps) == 0 {
		t.Fatal("expected a connected step to be appended")
	}
	if updated.loading.steps[0] != "✓ Connected to Oracle database" {
		t.Fatalf("unexpected step: %q", updated.loading.steps[0])
	}
	if cmd == nil {
		t.Fatal("expected next command after connected")
	}
	if updated.loading.current == "" {
		t.Fatal("expected loading.current to be set")
	}
}

func TestHandleWelcomeLoadingMsg_DBConnectedError_SwitchesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, _ := m.handleWelcomeLoadingMsg(dbConnectedMsg{err: errors.New("connection refused")})

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading on DB error, got %q", updated.screen)
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading.err to be set")
	}
}

func TestHandleWelcomeLoadingMsg_PermissionsChecked_AdvancesProgress(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.appConfig = newTestDatabaseSettings(t, "DB-PERM")
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()
	boltAdapter := newTestBoltAdapter(t)
	m.boltAdapter = boltAdapter

	updated, cmd := m.handleWelcomeLoadingMsg(permissionsCheckedMsg{err: nil})

	if len(updated.loading.steps) == 0 {
		t.Fatal("expected permissions step to be appended")
	}
	if updated.loading.steps[0] != "✓ Permissions verified" {
		t.Fatalf("unexpected step: %q", updated.loading.steps[0])
	}
	if cmd == nil {
		t.Fatal("expected deployTracerCmd after permissions")
	}
}

func TestHandleWelcomeLoadingMsg_PermissionsError_SwitchesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, _ := m.handleWelcomeLoadingMsg(permissionsCheckedMsg{err: errors.New("permission denied")})

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading on perm error, got %q", updated.screen)
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading.err to be set")
	}
}

func TestHandleWelcomeLoadingMsg_TracerDeployed_AdvancesProgress(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, cmd := m.handleWelcomeLoadingMsg(tracerDeployedMsg{err: nil})

	if len(updated.loading.steps) == 0 {
		t.Fatal("expected tracer step to be appended")
	}
	if updated.loading.steps[0] != "✓ Tracer package deployed" {
		t.Fatalf("unexpected step: %q", updated.loading.steps[0])
	}
	if cmd == nil {
		t.Fatal("expected registerSubscriberCmd after tracer deployed")
	}
}

func TestHandleWelcomeLoadingMsg_TracerError_SwitchesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, _ := m.handleWelcomeLoadingMsg(tracerDeployedMsg{err: errors.New("deploy failed")})

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading on tracer error, got %q", updated.screen)
	}
}

func TestHandleWelcomeLoadingMsg_SubscriberRegistered_AnimStillPlaying_SetsLoadingComplete(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.appConfig = newTestDatabaseSettings(t, "DB-SUB-PLAYING")
	m.welcome.loadingStarted = true
	m.welcome.complete = false // animation still playing
	m.welcome.progressBar = newProgressBarForTest()

	sub := newTestSubscriber(t)
	updated, cmd := m.handleWelcomeLoadingMsg(subscriberRegisteredMsg{subscriber: sub, err: nil})

	if !updated.welcome.loadingComplete {
		t.Fatal("expected loadingComplete to be set")
	}
	if updated.screen == screenMain {
		t.Fatal("expected screen to remain on welcome while animation plays")
	}
	// cmd should be the progress bar animation cmd, not nil
	if cmd == nil {
		t.Fatal("expected progress bar animation cmd")
	}
}

func TestHandleWelcomeLoadingMsg_SubscriberRegistered_AnimDone_GoesToLoadingForUpdateGate(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.appConfig = newTestDatabaseSettings(t, "DB-SUB-DONE")
	m.welcome.loadingStarted = true
	m.welcome.complete = true // animation already done
	m.welcome.progressBar = newProgressBarForTest()

	sub := newTestSubscriber(t)
	updated, cmd := m.handleWelcomeLoadingMsg(subscriberRegisteredMsg{subscriber: sub, err: nil})

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading when animation already done, got %q", updated.screen)
	}
	if !updated.loading.complete {
		t.Fatal("expected loading completion to be recorded")
	}
	if cmd == nil {
		t.Fatal("expected loading startup commands")
	}
}

func TestHandleWelcomeLoadingMsg_SubscriberError_SwitchesToLoadingScreen(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, _ := m.handleWelcomeLoadingMsg(subscriberRegisteredMsg{subscriber: nil, err: errors.New("subscribe failed")})

	if updated.screen != screenLoading {
		t.Fatalf("expected screenLoading on subscriber error, got %q", updated.screen)
	}
	if updated.loading.err == nil {
		t.Fatal("expected loading.err to be set")
	}
}

// ==========================================
// Cached permissions — progress jump check
// ==========================================

func TestHandleWelcomeLoadingMsg_CachedPermissions_JumpsToFiftyPercent(t *testing.T) {
	t.Parallel()

	m := newWelcomeTestModel(t)
	settings := newTestDatabaseSettings(t, "DB-CACHED")
	settings.MarkPermissionsValidated()
	m.appConfig = settings
	m.welcome.loadingStarted = true
	m.welcome.progressBar = newProgressBarForTest()

	updated, cmd := m.handleWelcomeLoadingMsg(dbConnectedMsg{err: nil})

	// With cached permissions the steps should include both "connected" and "permissions verified (cached)".
	if len(updated.loading.steps) < 2 {
		t.Fatalf("expected at least 2 steps for cached permissions path, got %d", len(updated.loading.steps))
	}
	found := false
	for _, s := range updated.loading.steps {
		if s == "✓ Permissions verified (cached)" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cached permissions step, got steps: %v", updated.loading.steps)
	}
	// cmd must not be nil — should be deploying tracer.
	if cmd == nil {
		t.Fatal("expected deployTracerCmd for cached permissions path")
	}
	// Progress target should be 50%, not 25%.
	if got := updated.welcome.progressBar.Percent(); got != 0.50 {
		t.Fatalf("expected progress 0.50 for cached permissions, got %f", got)
	}
}

// ==========================================
// helper: newTestSubscriber
// ==========================================

func newTestSubscriber(t *testing.T) *domain.Subscriber {
	t.Helper()

	batchSize, err := domain.NewBatchSize(100)
	if err != nil {
		t.Fatalf("NewBatchSize: %v", err)
	}
	waitTime, err := domain.NewWaitTime(5)
	if err != nil {
		t.Fatalf("NewWaitTime: %v", err)
	}
	sub, err := domain.NewSubscriber("TEST_SUB", batchSize, waitTime)
	if err != nil {
		t.Fatalf("NewSubscriber: %v", err)
	}
	return sub
}

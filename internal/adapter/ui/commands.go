package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/updater"
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

var (
	ErrPermissionServiceNotInitialized = errors.New("permission service not initialized")
	ErrTracerServiceNotInitialized     = errors.New("tracer service not initialized")
	ErrSubscriberServiceNotInitialized = errors.New("subscriber service not initialized")
	ErrUpdaterServiceNotAvailable      = errors.New("updater service not available")
)

// connectDBCmd connects to the Oracle database.
// isSwitch indicates whether this connection attempt is part of a database switch.
func connectDBCmd(m *Model, isSwitch bool) tea.Cmd {
	return func() tea.Msg {
		err := m.dbAdapter.Connect(m.ctx)
		return dbConnectedMsg{err: err, isSwitch: isSwitch}
	}
}

// checkPermissionsCmd deploys and verifies database permissions.
func checkPermissionsCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		if m.permissionService == nil {
			return permissionsCheckedMsg{err: fmt.Errorf("checkPermissionsCmd: %w", ErrPermissionServiceNotInitialized)}
		}
		_, err := m.permissionService.DeployAndCheck(m.ctx, m.appConfig.Username())
		return permissionsCheckedMsg{err: err}
	}
}

// deployTracerCmd deploys and verifies the tracer package.
func deployTracerCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		if m.tracerService == nil {
			return tracerDeployedMsg{err: fmt.Errorf("deployTracerCmd: %w", ErrTracerServiceNotInitialized)}
		}
		err := m.tracerService.DeployAndCheck(m.ctx)
		return tracerDeployedMsg{err: err}
	}
}

// registerSubscriberCmd registers a queue subscriber.
func registerSubscriberCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		if m.subscriberService == nil {
			return subscriberRegisteredMsg{subscriber: nil, err: fmt.Errorf("registerSubscriberCmd: %w", ErrSubscriberServiceNotInitialized)}
		}
		subscriber, err := m.subscriberService.RegisterSubscriber(m.ctx)
		return subscriberRegisteredMsg{subscriber: subscriber, err: err}
	}
}

// eventChannelClosedMsg is sent when the event channel is closed.
type eventChannelClosedMsg struct{}

// waitForEventCmd waits for one message from the event channel.
// After Update() processes the message, it must re-issue this command
// to receive the next message. See Section 4.5 for the pattern.
// It unblocks immediately if the context is cancelled or the channel is closed.
func waitForEventCmd(ctx context.Context, ch <-chan *domain.QueueMessage) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-ch:
			if !ok {
				return eventChannelClosedMsg{}
			}
			return queueMessageMsg{message: msg}
		case <-ctx.Done():
			return eventChannelClosedMsg{}
		}
	}
}

// waitForUpdateEventCmd waits for one message from the update event channel.
// After Update() processes the message, it must re-issue this command
// to receive the next message. This keeps the progress loop alive.
// It unblocks immediately if the context is cancelled or the channel is closed.
func waitForUpdateEventCmd(ctx context.Context, ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg, ok := <-ch:
			if !ok {
				return updateCompleteMsg{}
			}
			return msg
		case <-ctx.Done():
			return updateCompleteMsg{}
		}
	}
}

// waitForRetryTimerCmd waits for the retry timer to expire or the context to be cancelled.
func waitForRetryTimerCmd(ctx context.Context, timer *time.Timer, generation int) tea.Cmd {
	return func() tea.Msg {
		if timer == nil {
			return retryTimerExpiryMsg{generation: generation}
		}

		select {
		case <-timer.C:
			return retryTimerExpiryMsg{generation: generation}
		case <-ctx.Done():
			return retryTimerExpiryMsg{generation: generation}
		}
	}
}

// ==========================================
// Updater Commands
// ==========================================

// checkForUpdateCmd checks for available updates and returns the result.
func checkForUpdateCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		if m.updaterService == nil {
			return updateCheckResultMsg{info: nil, err: fmt.Errorf("checkForUpdateCmd: %w", ErrUpdaterServiceNotAvailable)}
		}
		info, err := m.updaterService.CheckForUpdate(m.ctx)
		if err != nil {
			return updateCheckResultMsg{info: nil, err: fmt.Errorf("checkForUpdate: %w", err)}
		}
		return updateCheckResultMsg{info: info, err: nil}
	}
}

// applyUpdateCmd downloads and applies the update, reporting progress via updateProgressMsg.
// Progress updates are sent through the updateEventChannel which must be processed by the Update loop.
func applyUpdateCmd(m *Model, info *updater.UpdateInfo) tea.Cmd {
	return func() tea.Msg {
		// DownloadAndApply blocks and calls progressFn at each stage.
		// We send progress messages through the channel for the Update loop to process.
		err := m.updaterService.ApplyUpdate(m.ctx, info, func(stage string) {
			if m.updateEventChannel != nil {
				select {
				case m.updateEventChannel <- updateProgressMsg{stage: stage}:
					// sent successfully
				default:
					// channel full, drop the message
				case <-m.ctx.Done():
					// context cancelled, stop
				}
			}
		})
		if err != nil {
			return updateErrorMsg{err: fmt.Errorf("applyUpdate: %w", err)}
		}
		return updateCompleteMsg{}
	}
}

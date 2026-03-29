package ui

import (
	"OmniView/internal/core/domain"
	"OmniView/internal/updater"
	"context"

	tea "charm.land/bubbletea/v2"
)

// connectDBCmd connects to the Oracle database.
func connectDBCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		err := m.dbAdapter.Connect(m.ctx)
		return dbConnectedMsg{err: err}
	}
}

// checkPermissionsCmd deploys and verifies database permissions.
func checkPermissionsCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		_, err := m.permissionService.DeployAndCheck(m.ctx, m.appConfig.Username())
		return permissionsCheckedMsg{err: err}
	}
}

// deployTracerCmd deploys and verifies the tracer package.
func deployTracerCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		err := m.tracerService.DeployAndCheck(m.ctx)
		return tracerDeployedMsg{err: err}
	}
}

// registerSubscriberCmd registers a queue subscriber.
func registerSubscriberCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
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

// ==========================================
// Updater Commands
// ==========================================

// checkForUpdateCmd checks for available updates and returns the result.
func checkForUpdateCmd(m *Model) tea.Cmd {
	return func() tea.Msg {
		info, err := m.updaterService.CheckForUpdate(m.ctx)
		return updateCheckResultMsg{info: info, err: err}
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
				m.updateEventChannel <- updateProgressMsg{stage: stage}
			}
		})
		if err != nil {
			return updateErrorMsg{err: err}
		}
		return updateCompleteMsg{}
	}
}

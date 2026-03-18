package ui

import (
	"OmniView/internal/core/domain"

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
		subscriber, err := m.subsciberService.RegisterSubscriber(m.ctx)
		return subscriberRegisteredMsg{subscriber: subscriber, err: err}
	}
}

// waitForEventCmd waits for one message from the event channel.
// After Update() processes the message, it must re-issue this command
// to receive the next message. See Section 4.5 for the pattern.
func waitForEventCmd(ch <-chan *domain.QueueMessage) tea.Cmd {
	return func() tea.Msg {
		msg := <-ch // blocks until a message arrives
		return queueMessageMsg{message: msg}
	}
}

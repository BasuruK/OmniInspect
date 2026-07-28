package domain

// ==========================================
// BroadcastMode
// ==========================================

// BroadcastMode represents the display filter mode for tracer messages. Controls which messages are visible in the TUI viewport.
type BroadcastMode int

// ==========================================
// Constants
// ==========================================

const (
	BroadcastModeGlobal     BroadcastMode = 0
	BroadcastModeSubscriber BroadcastMode = 1
	BroadcastModeBroadcast  BroadcastMode = 2
)

// ==========================================
// Constructor
// ==========================================

// NewBroadcastMode creates a BroadcastMode from its string representation. Returns BroadcastModeGlobal for unrecognized strings.
func NewBroadcastMode(mode string) BroadcastMode {
	switch mode {
	case "Global":
		return BroadcastModeGlobal
	case "Subscriber", "Only Subscriber":
		return BroadcastModeSubscriber
	case "Broadcast", "Only Broadcast":
		return BroadcastModeBroadcast
	default:
		return BroadcastModeGlobal
	}
}

// ==========================================
// String
// ==========================================

// String returns a human-readable string representation of the mode.
func (m BroadcastMode) String() string {
	switch m {
	case BroadcastModeGlobal:
		return "Global"
	case BroadcastModeSubscriber:
		return "Only Subscriber"
	case BroadcastModeBroadcast:
		return "Only Broadcast"
	default:
		return "Global"
	}
}

// ==========================================
// Navigation
// ==========================================

// Includes reports whether a message is visible under this mode. Global shows everything; Subscriber shows only subscriber-targeted messages;
// Broadcast shows only broadcast-to-all messages (whose QueueMessage mode is "Global" — see IsGlobalMessage for that naming quirk).
func (m BroadcastMode) Includes(msg *QueueMessage) bool {
	if msg == nil {
		return false
	}
	switch m {
	case BroadcastModeSubscriber:
		return !msg.IsGlobalMessage()
	case BroadcastModeBroadcast:
		return msg.IsGlobalMessage()
	default:
		return true
	}
}

// Next returns the next mode in the cycle: Global -> Subscriber -> Broadcast -> Global.
func (m BroadcastMode) Next() BroadcastMode {
	switch m {
	case BroadcastModeGlobal:
		return BroadcastModeSubscriber
	case BroadcastModeSubscriber:
		return BroadcastModeBroadcast
	case BroadcastModeBroadcast:
		return BroadcastModeGlobal
	default:
		return BroadcastModeGlobal
	}
}

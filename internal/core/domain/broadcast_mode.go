package domain

// ==========================================
// BroadcastMode
// ==========================================

// BroadcastMode represents the display filter mode for tracer messages.
// Controls which messages are visible in the TUI viewport.
type BroadcastMode int

// ==========================================
// Constants
// ==========================================

const (
	BroadcastModeGlobal      BroadcastMode = 0
	BroadcastModeSubscriber BroadcastMode = 1
	BroadcastModeBroadcast  BroadcastMode = 2
)

// ==========================================
// Constructor
// ==========================================

// NewBroadcastMode creates a BroadcastMode from its string representation.
// Returns BroadcastModeGlobal for unrecognized strings.
func NewBroadcastMode(mode string) BroadcastMode {
	switch mode {
	case "Global":
		return BroadcastModeGlobal
	case "Subscriber":
		return BroadcastModeSubscriber
	case "Broadcast":
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
		return "Subscriber"
	case BroadcastModeBroadcast:
		return "Broadcast"
	default:
		return "Global"
	}
}

// ==========================================
// Navigation
// ==========================================

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
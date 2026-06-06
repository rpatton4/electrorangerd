package ui

// Mode identifies one of the four primary application modes. Mode is a UI
// concern; the domain layer never carries this enum. Per CLAUDE.md, rule-
// selection in the domain happens through domain.ValidationProfile instead.
//
// The canonical names used in conversation and documentation are
// "Diagram Mode", "Forward Mode", "Dictionary Mode", and "Reverse Mode".
// In code the identifiers are bare (ModeDiagram, etc.) because the type
// name already supplies the "Mode" qualifier.
type Mode int

const (
	ModeDiagram Mode = iota
	ModeForward
	ModeDictionary
	ModeReverse
)

// Label returns a short human-readable name suitable for the top mode nav.
// The bare label is intentional — the nav IS the mode switcher, so the
// "Mode" suffix would be redundant on the button.
func (m Mode) Label() string {
	switch m {
	case ModeDiagram:
		return "Diagram"
	case ModeForward:
		return "Forward"
	case ModeDictionary:
		return "Dictionary"
	case ModeReverse:
		return "Reverse"
	default:
		return "Unknown"
	}
}

package ui

// Mode identifies one of the four primary application modes. Mode is a UI
// concern; the domain layer never carries this enum. Per CLAUDE.md, rule-
// selection in the domain happens through domain.ValidationProfile instead.
type Mode int

const (
	ModeDiagramEdit Mode = iota
	ModeForwardEngineering
	ModeDataDictionary
	ModeReverseEngineering
)

// Label returns a short human-readable name suitable for the top mode nav.
func (m Mode) Label() string {
	switch m {
	case ModeDiagramEdit:
		return "Diagram"
	case ModeForwardEngineering:
		return "Forward"
	case ModeDataDictionary:
		return "Dictionary"
	case ModeReverseEngineering:
		return "Reverse"
	default:
		return "Unknown"
	}
}

// allModes is the canonical ordering used by the mode navigator.
var allModes = [...]Mode{
	ModeDiagramEdit,
	ModeForwardEngineering,
	ModeDataDictionary,
	ModeReverseEngineering,
}

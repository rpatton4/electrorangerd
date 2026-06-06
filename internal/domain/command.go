package domain

// CommandKind identifies the type of schema-editing operation a Command represents.
type CommandKind int

const (
	CmdAddEntity CommandKind = iota
	CmdRemoveEntity
	CmdUpdateEntity
	CmdMoveEntity
	CmdAddAttribute
	CmdRemoveAttribute
	CmdAddRelationship
	CmdRemoveRelationship
	CmdUpdateRelationship
)

// Command is a unit of work on the History stack. Per-mode scoping of the
// undo/redo stack happens in Plan C inside adapter/ui; the domain Command
// shape itself is mode-agnostic.
type Command struct {
	Kind        CommandKind
	Description string
}

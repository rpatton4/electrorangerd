package domain

// CommandKind identifies the type of schema-editing operation a Command represents.
type CommandKind int

const (
	CmdAddEntity CommandKind = iota
	CmdRemoveEntity
	CmdAddAttribute
	CmdRemoveAttribute
	CmdAddRelationship
	CmdRemoveRelationship
)

// Command represents a discrete, reversible schema-editing operation used by
// the undo/redo history stack.
type Command struct {
	Kind        CommandKind
	Description string
}

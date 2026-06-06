package ui

import (
	"context"
	"image"
	"image/color"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/canvas"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/dialog"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

const (
	defaultDatabase       = "default"
	defaultSchema         = "public"
	entityNamePlaceholder = "Name"

	doubleClickInterval = 350 * time.Millisecond
	doubleClickRadius   = float32(5)

	entityBoxWDp   = unit.Dp(220)
	entityHeaderDp = unit.Dp(28)
	entityRowDp    = unit.Dp(24)
	entityKeyColDp = unit.Dp(28)
)

// editKind names what the diagramView is currently editing in-line.
type editKind int

const (
	editNone editKind = iota
	editHeader
	editAttrName
)

// editTarget addresses what is currently being edited or what the active
// context menu was opened on. kind selects the sub-area, entityID identifies
// the entity (the diagram-stable EntityID), attrIdx selects the attribute
// row when relevant.
type editTarget struct {
	kind     editKind
	entityID domain.EntityID
	attrIdx  int
}

// entitySub names which sub-area of an entity a pointer hit.
type entitySub int

const (
	subNone entitySub = iota
	subHeader
	subKeyMarker
	subAttrName
)

// diagramView is the per-mode view for ModeDiagram. It owns the in-memory
// domain.Project (entities, attributes, relationships, AND per-entity
// placements via project.Diagram), the canvas renderer, four context menus,
// the in-line text editor, and the diagram-local History stack. All
// project mutations go through the DiagramEditor port; the view never
// mutates project state in place.
type diagramView struct {
	diagramEditor port.DiagramEditor
	history       port.History
	canvas        *canvas.Canvas

	project domain.Project

	menu           *dialog.ContextMenu
	keyMenu        *dialog.ContextMenu
	addRowMenu     *dialog.ContextMenu
	rowMenu        *dialog.ContextMenu
	relMenu        *dialog.ContextMenu
	relDialog      *dialog.Modal
	sourceDropdown *dialog.Dropdown
	targetDropdown *dialog.Dropdown
	inputTag       struct{}

	// Drag state. dragPos is the in-flight ghost position updated every
	// pointer.Drag event; on Release it is committed through the port via
	// UpdatePlacement. Holding the ghost UI-local avoids a Project
	// allocation per frame during a drag.
	dragging bool
	dragID   domain.EntityID
	dragOff  f32.Point
	dragPos  domain.Position

	edit             editTarget
	editor           widget.Editor
	keyMenuTarget    editTarget
	addRowMenuTarget domain.EntityID
	rowMenuTarget    editTarget

	hovering bool
	hoverPos f32.Point

	// selected identifies the currently selected entity. Selection is
	// UI-only state — the domain doesn't carry a notion of selection,
	// because it's a view concern, not a project-state concern. Zero
	// means no selection.
	selected domain.EntityID

	// selectedRel identifies the currently selected relationship line.
	// Mutually exclusive with selected: clicking one clears the other.
	// Zero means no relationship is selected.
	selectedRel domain.RelationshipID

	// Relationship-drawing state. Active between a click on a handle of
	// the selected entity and the next click. linkCursor follows the
	// pointer so the in-flight line can be rendered from the clicked
	// handle to the current cursor.
	linking        bool
	linkFromID     domain.EntityID
	linkFromHandle int
	linkCursor     f32.Point

	// Relationship-dialog state. relDialogID is the relationship whose
	// metadata the modal is currently showing; zero when closed.
	relDialogID domain.RelationshipID

	lastPressAt  time.Time
	lastPressPos f32.Point
}

func newDiagramView(de port.DiagramEditor, history port.History) *diagramView {
	v := &diagramView{
		diagramEditor: de,
		history:       history,
		canvas:        canvas.New(),
		project: domain.Project{
			Name: "Untitled",
			Databases: []domain.Database{
				{Name: defaultDatabase, Schemas: []domain.Schema{{Name: defaultSchema}}},
			},
			Diagram: domain.Diagram{Placements: map[domain.EntityID]domain.Position{}},
		},
		menu:       dialog.NewContextMenu("Create Entity"),
		keyMenu:    dialog.NewContextMenu("PK", "FK", "IK"),
		addRowMenu: dialog.NewContextMenu("Add row (a)"),
		rowMenu:    dialog.NewContextMenu("Add row (a)", "Delete row (d)"),
		relMenu:    dialog.NewContextMenu("Remove (del)"),
		relDialog:  dialog.NewModal("Relationship Details"),
		sourceDropdown: dialog.NewDropdown("Select cardinality",
			domain.CrowsFootZeroOrOne.String(),
			domain.CrowsFootOne.String(),
			domain.CrowsFootZeroOrMany.String(),
			domain.CrowsFootMany.String(),
			domain.CrowsFootOneAndOnlyOne.String(),
			domain.CrowsFootOneOrMany.String(),
		),
		targetDropdown: dialog.NewDropdown("Select cardinality",
			domain.CrowsFootZeroOrOne.String(),
			domain.CrowsFootOne.String(),
			domain.CrowsFootZeroOrMany.String(),
			domain.CrowsFootMany.String(),
			domain.CrowsFootOneAndOnlyOne.String(),
			domain.CrowsFootOneOrMany.String(),
		),
	}
	v.editor.SingleLine = true
	v.editor.Submit = true
	return v
}

func (v *diagramView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &v.inputTag)

	// Focus routing: the inline editor owns focus while editing so it can
	// receive text; otherwise the diagram's own input tag holds focus so 'c'
	// fires and pointer events have a target with key context.
	if v.edit.kind != editNone {
		if !gtx.Focused(&v.editor) {
			gtx.Execute(key.FocusCmd{Tag: &v.editor})
		}
	} else if !gtx.Focused(&v.inputTag) {
		gtx.Execute(key.FocusCmd{Tag: &v.inputTag})
	}

	allMods := key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper

	v.handlePointer(gtx)
	v.handleKeys(gtx, allMods)
	v.handleEditorEvents(gtx)

	palette := entityPalette(th)
	v.layoutRelationships(gtx, palette)
	for _, rec := range domain.EntitiesIn(v.project) {
		ent := rec.Entity
		pos, ok := v.entityPosition(ent.ID)
		if !ok {
			continue
		}
		stk := op.Affine(f32.Affine2D{}.Offset(f32.Pt(pos.X, pos.Y))).Push(gtx.Ops)
		v.canvas.Entity(gtx, th.Material, palette, v.entityForRender(ent))
		if v.selected == ent.ID {
			v.canvas.EntitySelection(gtx, palette, ent)
		}
		if v.edit.kind != editNone && v.edit.entityID == ent.ID {
			v.layoutEditOverlay(gtx, th)
		}
		stk.Pop()
	}
	if v.linking {
		v.layoutLinkInFlight(gtx, palette)
	}

	if sel := v.menu.Layout(gtx, th); sel == 0 {
		v.createEntity(gtx, v.menu.Anchor())
	}
	if sel := v.keyMenu.Layout(gtx, th); sel >= 0 {
		v.applyKeyMarker(sel)
	}
	if sel := v.addRowMenu.Layout(gtx, th); sel == 0 {
		v.addRow(gtx, v.addRowMenuTarget)
		v.addRowMenuTarget = 0
	}
	if sel := v.rowMenu.Layout(gtx, th); sel >= 0 {
		switch sel {
		case 0:
			v.addRow(gtx, v.rowMenuTarget.entityID)
		case 1:
			v.deleteRow(gtx, v.rowMenuTarget.entityID, v.rowMenuTarget.attrIdx)
		}
		v.rowMenuTarget = editTarget{}
	}
	if sel := v.relMenu.Layout(gtx, th); sel == 0 {
		v.deleteRelationship(v.selectedRel)
	}

	saved, cancelled := v.relDialog.Layout(gtx, th, func(gtx layout.Context) layout.Dimensions {
		return v.layoutRelDialogBody(gtx, th)
	})
	if saved {
		v.saveRelationshipDialog()
		v.relDialogID = 0
	}
	if cancelled {
		v.relDialogID = 0
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (v *diagramView) handlePointer(gtx layout.Context) {
	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: &v.inputTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			v.hoverPos = pe.Position
			v.hovering = true
			if v.linking {
				v.linkCursor = pe.Position
			}
			if v.relDialog.IsOpen() {
				// Modal eats the press — its scrim and buttons own
				// every click while it's up.
				continue
			}
			v.onPress(gtx, pe)
		case pointer.Drag:
			v.hoverPos = pe.Position
			v.hovering = true
			if v.linking {
				v.linkCursor = pe.Position
			}
			if v.dragging && v.dragID != 0 {
				v.dragPos = domain.Position{
					X: pe.Position.X - v.dragOff.X,
					Y: pe.Position.Y - v.dragOff.Y,
				}
			}
		case pointer.Move, pointer.Enter:
			v.hoverPos = pe.Position
			v.hovering = true
			if v.linking {
				v.linkCursor = pe.Position
			}
		case pointer.Leave:
			v.hovering = false
		case pointer.Release, pointer.Cancel:
			v.endDrag(pe.Kind == pointer.Release)
		}
	}
}

// endDrag commits the in-flight drag position through the DiagramEditor on
// a successful Release; a Cancel discards the drag without touching state.
func (v *diagramView) endDrag(commit bool) {
	if !v.dragging {
		return
	}
	id := v.dragID
	pos := v.dragPos
	v.dragging = false
	v.dragID = 0
	if !commit {
		return
	}
	updated, err := v.diagramEditor.UpdatePlacement(context.TODO(), v.project, id, pos)
	if err != nil {
		return
	}
	v.project = updated
	v.history.Push(domain.Command{Kind: domain.CmdMoveEntity, Description: "Move entity"})
}

func (v *diagramView) onPress(gtx layout.Context, pe pointer.Event) {
	at := image.Pt(int(pe.Position.X), int(pe.Position.Y))

	if pe.Buttons&pointer.ButtonSecondary != 0 {
		v.commitEdit()
		v.keyMenu.Close()
		v.menu.Close()
		v.addRowMenu.Close()
		v.rowMenu.Close()
		v.relMenu.Close()
		// Right-click is contextual: on a row of an entity it offers
		// Add row + Delete row; on an entity's header it offers Add row
		// only; on a relationship line it offers Remove; on empty
		// diagram space it offers Create Entity.
		eID, sub, aIdx := v.hitSubArea(gtx, pe.Position)
		switch {
		case eID == 0:
			if relID, ok := v.hitRelationshipLine(gtx, pe.Position); ok {
				v.selectedRel = relID
				v.selected = 0
				v.relMenu.Open(at)
			} else {
				v.menu.Open(at)
			}
		case sub == subKeyMarker || sub == subAttrName:
			v.rowMenuTarget = editTarget{entityID: eID, attrIdx: aIdx}
			v.rowMenu.Open(at)
		default:
			v.addRowMenuTarget = eID
			v.addRowMenu.Open(at)
		}
		return
	}
	if pe.Buttons&pointer.ButtonPrimary == 0 {
		return
	}

	// If a menu is open, swallow this primary press here. The menu's own
	// scrim handles off-menu dismissal and its items handle selection —
	// closing it from this handler would short-circuit those drains in the
	// same frame and drop the click on the floor.
	if v.menu.IsOpen() || v.keyMenu.IsOpen() || v.addRowMenu.IsOpen() || v.rowMenu.IsOpen() || v.relMenu.IsOpen() {
		return
	}

	// Primary press. If an inline editor is open and the click landed
	// outside it, commit the edit and fall through; if it landed inside,
	// let the editor handle it without starting a drag.
	if v.edit.kind != editNone {
		if r, ok := v.editingRect(gtx); ok && pointInRect(pe.Position, r) {
			return
		}
		v.commitEdit()
	}

	// Relationship-drawing handling takes precedence over double-click,
	// drag, and selection so the user can chase a handle with a quick
	// follow-up click without it being interpreted as something else.
	if v.linking {
		v.completeLink(gtx, pe.Position)
		return
	}

	// A click on one of the currently-selected entity's handles starts
	// drawing a relationship instead of moving the entity.
	if v.selected != 0 {
		if _, h, ok := v.hitHandle(gtx, pe.Position); ok {
			v.startLink(v.selected, h, pe.Position)
			return
		}
	}

	isDouble := v.trackPrimaryClick(pe.Position, gtx.Now)
	if isDouble {
		if eID, _ := v.hitEntity(gtx, pe.Position); eID != 0 {
			v.openSubEditor(gtx, pe.Position)
			return
		}
		if relID, ok := v.hitRelationshipLine(gtx, pe.Position); ok {
			v.openRelationshipDialog(relID)
		}
		return
	}

	if id, hit := v.hitEntity(gtx, pe.Position); hit {
		v.selected = id
		v.selectedRel = 0
		pos, _ := v.project.Diagram.Placements[id]
		v.dragging = true
		v.dragID = id
		v.dragPos = pos
		v.dragOff = f32.Point{
			X: pe.Position.X - pos.X,
			Y: pe.Position.Y - pos.Y,
		}
	} else if relID, ok := v.hitRelationshipLine(gtx, pe.Position); ok {
		v.selected = 0
		v.selectedRel = relID
	} else {
		v.selected = 0
		v.selectedRel = 0
	}
}

func (v *diagramView) handleKeys(gtx layout.Context, allMods key.Modifiers) {
	for {
		ev, ok := gtx.Source.Event(
			key.Filter{Name: "C", Optional: allMods},
			key.Filter{Name: "A", Optional: allMods},
			key.Filter{Name: "D", Optional: allMods},
			key.Filter{Name: key.NameEscape, Optional: allMods},
			key.Filter{Name: key.NameDeleteForward, Optional: allMods},
			key.Filter{Name: key.NameDeleteBackward, Optional: allMods},
		)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		if ke.State != key.Press {
			continue
		}
		switch ke.Name {
		case "C":
			// While editing, 'c' is text input for the editor — ignore here.
			if v.edit.kind == editNone && ke.Modifiers == 0 {
				v.createEntity(gtx, canvasCentreTopLeft(gtx))
			}
		case "A":
			// 'a' is the "(a)" shortcut on the entity context menu. It adds
			// a row to the entity under the cursor. While editing, defer to
			// the inline editor's text input.
			if v.edit.kind == editNone && ke.Modifiers == 0 && v.hovering {
				if id, hit := v.hitEntity(gtx, v.hoverPos); hit {
					v.addRow(gtx, id)
				}
			}
		case "D":
			// 'd' is the "(d)" shortcut on the row context menu. It removes
			// the row under the cursor (only meaningful on a row, not the
			// header). While editing, defer to the inline editor's text input.
			if v.edit.kind == editNone && ke.Modifiers == 0 && v.hovering {
				if eID, sub, aIdx := v.hitSubArea(gtx, v.hoverPos); eID != 0 && (sub == subKeyMarker || sub == subAttrName) {
					v.deleteRow(gtx, eID, aIdx)
				}
			}
		case key.NameDeleteForward, key.NameDeleteBackward:
			// Delete / Backspace removes the currently-selected
			// relationship. While editing, defer to the inline editor
			// so the user can edit text normally.
			if v.edit.kind == editNone && ke.Modifiers == 0 && v.selectedRel != 0 {
				v.deleteRelationship(v.selectedRel)
			}
		case key.NameEscape:
			switch {
			case v.relDialog.IsOpen():
				v.relDialog.Close()
				v.relDialogID = 0
			case v.linking:
				v.cancelLink()
			case v.edit.kind != editNone:
				v.cancelEdit()
			case v.menu.IsOpen():
				v.menu.Close()
			case v.keyMenu.IsOpen():
				v.keyMenu.Close()
			case v.addRowMenu.IsOpen():
				v.addRowMenu.Close()
			case v.rowMenu.IsOpen():
				v.rowMenu.Close()
			case v.relMenu.IsOpen():
				v.relMenu.Close()
			}
		}
	}
}

func (v *diagramView) handleEditorEvents(gtx layout.Context) {
	if v.edit.kind == editNone {
		return
	}
	for {
		ev, ok := v.editor.Update(gtx)
		if !ok {
			break
		}
		if _, isSubmit := ev.(widget.SubmitEvent); isSubmit {
			v.commitEdit()
		}
	}
}

func (v *diagramView) openSubEditor(gtx layout.Context, at f32.Point) {
	eID, sub, aIdx := v.hitSubArea(gtx, at)
	if eID == 0 {
		return
	}
	ent, _, _, ok := domain.FindEntity(v.project, eID)
	if !ok {
		return
	}
	switch sub {
	case subHeader:
		v.beginTextEdit(editTarget{kind: editHeader, entityID: eID}, ent.Name)
	case subAttrName:
		if aIdx < 0 || aIdx >= len(ent.Attributes) {
			return
		}
		v.beginTextEdit(editTarget{kind: editAttrName, entityID: eID, attrIdx: aIdx}, ent.Attributes[aIdx].Name)
	case subKeyMarker:
		v.keyMenuTarget = editTarget{entityID: eID, attrIdx: aIdx}
		v.keyMenu.Open(image.Pt(int(at.X), int(at.Y)))
	}
}

func (v *diagramView) beginTextEdit(target editTarget, initial string) {
	v.menu.Close()
	v.keyMenu.Close()
	v.edit = target
	v.editor.SetText(initial)
	v.editor.SetCaret(len([]rune(initial)), 0)
}

func (v *diagramView) commitEdit() {
	if v.edit.kind == editNone {
		return
	}
	id := v.edit.entityID
	if id == 0 {
		v.edit = editTarget{}
		return
	}
	ent, _, _, ok := domain.FindEntity(v.project, id)
	if !ok {
		v.edit = editTarget{}
		return
	}

	text := v.editor.Text()
	cmdKind := domain.CmdUpdateEntity

	switch v.edit.kind {
	case editHeader:
		ent.Name = text
	case editAttrName:
		aIdx := v.edit.attrIdx
		if aIdx < 0 || aIdx >= len(ent.Attributes) {
			v.edit = editTarget{}
			return
		}
		attrs := append([]domain.Attribute(nil), ent.Attributes...)
		attrs[aIdx].Name = text
		ent.Attributes = attrs
	default:
		v.edit = editTarget{}
		return
	}

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, id, ent)
	if err == nil {
		v.project = updated
		v.history.Push(domain.Command{Kind: cmdKind, Description: "Edit entity"})
	}
	v.edit = editTarget{}
}

func (v *diagramView) cancelEdit() {
	v.edit = editTarget{}
}

func (v *diagramView) applyKeyMarker(sel int) {
	target := v.keyMenuTarget
	v.keyMenuTarget = editTarget{}
	if target.entityID == 0 {
		return
	}
	ent, _, _, ok := domain.FindEntity(v.project, target.entityID)
	if !ok {
		return
	}
	if target.attrIdx < 0 || target.attrIdx >= len(ent.Attributes) {
		return
	}

	var kind domain.KeyKind
	switch sel {
	case 0:
		kind = domain.KeyPrimary
	case 1:
		kind = domain.KeyForeign
	case 2:
		kind = domain.KeyIndex
	default:
		return
	}

	attrs := append([]domain.Attribute(nil), ent.Attributes...)
	attrs[target.attrIdx].KeyKind = kind
	ent.Attributes = attrs

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, target.entityID, ent)
	if err == nil {
		v.project = updated
		v.history.Push(domain.Command{Kind: domain.CmdUpdateEntity, Description: "Set key marker"})
	}
}

// deleteRelationship removes the relationship at id via the
// DiagramEditor port, clears the selection if it pointed at the deleted
// relationship, and records a CmdRemoveRelationship history entry.
func (v *diagramView) deleteRelationship(id domain.RelationshipID) {
	if id == 0 {
		return
	}
	updated, err := v.diagramEditor.DeleteRelationship(context.TODO(), v.project, id)
	if err != nil {
		return
	}
	v.project = updated
	if v.selectedRel == id {
		v.selectedRel = 0
	}
	v.history.Push(domain.Command{Kind: domain.CmdRemoveRelationship, Description: "Remove relationship"})
}

// hitRelationshipLine returns the ID of the relationship whose currently
// rendered line is within a forgiving tolerance of p, if any. Used to
// detect double-click targets on the lines.
func (v *diagramView) hitRelationshipLine(gtx layout.Context, p f32.Point) (domain.RelationshipID, bool) {
	tolerance := float32(gtx.Dp(unit.Dp(6)))
	tol2 := tolerance * tolerance
	for _, rel := range v.project.Relationships {
		fromEnt, _, _, ok := domain.FindEntity(v.project, rel.From.Entity)
		if !ok {
			continue
		}
		toEnt, _, _, ok := domain.FindEntity(v.project, rel.To.Entity)
		if !ok {
			continue
		}
		fromPos, ok := v.entityPosition(rel.From.Entity)
		if !ok {
			continue
		}
		toPos, ok := v.entityPosition(rel.To.Entity)
		if !ok {
			continue
		}
		fh, th := v.nearestHandlePair(gtx, fromEnt, fromPos, toEnt, toPos)
		fx, fy := canvas.HandlePosition(gtx, fromEnt, fh)
		tx, ty := canvas.HandlePosition(gtx, toEnt, th)
		a := f32.Pt(fromPos.X+fx, fromPos.Y+fy)
		b := f32.Pt(toPos.X+tx, toPos.Y+ty)
		if distanceSquaredToSegment(p, a, b) <= tol2 {
			return rel.ID, true
		}
	}
	return 0, false
}

// distanceSquaredToSegment returns the squared distance from p to the
// line segment a→b. Uses the standard projection-clamp formulation.
func distanceSquaredToSegment(p, a, b f32.Point) float32 {
	abx := b.X - a.X
	aby := b.Y - a.Y
	ab2 := abx*abx + aby*aby
	if ab2 == 0 {
		dx := p.X - a.X
		dy := p.Y - a.Y
		return dx*dx + dy*dy
	}
	apx := p.X - a.X
	apy := p.Y - a.Y
	t := (apx*abx + apy*aby) / ab2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	cx := a.X + abx*t
	cy := a.Y + aby*t
	dx := p.X - cx
	dy := p.Y - cy
	return dx*dx + dy*dy
}

// crowsFootOptions enumerates the 6 dropdown choices in the order they
// appear in the dialog. It is the source of truth for the index-to-enum
// mapping used by both Open (initialising dropdowns) and Save (reading
// them back).
var crowsFootOptions = []domain.CrowsFoot{
	domain.CrowsFootZeroOrOne,
	domain.CrowsFootOne,
	domain.CrowsFootZeroOrMany,
	domain.CrowsFootMany,
	domain.CrowsFootOneAndOnlyOne,
	domain.CrowsFootOneOrMany,
}

func indexOfCrowsFoot(cf domain.CrowsFoot) int {
	for i, opt := range crowsFootOptions {
		if opt == cf {
			return i
		}
	}
	return -1
}

func crowsFootFromIndex(i int) domain.CrowsFoot {
	if i < 0 || i >= len(crowsFootOptions) {
		return domain.CrowsFootUnspecified
	}
	return crowsFootOptions[i]
}

// openRelationshipDialog targets the modal at the given relationship and
// shows it. The source / target dropdowns are pre-selected to match the
// relationship's current cardinalities so the user can edit from the
// existing state.
func (v *diagramView) openRelationshipDialog(id domain.RelationshipID) {
	rel, _, ok := domain.FindRelationship(v.project, id)
	if !ok {
		return
	}
	v.menu.Close()
	v.keyMenu.Close()
	v.addRowMenu.Close()
	v.rowMenu.Close()
	v.sourceDropdown.Close()
	v.targetDropdown.Close()
	v.cancelLink()
	v.sourceDropdown.SetSelected(indexOfCrowsFoot(rel.SourceCardinality))
	v.targetDropdown.SetSelected(indexOfCrowsFoot(rel.TargetCardinality))
	v.relDialogID = id
	v.relDialog.Open()
}

// saveRelationshipDialog reads the dropdown selections and pushes the
// updated cardinalities through the DiagramEditor, replacing v.project on
// success and pushing a CmdUpdateRelationship onto history.
func (v *diagramView) saveRelationshipDialog() {
	rel, _, ok := domain.FindRelationship(v.project, v.relDialogID)
	if !ok {
		return
	}
	rel.SourceCardinality = crowsFootFromIndex(v.sourceDropdown.Selected())
	rel.TargetCardinality = crowsFootFromIndex(v.targetDropdown.Selected())
	updated, err := v.diagramEditor.UpdateRelationship(context.TODO(), v.project, v.relDialogID, rel)
	if err != nil {
		return
	}
	v.project = updated
	v.history.Push(domain.Command{Kind: domain.CmdUpdateRelationship, Description: "Update relationship cardinalities"})
}

// layoutRelDialogBody renders the modal's middle content as a 3-column
// row: column 1 holds the source name and dropdown, column 2 the word
// "to" centred vertically against the dropdowns, column 3 the target
// name and dropdown.
func (v *diagramView) layoutRelDialogBody(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	sourceName, targetName := "", ""
	if rel, _, ok := domain.FindRelationship(v.project, v.relDialogID); ok {
		if ent, _, _, ok := domain.FindEntity(v.project, rel.From.Entity); ok {
			sourceName = ent.Name
		}
		if ent, _, _, ok := domain.FindEntity(v.project, rel.To.Entity); ok {
			targetName = ent.Name
		}
	}
	column := func(name string, dd *dialog.Dropdown) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Body1(th.Material, name).Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return dd.Layout(gtx, th)
				}),
			)
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, column("Source: "+sourceName, v.sourceDropdown)),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Align "to" vertically with the dropdown bar by mirroring
			// the column's "name + spacer" prefix height.
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(material.Body1(th.Material, "").Layout),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body1(th.Material, "to").Layout)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Flexed(1, column("Target: "+targetName, v.targetDropdown)),
	)
}

// hitHandle reports the selected entity's handle index at p, if any, with
// a forgiving radius slightly larger than the visual circle so handles at
// box corners don't require pixel-perfect aim.
func (v *diagramView) hitHandle(gtx layout.Context, p f32.Point) (id domain.EntityID, handle int, ok bool) {
	if v.selected == 0 {
		return 0, -1, false
	}
	ent, _, _, found := domain.FindEntity(v.project, v.selected)
	if !found {
		return 0, -1, false
	}
	pos, ok := v.entityPosition(v.selected)
	if !ok {
		return 0, -1, false
	}
	radius := float32(gtx.Dp(unit.Dp(8)))
	r2 := radius * radius
	for h := 0; h < canvas.HandleCount; h++ {
		hx, hy := canvas.HandlePosition(gtx, ent, h)
		dx := p.X - (pos.X + hx)
		dy := p.Y - (pos.Y + hy)
		if dx*dx+dy*dy <= r2 {
			return v.selected, h, true
		}
	}
	return 0, -1, false
}

// nearestHandle returns the handle index on the given entity whose centre
// is closest to p.
func (v *diagramView) nearestHandle(gtx layout.Context, id domain.EntityID, p f32.Point) int {
	ent, _, _, ok := domain.FindEntity(v.project, id)
	if !ok {
		return -1
	}
	pos, ok := v.entityPosition(id)
	if !ok {
		return -1
	}
	best := 0
	var bestD2 float32 = 1e18
	for h := 0; h < canvas.HandleCount; h++ {
		hx, hy := canvas.HandlePosition(gtx, ent, h)
		dx := p.X - (pos.X + hx)
		dy := p.Y - (pos.Y + hy)
		d2 := dx*dx + dy*dy
		if d2 < bestD2 {
			bestD2 = d2
			best = h
		}
	}
	return best
}

// nearestHandlePair returns the (from, to) handle indices on the two
// entities whose centres minimise the squared distance — used to
// auto-route relationship lines so they re-anchor as entities move.
func (v *diagramView) nearestHandlePair(gtx layout.Context, fromEnt domain.Entity, fromPos domain.Position, toEnt domain.Entity, toPos domain.Position) (int, int) {
	bestFrom, bestTo := 0, 0
	var bestD2 float32 = 1e18
	for fh := 0; fh < canvas.HandleCount; fh++ {
		fx, fy := canvas.HandlePosition(gtx, fromEnt, fh)
		fxAbs := fromPos.X + fx
		fyAbs := fromPos.Y + fy
		for th := 0; th < canvas.HandleCount; th++ {
			tx, ty := canvas.HandlePosition(gtx, toEnt, th)
			dx := (toPos.X + tx) - fxAbs
			dy := (toPos.Y + ty) - fyAbs
			d2 := dx*dx + dy*dy
			if d2 < bestD2 {
				bestD2 = d2
				bestFrom = fh
				bestTo = th
			}
		}
	}
	return bestFrom, bestTo
}

// startLink begins drawing a relationship line out of the handle at index
// h on entity id. The line follows the cursor (tracked via Move/Drag
// events) until the user completes or cancels.
func (v *diagramView) startLink(id domain.EntityID, h int, cursor f32.Point) {
	v.menu.Close()
	v.keyMenu.Close()
	v.addRowMenu.Close()
	v.rowMenu.Close()
	v.linking = true
	v.linkFromID = id
	v.linkFromHandle = h
	v.linkCursor = cursor
}

// cancelLink discards any in-flight relationship-drawing state.
func (v *diagramView) cancelLink() {
	v.linking = false
	v.linkFromID = 0
	v.linkFromHandle = 0
}

// completeLink tries to attach the in-flight line to whichever entity sits
// under p. Clicking on the source entity, on empty space, or on a target
// where the service refuses (e.g. duplicate endpoints) cancels the link.
func (v *diagramView) completeLink(gtx layout.Context, p f32.Point) {
	if !v.linking {
		return
	}
	id, hit := v.hitEntity(gtx, p)
	if !hit || id == v.linkFromID {
		v.cancelLink()
		return
	}
	rel := domain.Relationship{
		From: domain.RelationshipEndpoint{Entity: v.linkFromID},
		To:   domain.RelationshipEndpoint{Entity: id},
	}
	updated, _, err := v.diagramEditor.AddRelationship(context.TODO(), v.project, rel)
	if err == nil {
		v.project = updated
		v.history.Push(domain.Command{Kind: domain.CmdAddRelationship, Description: "Add relationship"})
	}
	v.cancelLink()
}

// layoutRelationships draws every stored relationship as a straight line
// between the nearest pair of handles on its two endpoint entities, so
// lines re-route automatically as entities are moved.
func (v *diagramView) layoutRelationships(gtx layout.Context, palette canvas.EntityPalette) {
	for _, rel := range v.project.Relationships {
		fromEnt, _, _, ok := domain.FindEntity(v.project, rel.From.Entity)
		if !ok {
			continue
		}
		toEnt, _, _, ok := domain.FindEntity(v.project, rel.To.Entity)
		if !ok {
			continue
		}
		fromPos, ok := v.entityPosition(rel.From.Entity)
		if !ok {
			continue
		}
		toPos, ok := v.entityPosition(rel.To.Entity)
		if !ok {
			continue
		}
		fh, th := v.nearestHandlePair(gtx, fromEnt, fromPos, toEnt, toPos)
		fx, fy := canvas.HandlePosition(gtx, fromEnt, fh)
		tx, ty := canvas.HandlePosition(gtx, toEnt, th)
		from := f32.Pt(fromPos.X+fx, fromPos.Y+fy)
		to := f32.Pt(toPos.X+tx, toPos.Y+ty)
		v.canvas.RelationshipLine(gtx, palette, from, to, v.selectedRel == rel.ID)
		v.canvas.RelationshipMarker(gtx, palette, from, to, rel.SourceCardinality)
		v.canvas.RelationshipMarker(gtx, palette, to, from, rel.TargetCardinality)
	}
}

// layoutLinkInFlight draws the line currently being dragged out of a
// handle, from the originally-clicked handle to the live cursor.
func (v *diagramView) layoutLinkInFlight(gtx layout.Context, palette canvas.EntityPalette) {
	ent, _, _, ok := domain.FindEntity(v.project, v.linkFromID)
	if !ok {
		return
	}
	pos, ok := v.entityPosition(v.linkFromID)
	if !ok {
		return
	}
	hx, hy := canvas.HandlePosition(gtx, ent, v.linkFromHandle)
	v.canvas.RelationshipLine(gtx, palette,
		f32.Pt(pos.X+hx, pos.Y+hy),
		v.linkCursor,
		false,
	)
}

// entityForRender returns a copy of ent with the text being edited blanked
// out, so the inline editor can paint its own text on top without the
// underlying static text bleeding through. The chrome (rows, dividers,
// outline) is unchanged — only the text glyphs for the active field are
// suppressed. Returns ent unchanged when no edit is in flight on it.
func (v *diagramView) entityForRender(ent domain.Entity) domain.Entity {
	if v.edit.kind == editNone || v.edit.entityID != ent.ID {
		return ent
	}
	switch v.edit.kind {
	case editHeader:
		ent.Name = ""
	case editAttrName:
		aIdx := v.edit.attrIdx
		if aIdx < 0 || aIdx >= len(ent.Attributes) {
			return ent
		}
		attrs := append([]domain.Attribute(nil), ent.Attributes...)
		attrs[aIdx].Name = ""
		ent.Attributes = attrs
	}
	return ent
}

// entityPosition returns the screen-space placement to render and hit-test
// the entity at. While a drag is in flight the dragged entity's position
// is the transient ghost (v.dragPos); every other entity reads from
// project.Diagram.Placements.
func (v *diagramView) entityPosition(id domain.EntityID) (domain.Position, bool) {
	if v.dragging && v.dragID == id {
		return v.dragPos, true
	}
	pos, ok := v.project.Diagram.Placements[id]
	return pos, ok
}

// entityBox materialises an entity's full bounding box from its placement
// plus its current attribute count. Width is constant; height grows with
// the number of attribute rows.
func (v *diagramView) entityBox(gtx layout.Context, ent domain.Entity, pos domain.Position) (left, top, width, height float32) {
	boxW := float32(gtx.Dp(entityBoxWDp))
	headerH := float32(gtx.Dp(entityHeaderDp))
	rowH := float32(gtx.Dp(entityRowDp))
	return pos.X, pos.Y, boxW, headerH + float32(len(ent.Attributes))*rowH
}

// hitEntity returns the EntityID of the topmost entity whose bounding box
// contains p. Iterates entity insertion order in reverse so later-drawn
// entities win z-order on overlap.
func (v *diagramView) hitEntity(gtx layout.Context, p f32.Point) (domain.EntityID, bool) {
	records := domain.EntitiesIn(v.project)
	for i := len(records) - 1; i >= 0; i-- {
		ent := records[i].Entity
		pos, ok := v.entityPosition(ent.ID)
		if !ok {
			continue
		}
		l, t, w, h := v.entityBox(gtx, ent, pos)
		if p.X >= l && p.X < l+w && p.Y >= t && p.Y < t+h {
			return ent.ID, true
		}
	}
	return 0, false
}

// hitSubArea identifies which sub-area of which entity a pointer landed on.
// Returns the zero EntityID and subNone for clicks that don't fall on a
// meaningful editable area.
func (v *diagramView) hitSubArea(gtx layout.Context, p f32.Point) (id domain.EntityID, sub entitySub, attrIdx int) {
	eID, ok := v.hitEntity(gtx, p)
	if !ok {
		return 0, subNone, -1
	}
	ent, _, _, ok := domain.FindEntity(v.project, eID)
	if !ok {
		return 0, subNone, -1
	}
	pos, ok := v.entityPosition(eID)
	if !ok {
		return 0, subNone, -1
	}
	localX := p.X - pos.X
	localY := p.Y - pos.Y

	headerH := float32(gtx.Dp(entityHeaderDp))
	rowH := float32(gtx.Dp(entityRowDp))
	keyW := float32(gtx.Dp(entityKeyColDp))

	if localY < headerH {
		return eID, subHeader, -1
	}
	aIdx := int((localY - headerH) / rowH)
	if aIdx < 0 || aIdx >= len(ent.Attributes) {
		return eID, subNone, -1
	}
	if localX < keyW {
		return eID, subKeyMarker, aIdx
	}
	return eID, subAttrName, aIdx
}

// editingRect returns the screen-space rectangle of the currently active
// inline editor, or false if no inline edit is open or the underlying
// entity / placement is missing.
func (v *diagramView) editingRect(gtx layout.Context) (image.Rectangle, bool) {
	if v.edit.kind == editNone || v.edit.entityID == 0 {
		return image.Rectangle{}, false
	}
	pos, ok := v.entityPosition(v.edit.entityID)
	if !ok {
		return image.Rectangle{}, false
	}
	headerH := gtx.Dp(entityHeaderDp)
	rowH := gtx.Dp(entityRowDp)
	boxW := gtx.Dp(entityBoxWDp)
	keyW := gtx.Dp(entityKeyColDp)
	x0 := int(pos.X)
	y0 := int(pos.Y)

	switch v.edit.kind {
	case editHeader:
		return image.Rect(x0, y0, x0+boxW, y0+headerH), true
	case editAttrName:
		yTop := y0 + headerH + v.edit.attrIdx*rowH
		return image.Rect(x0+keyW, yTop, x0+boxW, yTop+rowH), true
	}
	return image.Rectangle{}, false
}

// trackPrimaryClick records each primary press and reports whether it is
// the second in a quick double-click (close together in time and space).
func (v *diagramView) trackPrimaryClick(at f32.Point, now time.Time) bool {
	if !v.lastPressAt.IsZero() && now.Sub(v.lastPressAt) < doubleClickInterval {
		dx := at.X - v.lastPressPos.X
		dy := at.Y - v.lastPressPos.Y
		if dx*dx+dy*dy <= doubleClickRadius*doubleClickRadius {
			v.lastPressAt = time.Time{}
			v.lastPressPos = f32.Point{}
			return true
		}
	}
	v.lastPressAt = now
	v.lastPressPos = at
	return false
}

// addRow appends a new empty Attribute to the entity at id via the
// DiagramEditor port.
func (v *diagramView) addRow(_ layout.Context, id domain.EntityID) {
	if id == 0 {
		return
	}
	ent, _, _, ok := domain.FindEntity(v.project, id)
	if !ok {
		return
	}
	attrs := append([]domain.Attribute(nil), ent.Attributes...)
	attrs = append(attrs, domain.Attribute{})
	ent.Attributes = attrs

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, id, ent)
	if err != nil {
		return
	}
	v.project = updated
	v.history.Push(domain.Command{Kind: domain.CmdAddAttribute, Description: "Add row"})
}

// deleteRow removes the Attribute at attrIdx from the entity at id via the
// DiagramEditor port. If the inline editor is open on the row being
// removed, the edit state is cleared first.
func (v *diagramView) deleteRow(_ layout.Context, id domain.EntityID, attrIdx int) {
	if id == 0 {
		return
	}
	ent, _, _, ok := domain.FindEntity(v.project, id)
	if !ok {
		return
	}
	if attrIdx < 0 || attrIdx >= len(ent.Attributes) {
		return
	}
	attrs := append([]domain.Attribute(nil), ent.Attributes...)
	attrs = append(attrs[:attrIdx], attrs[attrIdx+1:]...)
	ent.Attributes = attrs

	if v.edit.kind == editAttrName && v.edit.entityID == id && v.edit.attrIdx == attrIdx {
		v.edit = editTarget{}
	}

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, id, ent)
	if err != nil {
		return
	}
	v.project = updated
	v.history.Push(domain.Command{Kind: domain.CmdRemoveAttribute, Description: "Delete row"})
}

func (v *diagramView) createEntity(_ layout.Context, at image.Point) {
	entity := domain.Entity{
		Name:       entityNamePlaceholder,
		Attributes: []domain.Attribute{{}},
	}
	updated, id, err := v.diagramEditor.AddEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, entity)
	if err != nil {
		return
	}
	v.project = updated

	placed, err := v.diagramEditor.UpdatePlacement(context.TODO(), v.project, id, domain.Position{X: float32(at.X), Y: float32(at.Y)})
	if err == nil {
		v.project = placed
	}

	v.history.Push(domain.Command{Kind: domain.CmdAddEntity, Description: "Add entity"})

	// Select the new entity and open the header editor so the user can
	// type the entity's name right away. Focus is routed to the editor
	// automatically by Layout while v.edit.kind != editNone.
	v.selected = id
	v.selectedRel = 0
	v.beginTextEdit(editTarget{kind: editHeader, entityID: id}, entityNamePlaceholder)
}

// layoutEditOverlay paints an opaque background and the inline editor over
// the entity's sub-area being edited. The current transform origin is the
// entity's top-left in screen space.
func (v *diagramView) layoutEditOverlay(gtx layout.Context, th *theme.Theme) {
	headerH := gtx.Dp(entityHeaderDp)
	rowH := gtx.Dp(entityRowDp)
	boxW := gtx.Dp(entityBoxWDp)
	keyW := gtx.Dp(entityKeyColDp)

	pad := gtx.Dp(unit.Dp(8))
	var area image.Rectangle
	switch v.edit.kind {
	case editHeader:
		area = image.Rect(pad, 0, boxW-pad, headerH)
	case editAttrName:
		y0 := headerH + v.edit.attrIdx*rowH
		area = image.Rect(keyW+pad, y0, boxW-pad, y0+rowH)
	default:
		return
	}
	alignment := text.Start

	stk := op.Affine(f32.Affine2D{}.Offset(f32.Pt(float32(area.Min.X), float32(area.Min.Y)))).Push(gtx.Ops)
	defer stk.Pop()

	sz := image.Pt(area.Dx(), area.Dy())

	// No background fill, no outline. The entity render already skips
	// the text for the field being edited (see entityForRender) so the
	// row dividers and column lines stay intact; the editor paints its
	// glyphs and caret directly on top.

	// Record at natural size and centre vertically — see canvas/entity.go's
	// textInArea for why widget.Label's reported height needs this dance.
	macro := op.Record(gtx.Ops)
	local := gtx
	local.Constraints = layout.Constraints{Max: sz}
	v.editor.Alignment = alignment
	ed := material.Editor(th.Material, &v.editor, "")
	ed.Color = th.OnSurface
	dims := ed.Layout(local)
	call := macro.Stop()
	dy := (sz.Y - dims.Size.Y) / 2
	if dy < 0 {
		dy = 0
	}
	offset := op.Affine(f32.Affine2D{}.Offset(f32.Pt(0, float32(dy)))).Push(gtx.Ops)
	call.Add(gtx.Ops)
	offset.Pop()
}

func pointInRect(p f32.Point, r image.Rectangle) bool {
	return p.X >= float32(r.Min.X) && p.X < float32(r.Max.X) && p.Y >= float32(r.Min.Y) && p.Y < float32(r.Max.Y)
}

func canvasCentreTopLeft(gtx layout.Context) image.Point {
	boxW := gtx.Dp(entityBoxWDp)
	headerH := gtx.Dp(entityHeaderDp)
	rowH := gtx.Dp(entityRowDp)
	boxH := headerH + rowH
	return image.Pt(gtx.Constraints.Max.X/2-boxW/2, gtx.Constraints.Max.Y/2-boxH/2)
}

func entityPalette(th *theme.Theme) canvas.EntityPalette {
	return canvas.EntityPalette{
		Surface:       th.SurfaceContainer,
		HeaderSurface: th.SurfaceVariant,
		Stroke:        th.Outline,
		OnSurface:     th.OnSurface,
		Shadow:        color.NRGBA{A: 0x40},
		Selection:     th.Primary,
	}
}

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
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/canvas"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/dialog"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/diagram"
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

// editTarget addresses what is currently being edited: a kind tells us
// which sub-area, entityIdx selects the entity in the default schema, and
// attrIdx selects the attribute row when relevant.
type editTarget struct {
	kind      editKind
	entityIdx int
	attrIdx   int
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
// domain.Project, the per-entity placement boxes, the flat canvas renderer,
// the right-click context menu, the in-line text editor + key-marker
// dropdown, and the diagram-local History stack.
type diagramView struct {
	diagramEditor port.DiagramEditor
	history       port.History
	canvas        *canvas.Canvas

	project domain.Project
	boxes   []diagram.Box

	menu       *dialog.ContextMenu
	keyMenu    *dialog.ContextMenu
	addRowMenu *dialog.ContextMenu
	rowMenu    *dialog.ContextMenu
	inputTag   struct{}

	dragging bool
	dragIdx  int
	dragOff  f32.Point

	edit             editTarget
	editor           widget.Editor
	keyMenuTarget    editTarget
	addRowMenuTarget int
	rowMenuTarget    editTarget

	hovering bool
	hoverPos f32.Point

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
		},
		menu:             dialog.NewContextMenu("Create Entity"),
		keyMenu:          dialog.NewContextMenu("PK", "FK", "IK"),
		addRowMenu:       dialog.NewContextMenu("Add row (a)"),
		rowMenu:          dialog.NewContextMenu("Add row (a)", "Delete row (d)"),
		addRowMenuTarget: -1,
		rowMenuTarget:    editTarget{entityIdx: -1, attrIdx: -1},
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
	for i := range v.entities() {
		ent := v.entities()[i]
		box := v.boxes[i]
		stk := op.Affine(f32.Affine2D{}.Offset(f32.Pt(box.X, box.Y))).Push(gtx.Ops)
		v.canvas.Entity(gtx, th.Material, palette, ent)
		if v.edit.kind != editNone && v.edit.entityIdx == i {
			v.layoutEditOverlay(gtx, th)
		}
		stk.Pop()
	}

	if sel := v.menu.Layout(gtx, th); sel == 0 {
		v.createEntity(gtx, v.menu.Anchor())
	}
	if sel := v.keyMenu.Layout(gtx, th); sel >= 0 {
		v.applyKeyMarker(sel)
	}
	if sel := v.addRowMenu.Layout(gtx, th); sel == 0 {
		v.addRow(gtx, v.addRowMenuTarget)
		v.addRowMenuTarget = -1
	}
	if sel := v.rowMenu.Layout(gtx, th); sel >= 0 {
		switch sel {
		case 0:
			v.addRow(gtx, v.rowMenuTarget.entityIdx)
		case 1:
			v.deleteRow(gtx, v.rowMenuTarget.entityIdx, v.rowMenuTarget.attrIdx)
		}
		v.rowMenuTarget = editTarget{entityIdx: -1, attrIdx: -1}
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
			v.onPress(gtx, pe)
		case pointer.Drag:
			v.hoverPos = pe.Position
			v.hovering = true
			if v.dragging && v.dragIdx >= 0 && v.dragIdx < len(v.boxes) {
				v.boxes[v.dragIdx].Position = diagram.Position{
					X: pe.Position.X - v.dragOff.X,
					Y: pe.Position.Y - v.dragOff.Y,
				}
			}
		case pointer.Move, pointer.Enter:
			v.hoverPos = pe.Position
			v.hovering = true
		case pointer.Leave:
			v.hovering = false
		case pointer.Release, pointer.Cancel:
			v.dragging = false
		}
	}
}

func (v *diagramView) onPress(gtx layout.Context, pe pointer.Event) {
	at := image.Pt(int(pe.Position.X), int(pe.Position.Y))

	if pe.Buttons&pointer.ButtonSecondary != 0 {
		v.commitEdit()
		v.keyMenu.Close()
		v.menu.Close()
		v.addRowMenu.Close()
		v.rowMenu.Close()
		// Right-click is contextual: on a row of an entity it offers
		// Add row + Delete row; on an entity's header it offers Add row
		// only; on empty diagram space it offers Create Entity.
		eIdx, sub, aIdx := v.hitSubArea(gtx, pe.Position)
		switch {
		case eIdx < 0:
			v.menu.Open(at)
		case sub == subKeyMarker || sub == subAttrName:
			v.rowMenuTarget = editTarget{entityIdx: eIdx, attrIdx: aIdx}
			v.rowMenu.Open(at)
		default:
			v.addRowMenuTarget = eIdx
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
	if v.menu.IsOpen() || v.keyMenu.IsOpen() || v.addRowMenu.IsOpen() || v.rowMenu.IsOpen() {
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

	isDouble := v.trackPrimaryClick(pe.Position, gtx.Now)
	if isDouble {
		v.openSubEditor(gtx, pe.Position)
		return
	}

	if idx, hit := v.hitEntity(pe.Position); hit {
		v.dragging = true
		v.dragIdx = idx
		v.dragOff = f32.Point{
			X: pe.Position.X - v.boxes[idx].X,
			Y: pe.Position.Y - v.boxes[idx].Y,
		}
	}
}

func (v *diagramView) handleKeys(gtx layout.Context, allMods key.Modifiers) {
	for {
		ev, ok := gtx.Source.Event(
			key.Filter{Name: "C", Optional: allMods},
			key.Filter{Name: "A", Optional: allMods},
			key.Filter{Name: "D", Optional: allMods},
			key.Filter{Name: key.NameEscape, Optional: allMods},
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
				if idx, hit := v.hitEntity(v.hoverPos); hit {
					v.addRow(gtx, idx)
				}
			}
		case "D":
			// 'd' is the "(d)" shortcut on the row context menu. It removes
			// the row under the cursor (only meaningful on a row, not the
			// header). While editing, defer to the inline editor's text input.
			if v.edit.kind == editNone && ke.Modifiers == 0 && v.hovering {
				if eIdx, sub, aIdx := v.hitSubArea(gtx, v.hoverPos); eIdx >= 0 && (sub == subKeyMarker || sub == subAttrName) {
					v.deleteRow(gtx, eIdx, aIdx)
				}
			}
		case key.NameEscape:
			if v.edit.kind != editNone {
				v.cancelEdit()
			} else if v.menu.IsOpen() {
				v.menu.Close()
			} else if v.keyMenu.IsOpen() {
				v.keyMenu.Close()
			} else if v.addRowMenu.IsOpen() {
				v.addRowMenu.Close()
			} else if v.rowMenu.IsOpen() {
				v.rowMenu.Close()
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
	eIdx, sub, aIdx := v.hitSubArea(gtx, at)
	switch sub {
	case subHeader:
		v.beginTextEdit(editTarget{kind: editHeader, entityIdx: eIdx}, v.entities()[eIdx].Name)
	case subAttrName:
		v.beginTextEdit(editTarget{kind: editAttrName, entityIdx: eIdx, attrIdx: aIdx}, v.entities()[eIdx].Attributes[aIdx].Name)
	case subKeyMarker:
		v.keyMenuTarget = editTarget{entityIdx: eIdx, attrIdx: aIdx}
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
	eIdx := v.edit.entityIdx
	if eIdx < 0 || eIdx >= len(v.entities()) {
		v.edit = editTarget{}
		return
	}

	text := v.editor.Text()
	ent := v.entities()[eIdx]

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

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, eIdx, ent)
	if err == nil {
		v.project = updated
	}
	v.edit = editTarget{}
}

func (v *diagramView) cancelEdit() {
	v.edit = editTarget{}
}

func (v *diagramView) applyKeyMarker(sel int) {
	defer func() {
		v.keyMenuTarget = editTarget{}
	}()
	if v.keyMenuTarget.entityIdx < 0 || v.keyMenuTarget.entityIdx >= len(v.entities()) {
		return
	}
	ent := v.entities()[v.keyMenuTarget.entityIdx]
	if v.keyMenuTarget.attrIdx < 0 || v.keyMenuTarget.attrIdx >= len(ent.Attributes) {
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
	attrs[v.keyMenuTarget.attrIdx].KeyKind = kind
	ent.Attributes = attrs

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, v.keyMenuTarget.entityIdx, ent)
	if err == nil {
		v.project = updated
	}
}

func (v *diagramView) entities() []domain.Entity {
	return v.project.Databases[0].Schemas[0].Entities
}

// hitEntity returns the index of the topmost entity whose placement box
// contains p, or (-1, false) if none. Iterates in reverse z-order so a
// later-drawn entity wins over an earlier one when they overlap.
func (v *diagramView) hitEntity(p f32.Point) (int, bool) {
	for i := len(v.boxes) - 1; i >= 0; i-- {
		b := v.boxes[i]
		if p.X >= b.X && p.X < b.X+b.Width && p.Y >= b.Y && p.Y < b.Y+b.Height {
			return i, true
		}
	}
	return -1, false
}

// hitSubArea identifies which sub-area of which entity a pointer landed on.
// Returns subNone for clicks that don't fall on a meaningful editable area.
func (v *diagramView) hitSubArea(gtx layout.Context, p f32.Point) (entityIdx int, sub entitySub, attrIdx int) {
	eIdx, ok := v.hitEntity(p)
	if !ok {
		return -1, subNone, -1
	}
	box := v.boxes[eIdx]
	localX := p.X - box.X
	localY := p.Y - box.Y

	headerH := float32(gtx.Dp(entityHeaderDp))
	rowH := float32(gtx.Dp(entityRowDp))
	keyW := float32(gtx.Dp(entityKeyColDp))

	if localY < headerH {
		return eIdx, subHeader, -1
	}
	aIdx := int((localY - headerH) / rowH)
	attrs := v.entities()[eIdx].Attributes
	if aIdx < 0 || aIdx >= len(attrs) {
		return eIdx, subNone, -1
	}
	if localX < keyW {
		return eIdx, subKeyMarker, aIdx
	}
	return eIdx, subAttrName, aIdx
}

// editingRect returns the screen-space rectangle of the currently active
// inline editor, or false if no inline edit is open.
func (v *diagramView) editingRect(gtx layout.Context) (image.Rectangle, bool) {
	if v.edit.kind == editNone {
		return image.Rectangle{}, false
	}
	if v.edit.entityIdx < 0 || v.edit.entityIdx >= len(v.boxes) {
		return image.Rectangle{}, false
	}
	box := v.boxes[v.edit.entityIdx]
	headerH := gtx.Dp(entityHeaderDp)
	rowH := gtx.Dp(entityRowDp)
	boxW := gtx.Dp(entityBoxWDp)
	keyW := gtx.Dp(entityKeyColDp)

	switch v.edit.kind {
	case editHeader:
		return image.Rect(int(box.X), int(box.Y), int(box.X)+boxW, int(box.Y)+headerH), true
	case editAttrName:
		y0 := int(box.Y) + headerH + v.edit.attrIdx*rowH
		return image.Rect(int(box.X)+keyW, y0, int(box.X)+boxW, y0+rowH), true
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

// addRow appends a new empty Attribute to the entity at entityIdx via the
// DiagramEditor port and grows the entity's placement box height to match.
func (v *diagramView) addRow(gtx layout.Context, entityIdx int) {
	if entityIdx < 0 || entityIdx >= len(v.entities()) {
		return
	}
	ent := v.entities()[entityIdx]
	attrs := append([]domain.Attribute(nil), ent.Attributes...)
	attrs = append(attrs, domain.Attribute{})
	ent.Attributes = attrs

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, entityIdx, ent)
	if err != nil {
		return
	}
	v.project = updated

	rowH := gtx.Dp(entityRowDp)
	v.boxes[entityIdx].Height += float32(rowH)
	v.history.Push(domain.Command{Kind: domain.CmdAddAttribute, Description: "Add row"})
}

// deleteRow removes the Attribute at attrIdx from the entity at entityIdx
// via the DiagramEditor port and shrinks the entity's placement box height
// by one row.
func (v *diagramView) deleteRow(gtx layout.Context, entityIdx, attrIdx int) {
	if entityIdx < 0 || entityIdx >= len(v.entities()) {
		return
	}
	ent := v.entities()[entityIdx]
	if attrIdx < 0 || attrIdx >= len(ent.Attributes) {
		return
	}
	attrs := append([]domain.Attribute(nil), ent.Attributes...)
	attrs = append(attrs[:attrIdx], attrs[attrIdx+1:]...)
	ent.Attributes = attrs

	// If the inline editor is open on the row being removed, drop the edit
	// so the next frame doesn't try to render an overlay at a stale index.
	if v.edit.kind == editAttrName && v.edit.entityIdx == entityIdx && v.edit.attrIdx == attrIdx {
		v.edit = editTarget{}
	}

	updated, err := v.diagramEditor.UpdateEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, entityIdx, ent)
	if err != nil {
		return
	}
	v.project = updated

	rowH := gtx.Dp(entityRowDp)
	v.boxes[entityIdx].Height -= float32(rowH)
	if v.boxes[entityIdx].Height < 0 {
		v.boxes[entityIdx].Height = 0
	}
	v.history.Push(domain.Command{Kind: domain.CmdRemoveAttribute, Description: "Delete row"})
}

func (v *diagramView) createEntity(gtx layout.Context, at image.Point) {
	entity := domain.Entity{
		Name:       entityNamePlaceholder,
		Attributes: []domain.Attribute{{}},
	}
	updated, err := v.diagramEditor.AddEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, entity)
	if err != nil {
		return
	}
	v.project = updated

	boxW := gtx.Dp(entityBoxWDp)
	headerH := gtx.Dp(entityHeaderDp)
	rowH := gtx.Dp(entityRowDp)
	boxH := headerH + len(entity.Attributes)*rowH
	v.boxes = append(v.boxes, diagram.Box{
		Position: diagram.Position{X: float32(at.X), Y: float32(at.Y)},
		Width:    float32(boxW),
		Height:   float32(boxH),
	})

	v.history.Push(domain.Command{Kind: domain.CmdAddEntity, Description: "Add entity"})
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
	bgClip := clip.Rect{Max: sz}.Push(gtx.Ops)
	paint.ColorOp{Color: th.SurfaceContainer}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	bgClip.Pop()

	stroke := gtx.Dp(unit.Dp(1))
	drawRectStroke(gtx, sz, stroke, th.Primary)

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

func drawRectStroke(gtx layout.Context, size image.Point, stroke int, col color.NRGBA) {
	fill := func(r image.Rectangle) {
		defer clip.Rect{Min: r.Min, Max: r.Max}.Push(gtx.Ops).Pop()
		paint.ColorOp{Color: col}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)
	}
	fill(image.Rect(0, 0, size.X, stroke))
	fill(image.Rect(0, size.Y-stroke, size.X, size.Y))
	fill(image.Rect(0, 0, stroke, size.Y))
	fill(image.Rect(size.X-stroke, 0, size.X, size.Y))
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
	}
}

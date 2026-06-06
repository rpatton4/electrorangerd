package ui

import (
	"context"
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/canvas"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/dialog"
	"github.com/InfiniteSkye/electrorangerd/internal/adapter/ui/theme"
	"github.com/InfiniteSkye/electrorangerd/internal/diagram"
	"github.com/InfiniteSkye/electrorangerd/internal/domain"
	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

const (
	defaultDatabase = "default"
	defaultSchema   = "public"
	entityNamePlaceholder = "Name"
)

// diagramView is the per-mode view for ModeDiagram. It owns the in-memory
// domain.Project, the per-entity placement boxes, the flat canvas renderer,
// the right-click context menu, and the diagram-local History stack. Input
// (right-click + 'c' key + click-drag) is captured by registering the full
// main-area rectangle as an event target.
type diagramView struct {
	editor  port.DiagramEditor
	history port.History
	canvas  *canvas.Canvas

	project domain.Project
	boxes   []diagram.Box

	menu     *dialog.ContextMenu
	inputTag struct{}

	dragging bool
	dragIdx  int
	dragOff  f32.Point
}

func newDiagramView(editor port.DiagramEditor, history port.History) *diagramView {
	return &diagramView{
		editor:  editor,
		history: history,
		canvas:  canvas.New(),
		project: domain.Project{
			Name: "Untitled",
			Databases: []domain.Database{
				{Name: defaultDatabase, Schemas: []domain.Schema{{Name: defaultSchema}}},
			},
		},
		menu: dialog.NewContextMenu("Create Entity"),
	}
}

func (v *diagramView) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, &v.inputTag)

	if !gtx.Focused(&v.inputTag) {
		gtx.Execute(key.FocusCmd{Tag: &v.inputTag})
	}

	allMods := key.ModCtrl | key.ModCommand | key.ModShift | key.ModAlt | key.ModSuper

	for {
		ev, ok := gtx.Source.Event(pointer.Filter{
			Target: &v.inputTag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
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
			at := image.Pt(int(pe.Position.X), int(pe.Position.Y))
			switch {
			case pe.Buttons&pointer.ButtonSecondary != 0:
				v.menu.Open(at)
			case pe.Buttons&pointer.ButtonPrimary != 0:
				if v.menu.IsOpen() {
					v.menu.Close()
				} else if idx, hit := v.hitEntity(pe.Position); hit {
					v.dragging = true
					v.dragIdx = idx
					v.dragOff = f32.Point{
						X: pe.Position.X - v.boxes[idx].X,
						Y: pe.Position.Y - v.boxes[idx].Y,
					}
				}
			}
		case pointer.Drag:
			if v.dragging && v.dragIdx >= 0 && v.dragIdx < len(v.boxes) {
				v.boxes[v.dragIdx].Position = diagram.Position{
					X: pe.Position.X - v.dragOff.X,
					Y: pe.Position.Y - v.dragOff.Y,
				}
			}
		case pointer.Release, pointer.Cancel:
			v.dragging = false
		}
	}

	for {
		ev, ok := gtx.Source.Event(key.Filter{
			Name:     "C",
			Optional: allMods,
		})
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok {
			continue
		}
		if ke.State == key.Press && ke.Modifiers == 0 {
			v.createEntity(gtx, canvasCentreTopLeft(gtx))
		}
	}

	palette := entityPalette(th)
	for i := range v.entities() {
		ent := v.entities()[i]
		box := v.boxes[i]
		stk := op.Affine(f32.Affine2D{}.Offset(f32.Pt(box.X, box.Y))).Push(gtx.Ops)
		v.canvas.Entity(gtx, th.Material, palette, ent)
		stk.Pop()
	}

	if sel := v.menu.Layout(gtx, th); sel == 0 {
		v.createEntity(gtx, v.menu.Anchor())
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
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

func (v *diagramView) createEntity(gtx layout.Context, at image.Point) {
	entity := domain.Entity{
		Name:       entityNamePlaceholder,
		Attributes: []domain.Attribute{{}},
	}
	updated, err := v.editor.AddEntity(context.TODO(), v.project, defaultDatabase, defaultSchema, entity)
	if err != nil {
		return
	}
	v.project = updated

	boxW := gtx.Dp(unit.Dp(220))
	headerH := gtx.Dp(unit.Dp(28))
	rowH := gtx.Dp(unit.Dp(24))
	boxH := headerH + len(entity.Attributes)*rowH
	v.boxes = append(v.boxes, diagram.Box{
		Position: diagram.Position{X: float32(at.X), Y: float32(at.Y)},
		Width:    float32(boxW),
		Height:   float32(boxH),
	})

	v.history.Push(domain.Command{Kind: domain.CmdAddEntity, Description: "Add entity"})
}

func canvasCentreTopLeft(gtx layout.Context) image.Point {
	boxW := gtx.Dp(unit.Dp(220))
	headerH := gtx.Dp(unit.Dp(28))
	rowH := gtx.Dp(unit.Dp(24))
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

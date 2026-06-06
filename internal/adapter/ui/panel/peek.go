package panel

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"github.com/rpatton4/electrorangerd/internal/adapter/ui/theme"
	"github.com/rpatton4/electrorangerd/internal/port"
)

// Peek is the slide-out side panel that shows a single dictionary entry for
// the schema element currently selected in the active mode. The Open flag
// is owned by the caller (App) so mode views can react to its state without
// reaching back into the panel.
type Peek struct {
	Open       bool
	Dictionary port.DictionaryService
}

// New constructs a Peek panel wired to the given DictionaryService.
func New(dict port.DictionaryService) *Peek {
	return &Peek{Dictionary: dict}
}

// Layout draws the panel. When closed it occupies zero space; when open it
// shows a fixed-width column with a placeholder for the dictionary entry
// (real content arrives once a selection-aware mode wires it up).
func (p *Peek) Layout(gtx layout.Context, th *theme.Theme) layout.Dimensions {
	if !p.Open {
		return layout.Dimensions{}
	}
	const widthDp = 280

	gtx.Constraints.Max.X = gtx.Dp(unit.Dp(widthDp))
	gtx.Constraints.Min.X = gtx.Constraints.Max.X

	return layout.Background{}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			paintRect(gtx, th.SurfaceVariant)
			return layout.Dimensions{Size: gtx.Constraints.Max}
		},
		func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceBetween}.Layout(gtx,
					layout.Rigid(material.H6(th.Material, "Peek").Layout),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(material.Body2(th.Material, "Select a schema element to view its data-dictionary entry here.").Layout),
				)
			})
		},
	)
}

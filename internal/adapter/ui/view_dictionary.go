package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
	"gioui.org/x/markdown"
	"gioui.org/x/richtext"

	"github.com/InfiniteSkye/electrorangerd/internal/port"
)

const sampleMarkdown = `# Customer entity

The **Customer** entity records every party that has placed an order or
been invited to do so. PII fields (` + "`email`" + `, ` + "`phone`" + `) are
flagged in the data dictionary so downstream consumers can apply masking
when surfacing records.

- Primary key: ` + "`customer_id`" + ` (bigint)
- Created at: ` + "`created_at`" + ` (timestamptz, not null)
- Soft-delete: ` + "`deleted_at`" + ` (timestamptz, nullable)
`

// dictionaryView is the per-mode view for ModeDataDictionary. It uses the
// gioui.org/x/markdown renderer to display dictionary entry descriptions as
// Gio richtext. The dictionary-local History stack lives here; real editing
// UI arrives in a future iteration.
type dictionaryView struct {
	dictionary    port.DictionaryService
	history       port.History
	renderer      *markdown.Renderer
	richtextState richtext.InteractiveText
	cached        []richtext.SpanStyle
}

func newDictionaryView(dict port.DictionaryService, history port.History, th *material.Theme) *dictionaryView {
	v := &dictionaryView{
		dictionary: dict,
		history:    history,
		renderer:   newMarkdownRenderer(th),
	}
	v.rebuild()
	return v
}

func (v *dictionaryView) rebuild() {
	spans, err := v.renderer.Render([]byte(sampleMarkdown))
	if err == nil {
		v.cached = spans
	}
}

func (v *dictionaryView) Layout(gtx layout.Context, th *material.Theme) layout.Dimensions {
	if len(v.cached) == 0 {
		v.rebuild()
	}
	return layout.UniformInset(unit.Dp(24)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(material.H4(th, "Data Dictionary").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(material.Body2(th, "Descriptions are authored in Markdown and rendered via gioui.org/x/markdown.").Layout),
			layout.Rigid(layout.Spacer{Height: unit.Dp(24)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if len(v.cached) == 0 {
					return material.Body2(th, "(no entry selected)").Layout(gtx)
				}
				return richtext.Text(&v.richtextState, th.Shaper, v.cached...).Layout(gtx)
			}),
		)
	})
}

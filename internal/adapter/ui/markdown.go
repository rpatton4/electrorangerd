package ui

import (
	"gioui.org/widget/material"
	"gioui.org/x/markdown"
)

// newMarkdownRenderer constructs the Gio richtext-producing markdown renderer
// used by every panel that renders user-authored Markdown content (data
// dictionary descriptions, in-app help). The renderer is safe for concurrent
// use from a single goroutine; per CLAUDE.md, UI rendering happens on the
// window goroutine.
func newMarkdownRenderer(_ *material.Theme) *markdown.Renderer {
	return markdown.NewRenderer()
}

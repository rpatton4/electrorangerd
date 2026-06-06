package ui

import (
	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Theme wraps a Gio material theme. It is constructed once at startup and
// shared across all UI components owned by App.
type Theme struct {
	Material *material.Theme
}

// NewTheme constructs a Theme seeded with the standard Go font collection so
// that text rendering works out of the box without requiring system fonts.
func NewTheme() *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	return &Theme{Material: th}
}

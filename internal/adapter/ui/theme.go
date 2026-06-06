package ui

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/widget/material"
)

// Theme wraps a Gio material theme. It is constructed once at startup and
// shared across all UI components owned by App.
type Theme struct {
	Material *material.Theme
}

// NewTheme constructs a Theme seeded with the standard Go font collection
// and a dark-mode palette so that material text widgets render legibly on
// the app's dark background. ContrastBg/ContrastFg are left at the material
// defaults (indigo accent with white foreground) — those drive button chrome.
func NewTheme() *Theme {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	th.Bg = color.NRGBA{R: 0x0A, G: 0x0B, B: 0x10, A: 0xFF}
	th.Fg = color.NRGBA{R: 0xEC, G: 0xEC, B: 0xF1, A: 0xFF}
	return &Theme{Material: th}
}

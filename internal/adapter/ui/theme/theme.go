// Package theme defines the ElectroRangerD UI theme: Material 3 semantic
// colour roles + a small set of size tokens, plus factories for the dark
// and light variants. Sibling of internal/adapter/ui/panel, dialog, canvas
// so all UI subpackages can consume tokens without import cycles.
package theme

import (
	"image/color"

	"gioui.org/font/gofont"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Theme bundles Material 3 colour-role tokens, a layout size scale, and the
// underlying *material.Theme that gio's material widgets consume directly.
type Theme struct {
	Material *material.Theme

	Primary          color.NRGBA
	OnPrimary        color.NRGBA
	Surface          color.NRGBA
	OnSurface        color.NRGBA
	SurfaceVariant   color.NRGBA
	OnSurfaceVariant color.NRGBA
	SurfaceContainer color.NRGBA
	Outline          color.NRGBA
	Error            color.NRGBA
	OnError          color.NRGBA

	Sizes Sizes
}

// Sizes is the project's spacing / radius / stroke scale. Widget-specific
// layout constants (drawer height, peek panel width, animation durations)
// stay local to their owning package — they are not theme tokens.
type Sizes struct {
	SpaceSM, SpaceMD, SpaceLG, SpaceXL unit.Dp
	RadiusSM                           unit.Dp
	StrokeThin                         unit.Dp
}

// NewDarkTheme returns the dark variant. Colours match the previously
// hardcoded values across the UI so the dark theme renders identically to
// behaviour before this consolidation.
func NewDarkTheme() *Theme {
	return assemble(&Theme{
		Primary:          nrgba(0x7F9CF4),
		OnPrimary:        nrgba(0xFFFFFF),
		Surface:          nrgba(0x0A0B10),
		OnSurface:        nrgba(0xECECF1),
		SurfaceVariant:   nrgba(0x1A1A22),
		OnSurfaceVariant: nrgba(0x7A7B85),
		SurfaceContainer: nrgba(0x12141C),
		Outline:          nrgba(0x4A4B58),
		Error:            nrgba(0xF56565),
		OnError:          nrgba(0xFFFFFF),
	})
}

// NewLightTheme returns the light variant. First-pass colour choices — tune
// as needed once the dark/light parity is visually validated.
func NewLightTheme() *Theme {
	return assemble(&Theme{
		Primary:          nrgba(0x4F46E5),
		OnPrimary:        nrgba(0xFFFFFF),
		Surface:          nrgba(0xFAFAFA),
		OnSurface:        nrgba(0x1A1A22),
		SurfaceVariant:   nrgba(0xF0F0F4),
		OnSurfaceVariant: nrgba(0x5A5B65),
		SurfaceContainer: nrgba(0xFFFFFF),
		Outline:          nrgba(0xC0C1C9),
		Error:            nrgba(0xDC2626),
		OnError:          nrgba(0xFFFFFF),
	})
}

// NewBlackAndChromeTheme returns a strict-grayscale variant derived from the
// dark theme, with a polished-silver Primary. Every colour is R = G = B
// (no cool or warm tint). Error sacrifices its red cue to satisfy the
// grayscale constraint — it falls back to pure white for maximum contrast
// against the near-black surface.
func NewBlackAndChromeTheme() *Theme {
	return assemble(&Theme{
		Primary:          nrgba(0xC8C8C8),
		OnPrimary:        nrgba(0x0A0A0A),
		Surface:          nrgba(0x0A0A0A),
		OnSurface:        nrgba(0xECECEC),
		SurfaceVariant:   nrgba(0x1A1A1A),
		OnSurfaceVariant: nrgba(0x7A7A7A),
		SurfaceContainer: nrgba(0x121212),
		Outline:          nrgba(0x4A4A4A),
		Error:            nrgba(0xFFFFFF),
		OnError:          nrgba(0x0A0A0A),
	})
}

// assemble wires the *material.Theme (with font shaper + palette derived
// from M3 roles) onto t and stamps the default size scale. Both factories
// share this so the material setup never diverges.
func assemble(t *Theme) *Theme {
	m := material.NewTheme()
	m.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	m.Bg = t.Surface
	m.Fg = t.OnSurface
	m.ContrastBg = t.Primary
	m.ContrastFg = t.OnPrimary
	t.Material = m
	t.Sizes = Sizes{
		SpaceSM:    8,
		SpaceMD:    16,
		SpaceLG:    24,
		SpaceXL:    32,
		RadiusSM:   4,
		StrokeThin: 1,
	}
	return t
}

func nrgba(rgb uint32) color.NRGBA {
	return color.NRGBA{
		R: uint8(rgb >> 16),
		G: uint8(rgb >> 8),
		B: uint8(rgb),
		A: 0xFF,
	}
}

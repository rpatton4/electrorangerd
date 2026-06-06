// Package ui is the primary (driving) adapter — the GIOUI desktop interface.
// It calls inbound port interfaces and owns all presentation concerns,
// including the axonometric shear that gives the ERD canvas its 3D-styled
// look, layered shadows for depth cueing, sheared Bezier connectors, and
// clipped translate-Y drawer animations. Entity title text is rendered
// un-sheared (Gio rasterizes glyphs upright). This package never contains
// business logic — all behavior flows through the port interfaces it
// consumes.
package ui

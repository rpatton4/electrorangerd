// Package canvas is the 2.5D ERD canvas. It applies the axonometric shear
// (via f32.Affine2D) at render time so entity boxes look 3D-styled while
// staying inside Gio's 2D pipeline. Entity title text is rendered un-sheared
// per the convention documented in CLAUDE.md — Gio rasterizes glyphs upright
// and sheared text would look broken.
package canvas

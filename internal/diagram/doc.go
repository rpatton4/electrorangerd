// Package diagram holds pure, presentation-free layout and hit-testing for
// the ERD canvas. Coordinates here are logical 2D — they describe where
// entities and edges sit in the model's own coordinate system, with no
// view-space transforms applied. The axonometric shear that gives the
// canvas its 3D-styled look is applied in internal/adapter/ui at render
// time; mouse hit-testing must invert that shear before calling functions
// in this package. Do not pollute this package with Gio types, view
// matrices, or presentation concerns — it must stay testable without a
// UI context.
package diagram

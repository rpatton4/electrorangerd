package diagram

import "github.com/rpatton4/electrorangerd/internal/domain"

// Box is the bounding rectangle of a single entity node on the canvas. It
// pairs the entity's top-left placement with the box's dimensions. The
// placement itself lives on domain.Diagram.Placements; Box materialises it
// alongside size for layout and hit-test consumers.
type Box struct {
	domain.Position
	Width, Height float32
}

// Auto computes an initial layout for every entity in a Project, returning a
// map from EntityID to its assigned Box. The current implementation is a
// stub that returns nil; the real algorithm (force-directed or grid) will
// replace it without changing the signature.
func Auto(_ domain.Project) map[domain.EntityID]Box {
	return nil
}

package diagram

import "github.com/InfiniteSkye/electrorangerd/internal/domain"

// Position is a 2-D point on the ERD canvas, expressed in device-independent
// pixels. Both axes increase toward the bottom-right.
type Position struct {
	X, Y float32
}

// Box is the bounding rectangle of a single entity node on the canvas. It
// embeds Position for the top-left corner and adds the node dimensions.
type Box struct {
	Position
	Width, Height float32
}

// Auto computes an initial layout for all entities in schema, returning a map
// from entity name to its assigned Box. The current implementation is a stub
// that returns nil; the real algorithm (force-directed or grid) will replace it
// without changing the signature.
func Auto(_ domain.Schema) map[string]Box {
	return nil
}

package diagram

import "github.com/rpatton4/electrorangerd/internal/domain"

// Hit tests the point (x, y) against boxes and returns the EntityID of the
// first entity whose bounding box contains the point. If no entity is hit
// it returns the zero EntityID and ok=false.
//
// The current implementation is a stub; the real check will use axis-aligned
// bounding-box arithmetic without allocating.
func Hit(_ map[domain.EntityID]Box, _, _ float32) (id domain.EntityID, ok bool) {
	return 0, false
}

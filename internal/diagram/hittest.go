package diagram

import "github.com/InfiniteSkye/electrorangerd/internal/domain"

// Hit tests the point (x, y) against boxes and returns the fully-qualified
// EntityRef of the first entity whose bounding box contains the point. If no
// entity is hit it returns the zero EntityRef and ok=false.
//
// The current implementation is a stub; the real check will use axis-aligned
// bounding-box arithmetic without allocating.
func Hit(_ map[domain.EntityRef]Box, _, _ float32) (ref domain.EntityRef, ok bool) {
	return domain.EntityRef{}, false
}

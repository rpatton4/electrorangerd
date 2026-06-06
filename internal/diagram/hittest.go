package diagram

// Hit tests the point (x, y) against boxes and returns the name of the first
// entity whose bounding box contains the point. If no entity is hit it returns
// an empty string and ok=false.
//
// The current implementation is a stub; the real check will use axis-aligned
// bounding-box arithmetic without allocating.
func Hit(_ map[string]Box, _, _ float32) (entityName string, ok bool) {
	return "", false
}

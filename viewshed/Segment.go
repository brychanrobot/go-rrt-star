package viewshed

// Segment holds the start, end, and distance of a segment
type Segment struct {
	p1 *EndPoint
	p2 *EndPoint
	d  float64
}

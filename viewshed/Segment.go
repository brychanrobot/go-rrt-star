package viewshed

// Segment holds the start, end, and distance of a segment
type Segment struct {
	P1 *EndPoint
	P2 *EndPoint
	d  float64
}

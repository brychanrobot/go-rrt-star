package viewshed

// EndPoint holds an x, y and much much more
type EndPoint struct {
	*Point
	begin     bool
	segment   *Segment
	angle     float64
	visualize bool
}

// ByAngleThenBegin implements the SortInterface
type ByAngleThenBegin []*EndPoint

func (a ByAngleThenBegin) Len() int      { return len(a) }
func (a ByAngleThenBegin) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByAngleThenBegin) Less(i, j int) bool {
	if a[i].angle < a[j].angle {
		return true
	}
	if a[i].angle > a[j].angle {
		return false
	}
	if a[i].begin && !a[j].begin {
		return true
	}
	if !a[i].begin && a[j].begin {
		return false
	}

	return false
}

package viewshed

import (
	"math"
	"sort"

	"github.com/skelterjohn/geom"
)

// Viewshed calculates and stores a viewshed given obstacle segments
type Viewshed struct {
	Segments        []*Segment
	endpoints       []*EndPoint
	Center          geom.Coord
	ViewablePolygon []*geom.Coord
	open            []*Segment
}

func leftOf(s *Segment, p *geom.Coord) bool {
	cross := (s.P2.X-s.P1.X)*(p.Y-s.P1.Y) - (s.P2.Y-s.P1.Y)*(p.X-s.P1.X)
	return cross < 0
}

func interpolate(p *geom.Coord, q *geom.Coord, f float64) *geom.Coord {
	return &geom.Coord{X: p.X*(1-f) + q.X*f, Y: p.Y*(1-f) + q.Y*f}
}

func segmentInFrontOf(a *Segment, b *Segment, relativeTo *geom.Coord) bool {
	A1 := leftOf(a, interpolate(b.P1.Coord, b.P2.Coord, 0.01))
	A2 := leftOf(a, interpolate(b.P2.Coord, b.P1.Coord, 0.01))
	A3 := leftOf(a, relativeTo)

	B1 := leftOf(b, interpolate(a.P1.Coord, a.P2.Coord, 0.01))
	B2 := leftOf(b, interpolate(a.P2.Coord, a.P1.Coord, 0.01))
	B3 := leftOf(b, relativeTo)

	return (B1 == B2 && B2 != B3) || (A1 == A2 && A2 == A3)
}

func lineIntersection(p1 *geom.Coord, p2 *geom.Coord, p3 *geom.Coord, p4 *geom.Coord) *geom.Coord {
	s := ((p4.X-p3.X)*(p1.Y-p3.Y) - (p4.Y-p3.Y)*(p1.X-p3.X)) / ((p4.Y-p3.Y)*(p2.X-p1.X) - (p4.X-p3.X)*(p2.Y-p1.Y))
	return &geom.Coord{X: p1.X + s*(p2.X-p1.X), Y: p1.Y + s*(p2.Y-p1.Y)}
}

func squareDistance(p1 *geom.Coord, p2 *geom.Coord) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y

	return dx*dx + dy*dy
}

func (v *Viewshed) loadEdgeOfMap(width float64, height float64, margin float64) {
	v.addSegment(margin, margin, margin, height-margin)
	v.addSegment(margin, height-margin, width-margin, height-margin)
	v.addSegment(width-margin, height-margin, width-margin, margin)
	v.addSegment(width-margin, margin, margin, margin)
}

// LoadMap loads a map from width, height, and a list of walls
func (v *Viewshed) LoadMap(width float64, height float64, margin float64, blocks []*geom.Rect, walls []*Segment) {
	v.Segments = v.Segments[:0] //clear the slice
	v.endpoints = v.endpoints[:0]

	v.loadEdgeOfMap(width, height, margin)
	/*
		for _, block := range blocks {
			x := block.x
			y := block.y
			r := block.r

			v.addSegment(x-r, y-r, x-r, y+r)
			v.addSegment(x-r, y+r, x+r, y+r)
			v.addSegment(x+r, y+r, x+r, y-r)
			v.addSegment(x+r, y-r, x-r, y-r)
		}
	*/
	for _, block := range blocks {
		v.addSegmentsFromRectangle(block)
	}

	for _, wall := range walls {
		v.addSegment(wall.P1.X, wall.P1.Y, wall.P2.X, wall.P2.Y)
	}
}

func (v *Viewshed) addSegmentsFromRectangle(rect *geom.Rect) {
	v.addSegment(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y)
	v.addSegment(rect.Max.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
	v.addSegment(rect.Max.X, rect.Max.Y, rect.Min.X, rect.Max.Y)
	v.addSegment(rect.Min.X, rect.Max.Y, rect.Min.X, rect.Min.Y)
}

func (v *Viewshed) addSegment(x1 float64, y1 float64, x2 float64, y2 float64) {
	p1 := EndPoint{Coord: &geom.Coord{X: x1, Y: y1}, visualize: true}
	p2 := EndPoint{Coord: &geom.Coord{X: x2, Y: y2}, visualize: false} //not sure why visualize is false

	segment := Segment{P1: &p1, P2: &p2}
	p1.segment = &segment
	p2.segment = &segment
	v.Segments = append(v.Segments, &segment)
	v.endpoints = append(v.endpoints, &p1)
	v.endpoints = append(v.endpoints, &p2)
}

func (v *Viewshed) addTriangle(angle1 float64, angle2 float64, segment *Segment) {
	p1 := v.Center
	p2 := geom.Coord{X: v.Center.X + math.Cos(angle1), Y: v.Center.Y + math.Sin(angle1)}
	p3 := geom.Coord{}
	p4 := geom.Coord{}

	if segment != nil {
		p3.X = segment.P1.X
		p3.Y = segment.P1.Y
		p4.X = segment.P2.X
		p4.Y = segment.P2.Y
	} else {
		p3.X = v.Center.X + math.Cos(angle1)*500
		p3.Y = v.Center.Y + math.Sin(angle1)*500
		p4.X = v.Center.X + math.Cos(angle2)*500
		p4.Y = v.Center.Y + math.Sin(angle2)*500
	}
	pBegin := lineIntersection(&p3, &p4, &p1, &p2)
	p2.X = v.Center.X + math.Cos(angle2)
	p2.Y = v.Center.Y + math.Sin(angle2)
	pEnd := lineIntersection(&p3, &p4, &p1, &p2)

	v.ViewablePolygon = append(v.ViewablePolygon, pBegin)
	v.ViewablePolygon = append(v.ViewablePolygon, pEnd)
}

// Area2DPolygon compute the area of a 2D polygon
// http://geomalgorithms.com/a01-_area.html#2D%20Polygons
//  Input:  int n = the number of vertices in the polygon
//          Point* V = an array of n+1 vertex points with V[n]=V[0]
//  Return: the (float) area of the polygon
func Area2DPolygon(points []*geom.Coord) float64 {
	n := len(points)
	closedPoints := append(points, points[0])
	area := 0.0
	//var i, j, k int // indices

	if n < 3 {
		return 0
	} // a degenerate polygon

	for i := 1; i < n; i++ {
		area += closedPoints[i].X * (closedPoints[i+1].Y - closedPoints[i-1].Y)
	}
	area += closedPoints[n].X * (closedPoints[1].Y - closedPoints[n-1].Y) // wrap-around term
	return area / 2.0
}

// UpdateCenterLocation updates the center and recalculates all angles
func (v *Viewshed) UpdateCenterLocation(x float64, y float64) {
	//y = -y
	v.Center = geom.Coord{X: x, Y: y}

	for _, segment := range v.Segments {
		dx := 0.5*(segment.P1.X+segment.P2.X) - x
		dy := 0.5*(segment.P1.Y+segment.P2.Y) - y
		segment.d = dx*dx + dy*dy
		//segment.P1.angle = math.Mod(math.Atan2(segment.P1.Y-y, segment.P1.X-x)+2*math.Pi, 2*math.Pi)
		//segment.P2.angle = math.Mod(math.Atan2(segment.P2.Y-y, segment.P2.X-x)+2*math.Pi, 2*math.Pi)

		segment.P1.angle = math.Atan2(segment.P1.Y-y, segment.P1.X-x)
		segment.P2.angle = math.Atan2(segment.P2.Y-y, segment.P2.X-x)

		dAngle := segment.P2.angle - segment.P1.angle
		if dAngle <= -math.Pi {
			dAngle += 2 * math.Pi
		}
		if dAngle > math.Pi {
			dAngle -= 2 * math.Pi
		}

		segment.P1.begin = dAngle > 0
		//segment.P1.begin = segment.P1.angle < segment.P2.angle
		segment.P2.begin = !segment.P1.begin

		//log.Printf("p:%.0f, a:%.3f", segment.P1.Point, segment.P1.angle*180/math.Pi)
		//log.Printf("p:%.0f, a:%.3f", segment.P2.Point, segment.P2.angle*180/math.Pi)
	}
}

func isWithinRange(target float64, a float64, b float64) bool {
	/*
		if math.Mod((a-b)+2*math.Pi, 2*math.Pi) >= 180 {
			tmp := a
			a = b
			b = tmp
		}
	*/

	if math.Abs(a-b) > math.Pi {
		return a >= target && b >= target || a <= target && b <= target
	}
	return a <= target && target <= b || b <= target && target <= a
}

func isPassThrough(point *EndPoint, segment *Segment) bool {
	if point.X == segment.P1.X && point.Y == segment.P1.Y {
		return point.begin == segment.P1.begin
	}
	if point.X == segment.P2.X && point.Y == segment.P2.Y {
		return point.begin == segment.P2.begin
	}
	return false
}

// Sweep computes a visibility polygon and returns all of the points
func (v *Viewshed) Sweep() {
	v.ViewablePolygon = v.ViewablePolygon[:0] // clear output
	sort.Sort(ByAngleThenBegin(v.endpoints))

	//var previousSegment *Segment
	for i := 0; i < len(v.endpoints); i += 2 {
		e := v.endpoints[i]

		//log.Printf("##################%.0f, %.3f, %b", e.Point, e.angle*180/math.Pi, e.begin)
		var intersectedSegments []*Segment
		var hasPassThrough bool
		for _, segment := range v.Segments {
			//if (segment.P1.angle < e.angle && e.angle < segment.P2.angle) ||
			//	(segment.P2.angle < e.angle && e.angle < segment.P1.angle) {
			//isWithinRange := isWithinRange(e.angle, segment.P1.angle, segment.P2.angle)
			//isPassThrough := isPassThrough(e, segment)
			//log.Printf("p1: %.0f, %.3f, p2: %.0f, %.3f", segment.P1.Point, segment.P1.angle*180/math.Pi, segment.P2.Point, segment.P2.angle*180/math.Pi)
			//log.Printf("r: %t, p: %t", isWithinRange(e.angle, segment.P1.angle, segment.P2.angle), isPassThrough(e, segment))
			if segment != e.segment && isWithinRange(e.angle, segment.P1.angle, segment.P2.angle) {
				isPassThrough := isPassThrough(e, segment)
				if !isPassThrough {
					intersectedSegments = append(intersectedSegments, segment)
				}
				hasPassThrough = hasPassThrough || isPassThrough

			}
		}

		closestIntersection := e.Coord             //the intersection is the point if there isn't anything else
		closestIntersectionDist := math.MaxFloat64 //squareDistance(&v.center, e.Point)
		//closestIntersectionSegment := e.segment

		for _, segment := range intersectedSegments {
			intersection := lineIntersection(&v.Center, e.Coord, segment.P1.Coord, segment.P2.Coord)
			dist := squareDistance(&v.Center, intersection)
			if dist < closestIntersectionDist {
				closestIntersection = intersection
				closestIntersectionDist = dist
			}
		}

		if hasPassThrough && closestIntersectionDist > squareDistance(&v.Center, e.Coord) {
			if e.begin {
				v.ViewablePolygon = append(v.ViewablePolygon, closestIntersection)
				v.ViewablePolygon = append(v.ViewablePolygon, e.Coord)
			} else {
				v.ViewablePolygon = append(v.ViewablePolygon, e.Coord)
				v.ViewablePolygon = append(v.ViewablePolygon, closestIntersection)
			}
		} else {
			v.ViewablePolygon = append(v.ViewablePolygon, closestIntersection)
		}

		//previousSegment = closestIntersectionSegment
	}

	/*
		v.open = v.open[:0] // clear open
		currentAngle := 0.0

		for pass := 0; pass < 2; pass++ {
			for _, p := range v.endpoints {
				var currentOld *Segment
				if len(v.open) != 0 {
					currentOld = v.open[0]
				}

				if pass == 1 && p.angle > maxAngle {
					break
				}
				if p.begin {
					atEnd := true
					insertionPoint := 0
					for i, s := range v.open {
						if !segmentInFrontOf(p.segment, s, &v.center) {
							atEnd = false
							break
						}
						insertionPoint = i
					}

					if atEnd {
						v.open = append(v.open, p.segment)
					} else {
						// this spaghetti inserts an element at the insertionPoint
						v.open = append(v.open, nil)
						copy(v.open[insertionPoint+1:], v.open[insertionPoint:])
						v.open[insertionPoint] = p.segment
					}
				} else {
					for i, value := range v.open {
						if value == p.segment {
							v.open = append(v.open[:i], v.open[i+1:]...) // this looks like voodoo, but it's just deleting element i gotta love go
							break
						}
					}
				}

				var currentNew *Segment

				if len(v.open) != 0 {
					currentNew = v.open[0]
				}

				if currentOld != currentNew {
					if pass == 1 {
						v.addTriangle(currentAngle, p.angle, currentOld)
					}

					currentAngle = p.angle
				}
			}
		}
	*/
}

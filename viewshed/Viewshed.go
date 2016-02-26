package viewshed

import (
	"math"
	"sort"
)

// Viewshed calculates and stores a viewshed given obstacle segments
type Viewshed struct {
	segments  []*Segment
	endpoints []*EndPoint
	center    Point
	output    []*Point
	open      []*Segment
}

func leftOf(s *Segment, p *Point) bool {
	cross := (s.p2.x-s.p1.x)*(p.y-s.p1.y) - (s.p2.y-s.p1.y)*(p.x-s.p1.x)
	return cross < 0
}

func interpolate(p *Point, q *Point, f float64) *Point {
	return &Point{p.x*(1-f) + q.x*f, p.y*(1-f) + q.y*f}
}

func segmentInFrontOf(a *Segment, b *Segment, relativeTo *Point) bool {
	A1 := leftOf(a, interpolate(b.p1.Point, b.p2.Point, 0.01))
	A2 := leftOf(a, interpolate(b.p2.Point, b.p1.Point, 0.01))
	A3 := leftOf(a, relativeTo)

	B1 := leftOf(b, interpolate(a.p1.Point, a.p2.Point, 0.01))
	B2 := leftOf(b, interpolate(a.p2.Point, a.p1.Point, 0.01))
	B3 := leftOf(b, relativeTo)

	return (B1 == B2 && B2 != B3) || (A1 == A2 && A2 == A3)
}

func lineIntersection(p1 *Point, p2 *Point, p3 *Point, p4 *Point) *Point {
	s := ((p4.x-p3.x)*(p1.y-p3.y) - (p4.y-p3.y)*(p1.x-p3.x)) / ((p4.y-p3.y)*(p2.x-p1.x) - (p4.x-p3.x)*(p2.y-p1.y))
	return &Point{p1.x + s*(p2.x-p1.x), p1.y + s*(p2.y-p1.y)}
}

func (v *Viewshed) loadEdgeOfMap(width float64, height float64, margin float64) {
	v.addSegment(margin, margin, margin, height-margin)
	v.addSegment(margin, height-margin, width-margin, height-margin)
	v.addSegment(width-margin, height-margin, width-margin, margin)
	v.addSegment(width-margin, margin, margin, margin)
}

// LoadMap loads a map from width, height, and a list of walls
func (v *Viewshed) LoadMap(width float64, height float64, margin float64, blocks []*Block, walls []*Segment) {
	v.segments = v.segments[:0] //clear the slice
	v.endpoints = v.endpoints[:0]

	v.loadEdgeOfMap(width, height, margin)
	for _, block := range blocks {
		x := block.x
		y := block.y
		r := block.r

		v.addSegment(x-r, y-r, x-r, y+r)
		v.addSegment(x-r, y+r, x+r, y+r)
		v.addSegment(x+r, y+r, x+r, y-r)
		v.addSegment(x+r, y-r, x-r, y-r)
	}

	for _, wall := range walls {
		v.addSegment(wall.p1.x, wall.p1.y, wall.p2.x, wall.p2.y)
	}
}

func (v *Viewshed) addSegment(x1 float64, y1 float64, x2 float64, y2 float64) {
	p1 := EndPoint{Point: &Point{x1, y1}, visualize: true}
	p2 := EndPoint{Point: &Point{x1, y1}, visualize: false} //not sure why visualize is false

	segment := Segment{p1: &p1, p2: &p2}
	v.segments = append(v.segments, &segment)
	v.endpoints = append(v.endpoints, &p1)
	v.endpoints = append(v.endpoints, &p2)
}

func (v *Viewshed) addTriangle(angle1 float64, angle2 float64, segment *Segment) {
	p1 := v.center
	p2 := Point{v.center.x + math.Cos(angle1), v.center.y + math.Sin(angle1)}
	p3 := Point{}
	p4 := Point{}

	if segment != nil {
		p3.x = segment.p1.x
		p3.y = segment.p1.y
		p4.x = segment.p2.x
		p4.y = segment.p2.y
	} else {
		p3.x = v.center.x + math.Cos(angle1)*500
		p3.y = v.center.y + math.Sin(angle1)*500
		p4.x = v.center.x + math.Cos(angle2)*500
		p4.y = v.center.y + math.Sin(angle2)*500
	}
	pBegin := lineIntersection(&p3, &p4, &p1, &p2)
	p2.x = v.center.x + math.Cos(angle2)
	p2.y = v.center.y + math.Sin(angle2)
	pEnd := lineIntersection(&p3, &p4, &p1, &p2)
	v.output = append(v.output, pBegin)
	v.output = append(v.output, pEnd)
}

// UpdateCenterLocation updates the center and recalculates all angles
func (v *Viewshed) UpdateCenterLocation(x float64, y float64) {
	v.center = Point{x, y}

	for _, segment := range v.segments {
		dx := 0.5*(segment.p1.x+segment.p2.x) - x
		dy := 0.5*(segment.p1.y+segment.p2.y) - y
		segment.d = dx*dx + dy*dy
		segment.p1.angle = math.Atan2(segment.p1.y-y, segment.p1.x-x)
		segment.p2.angle = math.Atan2(segment.p2.y-y, segment.p2.x-x)
		dAngle := segment.p2.angle - segment.p1.angle
		if dAngle <= -math.Pi {
			dAngle += 2 * math.Pi
		}
		if dAngle > math.Pi {
			dAngle -= 2 * math.Pi
		}
		segment.p1.begin = dAngle > 0.0
		segment.p2.begin = !segment.p1.begin
	}
}

// Sweep computes a visibility polygon and returns all of the points
func (v *Viewshed) Sweep(maxAngle float64) {
	v.output = v.output[:0] // clear output
	sort.Sort(ByAngleThenBegin(v.endpoints))
	v.open = v.open[:0] // clear open
	currentAngle := 0.0

	for pass := 0; pass < 2; pass++ {
		for _, p := range v.endpoints {
			currentOld := v.open[0]

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
					v.open = v.open[0 : len(v.open)+1]
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
			}
		}
	}
}

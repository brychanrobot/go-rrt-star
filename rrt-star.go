package main

import (
	"image"
	"math"

	"github.com/dhconnelly/rtreego"
)

// Star holds all of the information for an rrt*
type Star struct {
	obstacles  *image.Gray
	rtree      *rtreego.Rtree
	rrtRoot    *Node
	maxSegment float64
	width      int
	height     int
}

// Create creates a new rrt Star
func Create(obstacles *image.Gray, width int, height int) *Star {
	rrtRoot := &Node{parent: nil, point: image.Pt(860, 260), cumulativeCost: 0}
	rtree := rtreego.NewTree(2, 25, 50)
	rtree.Insert(rrtRoot)

	return &Star{obstacles: obstacles, rtree: rtree, rrtRoot: rrtRoot, maxSegment: 20, width: width, height: height}
}

func (r *Star) lineHasIntersection(p1 image.Point, p2 image.Point, minObstacleColor uint8) bool {
	dx := float64(float64(p2.X) - float64(p1.X))
	dy := float64(float64(p2.Y) - float64(p1.Y))

	m := 20000.0 // a big number for a vertical slope

	if dx != 0 {
		m = dy / dx
	}

	b := -m*float64(p1.X) + float64(p1.Y)

	minX := int(math.Min(float64(p1.X), float64(p2.X)))
	maxX := int(math.Max(float64(p1.X), float64(p2.X)))
	for ix := minX; ix <= maxX; ix++ {
		y := m*float64(ix) + b
		if r.obstacles.GrayAt(ix, int(y)).Y > minObstacleColor {
			return true
		}
	}

	minY := int(math.Min(float64(p1.Y), float64(p2.Y)))
	maxY := int(math.Max(float64(p1.Y), float64(p2.Y)))
	for iY := minY; iY <= maxY; iY++ {
		x := (float64(iY) - b) / m
		if r.obstacles.GrayAt(int(x), iY).Y > minObstacleColor {
			return true
		}
	}

	return false
}

// SampleRrt does rrt but ignores obstacles
/*
func SampleRrt(obstacles *image.Gray) {
	point := randomPoint(width, height)

	nnSpatial := rtree.NearestNeighbor(rtreego.Point{float64(point.X), float64(point.Y)})
	nn := nnSpatial.(*Node)

	dist := euclideanDistance(nn.point, point)

	//log.Println(dist)

	if dist > maxSegment {
		angle := angleBetweenPoints(nn.point, point)
		x := int(maxSegment*math.Cos(angle)) + nn.point.X
		y := int(maxSegment*math.Sin(angle)) + nn.point.Y
		point = image.Pt(x, y)
	}

	newNode := nn.AddChild(point, dist)
	rtree.Insert(newNode)
	//invalidate()
}
*/

// SampleRrtStar performs one iteration of rrt*
func (r *Star) SampleRrtStar() {
	point := randomPoint(width, height)

	nnSpatial := r.rtree.NearestNeighbor(rtreego.Point{float64(point.X), float64(point.Y)})
	nn := nnSpatial.(*Node)

	dist := euclideanDistance(nn.point, point)

	//log.Println(dist)

	if dist > r.maxSegment {
		angle := angleBetweenPoints(nn.point, point)
		x := int(r.maxSegment*math.Cos(angle)) + nn.point.X
		y := int(r.maxSegment*math.Sin(angle)) + nn.point.Y
		point = image.Pt(x, y)
	}

	if r.obstacles.GrayAt(point.X, point.Y).Y < 50 {

		//newNode := nn.AddChild(point, dist)
		//rtree.Insert(newNode)
		//invalidate()
		rtreePoint := rtreego.Point{float64(point.X), float64(point.Y)}
		neighbors := r.rtree.SearchIntersect(rtreePoint.ToRect(3 * r.maxSegment))
		neighborCosts := []float64{}
		bestCost := 65000.0
		var bestNeighbor *Node
		for i, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			neighborCosts = append(neighborCosts, euclideanDistance(point, neighbor.point))
			if neighborCosts[i] < bestCost {
				bestCost = neighborCosts[i]
				bestNeighbor = neighbor
			}
		}

		if !r.lineHasIntersection(point, bestNeighbor.point, 200) {
			newNode := bestNeighbor.AddChild(point, bestCost)
			r.rtree.Insert(newNode)

			for i, neighborInterface := range neighbors {
				neighbor := neighborInterface.(*Node)
				if neighbor != bestNeighbor && !r.lineHasIntersection(newNode.point, neighbor.point, 200) {
					if neighborCosts[i]+newNode.cumulativeCost < neighbor.cumulativeCost {
						neighbor.Rewire(newNode, neighborCosts[i])
					}
				}
			}
		}
	}
}

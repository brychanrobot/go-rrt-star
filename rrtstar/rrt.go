package rrtstar

import (
	"image"
	"math"

	"github.com/brychanrobot/rrt-star/viewshed"
	"github.com/dhconnelly/rtreego"
)

// RrtStar holds all of the information for an rrt*
type RrtStar struct {
	obstacleImage *image.Gray
	obstacleRects []*image.Rectangle
	rtree         *rtreego.Rtree
	Root          *Node
	maxSegment    float64
	width         int
	height        int
	StartPoint    *image.Point
	EndPoint      *image.Point
	endNode       *Node
	BestPath      []*image.Point
	Viewshed      viewshed.Viewshed
	mapArea       float64
}

func randomOpenAreaPoint(obstacles *image.Gray, width int, height int) *image.Point {
	var point image.Point
	for true {
		point = randomPoint(width, height)
		if !pointIntersectsObstacle(point, obstacles, 200) {
			break
		}
	}

	return &point
}

// NewRrtStar creates a new rrt Star
func NewRrtStar(obstacleImage *image.Gray, obstacleRects []*image.Rectangle, width int, height int) *RrtStar {
	startPoint := randomOpenAreaPoint(obstacleImage, width, height)
	var endPoint *image.Point
	//make sure the enpoint is at least half the screen away from the start to guarantee some difficulty
	for endPoint == nil || euclideanDistance(startPoint, endPoint) < float64(width)/2.0 {
		endPoint = randomOpenAreaPoint(obstacleImage, width, height)
	}
	rrtRoot := &Node{parent: nil, Point: *startPoint, CumulativeCost: 0}
	rtree := rtreego.NewTree(2, 25, 50)
	rtree.Insert(rrtRoot)

	rrtStar := &RrtStar{
		obstacleImage: obstacleImage,
		obstacleRects: obstacleRects,
		rtree:         rtree,
		Root:          rrtRoot,
		maxSegment:    20,
		width:         width,
		height:        height,
		StartPoint:    startPoint,
		EndPoint:      endPoint,
		mapArea:       float64(width * height)}

	rrtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	rrtStar.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	rrtStar.Viewshed.Sweep()

	return rrtStar
}

func (r *RrtStar) lineIntersectsObstacle(p1 image.Point, p2 image.Point, minObstacleColor uint8) bool {
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
		if r.obstacleImage.GrayAt(ix, int(y)).Y > minObstacleColor {
			return true
		}
	}

	minY := int(math.Min(float64(p1.Y), float64(p2.Y)))
	maxY := int(math.Max(float64(p1.Y), float64(p2.Y)))
	for iY := minY; iY <= maxY; iY++ {
		x := (float64(iY) - b) / m
		if r.obstacleImage.GrayAt(int(x), iY).Y > minObstacleColor {
			return true
		}
	}

	return false
}

func pointIntersectsObstacle(point image.Point, obstacles *image.Gray, minObstacleColor uint8) bool {
	return obstacles.GrayAt(point.X, point.Y).Y > minObstacleColor
}

func (r *RrtStar) refreshBestPath() {
	if r.endNode == nil {
		rtreeEndPoint := rtreego.Point{float64(r.EndPoint.X), float64(r.EndPoint.Y)}
		neighbors := r.rtree.SearchIntersect(rtreeEndPoint.ToRect(2 * r.maxSegment))

		//neighborCosts := []float64{}
		bestCost := math.MaxFloat64
		var bestNeighbor *Node
		for _, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			cost := r.getCost(r.EndPoint, &neighbor.Point)
			if cost < bestCost && !r.lineIntersectsObstacle(*r.EndPoint, neighbor.Point, 200) {
				bestCost = cost
				bestNeighbor = neighbor
			}
		}

		if bestNeighbor != nil {
			unseenArea := (r.mapArea - r.getViewArea(r.EndPoint)) / r.mapArea
			r.endNode = bestNeighbor.AddChild(*r.EndPoint, bestCost, unseenArea)
			r.rtree.Insert(r.endNode)
			r.traceBestPath()
		}
	} else {
		r.traceBestPath()
	}
}

func (r *RrtStar) traceBestPath() {
	r.BestPath = r.BestPath[:0]
	currentNode := r.endNode
	for currentNode != nil {
		r.BestPath = append(r.BestPath, &currentNode.Point)
		currentNode = currentNode.parent
	}
}

func (r *RrtStar) getViewArea(point *image.Point) float64 {
	r.Viewshed.UpdateCenterLocation(float64(point.X), float64(point.Y))
	r.Viewshed.Sweep()
	return viewshed.Area2DPolygon(r.Viewshed.ViewablePolygon)
}

func (r *RrtStar) getCost(neighbor *image.Point, point *image.Point) float64 {
	dist := euclideanDistance(neighbor, point)
	return dist
}

// SampleRrtStar performs one iteration of rrt*
func (r *RrtStar) SampleRrtStar() {
	point := randomPoint(r.width, r.height)

	nnSpatial := r.rtree.NearestNeighbor(rtreego.Point{float64(point.X), float64(point.Y)})
	nn := nnSpatial.(*Node)

	cost := r.getCost(&nn.Point, &point)

	//log.Println(dist)

	if cost > r.maxSegment {
		angle := angleBetweenPoints(nn.Point, point)
		x := int(r.maxSegment*math.Cos(angle)) + nn.Point.X
		y := int(r.maxSegment*math.Sin(angle)) + nn.Point.Y
		point = image.Pt(x, y)
	}

	if r.obstacleImage.GrayAt(point.X, point.Y).Y < 50 {

		rtreePoint := rtreego.Point{float64(point.X), float64(point.Y)}
		neighbors := r.rtree.SearchIntersect(rtreePoint.ToRect(6 * r.maxSegment))

		neighborCosts := []float64{}
		bestCost := math.MaxFloat64
		var bestNeighbor *Node
		for i, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			neighborCosts = append(neighborCosts, r.getCost(&point, &neighbor.Point))
			if neighborCosts[i] < bestCost {
				bestCost = neighborCosts[i]
				bestNeighbor = neighbor
			}
		}

		if !r.lineIntersectsObstacle(point, bestNeighbor.Point, 200) {
			unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
			newNode := bestNeighbor.AddChild(point, bestCost, unseenArea)
			r.rtree.Insert(newNode)

			for i, neighborInterface := range neighbors {
				neighbor := neighborInterface.(*Node)
				if neighbor != bestNeighbor && !r.lineIntersectsObstacle(newNode.Point, neighbor.Point, 200) {
					if neighborCosts[i]+newNode.CumulativeCost < neighbor.CumulativeCost {
						neighbor.Rewire(newNode, neighborCosts[i])
					}
				}
			}
		}

		r.refreshBestPath()

	}
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

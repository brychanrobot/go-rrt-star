package rrtstar

import (
	"image"
	"image/png"
	"log"
	"math"
	"os"

	"github.com/brychanrobot/rrt-star/viewshed"
	"github.com/dhconnelly/rtreego"
	"github.com/gonum/matrix/mat64"
)

// RrtStar holds all of the information for an rrt*
type RrtStar struct {
	obstacleImage      *image.Gray
	obstacleRects      []*image.Rectangle
	rtree              *rtreego.Rtree
	Root               *Node
	maxSegment         float64
	rewireNeighborhood float64
	width              int
	height             int
	StartPoint         *image.Point
	EndPoint           *image.Point
	endNode            *Node
	BestPath           []*image.Point
	Viewshed           viewshed.Viewshed
	mapArea            float64
	IsAddingNodes      bool
	NumNodes           uint64
}

const (
	unseenK   = 100.0
	distanceK = 1.0
)

func (r *RrtStar) RenderUnseenCostMap(filename string) {
	costMap := mat64.NewDense(r.height, r.width, nil)
	costMapImg := image.NewGray(image.Rect(0, 0, r.width, r.height))

	for row := 0; row < r.height; row++ {
		for col := 0; col < r.width; col++ {
			if r.obstacleImage.GrayAt(col, row).Y < 200 {
				point := image.Pt(col, row)
				costMap.Set(row, col, r.getViewArea(&point))
			}
		}
	}

	costMap.Scale(255/mat64.Max(costMap), costMap)

	for row := 0; row < r.height; row++ {
		for col := 0; col < r.width; col++ {
			costMapImg.Pix[row*r.width+col] = uint8(costMap.At(row, col))
			//costMapImg.Set(row, col, uint8(costMap.At()))
		}
	}

	toimg, _ := os.Create(filename)
	defer toimg.Close()

	png.Encode(toimg, costMapImg)
}

// NewRrtStar creates a new rrt Star
func NewRrtStar(obstacleImage *image.Gray, obstacleRects []*image.Rectangle, maxSegment float64, width int, height int,
	startPoint, endPoint *image.Point) *RrtStar {

	if startPoint == nil {
		startPoint = randomOpenAreaPoint(obstacleImage, width, height)
	}
	//var endPoint *image.Point
	//make sure the endpoint is at least half the screen away from the start to guarantee some difficulty
	if endPoint == nil {
		for endPoint == nil || euclideanDistance(startPoint, endPoint) < float64(width)/2.0 {
			endPoint = randomOpenAreaPoint(obstacleImage, width, height)
		}
	}

	rrtRoot := &Node{parent: nil, Point: *startPoint, CumulativeCost: 0}
	rtree := rtreego.NewTree(2, 25, 50)
	rtree.Insert(rrtRoot)

	rrtStar := &RrtStar{
		obstacleImage:      obstacleImage,
		obstacleRects:      obstacleRects,
		rtree:              rtree,
		Root:               rrtRoot,
		maxSegment:         maxSegment,
		rewireNeighborhood: maxSegment * 6,
		width:              width,
		height:             height,
		StartPoint:         startPoint,
		EndPoint:           endPoint,
		mapArea:            float64(width * height),
		NumNodes:           1}

	rrtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	//rrtStar.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	//rrtStar.Viewshed.Sweep()

	//rrtStar.renderCostMap()
	rrtStar.Root.UnseenArea = rrtStar.getUnseenArea(startPoint)

	return rrtStar
}

func (r *RrtStar) getBestNeighbor(point *image.Point, neighborhoodSize, unseenArea float64) (*Node, float64, []*Node, []float64) {
	rtreePoint := rtreego.Point{float64(point.X), float64(point.Y)}
	spatialNeighbors := r.rtree.SearchIntersect(rtreePoint.ToRect(neighborhoodSize))
	neighborCosts := []float64{}
	neighbors := []*Node{}
	bestCost := math.MaxFloat64
	var bestNeighbor *Node
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)
		if neighbor.Point != *point && !r.lineIntersectsObstacle(*point, neighbor.Point, 200) {
			neighbors = append(neighbors, neighbor)
			cost := r.getCostKnownUnseenArea(&neighbor.Point, point, unseenArea)
			neighborCosts = append(neighborCosts, cost)
			if cost < bestCost {
				bestCost = cost
				bestNeighbor = neighbor
			}
		}
	}

	return bestNeighbor, bestCost, neighbors, neighborCosts
}

func (r *RrtStar) MoveStartPoint(dx, dy float64) {
	if dx != 0 || dy != 0 {
		r.StartPoint.X += int(dx)
		r.StartPoint.Y += int(dy)
		//log.Println(r.StartPoint)
		newRoot := &Node{parent: nil, Point: *r.StartPoint, CumulativeCost: 0}
		r.NumNodes++
		newRoot.UnseenArea = r.getUnseenArea(&newRoot.Point)
		r.rtree.Insert(newRoot)

		r.Root.Rewire(newRoot, r.getCostKnownUnseenArea(&newRoot.Point, &r.Root.Point, r.Root.UnseenArea))
		r.Root = newRoot

		_, _, neighbors, neighborCosts := r.getBestNeighbor(&r.Root.Point, r.rewireNeighborhood, r.Root.UnseenArea)
		for i, neighbor := range neighbors {
			if !r.lineIntersectsObstacle(r.Root.Point, neighbor.Point, 200) {
				if neighborCosts[i]+r.Root.CumulativeCost < neighbor.CumulativeCost {
					neighbor.Rewire(r.Root, neighborCosts[i])
				}
			}
		}
	}
}

func (r *RrtStar) Prune(minorAxisSquares int) {
	var squareSize int
	if r.height < r.width {
		squareSize = int(r.height / minorAxisSquares)
	} else {
		squareSize = int(r.width / minorAxisSquares)
	}

	log.Printf("sq: %d", squareSize)

	for cy := int(squareSize / 2); cy < r.height; cy += squareSize {
		for cx := int(squareSize / 2); cx < r.width; cx += squareSize {
			cPoint := image.Pt(cx, cy)
			bestNeighbor, _, neighbors, _ := r.getBestNeighbor(&cPoint, float64(squareSize), 0)
			for _, neighbor := range neighbors {
				if neighbor != bestNeighbor && !r.lineIntersectsObstacle(bestNeighbor.Point, neighbor.Point, 200) {
					cost := r.getCostKnownUnseenArea(&bestNeighbor.Point, &neighbor.Point, neighbor.UnseenArea)
					if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
						neighbor.Rewire(bestNeighbor, cost)
					}
				}
			}

			for _, neighbor := range neighbors {
				if len(neighbor.Children) == 0 {
					neighbor.parent.RemoveChild(neighbor)
					r.rtree.Delete(neighbor)
					r.NumNodes--
				}
			}
		}
	}
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

func (r *RrtStar) refreshBestPath() {
	if r.endNode == nil {
		rtreeEndPoint := rtreego.Point{float64(r.EndPoint.X), float64(r.EndPoint.Y)}
		neighbors := r.rtree.SearchIntersect(rtreeEndPoint.ToRect(2 * r.maxSegment))

		_, unseenArea := r.getCost(r.StartPoint, r.EndPoint)
		//neighborCosts := []float64{}
		bestCost := math.MaxFloat64
		var bestNeighbor *Node
		for _, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			cost := r.getCostKnownUnseenArea(r.EndPoint, &neighbor.Point, unseenArea)
			if cost < bestCost && !r.lineIntersectsObstacle(*r.EndPoint, neighbor.Point, 200) {
				bestCost = cost
				bestNeighbor = neighbor
			}
		}

		if bestNeighbor != nil {
			r.endNode = bestNeighbor.AddChild(*r.EndPoint, bestCost, unseenArea)
			r.NumNodes++
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

func (r *RrtStar) getUnseenArea(point *image.Point) float64 {
	return (r.mapArea - r.getViewArea(point)) / r.mapArea
}

func (r *RrtStar) getCost(neighbor *image.Point, point *image.Point) (float64, float64) {

	unseenArea := r.getUnseenArea(point)
	cost := r.getCostKnownUnseenArea(neighbor, point, unseenArea)
	return cost, unseenArea
}

func (r *RrtStar) getCostKnownUnseenArea(neighbor *image.Point, point *image.Point, unseenArea float64) float64 {
	dist := euclideanDistance(neighbor, point)
	return dist*distanceK + unseenArea*unseenK
}

func (r *RrtStar) sampleRrtStarWithNewNode() {
	point := randomPoint(r.width, r.height)

	nnSpatial := r.rtree.NearestNeighbor(rtreego.Point{float64(point.X), float64(point.Y)})
	nn := nnSpatial.(*Node)

	//cost, unseenArea := r.getCost(&nn.Point, &point)
	dist := euclideanDistance(&nn.Point, &point)

	//log.Println(dist)

	if dist > r.maxSegment {
		angle := angleBetweenPoints(nn.Point, point)
		x := int(r.maxSegment*math.Cos(angle)) + nn.Point.X
		y := int(r.maxSegment*math.Sin(angle)) + nn.Point.Y
		point = image.Pt(x, y)
	}

	if r.obstacleImage.GrayAt(point.X, point.Y).Y < 50 {

		unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
		bestNeighbor, bestCost, neighbors, neighborCosts := r.getBestNeighbor(&point, r.rewireNeighborhood, unseenArea)

		if bestNeighbor != nil { //!r.lineIntersectsObstacle(point, bestNeighbor.Point, 200) {
			//unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
			newNode := bestNeighbor.AddChild(point, bestCost, unseenArea)
			r.NumNodes++
			r.rtree.Insert(newNode)

			for i, neighbor := range neighbors {
				//neighbor := neighborInterface.(*Node)
				if neighbor != bestNeighbor && !r.lineIntersectsObstacle(newNode.Point, neighbor.Point, 200) {
					if neighborCosts[i]+newNode.CumulativeCost < neighbor.CumulativeCost {
						neighbor.Rewire(newNode, neighborCosts[i])
					}
				}
			}
		}

		//r.refreshBestPath()

	}
}

func (r *RrtStar) sampleRrtStarWithoutNewNode() {
	point := randomPoint(r.width, r.height)
	bestNeighbor, _, neighbors, _ := r.getBestNeighbor(&point, float64(r.rewireNeighborhood), 0)
	for _, neighbor := range neighbors {
		if neighbor != bestNeighbor && !r.lineIntersectsObstacle(bestNeighbor.Point, neighbor.Point, 200) {
			cost := r.getCostKnownUnseenArea(&bestNeighbor.Point, &neighbor.Point, neighbor.UnseenArea)
			if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
				neighbor.Rewire(bestNeighbor, cost)
			}
		}
	}
}

// SampleRrtStar performs one iteration of rrt*
func (r *RrtStar) SampleRrtStar() {
	nodeRatio := uint64(0.01 * float64(r.width*r.height))
	r.IsAddingNodes = r.NumNodes < nodeRatio
	if r.IsAddingNodes {
		r.sampleRrtStarWithNewNode()
	} else {
		r.sampleRrtStarWithoutNewNode()
	}
	r.refreshBestPath()
}

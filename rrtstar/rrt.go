package rrtstar

import (
	"image"
	"image/png"
	"log"
	"math"
	"os"

	"github.com/brychanrobot/go-halton"
	"github.com/brychanrobot/rrt-star/viewshed"
	"github.com/dhconnelly/rtreego"
	"github.com/gonum/matrix/mat64"
	"github.com/skelterjohn/geom"
)

// RrtStar holds all of the information for an rrt*
type RrtStar struct {
	obstacleImage      *image.Gray
	obstacleRects      []*geom.Rect
	rtree              *rtreego.Rtree
	Root               *Node
	maxSegment         float64
	rewireNeighborhood float64
	width              int
	height             int
	StartPoint         *geom.Coord
	EndPoint           *geom.Coord
	endNode            *Node
	BestPath           []*geom.Coord
	Viewshed           viewshed.Viewshed
	mapArea            float64
	IsAddingNodes      bool
	NumNodes           uint64
	haltonX            *halton.HaltonSampler
	haltonY            *halton.HaltonSampler
}

const (
	unseenK   = 0.0 //100.0
	distanceK = 1.0
)

//Getters
func (r *RrtStar) GetRoot() *Node {
	return r.Root
}

func (r *RrtStar) GetStartPoint() *geom.Coord {
	return r.StartPoint
}

func (r *RrtStar) GetEndPoint() *geom.Coord {
	return r.EndPoint
}

func (r *RrtStar) GetBestPath() []*geom.Coord {
	return r.BestPath
}

func (r *RrtStar) GetViewshed() *viewshed.Viewshed {
	return &r.Viewshed
}

func (r *RrtStar) GetIsAddingNodes() bool {
	return r.IsAddingNodes
}

func (r *RrtStar) GetNumNodes() uint64 {
	return r.NumNodes
}

func (r *RrtStar) RenderUnseenCostMap(filename string) {
	costMap := mat64.NewDense(r.height, r.width, nil)
	costMapImg := image.NewGray(image.Rect(0, 0, r.width, r.height))

	for row := 0; row < r.height; row++ {
		for col := 0; col < r.width; col++ {
			if r.obstacleImage.GrayAt(col, row).Y < 200 {
				point := geom.Coord{X: float64(col), Y: float64(row)}
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
func NewRrtStar(obstacleImage *image.Gray, obstacleRects []*geom.Rect, maxSegment float64, width, height int,
	startPoint, endPoint *geom.Coord) *RrtStar {

	if startPoint == nil {
		startPoint = randomOpenAreaPoint(obstacleImage, width, height)
	}
	//var endPoint *geom.Coord
	//make sure the endpoint is at least half the screen away from the start to guarantee some difficulty
	if endPoint == nil {
		for endPoint == nil || euclideanDistance(startPoint, endPoint) < float64(width)/2.0 {
			endPoint = randomOpenAreaPoint(obstacleImage, width, height)
		}
	}

	rrtRoot := &Node{parent: nil, Coord: *startPoint, CumulativeCost: 0}
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
		NumNodes:           1,
		haltonX:            halton.NewHaltonSampler(19),
		haltonY:            halton.NewHaltonSampler(23)}

	rrtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	//rrtStar.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	//rrtStar.Viewshed.Sweep()

	//rrtStar.renderCostMap()
	rrtStar.Root.UnseenArea = rrtStar.getUnseenArea(startPoint)

	return rrtStar
}

func (r *RrtStar) getBestNeighbor(point *geom.Coord, neighborhoodSize, unseenArea float64) (*Node, float64, []*Node, []float64) {
	rtreePoint := rtreego.Point{point.X, point.Y}
	spatialNeighbors := r.rtree.SearchIntersect(rtreePoint.ToRect(neighborhoodSize))
	neighborCosts := []float64{}
	neighbors := []*Node{}
	bestCost := math.MaxFloat64
	bestCumulativeCost := math.MaxFloat64
	var bestNeighbor *Node
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)
		if neighbor.Coord != *point && !r.lineIntersectsObstacle(*point, neighbor.Coord, 200) {
			neighbors = append(neighbors, neighbor)
			cost := r.getCostKnownUnseenArea(&neighbor.Coord, point, unseenArea)
			neighborCosts = append(neighborCosts, cost)
			if cost+neighbor.CumulativeCost < bestCumulativeCost {
				bestCost = cost
				bestCumulativeCost = cost + neighbor.CumulativeCost
				bestNeighbor = neighbor
			}
		}
	}

	return bestNeighbor, bestCost, neighbors, neighborCosts
}

func (r *RrtStar) MoveStartPoint(dx, dy float64) {
	if dx != 0 || dy != 0 {
		r.StartPoint.X += dx
		r.StartPoint.Y += dy
		//log.Println(r.StartPoint)
		newRoot := &Node{parent: nil, Coord: *r.StartPoint, CumulativeCost: 0}
		r.NumNodes++
		newRoot.UnseenArea = r.getUnseenArea(&newRoot.Coord)
		r.rtree.Insert(newRoot)

		r.Root.Rewire(newRoot, r.getCostKnownUnseenArea(&newRoot.Coord, &r.Root.Coord, r.Root.UnseenArea))
		r.Root = newRoot

		_, _, neighbors, neighborCosts := r.getBestNeighbor(&r.Root.Coord, r.rewireNeighborhood, r.Root.UnseenArea)
		for i, neighbor := range neighbors {
			if !r.lineIntersectsObstacle(r.Root.Coord, neighbor.Coord, 200) {
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
			cPoint := geom.Coord{X: float64(cx), Y: float64(cy)}
			bestNeighbor, _, neighbors, _ := r.getBestNeighbor(&cPoint, float64(squareSize), 0)
			for _, neighbor := range neighbors {
				if neighbor != bestNeighbor && !r.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
					cost := r.getCostKnownUnseenArea(&bestNeighbor.Coord, &neighbor.Coord, neighbor.UnseenArea)
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

func (r *RrtStar) nextHaltonPoint(width, height int) geom.Coord {
	x := r.haltonX.Next() * float64(width)
	y := r.haltonY.Next() * float64(height)
	return geom.Coord{X: x, Y: y}
}

func (r *RrtStar) lineIntersectsObstacle(p1 geom.Coord, p2 geom.Coord, minObstacleColor uint8) bool {
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y

	m := 20000.0 // a big number for a vertical slope

	if dx != 0 {
		m = dy / dx
	}

	b := -m*p1.X + p1.Y

	minX := math.Min(p1.X, p2.X)
	maxX := math.Max(p1.X, p2.X)
	for ix := minX; ix <= maxX; ix++ {
		y := m*ix + b
		if r.obstacleImage.GrayAt(int(ix), int(y)).Y > minObstacleColor {
			return true
		}
	}

	minY := math.Min(p1.Y, p2.Y)
	maxY := math.Max(p1.Y, p2.Y)
	for iY := minY; iY <= maxY; iY++ {
		x := (iY - b) / m
		if r.obstacleImage.GrayAt(int(x), int(iY)).Y > minObstacleColor {
			return true
		}
	}

	return false
}

func (r *RrtStar) refreshBestPath() {
	if r.endNode == nil {
		rtreeEndPoint := rtreego.Point{r.EndPoint.X, r.EndPoint.Y}
		neighbors := r.rtree.SearchIntersect(rtreeEndPoint.ToRect(2 * r.maxSegment))

		_, unseenArea := r.getCost(r.StartPoint, r.EndPoint)
		//neighborCosts := []float64{}
		bestCost := math.MaxFloat64
		var bestNeighbor *Node
		for _, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			cost := r.getCostKnownUnseenArea(r.EndPoint, &neighbor.Coord, unseenArea)
			if cost < bestCost && !r.lineIntersectsObstacle(*r.EndPoint, neighbor.Coord, 200) {
				bestCost = cost
				bestNeighbor = neighbor
			}
		}

		if bestNeighbor != nil {
			r.endNode = bestNeighbor.AddAndCreateChild(*r.EndPoint, bestCost, unseenArea)
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
		r.BestPath = append(r.BestPath, &currentNode.Coord)
		currentNode = currentNode.parent
	}
}

func (r *RrtStar) getViewArea(point *geom.Coord) float64 {
	r.Viewshed.UpdateCenterLocation(point.X, point.Y)
	r.Viewshed.Sweep()
	return viewshed.Area2DPolygon(r.Viewshed.ViewablePolygon)
}

func (r *RrtStar) getUnseenArea(point *geom.Coord) float64 {
	return (r.mapArea - r.getViewArea(point)) / r.mapArea
}

func (r *RrtStar) getCost(neighbor *geom.Coord, point *geom.Coord) (float64, float64) {

	unseenArea := r.getUnseenArea(point)
	cost := r.getCostKnownUnseenArea(neighbor, point, unseenArea)
	return cost, unseenArea
}

func (r *RrtStar) getCostKnownUnseenArea(neighbor *geom.Coord, point *geom.Coord, unseenArea float64) float64 {
	dist := euclideanDistance(neighbor, point)
	return dist*distanceK + unseenArea*unseenK
}

func (r *RrtStar) sampleRrtStarWithNewNode() {
	point := r.nextHaltonPoint(r.width, r.height)

	nnSpatial := r.rtree.NearestNeighbor(rtreego.Point{point.X, point.Y})
	nn := nnSpatial.(*Node)

	//cost, unseenArea := r.getCost(&nn.Point, &point)
	dist := euclideanDistance(&nn.Coord, &point)

	//log.Println(dist)

	if dist > r.maxSegment {
		angle := angleBetweenPoints(nn.Coord, point)
		x := r.maxSegment*math.Cos(angle) + nn.Coord.X
		y := r.maxSegment*math.Sin(angle) + nn.Coord.Y
		point = geom.Coord{X: x, Y: y}
	}

	if r.obstacleImage.GrayAt(int(point.X), int(point.Y)).Y < 50 {

		unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
		bestNeighbor, bestCost, neighbors, neighborCosts := r.getBestNeighbor(&point, r.rewireNeighborhood, unseenArea)

		if bestNeighbor != nil { //!r.lineIntersectsObstacle(point, bestNeighbor.Point, 200) {
			//unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
			newNode := bestNeighbor.AddAndCreateChild(point, bestCost, unseenArea)
			r.NumNodes++
			r.rtree.Insert(newNode)

			for i, neighbor := range neighbors {
				//neighbor := neighborInterface.(*Node)
				if neighbor != bestNeighbor && !r.lineIntersectsObstacle(newNode.Coord, neighbor.Coord, 200) {
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
	point := r.nextHaltonPoint(r.width, r.height)
	bestNeighbor, _, neighbors, _ := r.getBestNeighbor(&point, float64(r.rewireNeighborhood), 0)
	for _, neighbor := range neighbors {
		if neighbor != bestNeighbor && !r.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
			cost := r.getCostKnownUnseenArea(&bestNeighbor.Coord, &neighbor.Coord, neighbor.UnseenArea)
			if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
				neighbor.Rewire(bestNeighbor, cost)
			}
		}
	}
}

// SampleRrtStar performs one iteration of rrt*
func (r *RrtStar) Sample() {
	nodeRatio := uint64(0.01 * float64(r.width*r.height))
	r.IsAddingNodes = r.NumNodes < nodeRatio
	if r.IsAddingNodes {
		r.sampleRrtStarWithNewNode()
	} else {
		r.sampleRrtStarWithoutNewNode()
	}
	r.refreshBestPath()
}

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

// FmtStar holds all of the information for an rrt*
type FmtStar struct {
	obstacleImage      *image.Gray
	obstacleRects      []*geom.Rect
	rtree              *rtreego.Rtree
	rtreeOpen          *rtreego.Rtree
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
	nodeThreshold      uint64
	IsAddingNodes      bool
	NumNodes           uint64
	haltonX            *halton.HaltonSampler
	haltonY            *halton.HaltonSampler
	open               []*Node
}

const (
//unseenK   = 100.0
//distanceK = 1.0
)

//Getters
func (f *FmtStar) GetRoot() *Node {
	return f.Root
}

func (f *FmtStar) GetStartPoint() *geom.Coord {
	return f.StartPoint
}

func (f *FmtStar) GetEndPoint() *geom.Coord {
	return f.EndPoint
}

func (f *FmtStar) GetBestPath() []*geom.Coord {
	return f.BestPath
}

func (f *FmtStar) GetViewshed() *viewshed.Viewshed {
	return &f.Viewshed
}

func (f *FmtStar) GetIsAddingNodes() bool {
	return f.IsAddingNodes
}

func (f *FmtStar) GetNumNodes() uint64 {
	return f.NumNodes
}

func (f *FmtStar) RenderUnseenCostMap(filename string) {
	costMap := mat64.NewDense(f.height, f.width, nil)
	costMapImg := image.NewGray(image.Rect(0, 0, f.width, f.height))

	for row := 0; row < f.height; row++ {
		for col := 0; col < f.width; col++ {
			if f.obstacleImage.GrayAt(col, row).Y < 200 {
				point := geom.Coord{X: float64(col), Y: float64(row)}
				costMap.Set(row, col, f.getViewArea(&point))
			}
		}
	}

	costMap.Scale(255/mat64.Max(costMap), costMap)

	for row := 0; row < f.height; row++ {
		for col := 0; col < f.width; col++ {
			costMapImg.Pix[row*f.width+col] = uint8(costMap.At(row, col))
			//costMapImg.Set(row, col, uint8(costMap.At()))
		}
	}

	toimg, _ := os.Create(filename)
	defer toimg.Close()

	png.Encode(toimg, costMapImg)
}

// NewFmtStar creates a new rrt Star
func NewFmtStar(obstacleImage *image.Gray, obstacleRects []*geom.Rect, maxSegment float64, width, height int,
	startPoint, endPoint *geom.Coord) *FmtStar {

	nodeThreshold := uint64(0.01 * float64(width*height))

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

	fmtRoot := &Node{parent: nil, Coord: *startPoint, CumulativeCost: 0, Status: Open}
	rtree := rtreego.NewTree(2, 25, 50)
	rtree.Insert(fmtRoot)

	rtreeOpen := rtreego.NewTree(2, 25, 50)
	rtreeOpen.Insert(fmtRoot)

	endNode := &Node{parent: nil, Coord: *endPoint, CumulativeCost: 0}
	rtree.Insert(endNode)

	fmtStar := &FmtStar{
		obstacleImage:      obstacleImage,
		obstacleRects:      obstacleRects,
		rtree:              rtree,
		rtreeOpen:          rtreeOpen,
		Root:               fmtRoot,
		maxSegment:         maxSegment,
		rewireNeighborhood: maxSegment * 6,
		width:              width,
		height:             height,
		StartPoint:         startPoint,
		EndPoint:           endPoint,
		endNode:            endNode,
		mapArea:            float64(width * height),
		nodeThreshold:      nodeThreshold,
		NumNodes:           1,
		haltonX:            halton.NewHaltonSampler(19),
		haltonY:            halton.NewHaltonSampler(23)}

	fmtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	//rrtStaf.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	//rrtStaf.Viewshed.Sweep()

	//rrtStaf.renderCostMap()
	fmtStar.Root.UnseenArea = fmtStar.getUnseenArea(startPoint)

	for n := uint64(0); n < nodeThreshold; n++ {
		point := fmtStar.nextHaltonPoint(width, height)
		if fmtStar.obstacleImage.GrayAt(int(point.X), int(point.Y)).Y < 50 {
			node := &Node{parent: nil, Coord: point, CumulativeCost: math.MaxFloat64}
			rtree.Insert(node)
		}
	}

	fmtStar.open = append(fmtStar.open, fmtStar.Root)

	return fmtStar
}

func (f *FmtStar) getBestNeighbor(point *geom.Coord, neighborhoodSize, unseenArea float64) (*Node, float64, []*Node, []float64) {
	rtreePoint := rtreego.Point{point.X, point.Y}
	spatialNeighbors := f.rtree.SearchIntersect(rtreePoint.ToRect(neighborhoodSize))
	neighborCosts := []float64{}
	neighbors := []*Node{}
	bestCost := math.MaxFloat64
	bestCumulativeCost := math.MaxFloat64
	var bestNeighbor *Node
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)
		if neighbor.Coord != *point && !f.lineIntersectsObstacle(*point, neighbor.Coord, 200) {
			neighbors = append(neighbors, neighbor)
			cost := f.getCostKnownUnseenArea(&neighbor.Coord, point, unseenArea)
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

func (f *FmtStar) getBestOpenNeighbor(point *geom.Coord, neighborhoodSize, unseenArea float64) (*Node, float64, []*Node, []float64) {
	rtreePoint := rtreego.Point{point.X, point.Y}
	spatialNeighbors := f.rtreeOpen.SearchIntersect(rtreePoint.ToRect(neighborhoodSize))
	neighborCosts := []float64{}
	neighbors := []*Node{}
	bestCost := math.MaxFloat64
	bestCumulativeCost := math.MaxFloat64
	var bestNeighbor *Node
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)
		neighbors = append(neighbors, neighbor)
		cost := f.getCostKnownUnseenArea(&neighbor.Coord, point, unseenArea)
		neighborCosts = append(neighborCosts, cost)
		if cost+neighbor.CumulativeCost < bestCumulativeCost {
			bestCost = cost
			bestCumulativeCost = cost + neighbor.CumulativeCost
			bestNeighbor = neighbor
		}
	}

	return bestNeighbor, bestCost, neighbors, neighborCosts
}

func (f *FmtStar) MoveStartPoint(dx, dy float64) {
	if dx != 0 || dy != 0 {
		f.StartPoint.X += dx
		f.StartPoint.Y += dy
		//log.Println(f.StartPoint)
		newRoot := &Node{parent: nil, Coord: *f.StartPoint, CumulativeCost: 0, Status: Closed}
		f.NumNodes++
		newRoot.UnseenArea = f.getUnseenArea(&newRoot.Coord)
		f.rtree.Insert(newRoot)

		f.Root.Rewire(newRoot, f.getCostKnownUnseenArea(&newRoot.Coord, &f.Root.Coord, f.Root.UnseenArea))
		f.Root = newRoot

		_, _, neighbors, neighborCosts := f.getBestNeighbor(&f.Root.Coord, f.rewireNeighborhood, f.Root.UnseenArea)
		for i, neighbor := range neighbors {
			if !f.lineIntersectsObstacle(f.Root.Coord, neighbor.Coord, 200) {
				if neighborCosts[i]+f.Root.CumulativeCost < neighbor.CumulativeCost {
					neighbor.Rewire(f.Root, neighborCosts[i])
				}
			}
		}
	}
}

func (f *FmtStar) Prune(minorAxisSquares int) {
	var squareSize int
	if f.height < f.width {
		squareSize = int(f.height / minorAxisSquares)
	} else {
		squareSize = int(f.width / minorAxisSquares)
	}

	log.Printf("sq: %d", squareSize)

	for cy := int(squareSize / 2); cy < f.height; cy += squareSize {
		for cx := int(squareSize / 2); cx < f.width; cx += squareSize {
			cPoint := geom.Coord{X: float64(cx), Y: float64(cy)}
			bestNeighbor, _, neighbors, _ := f.getBestNeighbor(&cPoint, float64(squareSize), 0)
			for _, neighbor := range neighbors {
				if neighbor != bestNeighbor && !f.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
					cost := f.getCostKnownUnseenArea(&bestNeighbor.Coord, &neighbor.Coord, neighbor.UnseenArea)
					if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
						neighbor.Rewire(bestNeighbor, cost)
					}
				}
			}

			for _, neighbor := range neighbors {
				if len(neighbor.Children) == 0 {
					neighbor.parent.RemoveChild(neighbor)
					f.rtree.Delete(neighbor)
					f.NumNodes--
				}
			}
		}
	}
}

func (f *FmtStar) nextHaltonPoint(width, height int) geom.Coord {
	x := f.haltonX.Next() * float64(width)
	y := f.haltonY.Next() * float64(height)
	return geom.Coord{X: x, Y: y}
}

func (f *FmtStar) lineIntersectsObstacle(p1 geom.Coord, p2 geom.Coord, minObstacleColor uint8) bool {
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
		if f.obstacleImage.GrayAt(int(ix), int(y)).Y > minObstacleColor {
			return true
		}
	}

	minY := math.Min(p1.Y, p2.Y)
	maxY := math.Max(p1.Y, p2.Y)
	for iY := minY; iY <= maxY; iY++ {
		x := (iY - b) / m
		if f.obstacleImage.GrayAt(int(x), int(iY)).Y > minObstacleColor {
			return true
		}
	}

	return false
}

func (f *FmtStar) refreshBestPath() {
	// if f.endNode == nil {
	// 	rtreeEndPoint := rtreego.Point{f.EndPoint.X, f.EndPoint.Y}
	// 	neighbors := f.rtree.SearchIntersect(rtreeEndPoint.ToRect(2 * f.maxSegment))
	//
	// 	_, unseenArea := f.getCost(f.StartPoint, f.EndPoint)
	// 	//neighborCosts := []float64{}
	// 	bestCost := math.MaxFloat64
	// 	var bestNeighbor *Node
	// 	for _, neighborSpatial := range neighbors {
	// 		neighbor := neighborSpatial.(*Node)
	// 		cost := f.getCostKnownUnseenArea(f.EndPoint, &neighbor.Coord, unseenArea)
	// 		if cost < bestCost && !f.lineIntersectsObstacle(*f.EndPoint, neighbor.Coord, 200) {
	// 			bestCost = cost
	// 			bestNeighbor = neighbor
	// 		}
	// 	}
	//
	// 	if bestNeighbor != nil {
	// 		f.endNode = bestNeighbor.AddChild(*f.EndPoint, bestCost, unseenArea)
	// 		f.NumNodes++
	// 		f.rtree.Insert(f.endNode)
	// 		f.traceBestPath()
	// 	}
	// } else {
	//f.traceBestPath()
	//}

	f.traceBestPath()
}

func (f *FmtStar) traceBestPath() {
	f.BestPath = f.BestPath[:0]
	currentNode := f.endNode
	for currentNode != nil {
		f.BestPath = append(f.BestPath, &currentNode.Coord)
		currentNode = currentNode.parent
	}
}

func (f *FmtStar) getViewArea(point *geom.Coord) float64 {
	f.Viewshed.UpdateCenterLocation(point.X, point.Y)
	f.Viewshed.Sweep()
	return viewshed.Area2DPolygon(f.Viewshed.ViewablePolygon)
}

func (f *FmtStar) getUnseenArea(point *geom.Coord) float64 {
	return (f.mapArea - f.getViewArea(point)) / f.mapArea
}

func (f *FmtStar) getCost(neighbor *geom.Coord, point *geom.Coord) (float64, float64) {

	unseenArea := f.getUnseenArea(point)
	cost := f.getCostKnownUnseenArea(neighbor, point, unseenArea)
	return cost, unseenArea
}

func (f *FmtStar) getCostKnownUnseenArea(neighbor *geom.Coord, point *geom.Coord, unseenArea float64) float64 {
	dist := euclideanDistance(neighbor, point)
	return dist*distanceK + unseenArea*unseenK
}

func (f *FmtStar) popBestOpenNode() *Node {
	var bestNode *Node
	bestCost := math.MaxFloat64
	bestIndex := 0
	for i, node := range f.open {
		if node.CumulativeCost < bestCost {
			bestNode = node
			bestCost = node.CumulativeCost
			bestIndex = i
		}
	}
	f.open = append(f.open[:bestIndex], f.open[bestIndex+1:]...) // this looks like voodoo, but it's just deleting element i, gotta love go
	return bestNode
}

func (f *FmtStar) sampleFmtStar() {
	bestOpenNode := f.popBestOpenNode()
	rtreePoint := rtreego.Point{bestOpenNode.Coord.X, bestOpenNode.Coord.Y}
	spatialNeighbors := f.rtree.SearchIntersect(rtreePoint.ToRect(f.rewireNeighborhood))
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)

		if neighbor.Status == Unvisited {
			unseenArea := (f.mapArea - f.getViewArea(&neighbor.Coord)) / f.mapArea
			bestParent, bestCost, _, _ := f.getBestOpenNeighbor(&neighbor.Coord, f.rewireNeighborhood, unseenArea)

			if bestParent != nil && !f.lineIntersectsObstacle(neighbor.Coord, bestParent.Coord, 200) {
				bestParent.AddChild(neighbor, bestCost, unseenArea)
				neighbor.Status = Open
				f.open = append(f.open, neighbor)
				f.rtreeOpen.Insert(neighbor)
				f.NumNodes++
			}
		}
	}
	f.rtreeOpen.Delete(bestOpenNode)
	bestOpenNode.Status = Closed

	//fmt.Printf("b:%d, a:%d, n:%d, c:%d\n", lenBefore, lenAfter, len(spatialNeighbors), len(bestOpenNode.Children))
}

func (f *FmtStar) sampleFmtStarWithRewire() {
	point := f.nextHaltonPoint(f.width, f.height)
	bestNeighbor, _, neighbors, _ := f.getBestNeighbor(&point, float64(f.rewireNeighborhood*1.5), 0)
	for _, neighbor := range neighbors {
		if bestNeighbor != nil && neighbor != bestNeighbor && !f.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
			cost := f.getCostKnownUnseenArea(&bestNeighbor.Coord, &neighbor.Coord, neighbor.UnseenArea)
			if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
				neighbor.Rewire(bestNeighbor, cost)
			}
		}
	}
}

// SampleFmtStar performs one iteration of rrt*
func (f *FmtStar) Sample() {
	f.IsAddingNodes = len(f.open) != 0 //f.NumNodes < r.nodeThreshold
	if f.IsAddingNodes {
		f.sampleFmtStar()
	} else {
		f.sampleFmtStarWithRewire()
	}
	f.refreshBestPath()
}

package rrtstar

import (
	"image"
	"math"

	"github.com/brychanrobot/go-halton"
	"github.com/dhconnelly/rtreego"
	"github.com/skelterjohn/geom"
)

// FmtStar holds all of the information for an rrt*
type FmtStar struct {
	PlannerBase
	rtreeOpen *rtreego.Rtree
	open      []*Node
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
		PlannerBase: PlannerBase{
			obstacleImage:      obstacleImage,
			obstacleRects:      obstacleRects,
			rtree:              rtree,
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
			haltonX:            halton.NewHaltonSampler(2),
			haltonY:            halton.NewHaltonSampler(3),
			unseenAreaMap:      make(map[geom.Coord]float64)},

		rtreeOpen: rtreeOpen}

	fmtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	//rrtStaf.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	//rrtStaf.Viewshed.Sweep()

	//rrtStaf.renderCostMap()
	//fmtStar.Root.UnseenArea = fmtStar.getUnseenArea(startPoint)

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

func (f *FmtStar) getBestOpenNeighbor(point *geom.Coord, neighborhoodSize float64) (*Node, float64, []*Node, []float64) {
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
		cost := f.getCost(&neighbor.Coord, point)
		neighborCosts = append(neighborCosts, cost)
		if cost+neighbor.CumulativeCost < bestCumulativeCost {
			bestCost = cost
			bestCumulativeCost = cost + neighbor.CumulativeCost
			bestNeighbor = neighbor
		}
	}

	return bestNeighbor, bestCost, neighbors, neighborCosts
}

func (f *FmtStar) refreshBestPath() {
	f.traceBestPath()
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
			bestParent, bestCost, _, _ := f.getBestOpenNeighbor(&neighbor.Coord, f.rewireNeighborhood)

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
	bestNeighbor, _, neighbors, _ := f.getBestNeighbor(&point, float64(f.rewireNeighborhood*1.5))
	for _, neighbor := range neighbors {
		if bestNeighbor != nil && neighbor != bestNeighbor && !f.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
			cost := f.getCost(&bestNeighbor.Coord, &neighbor.Coord)
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

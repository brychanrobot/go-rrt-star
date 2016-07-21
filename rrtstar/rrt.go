package rrtstar

import (
	"image"
	"math"

	"github.com/brychanrobot/go-halton"
	"github.com/dhconnelly/rtreego"
	"github.com/skelterjohn/geom"
)

// RrtStar holds all of the information for an rrt*
type RrtStar struct {
	PlannerBase
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
		PlannerBase: PlannerBase{
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
			haltonY:            halton.NewHaltonSampler(23),
			unseenAreaMap:      make(map[geom.Coord]float64)}}

	rrtStar.Viewshed.LoadMap(float64(width), float64(height), 0, obstacleRects, nil)
	//rrtStar.Viewshed.UpdateCenterLocation(float64(startPoint.X), float64(startPoint.Y))
	//rrtStar.Viewshed.Sweep()

	//rrtStar.renderCostMap()
	rrtStar.Root.UnseenArea = rrtStar.getUnseenArea(startPoint)

	return rrtStar
}

func (r *RrtStar) refreshBestPath() {
	if r.endNode == nil {
		rtreeEndPoint := rtreego.Point{r.EndPoint.X, r.EndPoint.Y}
		neighbors := r.rtree.SearchIntersect(rtreeEndPoint.ToRect(2 * r.maxSegment))

		//_, unseenArea := r.getCost(r.StartPoint, r.EndPoint)
		//neighborCosts := []float64{}
		bestCost := math.MaxFloat64
		var bestNeighbor *Node
		for _, neighborSpatial := range neighbors {
			neighbor := neighborSpatial.(*Node)
			cost := r.getCost(r.EndPoint, &neighbor.Coord)
			if cost < bestCost && !r.lineIntersectsObstacle(*r.EndPoint, neighbor.Coord, 200) {
				bestCost = cost
				bestNeighbor = neighbor
			}
		}

		if bestNeighbor != nil {
			r.endNode = bestNeighbor.AddAndCreateChild(*r.EndPoint, bestCost, 0.0)
			r.NumNodes++
			r.rtree.Insert(r.endNode)
			r.traceBestPath()
		}
	} else {
		r.traceBestPath()
	}
}

/*func (r *RrtStar) getCostKnownUnseenArea(neighbor *geom.Coord, point *geom.Coord, unseenArea float64) float64 {
	dist := euclideanDistance(neighbor, point)
	return dist*distanceK + unseenArea*unseenK
}
*/

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

		//unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
		bestNeighbor, bestCost, neighbors, neighborCosts := r.getBestNeighbor(&point, r.rewireNeighborhood)

		if bestNeighbor != nil { //!r.lineIntersectsObstacle(point, bestNeighbor.Point, 200) {
			//unseenArea := (r.mapArea - r.getViewArea(&point)) / r.mapArea
			newNode := bestNeighbor.AddAndCreateChild(point, bestCost, 0.0)
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
	bestNeighbor, _, neighbors, _ := r.getBestNeighbor(&point, float64(r.rewireNeighborhood))
	for _, neighbor := range neighbors {
		if neighbor != bestNeighbor && !r.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
			cost := r.getCost(&bestNeighbor.Coord, &neighbor.Coord)
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

package rrtstar

import (
	"image"
	"image/png"
	"log"
	"math"
	"os"

	halton "github.com/brychanrobot/go-halton"
	"github.com/brychanrobot/go-rrt-star/viewshed"
	"github.com/dhconnelly/rtreego"
	"github.com/gonum/matrix/mat64"
	"github.com/skelterjohn/geom"
)

const (
	unseenK   = 4.0
	distanceK = 0.00
)

type Planner interface {
	GetRoot() *Node
	GetStartPoint() *geom.Coord
	GetEndPoint() *geom.Coord
	GetBestPath() []*geom.Coord
	GetViewshed() *viewshed.Viewshed
	GetIsAddingNodes() bool
	GetNumNodes() uint64

	Sample()
	RenderUnseenCostMap(filename string)
	MoveStartPoint(dx, dy float64)
	MoveEndPoint(dx, dy float64)
}

type PlannerBase struct {
	obstacleImage      *image.Gray
	obstacleRects      []*geom.Rect
	rtree              *rtreego.Rtree
	Root               *Node
	StartPoint         *geom.Coord
	EndPoint           *geom.Coord
	maxSegment         float64
	rewireNeighborhood float64
	width              int
	height             int
	mapArea            float64
	endNode            *Node
	BestPath           []*geom.Coord
	Viewshed           viewshed.Viewshed
	nodeThreshold      uint64
	IsAddingNodes      bool
	NumNodes           uint64
	haltonX            *halton.HaltonSampler
	haltonY            *halton.HaltonSampler
	unseenAreaMap      map[geom.Coord]float64
	obstacleArea       float64
}

//Getters
func (p *PlannerBase) GetRoot() *Node {
	return p.Root
}

func (p *PlannerBase) GetStartPoint() *geom.Coord {
	return p.StartPoint
}

func (p *PlannerBase) GetEndPoint() *geom.Coord {
	return p.EndPoint
}

func (p *PlannerBase) GetBestPath() []*geom.Coord {
	return p.BestPath
}

func (p *PlannerBase) GetViewshed() *viewshed.Viewshed {
	return &p.Viewshed
}

func (p *PlannerBase) GetIsAddingNodes() bool {
	return p.IsAddingNodes
}

func (p *PlannerBase) GetNumNodes() uint64 {
	return p.NumNodes
}

func (p *PlannerBase) RenderUnseenCostMap(filename string) {
	costMap := mat64.NewDense(p.height, p.width, nil)
	costMapImg := image.NewGray(image.Rect(0, 0, p.width, p.height))

	for row := 0; row < p.height; row++ {
		for col := 0; col < p.width; col++ {
			if p.obstacleImage.GrayAt(col, row).Y < 200 {
				point := geom.Coord{X: float64(col), Y: float64(row)}
				costMap.Set(row, col, p.getViewArea(&point))
			}
		}
	}

	costMap.Scale(255/mat64.Max(costMap), costMap)

	for row := 0; row < p.height; row++ {
		for col := 0; col < p.width; col++ {
			costMapImg.Pix[row*p.width+col] = uint8(costMap.At(row, col))
			//costMapImg.Set(row, col, uint8(costMap.At()))
		}
	}

	toimg, _ := os.Create(filename)
	defer toimg.Close()

	png.Encode(toimg, costMapImg)
}

func (p *PlannerBase) traceBestPath() {
	p.BestPath = p.BestPath[:0]
	currentNode := p.endNode
	for currentNode != nil {
		p.BestPath = append(p.BestPath, &currentNode.Coord)
		currentNode = currentNode.parent
	}
}

func (p *PlannerBase) getViewArea(point *geom.Coord) float64 {
	p.Viewshed.UpdateCenterLocation(point.X, point.Y)
	p.Viewshed.Sweep()
	return viewshed.Area2DPolygon(p.Viewshed.ViewablePolygon)
}

func (p *PlannerBase) getUnseenArea(point *geom.Coord) float64 {
	//return (p.mapArea - p.getViewArea(point)) / p.mapArea
	value := p.unseenAreaMap[*point]
	if value == 0 {
		value = (p.mapArea - p.obstacleArea - p.getViewArea(point)) / (p.mapArea + p.obstacleArea)
		p.unseenAreaMap[*point] = value
	}

	return value
}

func (p *PlannerBase) getEdgeUnseenArea(p1, p2 *geom.Coord) (float64, float64) {
	/*angle := angleBetweenPoints(*p1, *p2)
	dist := euclideanDistance(p1, p2)
	sum := 0.0
	for i := 5.0; i < dist; i += 5 {
		x := p.maxSegment*math.Cos(angle) + p1.X
		y := p.maxSegment*math.Sin(angle) + p1.Y
		point := geom.Coord{X: x, Y: y}
		value := p.unseenAreaMap[point]
		if value == 0 {
			value = p.getUnseenArea(&point)
			p.unseenAreaMap[point] = value
		}

		sum += value
	}
	return sum
	*/

	dist := euclideanDistance(p1, p2)

	a1 := p.getUnseenArea(p1)
	//am := p.getUnseenArea(&geom.Coord{X: float64(int((p1.X + p2.X) / 2.0)), Y: float64(int((p1.Y + p2.Y) / 2.0))})
	a2 := p.getUnseenArea(p2)

	unseenArea := ((a1 + a2) / 2.0) * dist

	return unseenArea, dist
}

func (p *PlannerBase) getCost(neighbor *geom.Coord, point *geom.Coord) float64 {

	unseenArea, dist := p.getEdgeUnseenArea(neighbor, point)
	//dist := euclideanDistance(neighbor, point)
	return dist*distanceK + unseenArea*unseenK
}

func (p *PlannerBase) getBestNeighbor(point *geom.Coord, neighborhoodSize float64) (*Node, float64, []*Node, []float64) {
	rtreePoint := rtreego.Point{point.X, point.Y}
	spatialNeighbors := p.rtree.SearchIntersect(rtreePoint.ToRect(neighborhoodSize))
	neighborCosts := []float64{}
	neighbors := []*Node{}
	bestCost := math.MaxFloat64
	bestCumulativeCost := math.MaxFloat64
	var bestNeighbor *Node
	for _, spatialNeighbor := range spatialNeighbors {
		neighbor := spatialNeighbor.(*Node)
		if neighbor.Coord != *point && !p.lineIntersectsObstacle(*point, neighbor.Coord, 200) {
			neighbors = append(neighbors, neighbor)
			cost := p.getCost(&neighbor.Coord, point)
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

func (p *PlannerBase) MoveStartPoint(dx, dy float64) {
	if dx != 0 || dy != 0 {
		p.StartPoint.X += dx
		p.StartPoint.Y += dy
		//log.Println(p.StartPoint)
		newRoot := &Node{parent: nil, Coord: *p.StartPoint, CumulativeCost: 0}
		p.NumNodes++
		//newRoot.UnseenArea = p.getUnseenArea(&newRoot.Coord)
		p.rtree.Insert(newRoot)

		p.Root.Rewire(newRoot, p.getCost(&newRoot.Coord, &p.Root.Coord))
		p.Root = newRoot

		_, _, neighbors, neighborCosts := p.getBestNeighbor(&p.Root.Coord, p.rewireNeighborhood*1.5)
		for i, neighbor := range neighbors {
			if !p.lineIntersectsObstacle(p.Root.Coord, neighbor.Coord, 200) {
				if neighborCosts[i]+p.Root.CumulativeCost < neighbor.CumulativeCost {
					neighbor.Rewire(p.Root, neighborCosts[i])
				}
			}
		}
	}
}

func (p *PlannerBase) MoveEndPoint(dx, dy float64) {
	if dx != 0 || dy != 0 {
		p.EndPoint.X += dx
		p.EndPoint.Y += dy

		bestNeighbor, bestCost, _, _ := p.getBestNeighbor(p.EndPoint, p.rewireNeighborhood)

		if bestNeighbor != nil {
			p.endNode = bestNeighbor.AddAndCreateChild(*p.EndPoint, bestCost, 0.0)
			p.NumNodes++

			p.rtree.Insert(p.endNode)
		} else {
			p.EndPoint.X -= dx
			p.EndPoint.Y -= dy
		}
	}
}

func (p *PlannerBase) Prune(minorAxisSquares int) {
	var squareSize int
	if p.height < p.width {
		squareSize = int(p.height / minorAxisSquares)
	} else {
		squareSize = int(p.width / minorAxisSquares)
	}

	log.Printf("sq: %d", squareSize)

	for cy := int(squareSize / 2); cy < p.height; cy += squareSize {
		for cx := int(squareSize / 2); cx < p.width; cx += squareSize {
			cPoint := geom.Coord{X: float64(cx), Y: float64(cy)}
			bestNeighbor, _, neighbors, _ := p.getBestNeighbor(&cPoint, float64(squareSize))
			for _, neighbor := range neighbors {
				if neighbor != bestNeighbor && !p.lineIntersectsObstacle(bestNeighbor.Coord, neighbor.Coord, 200) {
					cost := p.getCost(&bestNeighbor.Coord, &neighbor.Coord)
					if cost+bestNeighbor.CumulativeCost < neighbor.CumulativeCost {
						neighbor.Rewire(bestNeighbor, cost)
					}
				}
			}

			for _, neighbor := range neighbors {
				if len(neighbor.Children) == 0 {
					neighbor.parent.RemoveChild(neighbor)
					p.rtree.Delete(neighbor)
					p.NumNodes--
				}
			}
		}
	}
}

func (p *PlannerBase) nextHaltonPoint(width, height int) geom.Coord {
	x := p.haltonX.Next() * float64(width)
	y := p.haltonY.Next() * float64(height)
	return geom.Coord{X: x, Y: y}
}

func (p *PlannerBase) lineIntersectsObstacle(p1 geom.Coord, p2 geom.Coord, minObstacleColor uint8) bool {
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
		if p.obstacleImage.GrayAt(int(ix), int(y)).Y > minObstacleColor {
			return true
		}
	}

	minY := math.Min(p1.Y, p2.Y)
	maxY := math.Max(p1.Y, p2.Y)
	for iY := minY; iY <= maxY; iY++ {
		x := (iY - b) / m
		if p.obstacleImage.GrayAt(int(x), int(iY)).Y > minObstacleColor {
			return true
		}
	}

	return false
}

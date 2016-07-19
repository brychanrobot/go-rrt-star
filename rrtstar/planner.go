package rrtstar

import (
	"github.com/brychanrobot/rrt-star/viewshed"
	"github.com/skelterjohn/geom"
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
}

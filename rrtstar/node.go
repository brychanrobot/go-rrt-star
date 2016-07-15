package rrtstar

import (
	"github.com/dhconnelly/rtreego"
	"github.com/skelterjohn/geom"
)

var (
	tolerance = 0.01
)

type Status uint32

const (
	Unvisited Status = iota
	Open
	Closed
)

// Node Represents an RRT Node
type Node struct {
	geom.Coord
	parent         *Node
	Children       []*Node
	CumulativeCost float64
	UnseenArea     float64
	Status         Status
}

// AddChild adds a child and updates cost
func (n *Node) AddAndCreateChild(point geom.Coord, cost, unseenArea float64) *Node {
	newNode := Node{
		parent:         n,
		Coord:          point,
		CumulativeCost: n.CumulativeCost + cost,
		UnseenArea:     unseenArea}
	n.Children = append(n.Children, &newNode)

	return &newNode
}

func (n *Node) AddChild(child *Node, cost, unseenArea float64) {
	child.CumulativeCost = n.CumulativeCost + cost
	child.UnseenArea = unseenArea
	n.Children = append(n.Children, child)
	child.parent = n

	//fmt.Printf("new child with cost %f\n", child.CumulativeCost)
}

// RemoveChild removes a child from a node
func (n *Node) RemoveChild(child *Node) {
	for i, value := range n.Children {
		if value == child {
			n.Children = append(n.Children[:i], n.Children[i+1:]...) // this looks like voodoo, but it's just deleting element i gotta love go
			break
		}
	}
}

func (n *Node) updateCumulativeCost(newCumulativeCost float64) {
	oldCumulativeCost := n.CumulativeCost
	n.CumulativeCost = newCumulativeCost
	for _, child := range n.Children {
		costToParent := child.CumulativeCost - oldCumulativeCost
		child.updateCumulativeCost(newCumulativeCost + costToParent)
	}
}

// Rewire attaches a node to a new parent and updates costs
func (n *Node) Rewire(newParent *Node, cost float64) {
	if n.parent != nil {
		n.parent.RemoveChild(n)
	}
	newParent.Children = append(newParent.Children, n)
	n.parent = newParent
	n.updateCumulativeCost(newParent.CumulativeCost + cost)
}

// Bounds returns empty rect with top left at the node point for rtreego
func (n *Node) Bounds() *rtreego.Rect {
	p := rtreego.Point{n.Coord.X, n.Coord.Y}

	return p.ToRect(tolerance)
}

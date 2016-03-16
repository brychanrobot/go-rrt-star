package rrtstar

import (
	"image"

	"github.com/dhconnelly/rtreego"
)

var (
	tolerance = 0.01
)

// Node Represents an RRT Node
type Node struct {
	parent         *Node
	Point          image.Point
	Children       []*Node
	CumulativeCost float64
	//UnseenArea     float64
}

// AddChild adds a child and updates cost
func (n *Node) AddChild(point image.Point, cost float64) *Node {
	newNode := Node{
		parent:         n,
		Point:          point,
		CumulativeCost: n.CumulativeCost + cost}
	n.Children = append(n.Children, &newNode)

	return &newNode
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
	n.parent.RemoveChild(n)
	newParent.Children = append(newParent.Children, n)
	n.parent = newParent
	n.updateCumulativeCost(newParent.CumulativeCost + cost)
}

// Bounds returns empty rect with top left at the node point for rtreego
func (n *Node) Bounds() *rtreego.Rect {
	p := rtreego.Point{float64(n.Point.X), float64(n.Point.Y)}

	return p.ToRect(tolerance)
}

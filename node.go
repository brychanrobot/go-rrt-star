package main

import (
	"image"

	"github.com/dhconnelly/rtreego"
)

var tolerance = 0.01

// Node Represents an RRT Node
type Node struct {
	parent         *Node
	point          image.Point
	children       []*Node
	cumulativeCost float64
}

// AddChild adds a child and updates cost
func (n *Node) AddChild(point image.Point, cost float64) *Node {
	newNode := Node{parent: n, point: point, cumulativeCost: n.cumulativeCost + cost}
	n.children = append(n.children, &newNode)
	//n.children.PushBack(&newNode)

	return &newNode
}

// RemoveChild removes a child from a node
func (n *Node) RemoveChild(child *Node) {
	for i, value := range n.children {
		if value == child {
			n.children = append(n.children[:i], n.children[i+1:]...) // this looks like voodoo, but it's just deleting element i gotta love go
		}
	}
}

func (n *Node) updateCumulativeCost(newCumulativeCost float64) {
	oldCumulativeCost := n.cumulativeCost
	n.cumulativeCost = newCumulativeCost
	for _, child := range n.children {
		costToParent := child.cumulativeCost - oldCumulativeCost
		child.updateCumulativeCost(newCumulativeCost + costToParent)
	}
}

// Rewire attaches a node to a new parent and updates costs
func (n *Node) Rewire(newParent *Node, cost float64) {
	n.parent.RemoveChild(n)
	newParent.children = append(newParent.children, n)
	n.parent = newParent
	n.updateCumulativeCost(newParent.cumulativeCost + cost)
}

// Bounds returns empty rect with top left at the node point for rtreego
func (n *Node) Bounds() *rtreego.Rect {
	p := rtreego.Point{float64(n.point.X), float64(n.point.Y)}

	return p.ToRect(tolerance)
}

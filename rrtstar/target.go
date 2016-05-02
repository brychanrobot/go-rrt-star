package rrtstar

import (
	"image"
	"math"
	"math/rand"
)

type MovementType uint32

const (
	RandomWalk MovementType = iota
)

const maxDistance = 5

type Target struct {
	image.Point
	movementType  MovementType
	obstacleImage *image.Gray
	mapBounds     image.Rectangle
	Importance    uint32
	heading       float64
}

func NewTarget(movementType MovementType, importance uint32, obstacleImage *image.Gray) *Target {
	target := &Target{
		movementType:  movementType,
		Importance:    importance,
		obstacleImage: obstacleImage,
		mapBounds:     obstacleImage.Bounds()}

	target.Point = *randomOpenAreaPoint(obstacleImage, target.mapBounds.Dx(), target.mapBounds.Dy())
	//log.Println(target.Point)
	return target
}

func (t *Target) walkRandomly() {
	isInObstacle := true
	var newPoint image.Point
	var newHeading float64
	for isInObstacle {
		newHeading = t.heading + rand.Float64()*math.Pi*0.1 - math.Pi*0.05
		distance := rand.Float64() * maxDistance

		dx := distance * math.Cos(newHeading)
		dy := distance * math.Sin(newHeading)

		//log.Printf("%f, %f\n", dx, dy)

		newPoint = t.Point.Add(image.Pt(int(dx), int(dy)))

		isInObstacle = !pointInRectangle(newPoint, t.mapBounds) || pointIntersectsObstacle(newPoint, t.obstacleImage, 20)
	}

	//log.Println(newPoint)
	t.Point = newPoint
	t.heading = newHeading
}

func (t *Target) MoveTarget() {
	switch t.movementType {
	case RandomWalk:
		t.walkRandomly()
	}
}

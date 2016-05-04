package rrtstar

import (
	"image"
	"math"
	"math/rand"
)

type MovementType uint32

const (
	RandomWalk MovementType = iota
	RandomRrt
)

const maxDistance = 5

type Waldo struct {
	image.Point
	movementType  MovementType
	obstacleImage *image.Gray
	obstacleRects []*image.Rectangle
	mapBounds     image.Rectangle
	Importance    uint32
	heading       float64
	//rrtStar       *RrtStar
	CurrentPath []*image.Point
}

func NewWaldo(movementType MovementType, importance uint32, obstacleImage *image.Gray) *Waldo {
	waldo := &Waldo{
		movementType:  movementType,
		Importance:    importance,
		obstacleImage: obstacleImage,
		mapBounds:     obstacleImage.Bounds()}

	waldo.Point = *randomOpenAreaPoint(obstacleImage, waldo.mapBounds.Dx(), waldo.mapBounds.Dy())
	//log.Println(waldo.Point)
	return waldo
}

func (w *Waldo) walkRandomly() {
	isInObstacle := true
	var newPoint image.Point
	var newHeading float64
	for isInObstacle {
		newHeading = w.heading + rand.Float64()*math.Pi*0.1 - math.Pi*0.05
		distance := rand.Float64() * maxDistance

		dx := distance * math.Cos(newHeading)
		dy := distance * math.Sin(newHeading)

		//log.Printf("%f, %f\n", dx, dy)

		newPoint = w.Point.Add(image.Pt(int(dx), int(dy)))

		isInObstacle = !rectangleContainsPoint(w.mapBounds, newPoint) || pointIntersectsObstacle(newPoint, w.obstacleImage, 20)
	}

	//log.Println(newPoint)
	w.Point = newPoint
	w.heading = newHeading
}

func (w *Waldo) followRrtPath() {
	if len(w.CurrentPath) == 0 {
		rrtStar := NewRrtStar(w.obstacleImage, w.obstacleRects, 30, w.mapBounds.Dx(), w.mapBounds.Dy(), &w.Point, nil)
		for len(rrtStar.BestPath) == 0 {
			rrtStar.SampleRrtStar()
		}
		w.CurrentPath = rrtStar.BestPath[:len(rrtStar.BestPath)-1]
	}

}

func (w *Waldo) MoveWaldo() {
	switch w.movementType {
	case RandomWalk:
		w.walkRandomly()
	case RandomRrt:
		w.followRrtPath()
	}
}

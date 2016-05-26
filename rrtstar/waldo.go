package rrtstar

import (
	"image"
	"math"

	"github.com/skelterjohn/geom"
)

type MovementType uint32

const (
	RandomWalk MovementType = iota
	RandomRrt
)

const maxTravel = 2

type Waldo struct {
	geom.Coord
	movementType  MovementType
	obstacleImage *image.Gray
	obstacleRects []*geom.Rect
	mapBounds     geom.Rect
	Importance    uint32
	heading       float64
	//rrtStar       *RrtStar
	CurrentPath     []*geom.Coord
	Replanning      bool
	CurrentWaypoint *geom.Coord
}

func NewWaldo(movementType MovementType, importance uint32, obstacleImage *image.Gray) *Waldo {
	mapBounds := obstacleImage.Bounds()
	waldo := &Waldo{
		movementType:  movementType,
		Importance:    importance,
		obstacleImage: obstacleImage,
		mapBounds:     geom.Rect{Min: geom.Coord{X: float64(mapBounds.Min.X), Y: float64(mapBounds.Min.Y)}, Max: geom.Coord{X: float64(mapBounds.Max.X), Y: float64(mapBounds.Max.Y)}}}

	waldo.Coord = *randomOpenAreaPoint(obstacleImage, int(waldo.mapBounds.Width()), int(waldo.mapBounds.Height()))
	//log.Println(waldo.Point)
	return waldo
}

/*
func (w *Waldo) walkRandomly() {
	isInObstacle := true
	var newPoint geom.Coord
	var newHeading float64
	for isInObstacle {
		newHeading = w.heading + rand.Float64()*math.Pi*0.1 - math.Pi*0.05
		distance := rand.Float64() * maxTravel

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
*/

func (w *Waldo) followRrtPath() {
	if !w.Replanning {
		if len(w.CurrentPath) == 0 {
			w.Replanning = true
			go func() {
				rrtStar := NewRrtStar(w.obstacleImage, w.obstacleRects, 30, int(w.mapBounds.Width()), int(w.mapBounds.Height()), &w.Coord, nil)
				for len(rrtStar.BestPath) == 0 {
					rrtStar.SampleRrtStar()
				}
				w.CurrentPath = rrtStar.BestPath[:len(rrtStar.BestPath)-1]
				w.Replanning = false
			}()
			return
		}

		w.CurrentWaypoint = w.CurrentPath[len(w.CurrentPath)-1]
		if euclideanDistance(&w.Coord, w.CurrentWaypoint) <= maxTravel {
			w.CurrentPath = w.CurrentPath[:len(w.CurrentPath)-1]
			w.Coord = *w.CurrentWaypoint
		} else {
			angle := angleBetweenFloatPoints(w.X, w.Y, float64(w.CurrentWaypoint.X), float64(w.CurrentWaypoint.Y))
			w.Coord.X += maxTravel * math.Cos(angle)
			w.Coord.Y += maxTravel * math.Sin(angle)
		}
	}

}

func (w *Waldo) MoveWaldo() {
	switch w.movementType {
	case RandomWalk:
		//w.walkRandomly()
	case RandomRrt:
		w.followRrtPath()
	}
}

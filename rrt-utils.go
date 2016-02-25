package main

import (
	"image"
	"math"
	"math/rand"
)

func randomPoint(dx int, dy int) image.Point {
	point := image.Pt(int(rand.Int31n(int32(dx))), int(rand.Int31n(int32(dy))))

	return point
}

func euclideanDistance(p1 image.Point, p2 image.Point) float64 {
	dx := float64(p1.X - p2.X)
	dy := float64(p1.Y - p2.Y)
	ss := dx*dx + dy*dy
	return math.Sqrt(ss)
}

func angleBetweenPoints(p1 image.Point, p2 image.Point) float64 {
	return math.Atan2(float64(p2.Y-p1.Y), float64(p2.X-p1.X))
}

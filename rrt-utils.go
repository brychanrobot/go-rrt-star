package main

import (
	"image"
	"math"
	"math/rand"

	"github.com/harrydb/go/img/grayscale"
	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/llgcode/draw2d/draw2dkit"
	"github.com/lucasb-eyer/go-colorful"
)

func randomPoint(dx int, dy int) image.Point {
	point := image.Pt(int(rand.Int31n(int32(dx))), int(rand.Int31n(int32(dy))))

	return point
}

func randomPointFromRectangle(rect *image.Rectangle) image.Point {
	x := rect.Min.X + int(rand.Int31n(int32(rect.Dx())))
	y := rect.Min.Y + int(rand.Int31n(int32(rect.Dy())))
	point := image.Pt(x, y)

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

func hasIntersection(rect *image.Rectangle, obstacles []*image.Rectangle) bool {
	for _, obstacle := range obstacles {
		if rect.Overlaps(*obstacle) {
			return true
		}
	}
	return false
}

func generateObstacles(width int, height int, count int) ([]*image.Rectangle, *image.Gray) {
	var obstacles []*image.Rectangle
	//mapBottomRight := image.Pt(width, height)
	for x := 0; x < count; x++ {
		// keep trying until we get a non-intersecting rectangle
		var rect image.Rectangle
		for true {
			topLeft := randomPoint(int(width), int(height))
			bottomRight := randomPoint(int(width), int(height))
			rect = image.Rect(topLeft.X, topLeft.Y, bottomRight.X, bottomRight.Y)
			if !hasIntersection(&rect, obstacles) {
				break
			}
		}

		obstacles = append(obstacles, &rect)
	}

	return obstacles, generateObstacleImage(width, height, obstacles)
}

func generateObstacleImage(width int, height int, obstacles []*image.Rectangle) *image.Gray {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	gc := draw2dimg.NewGraphicContext(img)

	//gc.SetFillColor(image.NewUniform(colorful.FastHappyColor()))
	gc.SetFillColor(image.NewUniform(colorful.Color{1, 1, 1}))

	for _, obstacle := range obstacles {
		draw2dkit.Rectangle(gc, float64(obstacle.Min.X), float64(obstacle.Min.Y), float64(obstacle.Max.X), float64(obstacle.Max.Y))
	}

	gc.Fill()

	gray := grayscale.Convert(img, grayscale.ToGrayLuma709)

	return gray
}

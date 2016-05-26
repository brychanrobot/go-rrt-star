package rrtstar

import (
	"image"
	"math"
	"math/rand"

	"github.com/harrydb/go/img/grayscale"
	"github.com/llgcode/draw2d/draw2dimg"
	"github.com/llgcode/draw2d/draw2dkit"
	"github.com/lucasb-eyer/go-colorful"
	"github.com/skelterjohn/geom"
)

func inflateRectangle(r *geom.Rect, amount float64) {
	offset := geom.Coord{X: amount, Y: amount}
	r.Min.Minus(offset)
	r.Max.Plus(offset)
}

func rectangleContainsPoint(rect geom.Rect, point geom.Coord) bool {
	return (point.X > rect.Min.X && point.Y > rect.Min.Y) && (point.X < rect.Max.X && point.Y < rect.Max.Y)
}

func pointIntersectsObstacle(point geom.Coord, obstacles *image.Gray, minObstacleColor uint8) bool {
	return obstacles.GrayAt(int(point.X), int(point.Y)).Y > minObstacleColor
}

func randomOpenAreaPoint(obstacles *image.Gray, width int, height int) *geom.Coord {
	var point geom.Coord
	for true {
		point = randomPoint(width, height)
		if !pointIntersectsObstacle(point, obstacles, 200) {
			break
		}
	}

	return &point
}

func randomPoint(dx int, dy int) geom.Coord {
	point := geom.Coord{X: float64(rand.Int31n(int32(dx))), Y: float64(rand.Int31n(int32(dy)))}

	return point
}

func randomPointFromRectangle(rect *geom.Rect) geom.Coord {
	x := rect.Min.X + float64(rand.Int31n(int32(rect.Width())))
	y := rect.Min.Y + float64(rand.Int31n(int32(rect.Height())))
	point := geom.Coord{X: x, Y: y}

	return point
}

func euclideanDistance(p1 *geom.Coord, p2 *geom.Coord) float64 {
	dx := float64(p1.X - p2.X)
	dy := float64(p1.Y - p2.Y)
	ss := dx*dx + dy*dy
	return math.Sqrt(ss)
}

func angleBetweenPoints(p1 geom.Coord, p2 geom.Coord) float64 {
	return math.Atan2(float64(p2.Y-p1.Y), float64(p2.X-p1.X))
}

func angleBetweenFloatPoints(x1, y1, x2, y2 float64) float64 {
	return math.Atan2(y2-y1, x2-x1)
}

func hasIntersection(rect *geom.Rect, obstacles []*geom.Rect) bool {
	for _, obstacle := range obstacles {
		if geom.RectsIntersect(*obstacle, *rect) {
			return true
		}
	}
	return false
}

func GenerateObstacles(width int, height int, count int) ([]*geom.Rect, *image.Gray) {
	var obstacles []*geom.Rect
	//mapBottomRight := image.Pt(width, height)
	for x := 0; x < count; x++ {
		// keep trying until we get a non-intersecting rectangle
		var rect geom.Rect
		for true {
			topLeft := randomPoint(int(width), int(height))
			bottomRight := randomPoint(int(width), int(height))
			rect = geom.Rect{Min: topLeft, Max: bottomRight}
			if rect.Width() > 2 && rect.Height() > 2 && !hasIntersection(&rect, obstacles) {
				break
			}
		}

		obstacles = append(obstacles, &rect)
	}

	return obstacles, generateObstacleImage(width, height, obstacles)
}

func generateObstacleImage(width int, height int, obstacles []*geom.Rect) *image.Gray {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	gc := draw2dimg.NewGraphicContext(img)

	//gc.SetFillColor(image.NewUniform(colorful.FastHappyColor()))
	gc.SetFillColor(image.NewUniform(colorful.Color{R: 1, G: 1, B: 1}))

	for _, obstacle := range obstacles {
		draw2dkit.Rectangle(gc, float64(obstacle.Min.X), float64(obstacle.Min.Y), float64(obstacle.Max.X), float64(obstacle.Max.Y))
		inflateRectangle(obstacle, -5)
	}

	gc.Fill()

	gray := grayscale.Convert(img, grayscale.ToGrayLuma709)

	return gray
}

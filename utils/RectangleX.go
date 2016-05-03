package utils

import "image"

type RectangleX image.Rectangle

func (r *RectangleX) Contains(point image.Point) bool {
	return (point.X > r.Min.X && point.Y > r.Min.Y) && (point.X < r.Max.X && point.Y < r.Max.Y)
}

func (r *RectangleX) Inflate(amount int) {
	offset := image.Pt(amount, amount)
	r.Min.Sub(offset)
	r.Max.Add(offset)
}

// Open an OpenGl window and display a rectangle using a OpenGl GraphicContext
package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/disintegration/imaging"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
	"github.com/harrydb/go/img/grayscale"
	"github.com/llgcode/draw2d"
	"github.com/lucasb-eyer/go-colorful"
)

var (
	// global rotation
	rotate           int
	width, height    int
	redraw           = true
	font             draw2d.FontData
	obstaclesTexture uint32
	rrtstar          *Star
)

func reshape(window *glfw.Window, w, h int) {
	gl.ClearColor(1, 1, 1, 1)
	/* Establish viewing area to cover entire window. */
	gl.Viewport(0, 0, int32(w), int32(h))
	/* PROJECTION Matrix mode. */
	gl.MatrixMode(gl.PROJECTION)
	/* Reset project matrix. */
	gl.LoadIdentity()
	/* Map abstract coords directly to window coords. */
	gl.Ortho(0, float64(w), 0, float64(h), -1, 1)
	/* Invert Y axis so increasing Y goes down. */
	gl.Scalef(1, -1, 1)
	/* Shift origin up to upper-left corner. */
	gl.Translatef(0, float32(-h), 0)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	width, height = w, h
	invalidate()
}

func readImage(filename string) *image.RGBA {
	reader, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		panic("unsupported stride")
	}
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	return rgba
}

func readImageGray(filename string) *image.Gray {
	reader, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Fatal(err)
	}

	/*
		for i := range img.Pix {
			gray.Pix[i] = 255 - img.Pix[i]
		}
	*/

	inverted := imaging.Invert(img)
	gray := grayscale.Convert(inverted, grayscale.ToGrayLuma709)

	//log.Println(gray.Pix)

	return gray
}

func convertUint8ToFloat64(in []uint8, multiplier float64) []float64 {
	out := make([]float64, len(in))
	for i, value := range in {
		out[i] = float64(value) * multiplier
	}

	return out
}

// Ask to refresh
func invalidate() {
	redraw = true
}

func getTexture(rgba *image.RGBA) uint32 {
	var texture uint32
	//gl.Enable(gl.TEXTURE_2D)
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return texture
}

func getTextureGray(gray *image.Gray) uint32 {
	var texture uint32
	//gl.Enable(gl.TEXTURE_2D)
	gl.GenTextures(1, &texture)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.LUMINANCE,
		int32(gray.Rect.Size().X),
		int32(gray.Rect.Size().Y),
		0,
		gl.LUMINANCE,
		gl.UNSIGNED_BYTE,
		gl.Ptr(gray.Pix))

	//gl.Disable(gl.TEXTURE_2D)

	return texture
}

func drawCircle(point image.Point, radius float64, numSegments int, color colorful.Color) {
	theta := 2 * 3.1415926 / float64(numSegments)
	tangentialFactor := math.Tan(theta) //calculate the tangential factor

	radialFactor := math.Cos(theta) //calculate the radial factor

	x := radius //we start at angle = 0
	y := 0.0

	cx := float64(point.X)
	cy := float64(point.Y)

	gl.Begin(gl.LINE_LOOP)
	gl.Color3d(color.R, color.G, color.B)
	for ii := 0; ii < numSegments; ii++ {
		gl.Vertex2d(x+cx, y+cy) //output vertex

		//calculate the tangential vector
		//remember, the radial vector is (x, y)
		//to get the tangential vector we flip those coordinates and negate one of them

		tx := -y
		ty := x

		//add the tangential vector

		x += tx * tangentialFactor
		y += ty * tangentialFactor

		//correct using the radial factor

		x *= radialFactor
		y *= radialFactor
	}
	gl.End()
}

func drawPoint(point image.Point, radius float32, color colorful.Color) {
	gl.Enable(gl.POINT_SMOOTH)
	gl.Hint(gl.POINT_SMOOTH_HINT, gl.NICEST)
	gl.PointSize(radius)

	gl.Begin(gl.POINTS)
	gl.Color3d(color.R, color.G, color.B)
	gl.Vertex2d(float64(point.X), float64(point.Y))
	gl.End()
}

func drawLine(p1 image.Point, p2 image.Point, color colorful.Color) {
	gl.Color3d(color.R, color.G, color.B)
	gl.Begin(gl.LINES)
	gl.Vertex2d(float64(p1.X), float64(p1.Y))
	gl.Vertex2d(float64(p2.X), float64(p2.Y))
	gl.End()
}

func drawTree(node *Node) {
	for _, child := range node.children {
		hue := math.Max(0, 60-child.cumulativeCost/12.0)
		drawLine(node.point, child.point, colorful.Hsv(hue, 1, 1))
		drawTree(child)
	}

	drawPoint(node.point, 2, colorful.Hsv(math.Max(0, 60-node.cumulativeCost/12.0), 1, 1))
}

func drawBackground(color colorful.Color) {
	gl.Color3d(color.R, color.G, color.B)
	gl.Enable(gl.TEXTURE_2D)
	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0, 0)
	gl.Vertex3f(0, 0, 0)

	gl.TexCoord2f(0, 1)
	gl.Vertex3f(0, float32(height), 0)

	gl.TexCoord2f(1, 1)
	gl.Vertex3f(float32(width), float32(height), 0)

	gl.TexCoord2f(1, 0)
	gl.Vertex3f(float32(width), 0, 0)
	gl.End()
	gl.Disable(gl.TEXTURE_2D)
}

func display() {
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	gl.LineWidth(1)

	drawBackground(colorful.Hsv(210, 1, 0.6))

	drawTree(rrtstar.rrtRoot)

	gl.Flush() /* Single buffered, so needs a flush. */
}

func init() {
	runtime.LockOSThread()
}

func main() {
	rand.Seed(time.Now().UnixNano()) // apparently golang random is deterministic by default

	//obstacles := readImageGray("dragon.png")
	_, obstacles := generateObstacles(700, 700, 5)
	bounds := obstacles.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()

	//obstacleMatrix := mat64.NewDense(height, width, convertUint8ToFloat64(obstacles.Pix, 1/255.0))

	//log.Print("%s", imgMatrix)

	rrtstar = Create(obstacles, width, height)

	err := glfw.Init()
	if err != nil {
		panic(err)
	}
	defer glfw.Terminate()
	window, err := glfw.CreateWindow(width, height, "Show RoundedRect", nil, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()
	window.SetSizeCallback(reshape)
	window.SetKeyCallback(onKey)
	window.SetCharCallback(onChar)

	glfw.SwapInterval(1)

	err = gl.Init()
	if err != nil {
		panic(err)
	}

	fmt.Println(gl.GoStr(gl.GetString(gl.VERSION)))
	fmt.Println(gl.GoStr(gl.GetString(gl.VENDOR)))
	fmt.Println(gl.GoStr(gl.GetString(gl.RENDERER)))

	obstaclesTexture = getTextureGray(obstacles)
	defer gl.DeleteTextures(1, &obstaclesTexture)

	reshape(window, width, height)
	for i := 0; !window.ShouldClose(); i++ {

		if i < 15000 {
			rrtstar.SampleRrtStar()
			if i%100 == 0 {
				invalidate()
			}
		}

		if redraw {
			log.Println("redrawing", i)

			display()
			window.SwapBuffers()
			redraw = false
		}
		glfw.PollEvents()
		//		time.Sleep(2 * time.Second)
	}
}

func onChar(w *glfw.Window, char rune) {
	log.Println(char)
}

func onKey(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	switch {
	case key == glfw.KeyEscape && action == glfw.Press,
		key == glfw.KeyQ && action == glfw.Press:
		w.SetShouldClose(true)
	}
}

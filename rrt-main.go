// Open an OpenGl window and display a rectangle using a OpenGl GraphicContext
package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/brychanrobot/rrt-star/rrtstar"
	"github.com/brychanrobot/rrt-star/viewshed"
	"github.com/disintegration/imaging"
	"github.com/go-gl/gl/v2.1/gl"
	"github.com/go-gl/glfw/v3.1/glfw"
	"github.com/go-gl/gltext"
	"github.com/harrydb/go/img/grayscale"
	"github.com/lucasb-eyer/go-colorful"
)

var (
	// global rotation
	rotate           int
	width, height    int
	redraw           = true
	font             *gltext.Font
	obstaclesTexture uint32
	obstacleRects    []*image.Rectangle
	rrtStar          *rrtstar.RrtStar
	frames           []*image.NRGBA
	frameCount       int

	cursorX float64
	cursorY float64

	moveX float64
	moveY float64

	targets []*rrtstar.Target
)

type Alignment uint32

const (
	Center Alignment = iota
	Top
	Bottom
	Left
	Right
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

// loadFont loads the specified font at the given scale.
func loadFont(file string, scale int32) (*gltext.Font, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	return gltext.LoadTruetype(fd, scale, 32, 127, gltext.LeftToRight)
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
	gl.LineWidth(1)
	gl.Begin(gl.LINES)
	gl.Vertex2d(float64(p1.X), float64(p1.Y))
	gl.Vertex2d(float64(p2.X), float64(p2.Y))
	gl.End()
}

func drawTree(node *rrtstar.Node, lineHue float64) {
	for _, child := range node.Children {
		hue := int(lineHue+child.CumulativeCost/12.0) % 360
		drawLine(node.Point, child.Point, colorful.Hsv(float64(hue), 1, 0.6))
		drawTree(child, lineHue)
	}

	drawPoint(node.Point, 2, colorful.Hsv(float64(int(lineHue+node.CumulativeCost/12.0)%360), 1, 0.6))
}

func drawNode(node *rrtstar.Node, lineHue float64) {
	for _, child := range node.Children {
		hue := int(lineHue+child.CumulativeCost/12.0) % 360
		color := colorful.Hsv(float64(hue), 1, 0.6)
		//drawLine(node.Point, child.Point, colorful.Hsv(float64(hue), 1, 0.6))
		//drawTree(child, lineHue)
		gl.Color3d(color.R, color.G, color.B)
		gl.Vertex2d(float64(node.Point.X), float64(node.Point.Y))
		gl.Vertex2d(float64(child.Point.X), float64(child.Point.Y))

		drawNode(child, lineHue)
	}
}

func drawTreeFaster(node *rrtstar.Node, lineHue float64) {
	gl.LineWidth(1)
	gl.Begin(gl.LINES)

	drawNode(node, lineHue)

	gl.End()
}

func drawPath(path []*image.Point, color colorful.Color, thickness float32) {
	gl.Enable(gl.LINE_SMOOTH)
	//gl.Enable(gl.BLEND)

	gl.LineWidth(thickness)
	gl.Begin(gl.LINE_STRIP)
	gl.Color3d(color.R, color.G, color.B)
	for _, point := range path {
		gl.Vertex2d(float64(point.X), float64(point.Y))
	}
	gl.End()

	gl.Disable(gl.LINE_SMOOTH)
	//gl.Disable(gl.BLEND)
}

func drawViewshed(path []*viewshed.Point, center *viewshed.Point, color colorful.Color, thickness float32) {
	gl.Enable(gl.LINE_SMOOTH)
	//gl.Enable(gl.SMOOTH)
	//gl.ShadeModel(gl.SMOOTH_QUADRATIC_CURVE_TO_NV)

	gl.Begin(gl.TRIANGLE_FAN)
	gl.Color4d(color.R, color.G, color.B, 0.5)
	gl.Vertex2d(center.X, center.Y)
	for _, point := range path {
		gl.Vertex2d(point.X, point.Y)
	}
	gl.Vertex2d(path[0].X, path[0].Y)
	gl.End()

	gl.LineWidth(thickness)
	gl.Begin(gl.LINE_LOOP)
	gl.Color4d(color.R, color.G, color.B, 1)
	for _, point := range path {
		gl.Vertex2d(point.X, point.Y)
	}
	gl.End()

	//gl.Disable(gl.SMOOTH)
	gl.Disable(gl.LINE_SMOOTH)
	//gl.Disable(gl.BLEND)
}

func drawViewshedSegments(segments []*viewshed.Segment, color colorful.Color, thickness float32) {
	gl.LineWidth(thickness)
	gl.Begin(gl.LINES)
	gl.Color3d(color.R, color.G, color.B)
	for _, segment := range segments {
		gl.Vertex2d(segment.P1.X, segment.P1.Y)
		gl.Vertex2d(segment.P2.X, segment.P2.Y)
	}
	gl.End()
}

func drawBackground(color colorful.Color) {
	gl.Color3d(color.R, color.G, color.B)
	gl.Enable(gl.TEXTURE_2D)
	gl.Begin(gl.QUADS)
	gl.TexCoord2f(0, 0)
	gl.Vertex2f(0, 0)

	gl.TexCoord2f(0, 1)
	gl.Vertex2f(0, float32(height))

	gl.TexCoord2f(1, 1)
	gl.Vertex2f(float32(width), float32(height))

	gl.TexCoord2f(1, 0)
	gl.Vertex2f(float32(width), 0)
	gl.End()
	gl.Disable(gl.TEXTURE_2D)
}

func drawObstacles(obstacleRects []*image.Rectangle, color colorful.Color) {
	gl.Begin(gl.QUADS)
	gl.Color3d(color.R, color.G, color.B)
	for _, rect := range obstacleRects {
		gl.Vertex2d(float64(rect.Min.X), float64(rect.Min.Y))
		gl.Vertex2d(float64(rect.Max.X), float64(rect.Min.Y))
		gl.Vertex2d(float64(rect.Max.X), float64(rect.Max.Y))
		gl.Vertex2d(float64(rect.Min.X), float64(rect.Max.Y))
	}
	gl.End()
}

func drawTargets(targets []*rrtstar.Target, color colorful.Color) {
	for _, target := range targets {
		drawPoint(target.Point, 40, color)
		drawString(fmt.Sprintf("%d", target.Importance), target.Point, Center, Center, colorful.Hsv(310, 1, 0))
	}
}

func drawString(value string, point image.Point, hAlign, vAlign Alignment, color colorful.Color) {
	sw, sh := font.Metrics(value)

	var topLeft image.Point
	switch hAlign {
	case Left:
		topLeft.X = point.X
	case Center:
		topLeft.X = point.X - sw/2
	case Right:
		topLeft.X = point.X - sw
	default:
		topLeft.X = point.X
	}

	switch vAlign {
	case Top:
		topLeft.Y = point.Y
	case Center:
		topLeft.Y = point.Y - sh*2/5
	case Bottom:
		topLeft.Y = point.Y - sh
	default:
		topLeft.Y = point.Y
	}

	gl.Color4d(0, 0, 0, 0)
	gl.Rectd(float64(topLeft.X), float64(topLeft.Y), float64(sw), float64(sh))

	// Render the string.
	gl.Color3d(color.R, color.G, color.B)

	font.Printf(float32(topLeft.X), float32(topLeft.Y), value)
}

func display(iteration uint64, showTree, showViewshed, showPath, showIterationCount bool) {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.ClearColor(0, 0, 0, 0)

	//drawBackground(colorful.Hsv(210, 1, 0.6))
	drawObstacles(obstacleRects, colorful.Hsv(210, 1, 0.6))

	if showViewshed {
		drawViewshed(rrtStar.Viewshed.ViewablePolygon, &rrtStar.Viewshed.Center, colorful.Hsv(330, 1, 1), 3)
	}

	if showTree {
		drawTreeFaster(rrtStar.Root, 250)
	}

	drawTargets(targets, colorful.Hsv(110, 1, 1))

	if showPath {
		drawPath(rrtStar.BestPath, colorful.Hsv(100, 1, 1), 3)

		drawPoint(*rrtStar.StartPoint, 20, colorful.Hsv(20, 1, 1))
		drawPoint(*rrtStar.EndPoint, 20, colorful.Hsv(60, 1, 1))
	}

	if showIterationCount {
		drawString(fmt.Sprintf("%d", iteration), image.Pt(10, 10), Left, Top, colorful.Hsv(180, 1, 1))
	}

	if showViewshed {
		drawPoint(image.Pt(int(cursorX), int(cursorY)), 10, colorful.Hsv(330, 1, 1))
	}

	gl.Flush() /* Single buffered, so needs a flush. */
}

func init() {
	runtime.LockOSThread()
}

func main() {
	isFullscreen := flag.Bool("full", false, "the map will expand to fullscreen on the primary monitor if set")
	isLooping := flag.Bool("loop", false, "will loop with random obstacles if set")
	numObstacles := flag.Int("obstacles", 15, "sets the number of obstacles generated")
	monitorNum := flag.Int("monitor", 0, "sets which monitor to display on in fullscreen. default to primary")
	iterations := flag.Int("i", 25000, "sets the number of iterations. default to 25000")
	iterationsPerFrame := flag.Int("if", 50, "sets the number of iterations to evaluate between frames")
	record := flag.Bool("r", false, "records the session")
	renderCostmap := flag.Bool("cm", false, "renders a costmap before executing")
	showPath := flag.Bool("path", true, "shows the path and endpoints")
	showIterationCount := flag.Bool("count", true, "shows the iteration count")
	showTree := flag.Bool("tree", false, "draws the tree")
	showViewshed := flag.Bool("viewshed", false, "draws the viewshed at the mouse cursor location")
	numTargets := flag.Int("targets", 2, "the number of targets to simulate")
	flag.Parse()

	glfwErr := glfw.Init()
	if glfwErr != nil {
		panic(glfwErr)
	}
	defer glfw.Terminate()

	width = 700
	height = 700
	var monitor *glfw.Monitor
	if *isFullscreen {
		monitor = glfw.GetMonitors()[*monitorNum]
		vidMode := monitor.GetVideoMode()
		width = vidMode.Width
		height = vidMode.Height
	}

	log.Printf("w: %d, h: %d", width, height)

	glfw.WindowHint(glfw.AutoIconify, glfw.False)

	window, err := glfw.CreateWindow(width, height, "rrt*", monitor, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()
	window.SetSizeCallback(reshape)
	window.SetKeyCallback(onKey)
	window.SetCharCallback(onChar)
	window.SetCursorPosCallback(onCursor)
	window.SetInputMode(glfw.CursorMode, glfw.CursorHidden)
	glfw.SwapInterval(1)

	glErr := gl.Init()
	if glErr != nil {
		panic(glErr)
	}

	font, err = loadFont("luxisr.ttf", int32(44))
	if err != nil {
		log.Printf("LoadFont: %v", err)
		return
	}
	defer font.Release()

	for !window.ShouldClose() {

		rand.Seed(time.Now().UnixNano()) // apparently golang random is deterministic by default
		//obstacles := readImageGray("dragon.png")
		var obstacleImage *image.Gray
		obstacleRects, obstacleImage = rrtstar.GenerateObstacles(width, height, *numObstacles)
		rrtStar = rrtstar.NewRrtStar(obstacleImage, obstacleRects, 20, width, height, nil, nil)

		if *renderCostmap {
			rrtStar.RenderUnseenCostMap("unseen.png")
		}

		for i := 0; i < *numTargets; i++ {
			target := rrtstar.NewTarget(rrtstar.RandomRrt, uint32(rand.Int31n(5))+1, obstacleImage)
			targets = append(targets, target)
		}

		//obstaclesTexture = getTextureGray(obstacleImage)
		//defer gl.DeleteTextures(1, &obstaclesTexture)
		reshape(window, width, height)
		for i := 0; !window.ShouldClose(); i++ {

			if i < *iterations {
				rrtStar.SampleRrtStar()
				if i%*iterationsPerFrame == 0 {
					rrtStar.MoveStartPoint(moveX, moveY)

					for _, target := range targets {
						target.MoveTarget()
					}

					invalidate()
				}
			} else if *isLooping {
				break
			}

			if redraw {
				//log.Println("redrawing", i)
				if *showViewshed && (cursorX != rrtStar.Viewshed.Center.X || cursorY != -rrtStar.Viewshed.Center.Y) {
					rrtStar.Viewshed.UpdateCenterLocation(cursorX, cursorY)
					rrtStar.Viewshed.Sweep()
					//fmt.Printf("\rarea: %.0f", viewshed.Area2DPolygon(rrtStar.Viewshed.ViewablePolygon))
				}

				display(rrtStar.NumNodes, *showTree, *showViewshed, *showPath, *showIterationCount)
				window.SwapBuffers()
				redraw = false

				if *record {
					saveFrame(width, height, true)
				}
			}
			glfw.PollEvents()
			//		time.Sleep(2 * time.Second)
		}
	}
}

func saveFrame(width int, height int, toFile bool) {

	screenshot := image.NewNRGBA(image.Rect(0, 0, width, height))

	gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(screenshot.Pix))
	//gl.ReadPixels(0, 0, int32(width), int32(height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(&screenshot.Pix[0]))
	if gl.NO_ERROR != gl.GetError() {
		log.Println("panic pixels")
		panic("unable to read pixels")
	}

	flipped := imaging.FlipV(screenshot)

	if toFile {
		//filename := time.Now().Format("video/2006Jan02_15-04-05.999.png")
		filename := fmt.Sprintf("video/%06d.png", frameCount)
		frameCount++

		os.Mkdir("video", os.ModeDir)
		outFile, _ := os.Create(filename)
		defer outFile.Close()

		png.Encode(outFile, flipped)
	} else {
		frames = append(frames, flipped)
	}
}

func onChar(w *glfw.Window, char rune) {
	//log.Println(char)
	if char == 'p' {
		rrtStar.Prune(30)
	}
}

func onKey(w *glfw.Window, key glfw.Key, scancode int, action glfw.Action, mods glfw.ModifierKey) {
	//log.Println(action)
	switch {
	case key == glfw.KeyEscape, key == glfw.KeyQ:
		w.SetShouldClose(true)
	case key == glfw.KeyUp:
		//log.Println("up")
		if action != glfw.Release {
			moveY = -5
		} else {
			moveY = 0
		}
	case key == glfw.KeyDown:
		//log.Println("down")
		if action != glfw.Release {
			moveY = 5
		} else {
			moveY = 0
		}
	case key == glfw.KeyLeft:
		//log.Println("left")
		if action != glfw.Release {
			moveX = -5
		} else {
			moveX = 0
		}
	case key == glfw.KeyRight:
		//log.Println("right")
		if action != glfw.Release {
			moveX = 5
		} else {
			moveX = 0
		}
	case key == glfw.KeyP:
		//rrtStar.Prune(100)
	}
}

func onCursor(w *glfw.Window, xpos float64, ypos float64) {
	cursorX = xpos
	cursorY = ypos
}

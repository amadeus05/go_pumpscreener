package chart

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"time"

	"pumpscreener/src/domain"
)

const (
	width  = 1024
	height = 576
)

var (
	bg        = color.RGBA{R: 24, G: 31, B: 40, A: 255}
	grid      = color.RGBA{R: 51, G: 62, B: 74, A: 255}
	textColor = color.RGBA{R: 155, G: 166, B: 180, A: 255}
	upColor   = color.RGBA{R: 40, G: 190, B: 140, A: 255}
	downColor = color.RGBA{R: 238, G: 80, B: 105, A: 255}
	priceLine = color.RGBA{R: 230, G: 236, B: 244, A: 255}
)

type Candle struct {
	Start time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

type Signal struct {
	DomainSignal domain.Signal
	Candles      []Candle
}

func RenderSignal(signal Signal) ([]byte, error) {
	candles := candlesUntil(signal.Candles, signal.DomainSignal.Triggered)
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles for chart")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	plot := image.Rect(56, 32, width-72, height-48)
	drawGrid(img, plot)

	minPrice, maxPrice := priceRange(candles, signal.DomainSignal.Price)
	if minPrice <= 0 || maxPrice <= minPrice {
		return nil, fmt.Errorf("invalid chart price range")
	}

	drawCandles(img, plot, candles, minPrice, maxPrice)
	drawPriceAxis(img, plot, minPrice, maxPrice)
	drawTimeAxis(img, plot, candles)
	drawCurrentPriceLabel(img, plot, signal.DomainSignal, minPrice, maxPrice)

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func drawGrid(img *image.RGBA, plot image.Rectangle) {
	for i := 0; i <= 8; i++ {
		x := plot.Min.X + (plot.Dx()*i)/8
		line(img, x, plot.Min.Y, x, plot.Max.Y, grid)
	}
	for i := 0; i <= 6; i++ {
		y := plot.Min.Y + (plot.Dy()*i)/6
		line(img, plot.Min.X, y, plot.Max.X, y, grid)
	}
}

func drawCandles(img *image.RGBA, plot image.Rectangle, candles []Candle, minPrice, maxPrice float64) {
	step := float64(plot.Dx()) / float64(max(len(candles), 1))
	bodyWidth := int(math.Max(3, step*0.58))

	for i, candle := range candles {
		x := plot.Min.X + int((float64(i)+0.5)*step)
		openY := yFor(plot, candle.Open, minPrice, maxPrice)
		closeY := yFor(plot, candle.Close, minPrice, maxPrice)
		highY := yFor(plot, candle.High, minPrice, maxPrice)
		lowY := yFor(plot, candle.Low, minPrice, maxPrice)

		candleColor := upColor
		if candle.Close < candle.Open {
			candleColor = downColor
		}

		line(img, x, highY, x, lowY, candleColor)
		top := min(openY, closeY)
		bottom := max(openY, closeY)
		if top == bottom {
			bottom++
		}
		fillRect(img, image.Rect(x-bodyWidth/2, top, x+bodyWidth/2+1, bottom+1), candleColor)
	}
}

func drawPriceAxis(img *image.RGBA, plot image.Rectangle, minPrice, maxPrice float64) {
	for i := 0; i <= 6; i++ {
		price := maxPrice - ((maxPrice-minPrice)*float64(i))/6
		y := plot.Min.Y + (plot.Dy()*i)/6
		label := fmt.Sprintf("%.4f", price)
		drawSmallText(img, plot.Max.X+8, y-4, label, textColor)
	}
}

func drawTimeAxis(img *image.RGBA, plot image.Rectangle, candles []Candle) {
	if len(candles) == 0 {
		return
	}

	step := float64(plot.Dx()) / float64(max(len(candles), 1))
	for index := range candles {
		x := plot.Min.X + int((float64(index)+0.5)*step)
		y := plot.Max.Y + 10
		drawRotatedText(img, x, y, candles[index].Start.Format("15:04"), textColor)
	}
}

func drawCurrentPriceLabel(img *image.RGBA, plot image.Rectangle, signal domain.Signal, minPrice, maxPrice float64) {
	y := yFor(plot, signal.Price, minPrice, maxPrice)
	labelColor := upColor
	if signal.Rule.Direction == domain.DirectionDown {
		labelColor = downColor
	}

	line(img, plot.Min.X, y, plot.Max.X, y, priceLine)
	rect := image.Rect(plot.Max.X+4, y-10, width-8, y+10)
	fillRect(img, rect, labelColor)
	drawSmallText(img, rect.Min.X+6, y-3, fmt.Sprintf("%.4f", signal.Price), bg)
}

func candlesUntil(candles []Candle, at time.Time) []Candle {
	result := make([]Candle, 0, len(candles))
	for _, candle := range candles {
		if candle.Start.After(at) {
			continue
		}
		result = append(result, candle)
	}
	return result
}

func priceRange(candles []Candle, signalPrice float64) (float64, float64) {
	minPrice := math.MaxFloat64
	maxPrice := 0.0
	for _, candle := range candles {
		if candle.Low < minPrice {
			minPrice = candle.Low
		}
		if candle.High > maxPrice {
			maxPrice = candle.High
		}
	}
	if signalPrice > 0 {
		minPrice = math.Min(minPrice, signalPrice)
		maxPrice = math.Max(maxPrice, signalPrice)
	}

	padding := (maxPrice - minPrice) * 0.08
	if padding == 0 {
		padding = maxPrice * 0.01
	}
	return minPrice - padding, maxPrice + padding
}

func closestCandleIndex(candles []Candle, at time.Time) int {
	best := 0
	bestDistance := time.Duration(math.MaxInt64)
	for i, candle := range candles {
		distance := at.Sub(candle.Start)
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			best = i
			bestDistance = distance
		}
	}
	return best
}

func yFor(plot image.Rectangle, price, minPrice, maxPrice float64) int {
	ratio := (price - minPrice) / (maxPrice - minPrice)
	return plot.Max.Y - int(ratio*float64(plot.Dy()))
}

func line(img *image.RGBA, x1, y1, x2, y2 int, c color.RGBA) {
	if x1 == x2 {
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		for y := y1; y <= y2; y++ {
			set(img, x1, y, c)
		}
		return
	}
	if y1 == y2 {
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		for x := x1; x <= x2; x++ {
			set(img, x, y1, c)
		}
	}
}

func circle(img *image.RGBA, cx, cy, radius int, c color.RGBA) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				set(img, cx+x, cy+y, c)
			}
		}
	}
}

func fillRect(img *image.RGBA, rect image.Rectangle, c color.RGBA) {
	draw.Draw(img, rect, &image.Uniform{C: c}, image.Point{}, draw.Src)
}

func set(img *image.RGBA, x, y int, c color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, c)
	}
}

func drawSmallText(img *image.RGBA, x, y int, value string, c color.RGBA) {
	for _, ch := range value {
		drawDigitLike(img, x, y, ch, c)
		x += 6
	}
}

func drawRotatedText(img *image.RGBA, x, y int, value string, c color.RGBA) {
	labelWidth := len(value)*6 + 2
	labelHeight := 7
	label := image.NewRGBA(image.Rect(0, 0, labelWidth, labelHeight))
	drawSmallText(label, 0, 1, value, c)

	angle := -math.Pi / 4
	cosA := math.Cos(angle)
	sinA := math.Sin(angle)
	cx := float64(labelWidth) / 2

	for py := 0; py < labelHeight; py++ {
		for px := 0; px < labelWidth; px++ {
			source := label.RGBAAt(px, py)
			if source.A == 0 {
				continue
			}

			dx := float64(px) - cx
			dy := float64(py)
			rx := int(dx*cosA - dy*sinA)
			ry := int(dx*sinA + dy*cosA)
			set(img, x+rx, y+ry+16, source)
		}
	}
}

func drawSlantedText(img *image.RGBA, x, y int, value string, c color.RGBA) {
	for _, ch := range value {
		drawDigitLike(img, x, y, ch, c)
		x += 4
		y += 4
	}
}

func drawDigitLike(img *image.RGBA, x, y int, ch rune, c color.RGBA) {
	pattern, ok := tinyFont[ch]
	if !ok {
		return
	}
	for row, bits := range pattern {
		for col := 0; col < 3; col++ {
			if bits&(1<<(2-col)) != 0 {
				set(img, x+col, y+row, c)
			}
		}
	}
}

var tinyFont = map[rune][]int{
	'0': {7, 5, 5, 5, 7},
	'1': {2, 6, 2, 2, 7},
	'2': {7, 1, 7, 4, 7},
	'3': {7, 1, 7, 1, 7},
	'4': {5, 5, 7, 1, 1},
	'5': {7, 4, 7, 1, 7},
	'6': {7, 4, 7, 5, 7},
	'7': {7, 1, 1, 1, 1},
	'8': {7, 5, 7, 5, 7},
	'9': {7, 5, 7, 1, 7},
	'.': {0, 0, 0, 0, 2},
	':': {0, 2, 0, 2, 0},
}

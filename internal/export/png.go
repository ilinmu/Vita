package export

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"time"

	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/model"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	pngWidth  = 1200
	pngHeight = 720
)

type canvas struct {
	img *image.RGBA
}

func newCanvas(bg color.RGBA) *canvas {
	img := image.NewRGBA(image.Rect(0, 0, pngWidth, pngHeight))
	for y := 0; y < pngHeight; y++ {
		for x := 0; x < pngWidth; x++ {
			img.SetRGBA(x, y, bg)
		}
	}
	return &canvas{img: img}
}

func (c *canvas) blendPixel(x, y int, col color.RGBA, alpha float64) {
	if x < 0 || y < 0 || x >= pngWidth || y >= pngHeight || alpha <= 0 {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	a := alpha * float64(col.A) / 255.0
	dst := c.img.RGBAAt(x, y)
	r := uint8(float64(dst.R)*(1-a) + float64(col.R)*a)
	g := uint8(float64(dst.G)*(1-a) + float64(col.G)*a)
	b := uint8(float64(dst.B)*(1-a) + float64(col.B)*a)
	c.img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
}

func (c *canvas) drawLine(x0, y0, x1, y1, width float64, col color.RGBA) {
	minX := int(math.Floor(math.Min(x0, x1) - width - 2))
	maxX := int(math.Ceil(math.Max(x0, x1) + width + 2))
	minY := int(math.Floor(math.Min(y0, y1) - width - 2))
	maxY := int(math.Ceil(math.Max(y0, y1) + width + 2))
	dx, dy := x1-x0, y1-y0
	len2 := dx*dx + dy*dy
	rad := width / 2.0
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			var t float64
			if len2 > 0 {
				t = ((float64(x)-x0)*dx + (float64(y)-y0)*dy) / len2
				if t < 0 {
					t = 0
				} else if t > 1 {
					t = 1
				}
			}
			px, py := x0+t*dx, y0+t*dy
			d := math.Hypot(float64(x)-px, float64(y)-py) - rad
			if d < 1.0 {
				c.blendPixel(x, y, col, 0.5-d/1.5)
			}
		}
	}
}

func (c *canvas) drawDashedLine(x0, y0, x1, y1, width, dash, gap float64, col color.RGBA) {
	total := math.Hypot(x1-x0, y1-y0)
	if total <= 0 {
		return
	}
	ux, uy := (x1-x0)/total, (y1-y0)/total
	for pos := 0.0; pos < total; pos += dash + gap {
		end := math.Min(pos+dash, total)
		c.drawLine(x0+ux*pos, y0+uy*pos, x0+ux*end, y0+uy*end, width, col)
	}
}

func (c *canvas) drawCircle(cx, cy, r float64, col color.RGBA) {
	minX, maxX := int(cx-r-2), int(cx+r+2)
	minY, maxY := int(cy-r-2), int(cy+r+2)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			d := math.Hypot(float64(x)-cx, float64(y)-cy) - r
			if d < 1.0 {
				c.blendPixel(x, y, col, 0.5-d/1.5)
			}
		}
	}
}

func (c *canvas) fillPolygonBand(xs, yLo, yHi []float64, col color.RGBA) {
	if len(xs) < 2 {
		return
	}
	for i := 0; i < len(xs)-1; i++ {
		x0, x1 := xs[i], xs[i+1]
		if x1 <= x0 {
			continue
		}
		ix0 := int(math.Floor(x0))
		ix1 := int(math.Ceil(x1))
		for x := ix0; x <= ix1; x++ {
			t := (float64(x) - x0) / (x1 - x0)
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			top := yHi[i] + (yHi[i+1]-yHi[i])*t
			bot := yLo[i] + (yLo[i+1]-yLo[i])*t
			if top > bot {
				top, bot = bot, top
			}
			for y := int(math.Ceil(top)); y <= int(math.Floor(bot)); y++ {
				c.blendPixel(x, y, col, 1.0)
			}
		}
	}
}

func (c *canvas) drawText(x, y int, text string, col color.RGBA) {
	d := &font.Drawer{
		Dst:  c.img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func (c *canvas) encode() ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, c.img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GrowthPoint is one user measurement plotted on a growth PNG.
type GrowthPoint struct {
	AgeMonths float64
	Value     float64
}

// RenderGrowthChartPNG renders a 1200x720 growth curve PNG with optional reference percentile bands.
func RenderGrowthChartPNG(title, unitLabel string, std growth.Standard, gender model.Gender, metric growth.Metric, isChild bool, pts []GrowthPoint) ([]byte, error) {
	c := newCanvas(color.RGBA{R: 250, G: 251, B: 252, A: 255})
	textCol := color.RGBA{R: 15, G: 23, B: 42, A: 255}
	mutedCol := color.RGBA{R: 100, G: 116, B: 139, A: 255}
	gridCol := color.RGBA{R: 226, G: 232, B: 240, A: 255}
	accentCol := color.RGBA{R: 13, G: 148, B: 136, A: 255}

	c.drawText(72, 42, title, textCol)
	c.drawText(72, 62, fmt.Sprintf("Unit: %s | Generated: %s", unitLabel, time.Now().Format("2006-01-02 15:04")), mutedCol)

	plotL, plotT, plotR, plotB := 90.0, 90.0, 1140.0, 650.0
	plotW, plotH := plotR-plotL, plotB-plotT

	minX, maxX := 0.0, 36.0
	if len(pts) > 0 {
		maxPt := pts[len(pts)-1].AgeMonths
		for _, p := range pts {
			if p.AgeMonths > maxPt {
				maxPt = p.AgeMonths
			}
			if p.AgeMonths < minX {
				minX = p.AgeMonths
			}
		}
		if maxPt > maxX {
			maxX = math.Ceil(maxPt/12.0) * 12.0
		}
	}
	maxStd := growth.MaxAgeMonths(std, gender, metric)
	if isChild && maxStd > 0 && maxX > maxStd {
		maxX = maxStd
	}

	var curves growth.PercentileCurves
	if isChild && maxStd > 0 {
		curves = growth.CurveSeries(std, gender, metric, minX, maxX, 48)
	}

	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, v := range curves.P3 {
		minY = math.Min(minY, v)
	}
	for _, v := range curves.P97 {
		maxY = math.Max(maxY, v)
	}
	for _, p := range pts {
		minY = math.Min(minY, p.Value)
		maxY = math.Max(maxY, p.Value)
	}
	if math.IsInf(minY, 1) || minY >= maxY {
		minY, maxY = 0, 100
	}
	padY := (maxY - minY) * 0.1
	minY -= padY
	maxY += padY

	toX := func(m float64) float64 { return plotL + ((m-minX)/(maxX-minX))*plotW }
	toY := func(v float64) float64 { return plotB - ((v-minY)/(maxY-minY))*plotH }

	// Gridlines
	for i := 0; i <= 5; i++ {
		frac := float64(i) / 5.0
		y := plotB - frac*plotH
		val := minY + frac*(maxY-minY)
		c.drawLine(plotL, y, plotR, y, 1, gridCol)
		c.drawText(20, int(y)+4, fmt.Sprintf("%.1f", val), mutedCol)
	}
	for i := 0; i <= 6; i++ {
		frac := float64(i) / 6.0
		x := plotL + frac*plotW
		age := minX + frac*(maxX-minX)
		c.drawLine(x, plotT, x, plotB, 1, gridCol)
		c.drawText(int(x)-16, int(plotB)+24, fmt.Sprintf("%.0fm", age), mutedCol)
	}

	// Percentile bands
	if len(curves.Ages) > 1 {
		cxs := make([]float64, len(curves.Ages))
		y3 := make([]float64, len(curves.Ages))
		y15 := make([]float64, len(curves.Ages))
		y50 := make([]float64, len(curves.Ages))
		y85 := make([]float64, len(curves.Ages))
		y97 := make([]float64, len(curves.Ages))
		for i, a := range curves.Ages {
			cxs[i] = toX(a)
			y3[i] = toY(curves.P3[i])
			y15[i] = toY(curves.P15[i])
			y50[i] = toY(curves.P50[i])
			y85[i] = toY(curves.P85[i])
			y97[i] = toY(curves.P97[i])
		}
		c.fillPolygonBand(cxs, y3, y97, color.RGBA{R: 13, G: 148, B: 136, A: 26})
		c.fillPolygonBand(cxs, y15, y85, color.RGBA{R: 13, G: 148, B: 136, A: 36})
		for i := 0; i < len(cxs)-1; i++ {
			c.drawDashedLine(cxs[i], y50[i], cxs[i+1], y50[i+1], 1.5, 6, 4, color.RGBA{R: 13, G: 148, B: 136, A: 160})
		}
		last := len(cxs) - 1
		c.drawText(int(cxs[last])+6, int(y97[last])+4, "P97", mutedCol)
		c.drawText(int(cxs[last])+6, int(y50[last])+4, "P50", accentCol)
		c.drawText(int(cxs[last])+6, int(y3[last])+4, "P3", mutedCol)
	}

	// User points
	for i := 0; i < len(pts)-1; i++ {
		c.drawLine(toX(pts[i].AgeMonths), toY(pts[i].Value), toX(pts[i+1].AgeMonths), toY(pts[i+1].Value), 3.0, accentCol)
	}
	for _, p := range pts {
		c.drawCircle(toX(p.AgeMonths), toY(p.Value), 6, accentCol)
		c.drawCircle(toX(p.AgeMonths), toY(p.Value), 3, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	}

	c.drawLine(plotL, plotT, plotL, plotB, 1.5, mutedCol)
	c.drawLine(plotL, plotB, plotR, plotB, 1.5, mutedCol)
	return c.encode()
}

// RenderFeverChartPNG renders a 1200x720 fever temperature & medication overlay PNG.
func RenderFeverChartPNG(title string, temps []model.TemperatureRecord, meds []model.MedicationRecord) ([]byte, error) {
	c := newCanvas(color.RGBA{R: 250, G: 251, B: 252, A: 255})
	textCol := color.RGBA{R: 15, G: 23, B: 42, A: 255}
	mutedCol := color.RGBA{R: 100, G: 116, B: 139, A: 255}
	gridCol := color.RGBA{R: 226, G: 232, B: 240, A: 255}
	tempCol := color.RGBA{R: 239, G: 68, B: 68, A: 255}
	warnCol := color.RGBA{R: 245, G: 158, B: 11, A: 220}
	medCol := color.RGBA{R: 99, G: 102, B: 241, A: 255}

	c.drawText(72, 42, title, textCol)
	c.drawText(72, 62, fmt.Sprintf("Fever & Medication Timeline | Generated: %s", time.Now().Format("2006-01-02 15:04")), mutedCol)

	plotL, plotT, plotR, plotB := 90.0, 90.0, 1140.0, 650.0
	plotW, plotH := plotR-plotL, plotB-plotT

	minT, maxT := math.Inf(1), math.Inf(-1)
	minY, maxY := 35.5, 40.5
	for _, t := range temps {
		minT = math.Min(minT, float64(t.RecordedAt))
		maxT = math.Max(maxT, float64(t.RecordedAt))
		if t.TempC+0.5 > maxY {
			maxY = math.Ceil(t.TempC + 0.5)
		}
		if t.TempC-0.5 < minY {
			minY = math.Floor(t.TempC - 0.5)
		}
	}
	for _, m := range meds {
		minT = math.Min(minT, float64(m.RecordedAt))
		maxT = math.Max(maxT, float64(m.RecordedAt))
	}
	if math.IsInf(minT, 1) {
		now := float64(time.Now().UnixMilli())
		minT, maxT = now-float64((24*time.Hour).Milliseconds()), now
	}
	if maxT <= minT {
		pad := float64((2 * time.Hour).Milliseconds())
		minT -= pad
		maxT += pad
	}

	toX := func(ms float64) float64 { return plotL + ((ms-minT)/(maxT-minT))*plotW }
	toY := func(v float64) float64 { return plotB - ((v-minY)/(maxY-minY))*plotH }

	// Y grid (0.5 C steps)
	for v := math.Ceil(minY*2) / 2; v <= maxY; v += 0.5 {
		y := toY(v)
		c.drawLine(plotL, y, plotR, y, 1, gridCol)
		c.drawText(26, int(y)+4, fmt.Sprintf("%.1fC", v), mutedCol)
	}
	// X grid
	for i := 0; i <= 5; i++ {
		frac := float64(i) / 5.0
		x := plotL + frac*plotW
		ts := time.UnixMilli(int64(minT + frac*(maxT-minT)))
		c.drawLine(x, plotT, x, plotB, 1, gridCol)
		c.drawText(int(x)-32, int(plotB)+24, ts.Format("01-02 15:04"), mutedCol)
	}

	// Threshold lines: 37.3 C and 38.5 C
	if 37.3 >= minY && 37.3 <= maxY {
		y := toY(37.3)
		c.drawDashedLine(plotL, y, plotR, y, 1.5, 8, 5, warnCol)
		c.drawText(int(plotR)+6, int(y)+4, "37.3C", warnCol)
	}
	if 38.5 >= minY && 38.5 <= maxY {
		y := toY(38.5)
		c.drawDashedLine(plotL, y, plotR, y, 2, 8, 5, tempCol)
		c.drawText(int(plotR)+6, int(y)+4, "38.5C", tempCol)
	}

	// Medication vertical markers
	for _, m := range meds {
		x := toX(float64(m.RecordedAt))
		c.drawDashedLine(x, plotT, x, plotB, 1.8, 5, 5, medCol)
		c.drawCircle(x, plotT+14, 6, medCol)
		c.drawText(int(x)+8, int(plotT)+18, "MED", medCol)
	}

	// Temperature line & dots
	for i := 0; i < len(temps)-1; i++ {
		c.drawLine(toX(float64(temps[i].RecordedAt)), toY(temps[i].TempC),
			toX(float64(temps[i+1].RecordedAt)), toY(temps[i+1].TempC), 3.0, tempCol)
	}
	for _, t := range temps {
		dotCol := color.RGBA{R: 22, G: 163, B: 74, A: 255}
		if t.TempC >= 38.5 {
			dotCol = tempCol
		} else if t.TempC >= 37.3 {
			dotCol = warnCol
		}
		x, y := toX(float64(t.RecordedAt)), toY(t.TempC)
		c.drawCircle(x, y, 6.5, dotCol)
		c.drawText(int(x)-14, int(y)-10, fmt.Sprintf("%.1f", t.TempC), textCol)
	}

	c.drawLine(plotL, plotT, plotL, plotB, 1.5, mutedCol)
	c.drawLine(plotL, plotB, plotR, plotB, 1.5, mutedCol)
	return c.encode()
}

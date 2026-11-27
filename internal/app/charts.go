package app

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"
	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/model"
)

type growthPlotPoint struct {
	AgeMonths float64
	Value     float64
	DateLabel string
	PctLabel  string
}

func renderGrowthChart(
	c *ui.Context,
	pal Palette,
	pts []growthPlotPoint,
	std growth.Standard,
	gender model.Gender,
	metric growth.Metric,
	isChild bool,
	isCustom bool,
	unit string,
	selected *int,
) {
	t := c.Theme()
	minX, maxX := 0.0, 36.0
	if len(pts) > 0 {
		minPt, maxPt := pts[0].AgeMonths, pts[0].AgeMonths
		for _, p := range pts {
			minPt = math.Min(minPt, p.AgeMonths)
			maxPt = math.Max(maxPt, p.AgeMonths)
		}
		if !isChild || isCustom {
			minX = math.Max(0, math.Floor(minPt))
			maxX = math.Max(minX+6, math.Ceil(maxPt))
		} else {
			if maxPt > maxX {
				maxX = math.Ceil(maxPt/12.0) * 12.0
			}
		}
	}
	var curves growth.PercentileCurves
	if isChild && !isCustom {
		maxStd := growth.MaxAgeMonths(std, gender, metric)
		if maxStd > 0 && maxX > maxStd {
			maxX = maxStd
		}
		if maxStd > 0 && maxX > minX {
			curves = growth.CurveSeries(std, gender, metric, minX, maxX, 44)
		}
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
		if len(pts) > 0 {
			minY = pts[0].Value - 5
			maxY = pts[0].Value + 5
		} else {
			minY, maxY = 0, 100
		}
	}

	xScale := chart.NewLinear(minX, maxX)
	if !isChild || isCustom {
		xScale.Format = func(v float64) string { return fmt.Sprintf("%.0f月", v) }
	} else {
		xScale.Format = func(v float64) string {
			if v >= 24 && int(math.Round(v))%12 == 0 {
				return fmt.Sprintf("%.0f岁", v/12.0)
			}
			return fmt.Sprintf("%.0f月", v)
		}
	}
	yScale := chart.NewLinear(minY, maxY).Nice(5)
	yTicks := yScale.Ticks(5)
	xTicks := xScale.Ticks(5)

	// Normalize curve points
	cxs := make([]float32, len(curves.Ages))
	y3 := make([]float32, len(curves.Ages))
	y15 := make([]float32, len(curves.Ages))
	y50 := make([]float32, len(curves.Ages))
	y85 := make([]float32, len(curves.Ages))
	y97 := make([]float32, len(curves.Ages))
	for i, a := range curves.Ages {
		cxs[i] = xScale.At(a)
		y3[i] = yScale.At(curves.P3[i])
		y15[i] = yScale.At(curves.P15[i])
		y50[i] = yScale.At(curves.P50[i])
		y85[i] = yScale.At(curves.P85[i])
		y97[i] = yScale.At(curves.P97[i])
	}

	pxs := make([]float32, len(pts))
	pys := make([]float32, len(pts))
	for i, p := range pts {
		pxs[i] = xScale.At(p.AgeMonths)
		pys[i] = yScale.At(p.Value)
	}

	chart.Plot(c, chart.Axes{
		Left:   yTicks,
		Bottom: xTicks,
		Label: func(_ chart.Side, tk chart.Tick) {
			ui.Text(c, tk.Label).NoWrap().FontSize(11).TextColor(t.TextMuted)
		},
	}, func(area ui.Element) {
		area.Background(pal.Card).Radius(12).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
			for _, tk := range yTicks {
				y := chart.Y(r, tk.At)
				p.Line(r.X, y, r.X+r.W, y, 1, t.Border.Alpha(0.6))
			}
			if len(cxs) > 1 {
				p.FillPath(chart.Area(r, cxs, y3, y97, chart.CurveMonotone), pal.TealBandOut)
				p.FillPath(chart.Area(r, cxs, y15, y85, chart.CurveMonotone), pal.TealBandIn)
				p50Path := chart.Line(r, cxs, y50, chart.CurveMonotone)
				p.StrokePath(p50Path.Dash(0, 6, 4), 1.5, pal.Teal.Alpha(0.55))
			}
			if len(pxs) > 1 {
				p.StrokePath(chart.Line(r, pxs, pys, chart.CurveMonotone), 2.5, pal.Teal)
			}
		})

		for i := range pxs {
			chart.Point(c.Key(i), pxs[i], pys[i], 9, 9).Radius(4.5).Background(pal.Teal).Border(1.5, pal.Card)
		}

		if len(pxs) > 0 {
			chart.Select(c, area, selected, pxs, func(i int) string {
				return fmt.Sprintf("%s %.2f %s", pts[i].DateLabel, pts[i].Value, unit)
			})
			if idx := *selected; idx >= 0 && idx < len(pts) {
				chart.RuleX(c, pxs[idx], 1).Background(pal.Teal.Alpha(0.45))
				chart.Point(c, pxs[idx], pys[idx], 13, 13).Radius(6.5).Background(pal.Teal).Border(2, ui.RGB(255, 255, 255))
				hAlign := ui.Center
				if pxs[idx] < 0.25 {
					hAlign = ui.Start
				} else if pxs[idx] > 0.75 {
					hAlign = ui.End
				}
				vAlign := ui.End
				if pys[idx] > 0.78 {
					vAlign = ui.Start
				}
				chart.Anchor(c, pxs[idx], pys[idx], hAlign, vAlign, func() {
					lbl := fmt.Sprintf("%s · %.1f%s", pts[idx].DateLabel, pts[idx].Value, unit)
					if pts[idx].PctLabel != "" {
						lbl += " (" + pts[idx].PctLabel + ")"
					}
					ui.Text(c, lbl).NoWrap().FontSize(11).Bold().
						Padding(4, 8).Margin(4, 0).Radius(6).
						Background(t.Text).TextColor(t.Background)
				})
			}
		}
	}).Height(230).Gap(6).FillWidth()
}

func renderFeverChart(
	c *ui.Context,
	pal Palette,
	temps []model.TemperatureRecord,
	meds []model.MedicationRecord,
	from, to time.Time,
	selected *int,
) {
	t := c.Theme()
	minY, maxY := 35.5, 40.5
	for _, r := range temps {
		if r.TempC+0.4 > maxY {
			maxY = math.Ceil((r.TempC+0.4)*2) / 2
		}
		if r.TempC-0.4 < minY {
			minY = math.Floor((r.TempC-0.4)*2) / 2
		}
	}
	if !from.Before(to) {
		to = time.Now()
		from = to.Add(-24 * time.Hour)
	}

	xScale := chart.NewTime(from, to)
	xScale.Format = func(tm time.Time) string {
		if to.Sub(from) <= 36*time.Hour {
			return tm.Format("15:04")
		}
		return tm.Format("01/02")
	}
	yScale := chart.NewLinear(minY, maxY)
	yTicks := yScale.Ticks(5)
	xTicks := xScale.Ticks(4)

	txs := make([]float32, len(temps))
	tys := make([]float32, len(temps))
	for i, r := range temps {
		txs[i] = xScale.At(time.UnixMilli(r.RecordedAt))
		tys[i] = yScale.At(r.TempC)
	}

	mxs := make([]float32, len(meds))
	for i, m := range meds {
		mxs[i] = xScale.At(time.UnixMilli(m.RecordedAt))
	}

	y373 := yScale.At(37.3)
	y385 := yScale.At(38.5)

	chart.Plot(c, chart.Axes{
		Left:   yTicks,
		Bottom: xTicks,
		Label: func(_ chart.Side, tk chart.Tick) {
			ui.Text(c, tk.Label).NoWrap().FontSize(11).TextColor(t.TextMuted)
		},
	}, func(area ui.Element) {
		area.Background(pal.Card).Radius(12).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
			for _, tk := range yTicks {
				y := chart.Y(r, tk.At)
				p.Line(r.X, y, r.X+r.W, y, 1, t.Border.Alpha(0.55))
			}
			// 37.3 C low-fever reference
			if y373 >= 0 && y373 <= 1 {
				yy := chart.Y(r, y373)
				line := (&ui.Path{}).MoveTo(r.X, yy).LineTo(r.X+r.W, yy)
				p.StrokePath(line.Dash(0, 5, 4), 1.2, pal.Amber.Alpha(0.65))
			}
			// 38.5 C high-fever alert line
			if y385 >= 0 && y385 <= 1 {
				yy := chart.Y(r, y385)
				line := (&ui.Path{}).MoveTo(r.X, yy).LineTo(r.X+r.W, yy)
				p.StrokePath(line.Dash(0, 6, 4), 1.6, pal.Red.Alpha(0.85))
			}
			// Medication vertical markers
			for _, mx := range mxs {
				if mx < 0 || mx > 1 {
					continue
				}
				xx := chart.X(r, mx)
				line := (&ui.Path{}).MoveTo(xx, r.Y).LineTo(xx, r.Y+r.H)
				p.StrokePath(line.Dash(0, 4, 4), 1.4, pal.Indigo.Alpha(0.75))
			}
			// Temperature curve
			if len(txs) > 1 {
				p.StrokePath(chart.Line(r, txs, tys, chart.CurveMonotone), 2.5, pal.Red)
			}
		})

		// 38.5 label tag
		if y385 >= 0.08 && y385 <= 0.92 {
			chart.Anchor(c, 0.98, y385, ui.End, ui.End, func() {
				ui.Text(c, "38.5℃").NoWrap().FontSize(10).Bold().TextColor(pal.Red).Padding(0, 4, 2, 0)
			})
		}

		// Medication badge markers at top
		type medPtKey int
		for i, mx := range mxs {
			if mx < 0 || mx > 1 {
				continue
			}
			chart.Point(c.Key(medPtKey(i)), mx, 0.94, 10, 10).Radius(3).Background(pal.Indigo).Border(1.5, pal.Card)
		}

		// Temperature data points
		type tempPtKey int
		for i, r := range temps {
			col := tempColor(r.TempC, pal)
			chart.Point(c.Key(tempPtKey(i)), txs[i], tys[i], 9, 9).Radius(4.5).Background(col).Border(1.5, pal.Card)
		}

		if len(txs) > 0 {
			chart.Select(c, area, selected, txs, func(i int) string {
				ts := time.UnixMilli(temps[i].RecordedAt).Format("01-02 15:04")
				return fmt.Sprintf("%s %.1f℃ %s", ts, temps[i].TempC, model.SiteLabel(temps[i].Site))
			})
			if idx := *selected; idx >= 0 && idx < len(temps) {
				chart.RuleX(c, txs[idx], 1).Background(t.TextMuted.Alpha(0.45))
				col := tempColor(temps[idx].TempC, pal)
				chart.Point(c, txs[idx], tys[idx], 13, 13).Radius(6.5).Background(col).Border(2, ui.RGB(255, 255, 255))
				hAlign := ui.Center
				if txs[idx] < 0.25 {
					hAlign = ui.Start
				} else if txs[idx] > 0.75 {
					hAlign = ui.End
				}
				vAlign := ui.End
				if tys[idx] > 0.78 {
					vAlign = ui.Start
				}
				chart.Anchor(c, txs[idx], tys[idx], hAlign, vAlign, func() {
					ts := time.UnixMilli(temps[idx].RecordedAt).Format("01-02 15:04")
					lbl := fmt.Sprintf("%s · %.1f℃ (%s)", ts, temps[idx].TempC, model.SiteLabel(temps[idx].Site))
					ui.Text(c, lbl).NoWrap().FontSize(11).Bold().
						Padding(4, 8).Margin(4, 0).Radius(6).
						Background(t.Text).TextColor(t.Background)
				})
			}
		}
	}).Height(230).Gap(6).FillWidth()
}

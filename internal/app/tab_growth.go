package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/export"
	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

func (a *App) growthTab(c *ui.Context, pal Palette, member model.Member, hasMember bool) {
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(12, 16, 28, 16).Gap(12).Children(func() {
			if !hasMember {
				a.emptyOnboardingCard(c, pal)
				return
			}

			customMetrics := a.Store.CustomMetrics()
			totalChoices := 4 + len(customMetrics)
			if a.GrowthMetricIdx >= totalChoices {
				a.GrowthMetricIdx = 0
			}

			// Metric chips
			ui.ScrollHorizontal(c).FillWidth().Gap(8).Children(func() {
				builtIn := []string{"身高", "体重", "头围", "BMI"}
				for i, lbl := range builtIn {
					a.metricChip(c, pal, i, lbl)
				}
				for i, cm := range customMetrics {
					a.metricChip(c, pal, 4+i, cm.Name)
				}
				addBtn := ui.Row(c.Key("add-cm")).Padding(7, 12).Radius(16).Shrink(0).
					Background(t.Surface).Border(1, t.Border).Role(ui.RoleButton).Label("新建自定义指标")
				addBtn.Children(func() {
					ui.Text(c, "+ 自定义").NoWrap().FontSize(13).TextColor(pal.Teal)
				})
				if addBtn.Clicked() {
					a.CustomMetricName, a.CustomMetricUnit = "", ""
					a.ShowCustomDefForm = true
				}
			})

			isCustom := a.GrowthMetricIdx >= 4
			isChild := member.IsChildAt(time.Now())
			std := growth.StandardChina
			if a.GrowthStdIdx == 1 {
				std = growth.StandardWHO
			}

			// Standard switcher for pediatric built-in curves
			if !isCustom && isChild {
				if ui.Segmented(c, &a.GrowthStdIdx, "中国标准 WS/T 423", "WHO 国际标准").FillEqually().Label("参考标准").Changed() {
					_ = a.Store.UpdateConfig(func(cfg *localstore.Config) {
						if a.GrowthStdIdx == 1 {
							cfg.LastStandard = string(growth.StandardWHO)
						} else {
							cfg.LastStandard = string(growth.StandardChina)
						}
					})
				}
			}

			// Collect plot points
			var metric growth.Metric
			var unit, chartTitle string
			var pts []growthPlotPoint
			type rowEntry struct {
				id       string
				ts       int64
				valText  string
				pctText  string
				note     string
				isCustom bool
			}
			var rows []rowEntry

			if !isCustom {
				switch a.GrowthMetricIdx {
				case 0:
					metric, unit, chartTitle = growth.MetricHeight, "cm", "Height (cm)"
				case 1:
					metric, unit, chartTitle = growth.MetricWeight, "kg", "Weight (kg)"
				case 2:
					metric, unit, chartTitle = growth.MetricHeadCirc, "cm", "Head Circumference (cm)"
				default:
					metric, unit, chartTitle = growth.MetricBMI, "", "BMI"
				}
				for _, r := range a.Store.GrowthRecords(member.ID) {
					var val *float64
					switch metric {
					case growth.MetricHeight:
						val = r.HeightCM
					case growth.MetricWeight:
						val = r.WeightKG
					case growth.MetricHeadCirc:
						val = r.HeadCircCM
					case growth.MetricBMI:
						val = r.BMI()
					}
					if val == nil {
						continue
					}
					recAt := time.UnixMilli(r.RecordedAt)
					ageM := member.AgeMonthsAt(recAt)
					pctStr := ""
					if member.IsChildAt(recAt) {
						if p, _, ok := growth.Percentile(std, member.Gender, metric, ageM, *val); ok {
							pctStr = fmt.Sprintf("P%.0f", p)
						}
					}
					pts = append(pts, growthPlotPoint{
						AgeMonths: ageM,
						Value:     *val,
						DateLabel: recAt.Format("06/01/02"),
						PctLabel:  pctStr,
					})
					rows = append(rows, rowEntry{
						id:      r.ID,
						ts:      r.RecordedAt,
						valText: fmt.Sprintf("%.2f %s", *val, unit),
						pctText: pctStr,
						note:    r.Note,
					})
				}
			} else {
				cm := customMetrics[a.GrowthMetricIdx-4]
				unit = cm.Unit
				chartTitle = cm.Name
				for _, r := range a.Store.CustomMetricRecords(member.ID, cm.ID) {
					recAt := time.UnixMilli(r.RecordedAt)
					ageM := member.AgeMonthsAt(recAt)
					pts = append(pts, growthPlotPoint{
						AgeMonths: ageM,
						Value:     r.Value,
						DateLabel: recAt.Format("06/01/02"),
					})
					rows = append(rows, rowEntry{
						id:       r.ID,
						ts:       r.RecordedAt,
						valText:  fmt.Sprintf("%.2f %s", r.Value, unit),
						note:     r.Note,
						isCustom: true,
					})
				}
			}

			// Render Growth Chart
			renderGrowthChart(c, pal, pts, std, member.Gender, metric, isChild, isCustom, unit, &a.GrowthSelected)

			if !isCustom && isChild {
				ui.Text(c, "浅色带为 P3–P97 区间，深色带为 P15–P85 区间，虚线为 P50 中位线").
					FontSize(11).TextColor(t.TextMuted)
			} else if !isCustom && !isChild {
				ui.Text(c, "成人模式：展示个人历史变化趋势线").FontSize(11).TextColor(t.TextMuted)
			}

			// Action buttons
			ui.Row(c).Gap(10).Children(func() {
				addLbl := "+ 新增测量"
				if isCustom {
					addLbl = "+ 记录该指标"
				}
				if ui.PrimaryButton(c, addLbl).Grow(1).Basis(0).Clicked() {
					if isCustom {
						a.CustomValMetric = customMetrics[a.GrowthMetricIdx-4].ID
						a.CustomValInput, a.CustomValNote = "", ""
						a.ShowCustomValForm = true
					} else {
						a.openGrowthForm()
					}
				}
				if ui.Button(c, "导出图表 PNG").Clicked() {
					var pngPts []export.GrowthPoint
					for _, p := range pts {
						pngPts = append(pngPts, export.GrowthPoint{AgeMonths: p.AgeMonths, Value: p.Value})
					}
					raw, err := export.RenderGrowthChartPNG(
						fmt.Sprintf("%s - %s", member.Name, chartTitle),
						unit, std, member.Gender, metric, isChild && !isCustom, pngPts,
					)
					if err != nil {
						c.Toast("生成 PNG 失败: " + err.Error())
					} else {
						fname := fmt.Sprintf("vita-growth-%s.png", time.Now().Format("20060102-150405"))
						a.exportFile(c, fname, raw, member.Name+" 生长发育曲线")
					}
				}
			})

			// Records history (newest first)
			ui.Textf(c, "历史记录 (%d)", len(rows)).FontSize(14).Bold().TextColor(t.TextMuted)
			if len(rows) == 0 {
				ui.Text(c, "暂无记录，点击「+ 新增测量」添加。").FontSize(13).TextColor(t.TextMuted)
				return
			}
			ui.Column(c).Radius(12).Background(pal.Card).Border(1, t.Border).Dividers(0.5, t.Border).Clip().Children(func() {
				for i := len(rows) - 1; i >= 0; i-- {
					row := rows[i]
					mobile.SwipeRow(c.Key(row.id), nil, mobile.SwipeOptions{
						Trailing: []mobile.SwipeAction{{
							Label:       "删除",
							Destructive: true,
							OnTap: func() {
								if row.isCustom {
									_ = a.Store.DeleteCustomMetricRecord(row.id)
								} else {
									_ = a.Store.DeleteGrowthRecord(row.id)
								}
								a.TriggerSync(c, true)
							},
						}},
					}, func() {
						ui.Row(c).FillWidth().Padding(12, 14).AlignItems(ui.Center).Gap(10).Children(func() {
							ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
								ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
									ui.Text(c, row.valText).Bold()
									if row.pctText != "" {
										ui.Text(c, row.pctText).FontSize(11).Bold().Padding(2, 6).Radius(8).
											Background(pal.TealSoft).TextColor(pal.Teal)
									}
								})
								sub := time.UnixMilli(row.ts).Format("2006-01-02 15:04")
								if row.note != "" {
									sub += " · " + row.note
								}
								ui.Text(c, sub).FontSize(12).TextColor(t.TextMuted)
							})
							ui.Text(c, "‹ 左滑删").FontSize(11).TextColor(t.TextMuted)
						})
					})
				}
			})
		})
	})
}

func (a *App) metricChip(c *ui.Context, pal Palette, idx int, label string) {
	t := c.Theme()
	sel := a.GrowthMetricIdx == idx
	chip := ui.Row(c.Key(idx)).Padding(7, 13).Radius(16).Shrink(0).Role(ui.RoleButton).Label(label)
	if sel {
		chip.Background(pal.Teal)
	} else {
		chip.Background(t.Surface).Border(1, t.Border)
	}
	chip.Children(func() {
		col := t.Text
		if sel {
			col = t.AccentText
		}
		ui.Text(c, label).NoWrap().FontSize(13).FontWeight(500).TextColor(col)
	})
	if chip.Clicked() {
		a.GrowthMetricIdx = idx
		a.GrowthSelected = -1
	}
}

package app

import (
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/export"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

func (a *App) feverTab(c *ui.Context, pal Palette, member model.Member, hasMember bool) {
	t := c.Theme()
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(12, 16, 28, 16).Gap(12).Children(func() {
			if !hasMember {
				a.emptyOnboardingCard(c, pal)
				return
			}

			now := time.Now()
			lastT, lastM := a.Store.LatestTempAndMed(member.ID)

			// Elapsed status bar
			ui.Row(c).Padding(12, 14).Radius(14).Background(pal.Card).Border(1, t.Border).Gap(12).Children(func() {
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, "距上次测温").FontSize(11).TextColor(t.TextMuted)
					val := "暂无记录"
					col := t.Text
					if lastT != nil {
						val = fmt.Sprintf("%.1f℃ · %s", lastT.TempC, localstore.FormatElapsed(lastT.RecordedAt, now))
						col = tempColor(lastT.TempC, pal)
					}
					ui.Text(c, val).FontSize(14).Bold().TextColor(col)
				})
				ui.Box(c).Width(1).Height(28).Background(t.Border)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, "距上次用药").FontSize(11).TextColor(t.TextMuted)
					val := "暂无记录"
					if lastM != nil {
						val = fmt.Sprintf("%s · %s", lastM.Name, localstore.FormatElapsed(lastM.RecordedAt, now))
					}
					ui.Text(c, val).FontSize(14).Bold().TextColor(pal.Indigo)
				})
			})

			// Episode selector chips
			episodes := a.Store.FeverEpisodes(member.ID)
			ui.ScrollHorizontal(c).FillWidth().Gap(8).Children(func() {
				a.episodeChip(c, pal, "", "全部记录")
				for _, ep := range episodes {
					a.episodeChip(c, pal, ep.ID, ep.Title)
				}
				newEp := ui.Row(c.Key("new-ep")).Padding(6, 12).Radius(16).Shrink(0).
					Background(t.Surface).Border(1, t.Border).Role(ui.RoleButton).Label("新建发烧病程")
				newEp.Children(func() {
					ui.Text(c, "+ 新病程").NoWrap().FontSize(12).TextColor(pal.Teal)
				})
				if newEp.Clicked() {
					a.EpisodeTitle = time.Now().Format("01月02日 发烧病程")
					a.EpisodeNote = ""
					a.ShowEpisodeForm = true
				}
			})

			// Time range filter
			ui.Segmented(c, &a.FeverRangeIdx, "24小时", "3天", "7天").FillEqually().Label("时间范围")

			var span time.Duration
			switch a.FeverRangeIdx {
			case 0:
				span = 24 * time.Hour
			case 1:
				span = 72 * time.Hour
			default:
				span = 7 * 24 * time.Hour
			}

			allTemps := a.Store.TemperatureRecords(member.ID, a.FeverEpisodeID)
			allMeds := a.Store.MedicationRecords(member.ID, a.FeverEpisodeID)

			toTime := now
			fromTime := now.Add(-span)
			if a.FeverEpisodeID != "" && len(allTemps) > 0 {
				lastRec := time.UnixMilli(allTemps[len(allTemps)-1].RecordedAt)
				if now.Sub(lastRec) > span {
					toTime = lastRec.Add(time.Hour)
					fromTime = toTime.Add(-span)
				}
			}

			var visTemps []model.TemperatureRecord
			for _, r := range allTemps {
				rt := time.UnixMilli(r.RecordedAt)
				if !rt.Before(fromTime) && !rt.After(toTime) {
					visTemps = append(visTemps, r)
				}
			}
			var visMeds []model.MedicationRecord
			for _, m := range allMeds {
				mt := time.UnixMilli(m.RecordedAt)
				if !mt.Before(fromTime) && !mt.After(toTime) {
					visMeds = append(visMeds, m)
				}
			}

			renderFeverChart(c, pal, visTemps, visMeds, fromTime, toTime, &a.FeverSelected)

			ui.Text(c, "红色虚线为 38.5℃ 警戒线 · 紫色竖线与圆点为退烧药/降温处理").FontSize(11).TextColor(t.TextMuted)

			// Action buttons
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "+ 记体温").Grow(1).Basis(0).Clicked() {
					a.openTempForm()
				}
				if ui.Button(c, "+ 用药/降温").Grow(1).Basis(0).Clicked() {
					a.openMedForm()
				}
				if ui.Button(c, "导出 PNG").Clicked() {
					raw, err := export.RenderFeverChartPNG(member.Name+" - Fever Trend", visTemps, visMeds)
					if err != nil {
						c.Toast("生成 PNG 失败: " + err.Error())
					} else {
						fname := fmt.Sprintf("vita-fever-%s.png", time.Now().Format("20060102-150405"))
						a.exportFile(c, fname, raw, member.Name+" 发烧体温曲线")
					}
				}
			})

			// Interleaved timeline
			type feverEvent struct {
				id    string
				ts    int64
				isMed bool
				title string
				sub   string
				col   ui.Color
			}
			var events []feverEvent
			for _, r := range allTemps {
				sub := model.SiteLabel(r.Site)
				if r.Symptom != "" {
					sub += " · " + r.Symptom
				}
				if r.Note != "" {
					sub += " · " + r.Note
				}
				events = append(events, feverEvent{
					id:    r.ID,
					ts:    r.RecordedAt,
					title: fmt.Sprintf("%.1f ℃", r.TempC),
					sub:   sub,
					col:   tempColor(r.TempC, pal),
				})
			}
			for _, m := range allMeds {
				sub := model.MedTypeLabel(m.MedType)
				if m.Dosage != "" {
					sub += " · " + m.Dosage
				}
				if m.Note != "" {
					sub += " · " + m.Note
				}
				events = append(events, feverEvent{
					id:    m.ID,
					ts:    m.RecordedAt,
					isMed: true,
					title: m.Name,
					sub:   sub,
					col:   pal.Indigo,
				})
			}
			sort.Slice(events, func(i, j int) bool { return events[i].ts > events[j].ts })

			ui.Textf(c, "体温与处理时间线 (%d)", len(events)).FontSize(14).Bold().TextColor(t.TextMuted)
			if len(events) == 0 {
				ui.Text(c, "暂无记录，点击「+ 记体温」开始追踪。").FontSize(13).TextColor(t.TextMuted)
			} else {
				ui.Column(c).Radius(12).Background(pal.Card).Border(1, t.Border).Dividers(0.5, t.Border).Clip().Children(func() {
					for _, ev := range events {
						ev := ev
						mobile.SwipeRow(c.Key(ev.id), nil, mobile.SwipeOptions{
							Trailing: []mobile.SwipeAction{{
								Label:       "删除",
								Destructive: true,
								OnTap: func() {
									if ev.isMed {
										_ = a.Store.DeleteMedicationRecord(ev.id)
									} else {
										_ = a.Store.DeleteTemperatureRecord(ev.id)
									}
									a.TriggerSync(c, true)
								},
							}},
						}, func() {
							ui.Row(c).FillWidth().Padding(12, 14).AlignItems(ui.Center).Gap(10).Children(func() {
								tag := "体温"
								if ev.isMed {
									tag = "处理"
								}
								ui.Text(c, tag).FontSize(11).Bold().Padding(3, 7).Radius(8).
									Background(ev.col.Alpha(0.14)).TextColor(ev.col)
								ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
									ui.Text(c, ev.title).Bold()
									ui.Text(c, time.UnixMilli(ev.ts).Format("01-02 15:04")+" · "+ev.sub).FontSize(12).TextColor(t.TextMuted)
								})
							})
						})
					}
				})
			}

			ui.Text(c, "注：Vita 仅作为个人健康数据记录工具，不构成医疗诊断或用药剂量建议，高热或持续不适请及时就医。").
				FontSize(11).TextColor(t.TextMuted).Padding(4, 0, 0, 0)
		})
	})
}

func (a *App) episodeChip(c *ui.Context, pal Palette, epID, label string) {
	t := c.Theme()
	sel := a.FeverEpisodeID == epID
	chip := ui.Row(c.Key("ep-" + epID)).Padding(6, 12).Radius(16).Shrink(0).Role(ui.RoleButton).Label(label)
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
		ui.Text(c, label).NoWrap().FontSize(12).FontWeight(500).TextColor(col)
	})
	if chip.Clicked() {
		a.FeverEpisodeID = epID
		a.FeverSelected = -1
	}
}

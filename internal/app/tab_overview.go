package app

import (
	"fmt"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

func (a *App) overviewTab(c *ui.Context, pal Palette, member model.Member, hasMember bool) {
	t := c.Theme()
	scroll := ui.Scroll(c).Fill()
	if scroll.PullToRefresh(&a.Refresh) {
		a.TriggerSync(c, false)
	}

	scroll.Children(func() {
		ui.Column(c).Padding(12, 16, 28, 16).Gap(14).Children(func() {
			if !hasMember {
				a.emptyOnboardingCard(c, pal)
				return
			}

			now := time.Now()
			// Member Hero Card
			ui.Column(c).Padding(16).Radius(16).Background(pal.Card).Border(1, t.Border).Gap(10).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
					renderMemberAvatar(c, member, 48)
					ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, member.Name).FontSize(18).Bold()
							gTxt := "男"
							if member.Gender == model.GenderFemale {
								gTxt = "女"
							}
							ui.Text(c, gTxt).FontSize(11).Padding(2, 7).Radius(10).Background(pal.TealSoft).TextColor(pal.Teal)
						})
						ui.Text(c, member.AgeLabelAt(now)+" · 生日 "+member.BirthDate).FontSize(13).TextColor(t.TextMuted)
					})
				})

				// Latest Growth Summary
				gs := a.Store.GrowthRecords(member.ID)
				var latestH, latestW *float64
				var latestBMI *float64
				var hPctStr, wPctStr string
				std := growth.Standard(a.Store.Config().LastStandard)
				for i := len(gs) - 1; i >= 0; i-- {
					recAt := time.UnixMilli(gs[i].RecordedAt)
					ageM := member.AgeMonthsAt(recAt)
					if latestH == nil && gs[i].HeightCM != nil {
						latestH = gs[i].HeightCM
						if member.IsChildAt(recAt) {
							if p, _, ok := growth.Percentile(std, member.Gender, growth.MetricHeight, ageM, *latestH); ok {
								hPctStr = fmt.Sprintf("P%.0f", p)
							}
						}
					}
					if latestW == nil && gs[i].WeightKG != nil {
						latestW = gs[i].WeightKG
						if member.IsChildAt(recAt) {
							if p, _, ok := growth.Percentile(std, member.Gender, growth.MetricWeight, ageM, *latestW); ok {
								wPctStr = fmt.Sprintf("P%.0f", p)
							}
						}
					}
					if latestBMI == nil {
						latestBMI = gs[i].BMI()
					}
					if latestH != nil && latestW != nil {
						break
					}
				}

				ui.Row(c).Gap(10).Children(func() {
					a.metricBadge(c, pal, "身高", formatOptFloat(latestH, "%.1f cm"), hPctStr)
					a.metricBadge(c, pal, "体重", formatOptFloat(latestW, "%.2f kg"), wPctStr)
					a.metricBadge(c, pal, "BMI", formatOptFloat(latestBMI, "%.1f"), "")
				})
			})

			// Active Fever Banner if any recent temp in last 48h
			lastT, lastM := a.Store.LatestTempAndMed(member.ID)
			if lastT != nil && now.Sub(time.UnixMilli(lastT.RecordedAt)) <= 48*time.Hour {
				col := tempColor(lastT.TempC, pal)
				bg := pal.GreenSoft
				if lastT.TempC >= 38.5 {
					bg = pal.RedSoft
				} else if lastT.TempC >= 37.3 {
					bg = pal.AmberSoft
				}
				banner := ui.Row(c).Padding(12, 14).Radius(14).Background(bg).Border(1, col.Alpha(0.35)).AlignItems(ui.Center).Gap(10)
				banner.Children(func() {
					ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
						ui.Textf(c, "最近体温 %.1f℃（%s）", lastT.TempC, model.SiteLabel(lastT.Site)).Bold().TextColor(col)
						sub := "测温于 " + localstore.FormatElapsed(lastT.RecordedAt, now)
						if lastM != nil {
							sub += " · 上次" + lastM.Name + " " + localstore.FormatElapsed(lastM.RecordedAt, now)
						}
						ui.Text(c, sub).FontSize(12).TextColor(t.TextMuted)
					})
					ui.Text(c, "看趋势 ›").FontSize(13).Bold().TextColor(col)
				})
				if banner.Clicked() {
					a.Tab = 2
				}
			}

			// 2x2 Quick Log Grid
			ui.Text(c, "快速记录").FontSize(14).Bold().TextColor(t.TextMuted)
			ui.Column(c).Gap(10).Children(func() {
				ui.Row(c).Gap(10).Children(func() {
					a.quickActionCard(c, pal, "+ 记体温", "测温 & 症状", pal.Red, pal.RedSoft, a.openTempForm)
					a.quickActionCard(c, pal, "+ 身高体重", "生长 & 头围", pal.Teal, pal.TealSoft, a.openGrowthForm)
				})
				ui.Row(c).Gap(10).Children(func() {
					a.quickActionCard(c, pal, "+ 退烧药/降温", "退烧药 / 物理降温", pal.Indigo, pal.IndigoSoft, a.openMedForm)
					a.quickActionCard(c, pal, "+ 自定义指标", "脚长/视力等", pal.Amber, pal.AmberSoft, a.openCustomValForm)
				})
			})

			// Sync status bar
			if a.SyncMessage != "" || a.Store.PendingCount() > 0 {
				ui.Row(c).Padding(10, 12).Radius(10).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
					msg := a.SyncMessage
					if msg == "" {
						msg = fmt.Sprintf("待同步记录: %d 条", a.Store.PendingCount())
					}
					ui.Text(c, msg).FontSize(12).TextColor(t.TextMuted).Grow(1).Basis(0)
					if a.Store.Config().AuthToken != "" {
						if ui.Button(c, "同步").Clicked() {
							a.TriggerSync(c, false)
						}
					}
				})
			}

			// Recent Activity Feed
			ui.Text(c, "最近动态").FontSize(14).Bold().TextColor(t.TextMuted)
			a.recentFeed(c, pal, member.ID)
		})
	})
}

func (a *App) emptyOnboardingCard(c *ui.Context, pal Palette) {
	t := c.Theme()
	ui.Column(c).Padding(24).Radius(16).Background(pal.Card).Border(1, t.Border).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "欢迎使用 Vita").FontSize(22).Bold()
		ui.Text(c, "记录家人的身高体重生长曲线与发烧体温变化，离线可用，支持自托管多设备同步。").
			FontSize(14).TextColor(t.TextMuted).TextAlign(ui.Center)
		if ui.PrimaryButton(c, "+ 添加第一位家庭成员").FillWidth().Clicked() {
			a.openNewMemberForm()
		}
		if a.Store.Config().AuthToken == "" {
			if ui.Button(c, "或登录已有同步家庭账号").FillWidth().Clicked() {
				a.ShowAuthSheet = true
			}
		}
	})
}

func (a *App) metricBadge(c *ui.Context, pal Palette, label, val, pct string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Basis(0).Padding(10).Radius(10).Background(pal.CardElevated).Gap(3).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(t.TextMuted).Grow(1).Basis(0)
			if pct != "" {
				ui.Text(c, pct).FontSize(10).Bold().TextColor(pal.Teal)
			}
		})
		ui.Text(c, val).FontSize(15).Bold().NoWrap()
	})
}

func (a *App) quickActionCard(c *ui.Context, pal Palette, title, sub string, accent, bg ui.Color, onTap func()) {
	t := c.Theme()
	card := ui.Column(c).Grow(1).Basis(0).Padding(14).Radius(14).Background(pal.Card).Border(1, t.Border).Gap(4).
		Role(ui.RoleButton).Label(title)
	card.Children(func() {
		ui.Text(c, title).FontSize(15).Bold().TextColor(accent)
		ui.Text(c, sub).FontSize(12).TextColor(t.TextMuted)
	})
	if card.Clicked() && onTap != nil {
		onTap()
	}
}

func (a *App) recentFeed(c *ui.Context, pal Palette, memberID string) {
	t := c.Theme()
	type feedItem struct {
		id    string
		ts    int64
		tag   string
		title string
		sub   string
		col   ui.Color
	}
	var items []feedItem
	for _, r := range a.Store.GrowthRecords(memberID) {
		var parts []string
		if r.HeightCM != nil {
			parts = append(parts, fmt.Sprintf("身高 %.1fcm", *r.HeightCM))
		}
		if r.WeightKG != nil {
			parts = append(parts, fmt.Sprintf("体重 %.2fkg", *r.WeightKG))
		}
		if r.HeadCircCM != nil {
			parts = append(parts, fmt.Sprintf("头围 %.1fcm", *r.HeadCircCM))
		}
		title := stringsJoin(parts, " · ")
		if title == "" {
			title = "生长记录"
		}
		items = append(items, feedItem{
			id: "g-" + r.ID, ts: r.RecordedAt, tag: "生长", title: title, sub: r.Note, col: pal.Teal,
		})
	}
	for _, r := range a.Store.TemperatureRecords(memberID, "") {
		sub := model.SiteLabel(r.Site)
		if r.Symptom != "" {
			sub += " · " + r.Symptom
		}
		items = append(items, feedItem{
			id: "t-" + r.ID, ts: r.RecordedAt, tag: "体温",
			title: fmt.Sprintf("%.1f ℃", r.TempC), sub: sub, col: tempColor(r.TempC, pal),
		})
	}
	for _, r := range a.Store.MedicationRecords(memberID, "") {
		sub := model.MedTypeLabel(r.MedType)
		if r.Dosage != "" {
			sub += " · " + r.Dosage
		}
		items = append(items, feedItem{
			id: "m-" + r.ID, ts: r.RecordedAt, tag: "处理", title: r.Name, sub: sub, col: pal.Indigo,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ts > items[j].ts })
	if len(items) > 12 {
		items = items[:12]
	}
	if len(items) == 0 {
		ui.Text(c, "暂无记录，点击上方按钮开始第一条记录吧。").FontSize(13).TextColor(t.TextMuted).PaddingY(8)
		return
	}
	ui.Column(c).Radius(12).Background(pal.Card).Border(1, t.Border).Dividers(0.5, t.Border).Clip().Children(func() {
		for _, it := range items {
			it := it
			ui.Row(c.Key(it.id)).Padding(11, 14).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, it.tag).FontSize(11).Bold().Padding(3, 7).Radius(8).
					Background(it.col.Alpha(0.14)).TextColor(it.col)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, it.title).FontSize(14).FontWeight(500)
					if it.sub != "" {
						ui.Text(c, it.sub).FontSize(12).TextColor(t.TextMuted)
					}
				})
				ui.Text(c, time.UnixMilli(it.ts).Format("01/02 15:04")).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}

func formatOptFloat(v *float64, layout string) string {
	if v == nil {
		return "--"
	}
	return fmt.Sprintf(layout, *v)
}

func stringsJoin(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

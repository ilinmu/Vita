// Package app implements Vita's pure-Go Native UI using egoist/mygo.
package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
	"github.com/ilinmu/vita/internal/syncer"
)

// App holds the live UI state for Vita.
type App struct {
	Win    *mygo.Window
	Store  *localstore.Store
	Syncer *syncer.Client

	Tab     int
	Refresh ui.RefreshState

	// Growth tab state
	GrowthMetricIdx int // 0=height, 1=weight, 2=head, 3=bmi, >=4 custom metrics
	GrowthStdIdx    int // 0=china, 1=who
	GrowthSelected  int

	// Fever tab state
	FeverEpisodeID string // "" = all recent
	FeverRangeIdx  int    // 0=24h, 1=3d, 2=7d
	FeverSelected  int

	// Settings / Appearance
	ThemeIdx int // 0=system, 1=light, 2=dark

	// Sheets visibility
	ShowMemberSwitcher bool
	ShowMemberForm     bool
	ShowTempForm       bool
	ShowGrowthForm     bool
	ShowMedForm        bool
	ShowEpisodeForm    bool
	ShowCustomDefForm  bool
	ShowCustomValForm  bool
	ShowAuthSheet      bool
	ShowJoinSheet      bool

	// Form buffers: Member
	EditMemberID    string
	MemberName      string
	MemberGenderIdx int // 0=male, 1=female
	MemberBirth     string
	MemberColorIdx  int

	// Form buffers: Temperature
	TempInput     string
	TempSiteIdx   int // 0=ear, 1=armpit, 2=forehead, 3=oral
	TempSymptom   string
	TempNote      string
	TempEpisodeID string

	// Form buffers: Growth
	GrowthHeight string
	GrowthWeight string
	GrowthHead   string
	GrowthNote   string

	// Form buffers: Medication / Cooling
	MedTypeIdx int // 0=ibuprofen, 1=acetaminophen, 2=physical, 3=other
	MedName    string
	MedDosage  string
	MedNote    string

	// Form buffers: Fever Episode
	EpisodeTitle string
	EpisodeNote  string

	// Form buffers: Custom Metric Definition & Value
	CustomMetricName string
	CustomMetricUnit string
	CustomValMetric  string
	CustomValInput   string
	CustomValNote    string

	// Form buffers: Sync & Auth
	ServerLANInput    string
	ServerPublicInput string
	AuthModeIdx       int // 0=login, 1=register
	AuthUsername      string
	AuthPassword      string
	AuthFamilyName    string
	AuthInviteCode    string
	JoinCodeInput     string

	SyncBusy    bool
	SyncMessage string
}

// New creates an initialized App bound to store and syncer.
func New(store *localstore.Store, syncClient *syncer.Client) *App {
	cfg := store.Config()
	themeIdx := 0
	switch cfg.ThemeMode {
	case "light":
		themeIdx = 1
	case "dark":
		themeIdx = 2
	}
	stdIdx := 0
	if cfg.LastStandard == string(growth.StandardWHO) {
		stdIdx = 1
	}
	return &App{
		Store:             store,
		Syncer:            syncClient,
		GrowthSelected:    -1,
		FeverSelected:     -1,
		ThemeIdx:          themeIdx,
		GrowthStdIdx:      stdIdx,
		TempSiteIdx:       siteToIdx(cfg.LastTempSite),
		ServerLANInput:    cfg.LANURL,
		ServerPublicInput: cfg.PublicURL,
		MemberBirth:       time.Now().AddDate(-1, 0, 0).Format("2006-01-02"),
	}
}

func siteToIdx(s model.TempSite) int {
	switch s {
	case model.TempSiteArmpit:
		return 1
	case model.TempSiteForehead:
		return 2
	case model.TempSiteOral:
		return 3
	default:
		return 0
	}
}

func idxToSite(i int) model.TempSite {
	switch i {
	case 1:
		return model.TempSiteArmpit
	case 2:
		return model.TempSiteForehead
	case 3:
		return model.TempSiteOral
	default:
		return model.TempSiteEar
	}
}

func idxToMedType(i int) model.MedType {
	switch i {
	case 0:
		return model.MedIbuprofen
	case 1:
		return model.MedAcetaminophen
	case 2:
		return model.MedPhysical
	default:
		return model.MedOther
	}
}

func tabIcon(draw func(p *ui.Path, r ui.Rect)) func(p *ui.Painter, r ui.Rect, color ui.Color) {
	return func(p *ui.Painter, r ui.Rect, color ui.Color) {
		var path ui.Path
		draw(&path, r)
		p.StrokePath(&path, 1.8, color)
	}
}

var navTabs = []mobile.BarButton{
	{
		Label: "概览",
		Icon: tabIcon(func(p *ui.Path, r ui.Rect) {
			p.MoveTo(r.X+4, r.Y+10).LineTo(r.X+12, r.Y+3).LineTo(r.X+20, r.Y+10)
			p.MoveTo(r.X+6, r.Y+10).LineTo(r.X+6, r.Y+20).LineTo(r.X+18, r.Y+20).LineTo(r.X+18, r.Y+10)
		}),
	},
	{
		Label: "生长",
		Icon: tabIcon(func(p *ui.Path, r ui.Rect) {
			p.MoveTo(r.X+3, r.Y+19).LineTo(r.X+9, r.Y+13).LineTo(r.X+14, r.Y+15).LineTo(r.X+21, r.Y+6)
			p.MoveTo(r.X+16, r.Y+6).LineTo(r.X+21, r.Y+6).LineTo(r.X+21, r.Y+11)
		}),
	},
	{
		Label: "发烧",
		Icon: tabIcon(func(p *ui.Path, r ui.Rect) {
			p.MoveTo(r.X+3, r.Y+12).LineTo(r.X+7, r.Y+12).LineTo(r.X+10, r.Y+5).LineTo(r.X+14, r.Y+19).LineTo(r.X+17, r.Y+12).LineTo(r.X+21, r.Y+12)
		}),
	},
	{
		Label: "家庭",
		Icon: tabIcon(func(p *ui.Path, r ui.Rect) {
			p.Circle(r.X+9, r.Y+9, 3.5)
			p.MoveTo(r.X+3, r.Y+20).LineTo(r.X+4.5, r.Y+15.5).LineTo(r.X+13.5, r.Y+15.5).LineTo(r.X+15, r.Y+20)
			p.Circle(r.X+17, r.Y+10, 2.5)
		}),
	},
}

// View renders Vita's entire responsive UI tree.
func (a *App) View(c *ui.Context) {
	cfg := a.Store.Config()
	theme, pal := resolveTheme(c, cfg.ThemeMode)
	c.SetTheme(theme)

	member, hasMember := a.Store.ActiveMember()

	ui.Column(c).Fill().Background(theme.Background).Children(func() {
		a.renderTopBar(c, pal, member, hasMember)

		ui.Box(c.Key(a.Tab)).Grow(1).Basis(0).FillWidth().Children(func() {
			switch a.Tab {
			case 0:
				a.overviewTab(c, pal, member, hasMember)
			case 1:
				a.growthTab(c, pal, member, hasMember)
			case 2:
				a.feverTab(c, pal, member, hasMember)
			case 3:
				a.familyTab(c, pal)
			}
		})

		mobile.BottomBar(c, &a.Tab, navTabs...)
	})

	a.renderSheets(c, pal, member, hasMember)
}

func (a *App) renderTopBar(c *ui.Context, pal Palette, member model.Member, hasMember bool) {
	t := c.Theme()
	titles := []string{"Vita", "生长曲线", "发烧记录", "家庭与设置"}
	mobile.TopBar(c, mobile.TopBarOptions{
		Title: titles[a.Tab],
		Leading: func() {
			pillLabel := "添加成员"
			if hasMember {
				pillLabel = member.Name + " ▾"
			}
			btn := ui.Row(c).Padding(6, 10).Radius(16).Background(pal.TealSoft).AlignItems(ui.Center).
				Role(ui.RoleButton).Label("切换成员")
			btn.Children(func() {
				ui.Text(c, pillLabel).NoWrap().FontSize(13).Bold().TextColor(pal.Teal)
			})
			if btn.Clicked() {
				a.ShowMemberSwitcher = true
			}
		},
		Trailing: func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				modeIcon := "◐"
				switch a.ThemeIdx {
				case 1:
					modeIcon = "☀"
				case 2:
					modeIcon = "☾"
				}
				tb := ui.Row(c).Size(32, 32).Radius(16).Background(t.Surface).Border(1, t.Border).
					AlignItems(ui.Center).Justify(ui.Center).Role(ui.RoleButton).Label("切换主题")
				tb.Children(func() {
					ui.Text(c, modeIcon).FontSize(14).TextColor(t.Text)
				})
				if tb.Clicked() {
					a.cycleTheme(c)
				}
			})
		},
	})
}

func (a *App) cycleTheme(c *ui.Context) {
	a.ThemeIdx = (a.ThemeIdx + 1) % 3
	modes := []string{"system", "light", "dark"}
	labels := []string{"主题：跟随系统", "主题：浅色模式", "主题：深色模式"}
	_ = a.Store.UpdateConfig(func(cfg *localstore.Config) {
		cfg.ThemeMode = modes[a.ThemeIdx]
	})
	c.Toast(labels[a.ThemeIdx])
}

// TriggerSync runs a non-blocking incremental sync against the configured sync server.
func (a *App) TriggerSync(c *ui.Context, silent bool) {
	cfg := a.Store.Config()
	if cfg.AuthToken == "" || cfg.FamilyID == "" {
		if !silent && c != nil {
			c.Toast("尚未登录同步服务器，已保存在本机")
		}
		return
	}
	if a.SyncBusy {
		return
	}
	a.SyncBusy = true
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		err := a.Syncer.Sync(ctx, a.Store)
		msg := ""
		if err != nil {
			msg = "离线暂存：" + err.Error()
		} else {
			ep := "外网"
			if a.Syncer.ActiveEndpoint() == "lan" {
				ep = "局域网"
			}
			msg = fmt.Sprintf("同步完成 (%s)", ep)
		}
		if a.Win != nil {
			a.Win.Update(func() {
				a.SyncBusy = false
				a.SyncMessage = msg
				a.Refresh.Refreshing = false
			})
		} else {
			a.SyncBusy = false
			a.SyncMessage = msg
			a.Refresh.Refreshing = false
		}
	}()
}

func (a *App) exportFile(c *ui.Context, filename string, data []byte, shareText string) {
	dir := filepath.Join(a.Store.Dir(), "exports")
	_ = os.MkdirAll(dir, 0o755)
	outPath := filepath.Join(dir, filename)
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		c.Toast("导出失败: " + err.Error())
		return
	}
	go func() {
		err := mygo.Share(mygo.ShareOptions{
			Text:  shareText,
			Files: []string{outPath},
		})
		if err != nil && errors.Is(err, errors.ErrUnsupported) {
			if a.Win != nil {
				a.Win.Update(func() {})
			}
		}
	}()
	c.Toast("已导出至 " + outPath)
}

// --- Modal Sheets ---

func (a *App) renderSheets(c *ui.Context, pal Palette, member model.Member, hasMember bool) {
	t := c.Theme()

	// 1. Member switcher sheet
	mobile.Sheet(c, &a.ShowMemberSwitcher, mobile.SheetOptions{Detents: []float32{0.55}, Label: "选择成员"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(10).Children(func() {
				ui.Text(c, "家庭成员").FontSize(18).Bold()
				ms := a.Store.Members()
				for _, m := range ms {
					m := m
					active := hasMember && m.ID == member.ID
					row := ui.Row(c.Key(m.ID)).FillWidth().Padding(12, 14).Radius(12).
						Background(t.Surface).Border(1, func() ui.Color {
						if active {
							return pal.Teal
						}
						return t.Border
					}()).AlignItems(ui.Center).Gap(12)
					row.Children(func() {
						ui.Box(c).Size(36, 36).Radius(18).Background(ui.Hex(m.ColorHex)).Center().Children(func() {
							initial := "V"
							for _, r := range m.Name {
								initial = string(r)
								break
							}
							ui.Text(c, initial).Bold().TextColor(ui.RGB(255, 255, 255))
						})
						ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
							ui.Text(c, m.Name).Bold()
							gLabel := "男"
							if m.Gender == model.GenderFemale {
								gLabel = "女"
							}
							ui.Textf(c, "%s · %s", gLabel, m.AgeLabelAt(time.Now())).FontSize(13).TextColor(t.TextMuted)
						})
						if active {
							ui.Text(c, "当前").FontSize(12).Bold().TextColor(pal.Teal)
						}
					})
					if row.Clicked() {
						_ = a.Store.UpdateConfig(func(cfg *localstore.Config) {
							cfg.ActiveMemberID = m.ID
						})
						a.GrowthSelected = -1
						a.FeverSelected = -1
						a.ShowMemberSwitcher = false
					}
				}
				if ui.PrimaryButton(c, "+ 添加新成员").FillWidth().Clicked() {
					a.ShowMemberSwitcher = false
					a.openNewMemberForm()
				}
			})
		})
	})

	// 2. Member add/edit sheet
	mobile.Sheet(c, &a.ShowMemberForm, mobile.SheetOptions{Detents: []float32{0.68}, Label: "成员档案"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				title := "添加成员"
				if a.EditMemberID != "" {
					title = "编辑成员"
				}
				ui.Text(c, title).FontSize(18).Bold()
				ui.Text(c, "姓名 / 昵称").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.MemberName).Placeholder("例如：小核桃").Label("姓名")

				ui.Text(c, "性别（用于匹配生长参考标准）").FontSize(13).TextColor(t.TextMuted)
				ui.Segmented(c, &a.MemberGenderIdx, "男", "女").FillEqually().Label("性别")

				ui.Text(c, "出生日期 (YYYY-MM-DD)").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.MemberBirth).Placeholder("2024-06-01").Label("出生日期")

				ui.Text(c, "标识颜色").FontSize(13).TextColor(t.TextMuted)
				ui.Row(c).Gap(10).Children(func() {
					for i, hex := range memberSwatches {
						box := ui.Box(c.Key(i)).Size(30, 30).Radius(15).Background(ui.Hex(hex))
						if i == a.MemberColorIdx {
							box.Border(3, t.Text)
						}
						if box.Clicked() {
							a.MemberColorIdx = i
						}
					}
				})

				if ui.PrimaryButton(c, "保存成员").FillWidth().Clicked() {
					name := strings.TrimSpace(a.MemberName)
					if name == "" {
						c.Toast("请输入姓名")
						return
					}
					if _, err := time.Parse("2006-01-02", strings.TrimSpace(a.MemberBirth)); err != nil {
						c.Toast("日期格式应为 YYYY-MM-DD")
						return
					}
					g := model.GenderMale
					if a.MemberGenderIdx == 1 {
						g = model.GenderFemale
					}
					m, err := a.Store.SaveMember(model.Member{
						ID:        a.EditMemberID,
						Name:      name,
						Gender:    g,
						BirthDate: strings.TrimSpace(a.MemberBirth),
						ColorHex:  memberSwatches[a.MemberColorIdx%len(memberSwatches)],
					})
					if err != nil {
						c.Toast("保存失败: " + err.Error())
						return
					}
					_ = a.Store.UpdateConfig(func(cfg *localstore.Config) {
						cfg.ActiveMemberID = m.ID
					})
					a.ShowMemberForm = false
					a.TriggerSync(c, true)
					c.Toast("成员已保存")
				}
			})
		})
	})

	// 3. Temperature log sheet
	mobile.Sheet(c, &a.ShowTempForm, mobile.SheetOptions{Detents: []float32{0.72}, Label: "记录体温"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				ui.Text(c, "记录体温").FontSize(18).Bold()
				ui.Text(c, "体温 (℃)").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.TempInput).Placeholder("例如 38.5").Keyboard(ui.KeyboardDecimal).Label("体温")

				ui.Text(c, "测温部位（自动记住上次选择）").FontSize(13).TextColor(t.TextMuted)
				ui.Segmented(c, &a.TempSiteIdx, "耳温", "腋温", "额温", "口温").FillEqually().Label("测温部位")

				ui.Text(c, "伴随症状（可选）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.TempSymptom).Placeholder("如：精神尚可、轻微流涕").Label("症状")

				ui.Text(c, "备注（可选）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.TempNote).Placeholder("补充说明").Label("备注")

				if ui.PrimaryButton(c, "保存体温").FillWidth().Clicked() {
					if !hasMember {
						c.Toast("请先添加家庭成员")
						return
					}
					val, err := strconv.ParseFloat(strings.TrimSpace(a.TempInput), 64)
					if err != nil || val < 34.0 || val > 43.0 {
						c.Toast("请输入有效体温 (34.0 - 43.0 ℃)")
						return
					}
					epID := a.TempEpisodeID
					if epID == "" {
						epID = a.FeverEpisodeID
					}
					_, err = a.Store.SaveTemperatureRecord(model.TemperatureRecord{
						MemberID:  member.ID,
						EpisodeID: epID,
						TempC:     val,
						Site:      idxToSite(a.TempSiteIdx),
						Symptom:   strings.TrimSpace(a.TempSymptom),
						Note:      strings.TrimSpace(a.TempNote),
					})
					if err != nil {
						c.Toast("保存失败: " + err.Error())
						return
					}
					a.ShowTempForm = false
					a.TriggerSync(c, true)
					c.Toast(fmt.Sprintf("已记录 %.1f℃", val))
				}
			})
		})
	})

	// 4. Growth log sheet
	mobile.Sheet(c, &a.ShowGrowthForm, mobile.SheetOptions{Detents: []float32{0.72}, Label: "记录生长发育"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				ui.Text(c, "记录身高体重").FontSize(18).Bold()
				ui.Text(c, "身高 (cm，可留空)").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.GrowthHeight).Placeholder("例如 88.5").Keyboard(ui.KeyboardDecimal).Label("身高")

				ui.Text(c, "体重 (kg，可留空)").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.GrowthWeight).Placeholder("例如 12.6").Keyboard(ui.KeyboardDecimal).Label("体重")

				ui.Text(c, "头围 (cm，0-3岁常用，可留空)").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.GrowthHead).Placeholder("例如 48.2").Keyboard(ui.KeyboardDecimal).Label("头围")

				ui.Text(c, "备注（可选）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.GrowthNote).Placeholder("如：1岁半儿保体检").Label("备注")

				if ui.PrimaryButton(c, "保存测量记录").FillWidth().Clicked() {
					if !hasMember {
						c.Toast("请先添加家庭成员")
						return
					}
					var hPtr, wPtr, hcPtr *float64
					if s := strings.TrimSpace(a.GrowthHeight); s != "" {
						v, err := strconv.ParseFloat(s, 64)
						if err != nil || v <= 0 || v > 250 {
							c.Toast("身高数值无效")
							return
						}
						hPtr = &v
					}
					if s := strings.TrimSpace(a.GrowthWeight); s != "" {
						v, err := strconv.ParseFloat(s, 64)
						if err != nil || v <= 0 || v > 300 {
							c.Toast("体重数值无效")
							return
						}
						wPtr = &v
					}
					if s := strings.TrimSpace(a.GrowthHead); s != "" {
						v, err := strconv.ParseFloat(s, 64)
						if err != nil || v <= 0 || v > 80 {
							c.Toast("头围数值无效")
							return
						}
						hcPtr = &v
					}
					if hPtr == nil && wPtr == nil && hcPtr == nil {
						c.Toast("请至少填写一项测量数据")
						return
					}
					_, err := a.Store.SaveGrowthRecord(model.GrowthRecord{
						MemberID:   member.ID,
						HeightCM:   hPtr,
						WeightKG:   wPtr,
						HeadCircCM: hcPtr,
						Note:       strings.TrimSpace(a.GrowthNote),
					})
					if err != nil {
						c.Toast("保存失败: " + err.Error())
						return
					}
					a.ShowGrowthForm = false
					a.TriggerSync(c, true)
					c.Toast("生长记录已保存")
				}
			})
		})
	})

	// 5. Medication / cooling sheet
	mobile.Sheet(c, &a.ShowMedForm, mobile.SheetOptions{Detents: []float32{0.72}, Label: "记录退烧药/降温"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				ui.Text(c, "记录退烧药 / 物理降温").FontSize(18).Bold()
				ui.Text(c, "类型").FontSize(13).TextColor(t.TextMuted)
				ui.Segmented(c, &a.MedTypeIdx, "布洛芬", "对乙酰", "物理降温", "其他").FillEqually().Label("用药类型")

				ui.Text(c, "名称（如：美林、泰诺林、温水擦浴）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.MedName).Placeholder("可选，默认按类型名").Label("名称")

				ui.Text(c, "剂量 / 说明（仅记录，不作医嘱）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.MedDosage).Placeholder("例如：4 ml 或 贴退热贴").Label("剂量")

				ui.Text(c, "备注（可选）").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.MedNote).Placeholder("补充说明").Label("备注")

				if ui.PrimaryButton(c, "保存处理记录").FillWidth().Clicked() {
					if !hasMember {
						c.Toast("请先添加家庭成员")
						return
					}
					mt := idxToMedType(a.MedTypeIdx)
					name := strings.TrimSpace(a.MedName)
					if name == "" {
						name = model.MedTypeLabel(mt)
					}
					_, err := a.Store.SaveMedicationRecord(model.MedicationRecord{
						MemberID:  member.ID,
						EpisodeID: a.FeverEpisodeID,
						MedType:   mt,
						Name:      name,
						Dosage:    strings.TrimSpace(a.MedDosage),
						Note:      strings.TrimSpace(a.MedNote),
					})
					if err != nil {
						c.Toast("保存失败: " + err.Error())
						return
					}
					a.ShowMedForm = false
					a.TriggerSync(c, true)
					c.Toast("已记录 " + name)
				}
			})
		})
	})

	// 6. New Fever Episode sheet
	mobile.Sheet(c, &a.ShowEpisodeForm, mobile.SheetOptions{Detents: []float32{0.48}, Label: "新建发烧病程"}, func() {
		ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
			ui.Text(c, "新建发烧病程").FontSize(18).Bold()
			ui.Text(c, "病程名称（方便日后对照和就诊查看）").FontSize(13).TextColor(t.TextMuted)
			ui.TextInput(c, &a.EpisodeTitle).Placeholder("例如：2026年11月感冒发烧").Label("病程名称")
			ui.TextInput(c, &a.EpisodeNote).Placeholder("备注（可选）").Label("备注")
			if ui.PrimaryButton(c, "创建病程").FillWidth().Clicked() {
				if !hasMember {
					c.Toast("请先添加家庭成员")
					return
				}
				title := strings.TrimSpace(a.EpisodeTitle)
				if title == "" {
					title = time.Now().Format("01月02日 发烧记录")
				}
				ep, err := a.Store.SaveFeverEpisode(model.FeverEpisode{
					MemberID: member.ID,
					Title:    title,
					Note:     strings.TrimSpace(a.EpisodeNote),
				})
				if err != nil {
					c.Toast("创建失败: " + err.Error())
					return
				}
				a.FeverEpisodeID = ep.ID
				a.ShowEpisodeForm = false
				a.TriggerSync(c, true)
				c.Toast("已切换到病程：" + title)
			}
		})
	})

	// 7. New Custom Metric definition sheet
	mobile.Sheet(c, &a.ShowCustomDefForm, mobile.SheetOptions{Detents: []float32{0.48}, Label: "新建自定义指标"}, func() {
		ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
			ui.Text(c, "新建自定义指标").FontSize(18).Bold()
			ui.Text(c, "指标名称（如：脚长、左眼视力、牙数）").FontSize(13).TextColor(t.TextMuted)
			ui.TextInput(c, &a.CustomMetricName).Placeholder("例如：脚长").Label("指标名称")
			ui.Text(c, "单位（如：cm、颗、度）").FontSize(13).TextColor(t.TextMuted)
			ui.TextInput(c, &a.CustomMetricUnit).Placeholder("例如：cm").Label("单位")
			if ui.PrimaryButton(c, "创建指标").FillWidth().Clicked() {
				name := strings.TrimSpace(a.CustomMetricName)
				if name == "" {
					c.Toast("请输入指标名称")
					return
				}
				_, err := a.Store.SaveCustomMetric(model.CustomMetric{
					Name: name,
					Unit: strings.TrimSpace(a.CustomMetricUnit),
				})
				if err != nil {
					c.Toast("保存失败: " + err.Error())
					return
				}
				a.ShowCustomDefForm = false
				a.TriggerSync(c, true)
				c.Toast("已创建指标：" + name)
			}
		})
	})

	// 8. Record Custom Metric value sheet
	mobile.Sheet(c, &a.ShowCustomValForm, mobile.SheetOptions{Detents: []float32{0.55}, Label: "记录自定义指标"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				ui.Text(c, "记录自定义指标").FontSize(18).Bold()
				cms := a.Store.CustomMetrics()
				if len(cms) == 0 {
					ui.Text(c, "还没有自定义指标，先创建一个吧。").TextColor(t.TextMuted)
					if ui.PrimaryButton(c, "+ 新建指标").FillWidth().Clicked() {
						a.ShowCustomValForm = false
						a.CustomMetricName, a.CustomMetricUnit = "", ""
						a.ShowCustomDefForm = true
					}
					return
				}
				if a.CustomValMetric == "" {
					a.CustomValMetric = cms[0].ID
				}
				ui.ScrollHorizontal(c).FillWidth().Gap(8).Children(func() {
					for _, cm := range cms {
						cm := cm
						sel := cm.ID == a.CustomValMetric
						chip := ui.Row(c.Key(cm.ID)).Padding(6, 12).Radius(16).Shrink(0)
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
							ui.Text(c, cm.Name).NoWrap().FontSize(13).TextColor(col)
						})
						if chip.Clicked() {
							a.CustomValMetric = cm.ID
						}
					}
				})
				ui.Text(c, "数值").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.CustomValInput).Placeholder("输入数值").Keyboard(ui.KeyboardDecimal).Label("数值")
				ui.TextInput(c, &a.CustomValNote).Placeholder("备注（可选）").Label("备注")
				if ui.PrimaryButton(c, "保存记录").FillWidth().Clicked() {
					if !hasMember {
						c.Toast("请先添加家庭成员")
						return
					}
					v, err := strconv.ParseFloat(strings.TrimSpace(a.CustomValInput), 64)
					if err != nil {
						c.Toast("请输入有效数字")
						return
					}
					_, err = a.Store.SaveCustomMetricRecord(model.CustomMetricRecord{
						MemberID: member.ID,
						MetricID: a.CustomValMetric,
						Value:    v,
						Note:     strings.TrimSpace(a.CustomValNote),
					})
					if err != nil {
						c.Toast("保存失败: " + err.Error())
						return
					}
					a.ShowCustomValForm = false
					a.TriggerSync(c, true)
					c.Toast("自定义指标已保存")
				}
			})
		})
	})

	// 9. Auth Login / Register sheet
	mobile.Sheet(c, &a.ShowAuthSheet, mobile.SheetOptions{Detents: []float32{0.75}, Label: "账号同步"}, func() {
		ui.Scroll(c).Grow(1).Basis(0).Children(func() {
			ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
				ui.Text(c, "连接自托管同步服务").FontSize(18).Bold()
				ui.Segmented(c, &a.AuthModeIdx, "登录已有账号", "注册新账号").FillEqually().Label("认证模式")

				ui.Text(c, "用户名").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.AuthUsername).Placeholder("至少 2 个字符").Capitalize(ui.CapitalizeNone).Autocorrect(false).Label("用户名")

				ui.Text(c, "密码").FontSize(13).TextColor(t.TextMuted)
				ui.TextInput(c, &a.AuthPassword).Password().Placeholder("至少 6 位").Label("密码")

				if a.AuthModeIdx == 1 {
					ui.Text(c, "家庭邀请码（加入已有家庭填此项；首创家庭可留空）").FontSize(13).TextColor(t.TextMuted)
					ui.TextInput(c, &a.AuthInviteCode).Placeholder("6 位邀请码（可选）").Capitalize(ui.CapitalizeCharacters).Label("邀请码")
					ui.Text(c, "新建家庭名称（未填邀请码时生效）").FontSize(13).TextColor(t.TextMuted)
					ui.TextInput(c, &a.AuthFamilyName).Placeholder("例如：小核桃家").Label("家庭名称")
				}

				btnLbl := "登录并同步"
				if a.AuthModeIdx == 1 {
					btnLbl = "注册并同步"
				}
				if ui.PrimaryButton(c, btnLbl).FillWidth().Clicked() {
					a.submitAuth(c)
				}
			})
		})
	})

	// 10. Join Family sheet
	mobile.Sheet(c, &a.ShowJoinSheet, mobile.SheetOptions{Detents: []float32{0.42}, Label: "加入家庭"}, func() {
		ui.Column(c).Padding(8, 16, 24, 16).Gap(12).Children(func() {
			ui.Text(c, "输入 6 位家庭邀请码").FontSize(18).Bold()
			ui.TextInput(c, &a.JoinCodeInput).Placeholder("例如：AB12CD").Capitalize(ui.CapitalizeCharacters).Label("邀请码")
			if ui.PrimaryButton(c, "加入并合并数据").FillWidth().Clicked() {
				a.submitJoinFamily(c)
			}
		})
	})
}

func (a *App) openNewMemberForm() {
	a.EditMemberID = ""
	a.MemberName = ""
	a.MemberGenderIdx = 0
	a.MemberBirth = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
	a.MemberColorIdx = len(a.Store.Members()) % len(memberSwatches)
	a.ShowMemberForm = true
}

func (a *App) openTempForm() {
	a.TempInput = ""
	a.TempSymptom = ""
	a.TempNote = ""
	a.TempEpisodeID = a.FeverEpisodeID
	a.TempSiteIdx = siteToIdx(a.Store.Config().LastTempSite)
	a.ShowTempForm = true
}

func (a *App) openGrowthForm() {
	a.GrowthHeight = ""
	a.GrowthWeight = ""
	a.GrowthHead = ""
	a.GrowthNote = ""
	a.ShowGrowthForm = true
}

func (a *App) openMedForm() {
	a.MedTypeIdx = 0
	a.MedName = ""
	a.MedDosage = ""
	a.MedNote = ""
	a.ShowMedForm = true
}

func (a *App) submitAuth(c *ui.Context) {
	cfg := a.Store.Config()
	if cfg.LANURL == "" && cfg.PublicURL == "" {
		c.Toast("请先保存局域网或外网同步服务器地址")
		return
	}
	a.Syncer.SetEndpoints(cfg.LANURL, cfg.PublicURL)
	user := strings.TrimSpace(a.AuthUsername)
	pass := a.AuthPassword
	isReg := a.AuthModeIdx == 1
	famName := strings.TrimSpace(a.AuthFamilyName)
	inv := strings.TrimSpace(a.AuthInviteCode)

	c.Toast("正在连接服务器...")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		var resp model.AuthResponse
		var err error
		if isReg {
			resp, err = a.Syncer.Register(ctx, model.AuthRequest{
				Username:   user,
				Password:   pass,
				FamilyName: famName,
				InviteCode: inv,
			})
		} else {
			resp, err = a.Syncer.Login(ctx, user, pass)
		}
		if err != nil {
			a.finishAsyncMsg("认证失败: " + err.Error())
			return
		}
		var fam model.FamilyInfo
		if len(resp.Families) > 0 {
			fam = resp.Families[0]
		}
		_ = a.Store.UpdateConfig(func(cf *localstore.Config) {
			cf.AuthToken = resp.Token
			cf.UserID = resp.User.ID
			cf.Username = resp.User.Username
			cf.FamilyID = fam.ID
			cf.FamilyName = fam.Name
			cf.InviteCode = fam.InviteCode
		})
		if fam.ID != "" {
			_ = a.Store.AssignFamilyToLocalRecords(fam.ID)
			_ = a.Syncer.Sync(ctx, a.Store)
		}
		if a.Win != nil {
			a.Win.Update(func() {
				a.ShowAuthSheet = false
				a.SyncMessage = "已登录并完成首次同步"
			})
		} else {
			a.ShowAuthSheet = false
			a.SyncMessage = "已登录并完成首次同步"
		}
	}()
}

func (a *App) submitJoinFamily(c *ui.Context) {
	cfg := a.Store.Config()
	code := strings.TrimSpace(a.JoinCodeInput)
	if cfg.AuthToken == "" || code == "" {
		c.Toast("请填写有效邀请码")
		return
	}
	a.Syncer.SetEndpoints(cfg.LANURL, cfg.PublicURL)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		fam, err := a.Syncer.JoinFamily(ctx, cfg.AuthToken, code)
		if err != nil {
			a.finishAsyncMsg("加入失败: " + err.Error())
			return
		}
		_ = a.Store.UpdateConfig(func(cf *localstore.Config) {
			cf.FamilyID = fam.ID
			cf.FamilyName = fam.Name
			cf.InviteCode = fam.InviteCode
		})
		_ = a.Store.AssignFamilyToLocalRecords(fam.ID)
		_ = a.Syncer.Sync(ctx, a.Store)
		if a.Win != nil {
			a.Win.Update(func() {
				a.ShowJoinSheet = false
				a.SyncMessage = "已加入家庭：" + fam.Name
			})
		}
	}()
}

func (a *App) finishAsyncMsg(msg string) {
	if a.Win != nil {
		a.Win.Update(func() {
			a.SyncMessage = msg
		})
	} else {
		a.SyncMessage = msg
	}
}

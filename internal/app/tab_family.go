package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/export"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

func (a *App) familyTab(c *ui.Context, pal Palette) {
	t := c.Theme()
	cfg := a.Store.Config()

	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(12, 0, 32, 0).Gap(18).Children(func() {
			// 1. Members Section
			mobile.Section(c, "家庭成员", func() {
				for _, m := range a.Store.Members() {
					m := m
					gTxt := "男"
					if m.Gender == model.GenderFemale {
						gTxt = "女"
					}
					mobile.SwipeRow(c.Key("m-"+m.ID), nil, mobile.SwipeOptions{
						Trailing: []mobile.SwipeAction{{
							Label:       "删除",
							Destructive: true,
							OnTap: func() {
								_ = a.Store.DeleteMember(m.ID)
								a.TriggerSync(c, true)
							},
						}},
					}, func() {
						row := mobile.ListRow(c, mobile.RowOptions{
							Title:    m.Name,
							Subtitle: fmt.Sprintf("%s · %s (%s)", gTxt, m.AgeLabelAt(time.Now()), m.BirthDate),
							Leading: func() {
								renderMemberAvatar(c, m, 34)
							},
							Disclosure: true,
						})
						if row.Clicked() {
							a.openEditMemberForm(m)
						}
					})
				}
				if mobile.ListRow(c.Key("add-member"), mobile.RowOptions{
					Title: "+ 添加家庭成员",
				}).Clicked() {
					a.openNewMemberForm()
				}
			})

			// 2. Custom Metrics Section
			mobile.Section(c, "自定义指标（脚长/视力等）", func() {
				for _, cm := range a.Store.CustomMetrics() {
					cm := cm
					mobile.SwipeRow(c.Key("cm-"+cm.ID), nil, mobile.SwipeOptions{
						Trailing: []mobile.SwipeAction{{
							Label:       "删除",
							Destructive: true,
							OnTap: func() {
								_ = a.Store.DeleteCustomMetric(cm.ID)
								a.TriggerSync(c, true)
							},
						}},
					}, func() {
						mobile.ListRow(c, mobile.RowOptions{
							Title:    cm.Name,
							Subtitle: "单位: " + cm.Unit,
						})
					})
				}
				if mobile.ListRow(c.Key("add-cm-row"), mobile.RowOptions{
					Title: "+ 新建自定义指标",
				}).Clicked() {
					a.CustomMetricName, a.CustomMetricUnit = "", ""
					a.ShowCustomDefForm = true
				}
			})

			// 3. Self-hosted Sync Server Endpoints & Account
			ui.Column(c).PaddingX(16).Gap(8).Children(func() {
				ui.Text(c, "自托管双地址同步设置").FontSize(13).TextColor(t.TextMuted)
				ui.Column(c).Padding(14).Radius(12).Background(pal.Card).Border(1, t.Border).Gap(10).Children(func() {
					ui.Text(c, "局域网主地址（在家 Wi-Fi 优先直连，如 NAS / 家庭主机）").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &a.ServerLANInput).Placeholder("http://192.168.1.50:8420").
						Keyboard(ui.KeyboardURL).Autocorrect(false).Capitalize(ui.CapitalizeNone).Label("局域网地址")

					ui.Text(c, "外网备用地址（公网 HTTPS 反向代理或内网穿透，外出自动切换）").FontSize(12).TextColor(t.TextMuted)
					ui.TextInput(c, &a.ServerPublicInput).Placeholder("https://vita.yourdomain.com").
						Keyboard(ui.KeyboardURL).Autocorrect(false).Capitalize(ui.CapitalizeNone).Label("外网地址")

					if ui.Button(c, "保存服务器地址").FillWidth().Clicked() {
						lan := strings.TrimRight(strings.TrimSpace(a.ServerLANInput), "/")
						pub := strings.TrimRight(strings.TrimSpace(a.ServerPublicInput), "/")
						_ = a.Store.UpdateConfig(func(cf *localstore.Config) {
							cf.LANURL = lan
							cf.PublicURL = pub
						})
						a.Syncer.SetEndpoints(lan, pub)
						c.Toast("服务器地址已保存")
					}
				})
			})

			mobile.Section(c, "家庭账号与同步", func() {
				if cfg.AuthToken == "" {
					if mobile.ListRow(c.Key("login-row"), mobile.RowOptions{
						Title:      "登录 / 注册同步账号",
						Subtitle:   "当前为纯本机离线模式",
						Disclosure: true,
					}).Clicked() {
						a.ShowAuthSheet = true
					}
				} else {
					mobile.ListRow(c.Key("acct-info"), mobile.RowOptions{
						Title:    fmt.Sprintf("%s · %s", cfg.Username, cfg.FamilyName),
						Subtitle: fmt.Sprintf("家庭邀请码: %s（另一台手机注册/加入时填写）", cfg.InviteCode),
					})
					if mobile.ListRow(c.Key("sync-now"), mobile.RowOptions{
						Title:    "立即双向同步",
						Subtitle: syncSubtitle(cfg, a.Store.PendingCount(), a.SyncMessage),
					}).Clicked() {
						a.TriggerSync(c, false)
					}
					if mobile.ListRow(c.Key("join-fam"), mobile.RowOptions{
						Title:      "用邀请码加入其他家庭",
						Disclosure: true,
					}).Clicked() {
						a.JoinCodeInput = ""
						a.ShowJoinSheet = true
					}
					if mobile.ListRow(c.Key("logout"), mobile.RowOptions{
						Title: "退出登录（保留本机数据）",
					}).Clicked() {
						_ = a.Store.ResetAuth(false)
						a.SyncMessage = "已退出账号"
						c.Toast("已退出同步账号")
					}
				}
			})

			// 4. Theme Appearance
			ui.Column(c).PaddingX(16).Gap(8).Children(func() {
				ui.Text(c, "界面主题").FontSize(13).TextColor(t.TextMuted)
				if ui.Segmented(c, &a.ThemeIdx, "跟随系统", "浅色", "深色").FillEqually().Label("界面主题").Changed() {
					modes := []string{"system", "light", "dark"}
					_ = a.Store.UpdateConfig(func(cf *localstore.Config) {
						cf.ThemeMode = modes[a.ThemeIdx]
					})
				}
			})

			// 5. Export & Backup
			mobile.Section(c, "数据备份与导出", func() {
				if mobile.ListRow(c.Key("exp-json"), mobile.RowOptions{
					Title:    "导出 JSON 完整备份",
					Subtitle: "包含全部成员、生长曲线与发烧记录",
				}).Clicked() {
					raw, err := export.BackupJSON(a.Store.Snapshot())
					if err != nil {
						c.Toast("导出失败: " + err.Error())
					} else {
						a.exportFile(c, "vita-backup-"+time.Now().Format("20060102-150405")+".json", raw, "Vita JSON 备份")
					}
				}
				if mobile.ListRow(c.Key("exp-csv"), mobile.RowOptions{
					Title:    "导出 CSV 表格（Excel / Numbers 可直接打开）",
					Subtitle: "UTF-8 编码，就诊或整理数据时方便查看",
				}).Clicked() {
					raw, err := export.BackupCSV(a.Store.Snapshot())
					if err != nil {
						c.Toast("导出失败: " + err.Error())
					} else {
						a.exportFile(c, "vita-records-"+time.Now().Format("20060102-150405")+".csv", raw, "Vita 记录表格")
					}
				}
			})
		})
	})
}

func syncSubtitle(cfg localstore.Config, pending int, msg string) string {
	if msg != "" {
		return msg
	}
	last := "尚未同步"
	if cfg.LastSyncedAt > 0 {
		last = "上次同步 " + time.UnixMilli(cfg.LastSyncedAt).Format("01-02 15:04")
	}
	if pending > 0 {
		return fmt.Sprintf("%s · 待上传 %d 条", last, pending)
	}
	return last
}

package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/model"
)

func chineseDatePicker(c *ui.Context, key string, val *time.Time, minYear, maxYear int) {
	if maxYear < minYear {
		maxYear = minYear
	}
	v := *val
	if v.IsZero() {
		v = time.Now()
	}
	curY := max(minYear, min(maxYear, v.Year()))
	yIdx := curY - minYear
	mIdx := int(v.Month()) - 1
	lastDay := time.Date(curY, time.Month(mIdx+2), 0, 0, 0, 0, 0, v.Location()).Day()
	dIdx := min(v.Day()-1, lastDay-1)

	years := make([]string, maxYear-minYear+1)
	for i := range years {
		years[i] = fmt.Sprintf("%d年", minYear+i)
	}
	months := make([]string, 12)
	for i := range months {
		months[i] = fmt.Sprintf("%d月", i+1)
	}
	days := make([]string, lastDay)
	for i := range days {
		days[i] = fmt.Sprintf("%d日", i+1)
	}

	ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(4).Children(func() {
		chY := mobile.Wheel(c.Key("y"), &yIdx, years).Grow(1.3).Basis(0).Changed()
		chM := mobile.Wheel(c.Key("m"), &mIdx, months).Grow(1).Basis(0).Changed()
		chD := mobile.Wheel(c.Key("d"), &dIdx, days).Grow(1).Basis(0).Changed()
		if chY || chM || chD {
			ny := minYear + yIdx
			nm := time.Month(mIdx + 1)
			maxD := time.Date(ny, nm+1, 0, 0, 0, 0, 0, v.Location()).Day()
			nd := min(dIdx+1, maxD)
			*val = time.Date(ny, nm, nd, v.Hour(), v.Minute(), 0, 0, v.Location())
		}
	})
}

func chineseTimePicker(c *ui.Context, key string, val *time.Time) {
	v := *val
	if v.IsZero() {
		v = time.Now()
	}
	hIdx := v.Hour()
	mIdx := v.Minute()

	hours := make([]string, 24)
	for i := range hours {
		hours[i] = fmt.Sprintf("%02d时", i)
	}
	mins := make([]string, 60)
	for i := range mins {
		mins[i] = fmt.Sprintf("%02d分", i)
	}

	ui.Row(c.Key(key)).AlignItems(ui.Center).Gap(4).Children(func() {
		chH := mobile.Wheel(c.Key("hr"), &hIdx, hours).Grow(1).Basis(0).Changed()
		chM := mobile.Wheel(c.Key("mn"), &mIdx, mins).Grow(1).Basis(0).Changed()
		if chH || chM {
			*val = time.Date(v.Year(), v.Month(), v.Day(), hIdx, mIdx, 0, 0, v.Location())
		}
	})
}

// datePickerField renders a date-only picker row (YYYY-MM-DD) with an expandable wheel and optional day shortcuts.
func datePickerField(c *ui.Context, pal Palette, key, label string, val *time.Time, expanded *bool, minYear, maxYear int, showShortcuts bool) {
	t := c.Theme()
	if val.IsZero() {
		now := time.Now()
		*val = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	}

	ui.Column(c.Key(key)).Gap(8).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(t.TextMuted)
		row := ui.Row(c).FillWidth().Padding(11, 14).Radius(t.Radius).Background(t.Surface).Border(1, t.Border).
			AlignItems(ui.Center).Role(ui.RoleButton).Label(label)
		row.Children(func() {
			ui.Text(c, val.Format("2006-01-02")).FontWeight(500).Grow(1).Basis(0)
			chev := "选择日期 ▾"
			if *expanded {
				chev = "收起 ▴"
			}
			ui.Text(c, chev).FontSize(12).TextColor(pal.Teal)
		})
		if row.Clicked() {
			*expanded = !*expanded
		}

		if showShortcuts {
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
			presets := []struct {
				lbl string
				day time.Time
			}{
				{"今天", today},
				{"昨天", today.AddDate(0, 0, -1)},
				{"前天", today.AddDate(0, 0, -2)},
			}
			ui.Row(c).Wrap().Gap(6).Children(func() {
				for _, p := range presets {
					p := p
					active := val.Format("2006-01-02") == p.day.Format("2006-01-02")
					chip := tagChip(c, pal, p.lbl, active)
					if chip.Clicked() {
						*val = p.day
					}
				}
			})
		}

		if *expanded {
			ui.Box(c).Padding(6).Radius(12).Background(pal.CardElevated).Border(1, t.Border).Children(func() {
				chineseDatePicker(c, key+"-wheel", val, minYear, maxYear)
			})
		}
	})
}

// dateTimePickerField renders a date + time picker row (YYYY-MM-DD HH:MM) with quick time presets and expandable wheels.
func dateTimePickerField(c *ui.Context, pal Palette, key, label string, val *time.Time, expanded *bool, modeIdx *int) {
	t := c.Theme()
	if val.IsZero() {
		*val = time.Now()
	}

	ui.Column(c.Key(key)).Gap(8).Children(func() {
		ui.Text(c, label).FontSize(13).TextColor(t.TextMuted)
		row := ui.Row(c).FillWidth().Padding(11, 14).Radius(t.Radius).Background(t.Surface).Border(1, t.Border).
			AlignItems(ui.Center).Role(ui.RoleButton).Label(label)
		row.Children(func() {
			ui.Text(c, val.Format("2006-01-02 15:04")).FontWeight(500).Grow(1).Basis(0)
			chev := "修改时间 ▾"
			if *expanded {
				chev = "收起 ▴"
			}
			ui.Text(c, chev).FontSize(12).TextColor(pal.Teal)
		})
		if row.Clicked() {
			*expanded = !*expanded
		}

		presets := []struct {
			lbl string
			d   time.Duration
		}{
			{"现在", 0},
			{"30分钟前", 30 * time.Minute},
			{"1小时前", time.Hour},
			{"2小时前", 2 * time.Hour},
		}
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, p := range presets {
				p := p
				chip := tagChip(c, pal, p.lbl, false)
				if chip.Clicked() {
					*val = time.Now().Add(-p.d)
				}
			}
		})

		if *expanded {
			ui.Column(c).Padding(8).Radius(12).Background(pal.CardElevated).Border(1, t.Border).Gap(6).Children(func() {
				ui.Segmented(c, modeIdx, "选择日期", "选择时间").FillEqually().Label("日期时间切换")
				nowY := time.Now().Year()
				if *modeIdx == 0 {
					chineseDatePicker(c, key+"-dwheel", val, nowY-2, nowY+1)
				} else {
					chineseTimePicker(c, key+"-twheel", val)
				}
			})
		}
	})
}

func tagChip(c *ui.Context, pal Palette, label string, active bool) ui.Element {
	t := c.Theme()
	chip := ui.Row(c.Key(label)).Padding(5, 10).Radius(14).Shrink(0).Role(ui.RoleButton).Label(label)
	if active {
		chip.Background(pal.Teal)
	} else {
		chip.Background(t.Surface).Border(1, t.Border)
	}
	chip.Children(func() {
		col := t.Text
		if active {
			col = t.AccentText
		}
		ui.Text(c, label).NoWrap().FontSize(12).TextColor(col)
	})
	return chip
}

// quickTagPicker displays wrapping quick-select tags above an input field.
// In multi mode, tags are joined/toggled using "、".
func quickTagPicker(c *ui.Context, pal Palette, key string, value *string, tags []string, multi bool) {
	if len(tags) == 0 {
		return
	}
	ui.Row(c.Key(key)).Wrap().Gap(6).Children(func() {
		for _, tag := range tags {
			tag := tag
			active := hasTag(*value, tag, multi)
			chip := tagChip(c, pal, tag, active)
			if chip.Clicked() {
				*value = toggleTag(*value, tag, multi)
			}
		}
	})
}

func splitTags(s string) []string {
	s = strings.ReplaceAll(s, "，", "、")
	s = strings.ReplaceAll(s, ",", "、")
	raw := strings.Split(s, "、")
	var out []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func hasTag(current, tag string, multi bool) bool {
	cur := strings.TrimSpace(current)
	if !multi {
		return cur == tag
	}
	for _, p := range splitTags(cur) {
		if p == tag {
			return true
		}
	}
	return false
}

func toggleTag(current, tag string, multi bool) string {
	cur := strings.TrimSpace(current)
	if !multi {
		if cur == tag {
			return ""
		}
		return tag
	}
	parts := splitTags(cur)
	filtered := make([]string, 0, len(parts)+1)
	removed := false
	for _, p := range parts {
		if p == tag {
			removed = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !removed {
		filtered = append(filtered, tag)
	}
	return strings.Join(filtered, "、")
}

func growthNoteTags(member model.Member, hasMember bool, at time.Time) []string {
	var tags []string
	if hasMember && member.IsChildAt(at) {
		months := int(member.AgeMonthsAt(at) + 0.3)
		switch {
		case months <= 1:
			tags = append(tags, "满月体检")
		case months == 6:
			tags = append(tags, "半岁体检")
		case months >= 12 && months%12 == 0:
			tags = append(tags, fmt.Sprintf("%d岁体检", months/12))
		case months > 0 && months < 36:
			tags = append(tags, fmt.Sprintf("%d月龄儿保", months))
		}
	}
	tags = append(tags, "居家自测", "社区儿保", "医院体检", "幼儿园体检", "晨起空腹", "穿衣称重")
	return tags
}

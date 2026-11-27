package app

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
	"github.com/ilinmu/vita/internal/syncer"
)

func TestGenerateScreenshots(t *testing.T) {
	if os.Getenv("VITA_GEN_SCREENSHOTS") != "1" {
		t.Skip("set VITA_GEN_SCREENSHOTS=1 to regenerate site screenshots")
	}

	outDir := filepath.Join("..", "..", "site", "screenshots")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	st, err := localstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	child, _ := st.SaveMember(model.Member{
		Name:      "小核桃",
		Gender:    model.GenderMale,
		BirthDate: now.AddDate(-2, -4, 0).Format("2006-01-02"),
		ColorHex:  "#0D9488",
	})
	_, _ = st.SaveMember(model.Member{
		Name:      "小柚子",
		Gender:    model.GenderFemale,
		BirthDate: now.AddDate(-1, -2, 0).Format("2006-01-02"),
		ColorHex:  "#E11D48",
	})
	_, _ = st.SaveMember(model.Member{
		Name:      "妈妈",
		Gender:    model.GenderFemale,
		BirthDate: "1993-08-18",
		ColorHex:  "#6366F1",
	})
	_, _ = st.SaveMember(model.Member{
		Name:      "爸爸",
		Gender:    model.GenderMale,
		BirthDate: "1991-04-12",
		ColorHex:  "#2563EB",
	})
	_ = st.UpdateConfig(func(c *localstore.Config) {
		c.ActiveMemberID = child.ID
		c.LANURL = "http://192.168.1.50:8420"
		c.PublicURL = "https://vita.example.com"
		c.AuthToken = "demo-token"
		c.UserID = "demo-user"
		c.Username = "lin"
		c.FamilyID = "demo-family"
		c.FamilyName = "我们家"
		c.InviteCode = "K8M4NP"
		c.LastSyncedAt = now.Add(-6 * time.Minute).UnixMilli()
	})

	// 6 realistic growth records from 3m to 28m tracking around P65 height & P60 weight
	birth, _ := child.ParseBirthDate()
	growthSamples := []struct {
		months int
		h, w   float64
		hc     float64
		note   string
	}{
		{3, 62.6, 6.85, 40.8, "3月龄儿保"},
		{6, 69.1, 8.60, 43.6, "半岁体检"},
		{12, 77.4, 10.35, 46.4, "一岁体检"},
		{18, 83.8, 11.75, 47.7, "1岁半儿保"},
		{24, 89.6, 13.05, 48.7, "两岁体检"},
		{28, 92.8, 13.85, 49.2, "居家测量"},
	}
	for _, s := range growthSamples {
		h, w, hc := s.h, s.w, s.hc
		_, _ = st.SaveGrowthRecord(model.GrowthRecord{
			MemberID:   child.ID,
			RecordedAt: birth.AddDate(0, s.months, 0).UnixMilli(),
			HeightCM:   &h,
			WeightKG:   &w,
			HeadCircCM: &hc,
			Note:       s.note,
		})
	}

	cm, _ := st.SaveCustomMetric(model.CustomMetric{Name: "脚长", Unit: "cm", ColorHex: "#F59E0B"})
	for _, f := range []struct {
		months int
		v      float64
	}{
		{18, 13.2}, {24, 14.3}, {28, 15.0},
	} {
		_, _ = st.SaveCustomMetricRecord(model.CustomMetricRecord{
			MemberID:   child.ID,
			MetricID:   cm.ID,
			RecordedAt: birth.AddDate(0, f.months, 0).UnixMilli(),
			Value:      f.v,
		})
	}

	// Active fever episode across last 18h
	ep, _ := st.SaveFeverEpisode(model.FeverEpisode{
		MemberID:  child.ID,
		Title:     "秋季感冒",
		StartedAt: now.Add(-18 * time.Hour).UnixMilli(),
	})
	tempSamples := []struct {
		ago     time.Duration
		temp    float64
		site    model.TempSite
		symptom string
	}{
		{18 * time.Hour, 37.9, model.TempSiteEar, "轻微流涕"},
		{14 * time.Hour, 38.7, model.TempSiteEar, "手脚偏凉"},
		{11 * time.Hour, 39.1, model.TempSiteEar, "精神略倦"},
		{7 * time.Hour, 37.8, model.TempSiteEar, "出汗退热中"},
		{45 * time.Minute, 37.3, model.TempSiteEar, "精神恢复"},
	}
	for _, ts := range tempSamples {
		_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{
			MemberID:   child.ID,
			EpisodeID:  ep.ID,
			RecordedAt: now.Add(-ts.ago).UnixMilli(),
			TempC:      ts.temp,
			Site:       ts.site,
			Symptom:    ts.symptom,
		})
	}
	_, _ = st.SaveMedicationRecord(model.MedicationRecord{
		MemberID:   child.ID,
		EpisodeID:  ep.ID,
		RecordedAt: now.Add(-10*time.Hour - 40*time.Minute).UnixMilli(),
		MedType:    model.MedAntipyretic,
		Name:       "美林",
		Dosage:     "4 ml",
	})
	_, _ = st.SaveMedicationRecord(model.MedicationRecord{
		MemberID:   child.ID,
		EpisodeID:  ep.ID,
		RecordedAt: now.Add(-6 * time.Hour).UnixMilli(),
		MedType:    model.MedPhysical,
		Name:       "温水擦浴",
	})

	a := New(st, syncer.NewClient("http://192.168.1.50:8420", "https://vita.example.com"))

	save := func(filename string, look mobile.Look, w, h int, dark bool, tab, growthMetric int) {
		mobile.SetLook(look)
		mode := "light"
		if dark {
			mode = "dark"
		}
		_ = st.UpdateConfig(func(c *localstore.Config) { c.ThemeMode = mode })
		a.Tab = tab
		a.GrowthMetricIdx = growthMetric
		a.GrowthSelected = -1
		a.FeverSelected = -1
		a.FeverEpisodeID = ep.ID
		a.FeverRangeIdx = 0

		tt := ui.NewTesterWith(a.View, w, h, ui.TesterOptions{Dark: dark})
		tt.SetScale(2)
		tt.Frame()
		tt.Advance(250 * time.Millisecond)
		tt.Frame()

		var buf bytes.Buffer
		if err := png.Encode(&buf, tt.Image()); err != nil {
			t.Fatalf("encode %s: %v", filename, err)
		}
		p := filepath.Join(outDir, filename)
		if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
		t.Logf("wrote %s (%dx%d @2x)", p, w, h)
	}

	// iOS (Cupertino)
	save("ios-overview.png", mobile.Cupertino, 390, 844, false, 0, 0)
	save("ios-growth.png", mobile.Cupertino, 390, 844, false, 1, 0)
	save("ios-fever-dark.png", mobile.Cupertino, 390, 844, true, 2, 0)

	// Android (Material)
	save("android-growth.png", mobile.Material, 393, 852, false, 1, 1)
	save("android-fever.png", mobile.Material, 393, 852, false, 2, 0)
	save("android-family-dark.png", mobile.Material, 393, 852, true, 3, 0)

	// macOS Desktop
	save("macos-growth.png", mobile.Cupertino, 820, 620, false, 1, 0)
	save("macos-fever-dark.png", mobile.Cupertino, 820, 620, true, 2, 0)

	mobile.SetLook(mobile.Automatic)
}

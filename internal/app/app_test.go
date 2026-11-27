package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
	"github.com/ilinmu/vita/internal/syncer"
)

func TestAllTabsRenderLightAndDark(t *testing.T) {
	st, err := localstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := New(st, syncer.NewClient("", ""))

	// 1. Empty onboarding state
	tt := ui.NewTester(a.View, 390, 844)
	tt.Frame()

	// 2. Populate boy + girl + adult + custom metric + growth + fever + medication
	boy, _ := st.SaveMember(model.Member{
		Name:      "小核桃",
		Gender:    model.GenderMale,
		BirthDate: time.Now().AddDate(-2, -3, 0).Format("2006-01-02"),
		ColorHex:  "#0D9488",
	})
	girl, _ := st.SaveMember(model.Member{
		Name:      "小柚子",
		Gender:    model.GenderFemale,
		BirthDate: time.Now().AddDate(-1, -1, 0).Format("2006-01-02"),
		ColorHex:  "#E11D48",
	})
	adult, _ := st.SaveMember(model.Member{
		Name:      "爸爸",
		Gender:    model.GenderMale,
		BirthDate: "1992-05-10",
		ColorHex:  "#2563EB",
	})
	h1, w1, hc1 := 80.5, 10.8, 46.8
	h2, w2, hc2 := 89.2, 12.9, 48.5
	_, _ = st.SaveGrowthRecord(model.GrowthRecord{
		MemberID: boy.ID, RecordedAt: time.Now().AddDate(0, -6, 0).UnixMilli(),
		HeightCM: &h1, WeightKG: &w1, HeadCircCM: &hc1,
	})
	_, _ = st.SaveGrowthRecord(model.GrowthRecord{
		MemberID: boy.ID, RecordedAt: time.Now().UnixMilli(),
		HeightCM: &h2, WeightKG: &w2, HeadCircCM: &hc2,
	})
	cm, _ := st.SaveCustomMetric(model.CustomMetric{Name: "脚长", Unit: "cm"})
	_, _ = st.SaveCustomMetricRecord(model.CustomMetricRecord{
		MemberID: boy.ID, MetricID: cm.ID, Value: 14.5,
	})

	ep, _ := st.SaveFeverEpisode(model.FeverEpisode{MemberID: boy.ID, Title: "秋季感冒"})
	_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{
		MemberID: boy.ID, EpisodeID: ep.ID,
		RecordedAt: time.Now().Add(-3 * time.Hour).UnixMilli(),
		TempC:      38.9, Site: model.TempSiteEar, Symptom: "精神尚可",
	})
	_, _ = st.SaveMedicationRecord(model.MedicationRecord{
		MemberID: boy.ID, EpisodeID: ep.ID,
		RecordedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
		MedType:    model.MedAntipyretic, Name: "美林", Dosage: "4 ml",
	})
	_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{
		MemberID: boy.ID, EpisodeID: ep.ID,
		RecordedAt: time.Now().Add(-20 * time.Minute).UnixMilli(),
		TempC:      37.4, Site: model.TempSiteEar,
	})

	// Render all 4 tabs in light & dark mode
	for _, themeMode := range []string{"light", "dark"} {
		_ = st.UpdateConfig(func(c *localstore.Config) { c.ThemeMode = themeMode })
		for tab := 0; tab < 4; tab++ {
			a.Tab = tab
			tt.Frame()
		}
	}

	// Render girl member (dynamic rose accent + girl avatar)
	_ = st.UpdateConfig(func(c *localstore.Config) { c.ActiveMemberID = girl.ID })
	a.Tab = 0
	tt.Frame()

	// Render open forms with expanded date/time pickers
	openMemberFormAndRender := func() {
		a.openNewMemberForm()
		a.MemberBirthExpanded = true
		tt.Frame()
		a.ShowMemberForm = false
	}
	openMemberFormAndRender()

	a.openGrowthForm()
	a.GrowthDateExpanded = true
	tt.Frame()
	a.ShowGrowthForm = false

	a.openTempForm()
	a.TempTimeExpanded = true
	a.TempTimeModeIdx = 0
	tt.Frame()
	a.TempTimeModeIdx = 1
	tt.Frame()
	a.ShowTempForm = false

	a.openMedForm()
	a.MedTimeExpanded = true
	a.MedTypeIdx = 0
	tt.Frame()
	a.MedTypeIdx = 1
	tt.Frame()
	a.ShowMedForm = false

	// Render growth tab on adult member & custom metric
	_ = st.UpdateConfig(func(c *localstore.Config) { c.ActiveMemberID = adult.ID })
	a.Tab = 1
	a.GrowthMetricIdx = 1
	tt.Frame()
	a.GrowthMetricIdx = 4
	tt.Frame()
}

func TestTagTogglingAndDynamicMemberTheme(t *testing.T) {
	// Single-select mode
	v := toggleTag("", "美林", false)
	if v != "美林" {
		t.Fatalf("expected 美林, got %q", v)
	}
	v = toggleTag(v, "泰诺林", false)
	if v != "泰诺林" {
		t.Fatalf("expected 泰诺林, got %q", v)
	}
	v = toggleTag(v, "泰诺林", false)
	if v != "" {
		t.Fatalf("expected empty after untoggle, got %q", v)
	}

	// Multi-select mode
	m := toggleTag("", "精神尚可", true)
	m = toggleTag(m, "流涕鼻塞", true)
	if m != "精神尚可、流涕鼻塞" {
		t.Fatalf("unexpected multi tags: %q", m)
	}
	m = toggleTag(m, "精神尚可", true)
	if m != "流涕鼻塞" {
		t.Fatalf("unexpected multi tags after removal: %q", m)
	}

	// Dynamic member color resolution
	st, err := localstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := New(st, syncer.NewClient("", ""))
	var gotAccent ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		a.View(c)
		gotAccent = c.Theme().Accent
	}, 390, 844)
	tt.Frame()

	rose, _ := st.SaveMember(model.Member{
		Name: "小柚子", Gender: model.GenderFemale, BirthDate: "2025-01-01", ColorHex: "#E11D48",
	})
	_ = st.UpdateConfig(func(c *localstore.Config) { c.ActiveMemberID = rose.ID })
	tt.Frame()
	if gotAccent != ui.Hex("#E11D48") {
		t.Fatalf("expected member color #E11D48 as accent, got %+v", gotAccent)
	}
}

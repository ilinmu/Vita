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

	// 2. Populate child + adult + custom metric + growth + fever + medication
	child, _ := st.SaveMember(model.Member{
		Name:      "小核桃",
		Gender:    model.GenderMale,
		BirthDate: time.Now().AddDate(-2, -3, 0).Format("2006-01-02"),
	})
	adult, _ := st.SaveMember(model.Member{
		Name:      "爸爸",
		Gender:    model.GenderMale,
		BirthDate: "1992-05-10",
	})
	h1, w1, hc1 := 80.5, 10.8, 46.8
	h2, w2, hc2 := 89.2, 12.9, 48.5
	_, _ = st.SaveGrowthRecord(model.GrowthRecord{
		MemberID: child.ID, RecordedAt: time.Now().AddDate(0, -6, 0).UnixMilli(),
		HeightCM: &h1, WeightKG: &w1, HeadCircCM: &hc1,
	})
	_, _ = st.SaveGrowthRecord(model.GrowthRecord{
		MemberID: child.ID, RecordedAt: time.Now().UnixMilli(),
		HeightCM: &h2, WeightKG: &w2, HeadCircCM: &hc2,
	})
	cm, _ := st.SaveCustomMetric(model.CustomMetric{Name: "脚长", Unit: "cm"})
	_, _ = st.SaveCustomMetricRecord(model.CustomMetricRecord{
		MemberID: child.ID, MetricID: cm.ID, Value: 14.5,
	})

	ep, _ := st.SaveFeverEpisode(model.FeverEpisode{MemberID: child.ID, Title: "秋季感冒"})
	_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{
		MemberID: child.ID, EpisodeID: ep.ID,
		RecordedAt: time.Now().Add(-3 * time.Hour).UnixMilli(),
		TempC:      38.9, Site: model.TempSiteEar, Symptom: "精神一般",
	})
	_, _ = st.SaveMedicationRecord(model.MedicationRecord{
		MemberID: child.ID, EpisodeID: ep.ID,
		RecordedAt: time.Now().Add(-2 * time.Hour).UnixMilli(),
		MedType:    model.MedIbuprofen, Name: "美林", Dosage: "4ml",
	})
	_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{
		MemberID: child.ID, EpisodeID: ep.ID,
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

	// Render growth tab on adult member & custom metric
	_ = st.UpdateConfig(func(c *localstore.Config) { c.ActiveMemberID = adult.ID })
	a.Tab = 1
	a.GrowthMetricIdx = 1
	tt.Frame()
	a.GrowthMetricIdx = 4
	tt.Frame()
}

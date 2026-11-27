package export

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/ilinmu/vita/internal/growth"
	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

func TestExportJSONCSVAndPNG(t *testing.T) {
	st, err := localstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m, _ := st.SaveMember(model.Member{Name: "小核桃", Gender: model.GenderMale, BirthDate: "2024-01-01"})
	h := 85.0
	w := 12.1
	_, _ = st.SaveGrowthRecord(model.GrowthRecord{MemberID: m.ID, HeightCM: &h, WeightKG: &w})
	_, _ = st.SaveTemperatureRecord(model.TemperatureRecord{MemberID: m.ID, TempC: 38.8, Site: model.TempSiteEar})

	snap := st.Snapshot()
	j, err := BackupJSON(snap)
	if err != nil || !strings.Contains(string(j), "小核桃") {
		t.Fatalf("BackupJSON failed: %v", err)
	}
	c, err := BackupCSV(snap)
	if err != nil || !bytes.HasPrefix(c, []byte{0xEF, 0xBB, 0xBF}) || !strings.Contains(string(c), "38.8") {
		t.Fatalf("BackupCSV failed: %v", err)
	}

	gpng, err := RenderGrowthChartPNG("Height", "cm", growth.StandardChina, model.GenderMale, growth.MetricHeight, true, []GrowthPoint{
		{AgeMonths: 12, Value: 76.5},
		{AgeMonths: 24, Value: 88.5},
	})
	if err != nil {
		t.Fatalf("RenderGrowthChartPNG: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(gpng))
	if err != nil || cfg.Width != 1200 || cfg.Height != 720 {
		t.Fatalf("unexpected PNG config: %+v err=%v", cfg, err)
	}

	now := time.Now()
	fpng, err := RenderFeverChartPNG("Fever", []model.TemperatureRecord{
		{RecordedAt: now.Add(-4 * time.Hour).UnixMilli(), TempC: 38.9},
		{RecordedAt: now.UnixMilli(), TempC: 37.5},
	}, []model.MedicationRecord{
		{RecordedAt: now.Add(-3 * time.Hour).UnixMilli(), MedType: model.MedIbuprofen, Name: "美林"},
	})
	if err != nil {
		t.Fatalf("RenderFeverChartPNG: %v", err)
	}
	fcfg, err := png.DecodeConfig(bytes.NewReader(fpng))
	if err != nil || fcfg.Width != 1200 || fcfg.Height != 720 {
		t.Fatalf("unexpected fever PNG config: %+v err=%v", fcfg, err)
	}
}

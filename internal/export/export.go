// Package export produces JSON backups, UTF-8 BOM CSV spreadsheets, and
// pure-Go offscreen PNG chart images for sharing or medical visits.
package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
)

// BackupJSON serializes a full non-sensitive snapshot of records for backup.
func BackupJSON(snap localstore.Snapshot) ([]byte, error) {
	// Strip auth token from exported backup for safety
	snap.Config.AuthToken = ""
	return json.MarshalIndent(snap, "", "  ")
}

// BackupCSV exports growth, custom metric, temperature, and medication records as a UTF-8 BOM CSV.
func BackupCSV(snap localstore.Snapshot) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM so Excel/Numbers opens Chinese characters cleanly
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"成员", "记录类型", "时间", "核心数值", "辅助信息", "备注"}); err != nil {
		return nil, err
	}

	memberName := func(id string) string {
		if m, ok := snap.Members[id]; ok {
			return m.Name
		}
		return id
	}

	for _, r := range snap.GrowthRecords {
		if r.DeletedAt != nil {
			continue
		}
		var parts string
		if r.HeightCM != nil {
			parts += fmt.Sprintf("身高 %.1fcm ", *r.HeightCM)
		}
		if r.WeightKG != nil {
			parts += fmt.Sprintf("体重 %.2fkg ", *r.WeightKG)
		}
		if r.HeadCircCM != nil {
			parts += fmt.Sprintf("头围 %.1fcm ", *r.HeadCircCM)
		}
		if bmi := r.BMI(); bmi != nil {
			parts += fmt.Sprintf("BMI %.1f", *bmi)
		}
		_ = w.Write([]string{
			memberName(r.MemberID),
			"生长发育",
			time.UnixMilli(r.RecordedAt).Format("2006-01-02 15:04"),
			parts,
			"",
			r.Note,
		})
	}

	for _, r := range snap.CustomRecords {
		if r.DeletedAt != nil {
			continue
		}
		m := snap.CustomMetrics[r.MetricID]
		_ = w.Write([]string{
			memberName(r.MemberID),
			"自定义指标:" + m.Name,
			time.UnixMilli(r.RecordedAt).Format("2006-01-02 15:04"),
			fmt.Sprintf("%.2f %s", r.Value, m.Unit),
			"",
			r.Note,
		})
	}

	for _, r := range snap.TempRecords {
		if r.DeletedAt != nil {
			continue
		}
		_ = w.Write([]string{
			memberName(r.MemberID),
			"体温",
			time.UnixMilli(r.RecordedAt).Format("2006-01-02 15:04"),
			fmt.Sprintf("%.1f ℃", r.TempC),
			model.SiteLabel(r.Site) + " " + r.Symptom,
			r.Note,
		})
	}

	for _, r := range snap.MedicationRecords {
		if r.DeletedAt != nil {
			continue
		}
		_ = w.Write([]string{
			memberName(r.MemberID),
			"退烧/降温",
			time.UnixMilli(r.RecordedAt).Format("2006-01-02 15:04"),
			model.MedTypeLabel(r.MedType) + " " + r.Name,
			r.Dosage,
			r.Note,
		})
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

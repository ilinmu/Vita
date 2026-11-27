package localstore

import (
	"testing"
	"time"

	"github.com/ilinmu/vita/internal/model"
)

func floatPtr(v float64) *float64 { return &v }

func TestStorePersistenceAndMerge(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	m, err := st.SaveMember(model.Member{
		Name:      "小核桃",
		Gender:    model.GenderMale,
		BirthDate: "2024-06-01",
	})
	if err != nil {
		t.Fatalf("SaveMember: %v", err)
	}

	gr, err := st.SaveGrowthRecord(model.GrowthRecord{
		MemberID:   m.ID,
		RecordedAt: time.Now().Add(-24 * time.Hour).UnixMilli(),
		HeightCM:   floatPtr(88.5),
		WeightKG:   floatPtr(12.6),
	})
	if err != nil {
		t.Fatalf("SaveGrowthRecord: %v", err)
	}
	if st.PendingCount() != 2 {
		t.Fatalf("expected 2 pending ops, got %d", st.PendingCount())
	}

	// Reopen from disk
	st2, err := Open(dir)
	if err != nil {
		t.Fatalf("Reopen failed: %v", err)
	}
	ms := st2.Members()
	if len(ms) != 1 || ms[0].Name != "小核桃" {
		t.Fatalf("unexpected reopened members: %+v", ms)
	}
	gs := st2.GrowthRecords(m.ID)
	if len(gs) != 1 || *gs[0].HeightCM != 88.5 {
		t.Fatalf("unexpected reopened growth records: %+v", gs)
	}

	// Delete tombstone
	if err := st2.DeleteGrowthRecord(gr.ID); err != nil {
		t.Fatalf("DeleteGrowthRecord: %v", err)
	}
	if len(st2.GrowthRecords(m.ID)) != 0 {
		t.Fatal("expected deleted growth record to be excluded from active queries")
	}

	// Acknowledge sync
	_, ops := st2.SyncInput()
	if err := st2.ApplyRemoteChanges(ops, 10, nil); err != nil {
		t.Fatalf("ApplyRemoteChanges: %v", err)
	}
	if st2.PendingCount() != 0 {
		t.Fatalf("expected 0 pending ops after ack, got %d", st2.PendingCount())
	}
}

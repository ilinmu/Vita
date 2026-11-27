// Package localstore provides a crash-safe, zero-CGO offline-first local store
// persisted atomically as JSON, tracking pending incremental sync operations.
package localstore

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ilinmu/vita/internal/model"
)

const stateFilename = "vita_state.json"

// Config stores client settings, endpoints, and cached auth metadata.
type Config struct {
	LANURL         string         `json:"lan_url"`
	PublicURL      string         `json:"public_url"`
	AuthToken      string         `json:"auth_token"`
	UserID         string         `json:"user_id"`
	Username       string         `json:"username"`
	FamilyID       string         `json:"family_id"`
	FamilyName     string         `json:"family_name"`
	InviteCode     string         `json:"invite_code"`
	ThemeMode      string         `json:"theme_mode"` // "system" | "light" | "dark"
	LastTempSite   model.TempSite `json:"last_temp_site"`
	LastStandard   string         `json:"last_standard"` // "china" | "who"
	ActiveMemberID string         `json:"active_member_id"`
	LastSyncedAt   int64          `json:"last_synced_at"`
}

// Snapshot is the serializable on-disk state and export payload.
type Snapshot struct {
	Config            Config                              `json:"config"`
	LastSeq           int64                               `json:"last_seq"`
	PendingOps        map[string]model.SyncChange         `json:"pending_ops"`
	Members           map[string]model.Member             `json:"members"`
	GrowthRecords     map[string]model.GrowthRecord       `json:"growth_records"`
	CustomMetrics     map[string]model.CustomMetric       `json:"custom_metrics"`
	CustomRecords     map[string]model.CustomMetricRecord `json:"custom_metric_records"`
	FeverEpisodes     map[string]model.FeverEpisode       `json:"fever_episodes"`
	TempRecords       map[string]model.TemperatureRecord  `json:"temperature_records"`
	MedicationRecords map[string]model.MedicationRecord   `json:"medication_records"`
}

// Store manages local state and pending sync mutations with thread safety.
type Store struct {
	mu   sync.RWMutex
	dir  string
	path string
	data Snapshot
}

func emptySnapshot() Snapshot {
	return Snapshot{
		Config: Config{
			ThemeMode:    "system",
			LastTempSite: model.TempSiteEar,
			LastStandard: "china",
		},
		PendingOps:        map[string]model.SyncChange{},
		Members:           map[string]model.Member{},
		GrowthRecords:     map[string]model.GrowthRecord{},
		CustomMetrics:     map[string]model.CustomMetric{},
		CustomRecords:     map[string]model.CustomMetricRecord{},
		FeverEpisodes:     map[string]model.FeverEpisode{},
		TempRecords:       map[string]model.TemperatureRecord{},
		MedicationRecords: map[string]model.MedicationRecord{},
	}
}

// Open loads an existing local store from dir or initializes a new one.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		dir:  dir,
		path: filepath.Join(dir, stateFilename),
		data: emptySnapshot(),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Try backup recovery if primary missing
			if bak, berr := os.ReadFile(s.path + ".bak"); berr == nil {
				raw = bak
			} else {
				return s.saveLocked()
			}
		} else {
			return err
		}
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		// Fall back to backup if primary is corrupted
		bak, berr := os.ReadFile(s.path + ".bak")
		if berr != nil {
			return err
		}
		if uerr := json.Unmarshal(bak, &snap); uerr != nil {
			return err
		}
	}
	s.normalize(&snap)
	s.data = snap
	return nil
}

func (s *Store) normalize(snap *Snapshot) {
	if snap.PendingOps == nil {
		snap.PendingOps = map[string]model.SyncChange{}
	}
	if snap.Members == nil {
		snap.Members = map[string]model.Member{}
	}
	if snap.GrowthRecords == nil {
		snap.GrowthRecords = map[string]model.GrowthRecord{}
	}
	if snap.CustomMetrics == nil {
		snap.CustomMetrics = map[string]model.CustomMetric{}
	}
	if snap.CustomRecords == nil {
		snap.CustomRecords = map[string]model.CustomMetricRecord{}
	}
	if snap.FeverEpisodes == nil {
		snap.FeverEpisodes = map[string]model.FeverEpisode{}
	}
	if snap.TempRecords == nil {
		snap.TempRecords = map[string]model.TemperatureRecord{}
	}
	if snap.MedicationRecords == nil {
		snap.MedicationRecords = map[string]model.MedicationRecord{}
	}
	if snap.Config.ThemeMode == "" {
		snap.Config.ThemeMode = "system"
	}
	if snap.Config.LastTempSite == "" {
		snap.Config.LastTempSite = model.TempSiteEar
	}
	if snap.Config.LastStandard == "" {
		snap.Config.LastStandard = "china"
	}
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(&s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if prev, err := os.ReadFile(s.path); err == nil {
		_ = os.WriteFile(s.path+".bak", prev, 0o600)
	}
	return os.Rename(tmp, s.path)
}

func opKey(et model.EntityType, id string) string {
	return string(et) + ":" + id
}

func (s *Store) queueOpLocked(et model.EntityType, id string, updatedAt int64, deletedAt *int64, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	s.data.PendingOps[opKey(et, id)] = model.SyncChange{
		EntityType: et,
		EntityID:   id,
		UpdatedAt:  updatedAt,
		DeletedAt:  deletedAt,
		Payload:    raw,
	}
	return s.saveLocked()
}

// Dir returns the storage directory path.
func (s *Store) Dir() string {
	return s.dir
}

// Snapshot returns a copy of the store snapshot.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, _ := json.Marshal(s.data)
	var out Snapshot
	_ = json.Unmarshal(raw, &out)
	s.normalize(&out)
	return out
}

// Config returns a copy of current client configuration.
func (s *Store) Config() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Config
}

// UpdateConfig atomically updates config via fn and persists.
func (s *Store) UpdateConfig(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data.Config)
	return s.saveLocked()
}

// PendingCount returns the number of unsynced mutations.
func (s *Store) PendingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data.PendingOps)
}

// SyncInput returns sinceSeq and a snapshot of pending ops for upload.
func (s *Store) SyncInput() (int64, []model.SyncChange) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ops := make([]model.SyncChange, 0, len(s.data.PendingOps))
	for _, op := range s.data.PendingOps {
		ops = append(ops, op)
	}
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].UpdatedAt < ops[j].UpdatedAt
	})
	return s.data.LastSeq, ops
}

// ClearDataOnLogout resets auth & family tokens while optionally keeping local records or clearing them.
// For clean multi-account switching, SwitchFamilyReset clears server cursor and re-queues or resets state.
func (s *Store) ResetAuth(clearRecords bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lan, pub, theme := s.data.Config.LANURL, s.data.Config.PublicURL, s.data.Config.ThemeMode
	if clearRecords {
		s.data = emptySnapshot()
	} else {
		s.data.Config.AuthToken = ""
		s.data.Config.UserID = ""
		s.data.Config.Username = ""
		s.data.Config.FamilyID = ""
		s.data.Config.FamilyName = ""
		s.data.Config.InviteCode = ""
		s.data.LastSeq = 0
	}
	s.data.Config.LANURL = lan
	s.data.Config.PublicURL = pub
	s.data.Config.ThemeMode = theme
	return s.saveLocked()
}

// AssignFamilyToLocalRecords binds un-assigned local entities to familyID and queues them for sync.
func (s *Store) AssignFamilyToLocalRecords(familyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Config.FamilyID = familyID
	now := model.NowMS()
	for id, m := range s.data.Members {
		if m.FamilyID == "" {
			m.FamilyID = familyID
			m.UpdatedAt = now
			s.data.Members[id] = m
			_ = s.queueOpLocked(model.EntityMember, id, m.UpdatedAt, m.DeletedAt, m)
		}
	}
	for id, r := range s.data.GrowthRecords {
		if r.FamilyID == "" {
			r.FamilyID = familyID
			r.UpdatedAt = now
			s.data.GrowthRecords[id] = r
			_ = s.queueOpLocked(model.EntityGrowthRecord, id, r.UpdatedAt, r.DeletedAt, r)
		}
	}
	for id, m := range s.data.CustomMetrics {
		if m.FamilyID == "" {
			m.FamilyID = familyID
			m.UpdatedAt = now
			s.data.CustomMetrics[id] = m
			_ = s.queueOpLocked(model.EntityCustomMetric, id, m.UpdatedAt, m.DeletedAt, m)
		}
	}
	for id, r := range s.data.CustomRecords {
		if r.FamilyID == "" {
			r.FamilyID = familyID
			r.UpdatedAt = now
			s.data.CustomRecords[id] = r
			_ = s.queueOpLocked(model.EntityCustomMetricRecord, id, r.UpdatedAt, r.DeletedAt, r)
		}
	}
	for id, e := range s.data.FeverEpisodes {
		if e.FamilyID == "" {
			e.FamilyID = familyID
			e.UpdatedAt = now
			s.data.FeverEpisodes[id] = e
			_ = s.queueOpLocked(model.EntityFeverEpisode, id, e.UpdatedAt, e.DeletedAt, e)
		}
	}
	for id, r := range s.data.TempRecords {
		if r.FamilyID == "" {
			r.FamilyID = familyID
			r.UpdatedAt = now
			s.data.TempRecords[id] = r
			_ = s.queueOpLocked(model.EntityTemperatureRecord, id, r.UpdatedAt, r.DeletedAt, r)
		}
	}
	for id, r := range s.data.MedicationRecords {
		if r.FamilyID == "" {
			r.FamilyID = familyID
			r.UpdatedAt = now
			s.data.MedicationRecords[id] = r
			_ = s.queueOpLocked(model.EntityMedicationRecord, id, r.UpdatedAt, r.DeletedAt, r)
		}
	}
	return s.saveLocked()
}

// --- CRUD Mutations ---

func (s *Store) SaveMember(m model.Member) (model.Member, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = model.NewUUID()
	}
	if m.FamilyID == "" {
		m.FamilyID = s.data.Config.FamilyID
	}
	if m.ColorHex == "" {
		m.ColorHex = "#0D9488"
	}
	m.UpdatedAt = model.NowMS()
	s.data.Members[m.ID] = m
	if s.data.Config.ActiveMemberID == "" {
		s.data.Config.ActiveMemberID = m.ID
	}
	return m, s.queueOpLocked(model.EntityMember, m.ID, m.UpdatedAt, m.DeletedAt, m)
}

func (s *Store) DeleteMember(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.data.Members[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	m.DeletedAt = &now
	m.UpdatedAt = now
	s.data.Members[id] = m
	if err := s.queueOpLocked(model.EntityMember, id, m.UpdatedAt, m.DeletedAt, m); err != nil {
		return err
	}
	if s.data.Config.ActiveMemberID == id {
		s.data.Config.ActiveMemberID = ""
		for _, cand := range s.data.Members {
			if cand.DeletedAt == nil {
				s.data.Config.ActiveMemberID = cand.ID
				break
			}
		}
		return s.saveLocked()
	}
	return nil
}

func (s *Store) SaveGrowthRecord(r model.GrowthRecord) (model.GrowthRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = model.NewUUID()
	}
	if r.FamilyID == "" {
		r.FamilyID = s.data.Config.FamilyID
	}
	if r.RecordedAt == 0 {
		r.RecordedAt = model.NowMS()
	}
	r.UpdatedAt = model.NowMS()
	s.data.GrowthRecords[r.ID] = r
	return r, s.queueOpLocked(model.EntityGrowthRecord, r.ID, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) DeleteGrowthRecord(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.GrowthRecords[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	r.DeletedAt = &now
	r.UpdatedAt = now
	s.data.GrowthRecords[id] = r
	return s.queueOpLocked(model.EntityGrowthRecord, id, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) SaveCustomMetric(m model.CustomMetric) (model.CustomMetric, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.ID == "" {
		m.ID = model.NewUUID()
	}
	if m.FamilyID == "" {
		m.FamilyID = s.data.Config.FamilyID
	}
	if m.ColorHex == "" {
		m.ColorHex = "#6366F1"
	}
	m.UpdatedAt = model.NowMS()
	s.data.CustomMetrics[m.ID] = m
	return m, s.queueOpLocked(model.EntityCustomMetric, m.ID, m.UpdatedAt, m.DeletedAt, m)
}

func (s *Store) DeleteCustomMetric(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.data.CustomMetrics[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	m.DeletedAt = &now
	m.UpdatedAt = now
	s.data.CustomMetrics[id] = m
	return s.queueOpLocked(model.EntityCustomMetric, id, m.UpdatedAt, m.DeletedAt, m)
}

func (s *Store) SaveCustomMetricRecord(r model.CustomMetricRecord) (model.CustomMetricRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = model.NewUUID()
	}
	if r.FamilyID == "" {
		r.FamilyID = s.data.Config.FamilyID
	}
	if r.RecordedAt == 0 {
		r.RecordedAt = model.NowMS()
	}
	r.UpdatedAt = model.NowMS()
	s.data.CustomRecords[r.ID] = r
	return r, s.queueOpLocked(model.EntityCustomMetricRecord, r.ID, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) DeleteCustomMetricRecord(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.CustomRecords[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	r.DeletedAt = &now
	r.UpdatedAt = now
	s.data.CustomRecords[id] = r
	return s.queueOpLocked(model.EntityCustomMetricRecord, id, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) SaveFeverEpisode(ep model.FeverEpisode) (model.FeverEpisode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ep.ID == "" {
		ep.ID = model.NewUUID()
	}
	if ep.FamilyID == "" {
		ep.FamilyID = s.data.Config.FamilyID
	}
	if ep.StartedAt == 0 {
		ep.StartedAt = model.NowMS()
	}
	ep.UpdatedAt = model.NowMS()
	s.data.FeverEpisodes[ep.ID] = ep
	return ep, s.queueOpLocked(model.EntityFeverEpisode, ep.ID, ep.UpdatedAt, ep.DeletedAt, ep)
}

func (s *Store) DeleteFeverEpisode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep, ok := s.data.FeverEpisodes[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	ep.DeletedAt = &now
	ep.UpdatedAt = now
	s.data.FeverEpisodes[id] = ep
	return s.queueOpLocked(model.EntityFeverEpisode, id, ep.UpdatedAt, ep.DeletedAt, ep)
}

func (s *Store) SaveTemperatureRecord(r model.TemperatureRecord) (model.TemperatureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = model.NewUUID()
	}
	if r.FamilyID == "" {
		r.FamilyID = s.data.Config.FamilyID
	}
	if r.RecordedAt == 0 {
		r.RecordedAt = model.NowMS()
	}
	if r.Site == "" {
		r.Site = s.data.Config.LastTempSite
	} else {
		s.data.Config.LastTempSite = r.Site
	}
	r.UpdatedAt = model.NowMS()
	s.data.TempRecords[r.ID] = r
	return r, s.queueOpLocked(model.EntityTemperatureRecord, r.ID, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) DeleteTemperatureRecord(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.TempRecords[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	r.DeletedAt = &now
	r.UpdatedAt = now
	s.data.TempRecords[id] = r
	return s.queueOpLocked(model.EntityTemperatureRecord, id, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) SaveMedicationRecord(r model.MedicationRecord) (model.MedicationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = model.NewUUID()
	}
	if r.FamilyID == "" {
		r.FamilyID = s.data.Config.FamilyID
	}
	if r.RecordedAt == 0 {
		r.RecordedAt = model.NowMS()
	}
	r.UpdatedAt = model.NowMS()
	s.data.MedicationRecords[r.ID] = r
	return r, s.queueOpLocked(model.EntityMedicationRecord, r.ID, r.UpdatedAt, r.DeletedAt, r)
}

func (s *Store) DeleteMedicationRecord(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.MedicationRecords[id]
	if !ok {
		return nil
	}
	now := model.NowMS()
	r.DeletedAt = &now
	r.UpdatedAt = now
	s.data.MedicationRecords[id] = r
	return s.queueOpLocked(model.EntityMedicationRecord, id, r.UpdatedAt, r.DeletedAt, r)
}

// ApplyRemoteChanges acknowledges pushed ops up to ackedOps and merges server changes via LWW.
func (s *Store) ApplyRemoteChanges(pushed []model.SyncChange, serverSeq int64, remote []model.SyncChange) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, op := range pushed {
		k := opKey(op.EntityType, op.EntityID)
		if cur, ok := s.data.PendingOps[k]; ok && cur.UpdatedAt <= op.UpdatedAt {
			delete(s.data.PendingOps, k)
		}
	}

	for _, ch := range remote {
		k := opKey(ch.EntityType, ch.EntityID)
		if pend, ok := s.data.PendingOps[k]; ok && pend.UpdatedAt > ch.UpdatedAt {
			// Local un-pushed mutation is newer than server change
			continue
		}
		switch ch.EntityType {
		case model.EntityMember:
			var v model.Member
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.Members[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.Members[v.ID] = v
				}
			}
		case model.EntityGrowthRecord:
			var v model.GrowthRecord
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.GrowthRecords[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.GrowthRecords[v.ID] = v
				}
			}
		case model.EntityCustomMetric:
			var v model.CustomMetric
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.CustomMetrics[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.CustomMetrics[v.ID] = v
				}
			}
		case model.EntityCustomMetricRecord:
			var v model.CustomMetricRecord
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.CustomRecords[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.CustomRecords[v.ID] = v
				}
			}
		case model.EntityFeverEpisode:
			var v model.FeverEpisode
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.FeverEpisodes[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.FeverEpisodes[v.ID] = v
				}
			}
		case model.EntityTemperatureRecord:
			var v model.TemperatureRecord
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.TempRecords[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.TempRecords[v.ID] = v
				}
			}
		case model.EntityMedicationRecord:
			var v model.MedicationRecord
			if json.Unmarshal(ch.Payload, &v) == nil {
				if cur, ok := s.data.MedicationRecords[v.ID]; !ok || v.UpdatedAt >= cur.UpdatedAt {
					s.data.MedicationRecords[v.ID] = v
				}
			}
		}
		if pend, ok := s.data.PendingOps[k]; ok && ch.UpdatedAt >= pend.UpdatedAt {
			delete(s.data.PendingOps, k)
		}
	}

	if serverSeq > s.data.LastSeq {
		s.data.LastSeq = serverSeq
	}
	s.data.Config.LastSyncedAt = model.NowMS()
	// Ensure ActiveMemberID points to a valid non-deleted member if one exists
	if cur, ok := s.data.Members[s.data.Config.ActiveMemberID]; !ok || cur.DeletedAt != nil {
		s.data.Config.ActiveMemberID = ""
		ms := s.membersLocked()
		if len(ms) > 0 {
			s.data.Config.ActiveMemberID = ms[0].ID
		}
	}
	return s.saveLocked()
}

// --- Queries ---

func (s *Store) membersLocked() []model.Member {
	out := make([]model.Member, 0, len(s.data.Members))
	for _, m := range s.data.Members {
		if m.DeletedAt == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].BirthDate != out[j].BirthDate {
			return out[i].BirthDate > out[j].BirthDate // youngest first
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *Store) Members() []model.Member {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.membersLocked()
}

func (s *Store) ActiveMember() (model.Member, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id := s.data.Config.ActiveMemberID; id != "" {
		if m, ok := s.data.Members[id]; ok && m.DeletedAt == nil {
			return m, true
		}
	}
	ms := s.membersLocked()
	if len(ms) == 0 {
		return model.Member{}, false
	}
	return ms[0], true
}

func (s *Store) GrowthRecords(memberID string) []model.GrowthRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.GrowthRecord
	for _, r := range s.data.GrowthRecords {
		if r.DeletedAt == nil && (memberID == "" || r.MemberID == memberID) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecordedAt < out[j].RecordedAt
	})
	return out
}

func (s *Store) CustomMetrics() []model.CustomMetric {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.CustomMetric
	for _, m := range s.data.CustomMetrics {
		if m.DeletedAt == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *Store) CustomMetricRecords(memberID, metricID string) []model.CustomMetricRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.CustomMetricRecord
	for _, r := range s.data.CustomRecords {
		if r.DeletedAt == nil && (memberID == "" || r.MemberID == memberID) && (metricID == "" || r.MetricID == metricID) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecordedAt < out[j].RecordedAt
	})
	return out
}

func (s *Store) FeverEpisodes(memberID string) []model.FeverEpisode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.FeverEpisode
	for _, ep := range s.data.FeverEpisodes {
		if ep.DeletedAt == nil && (memberID == "" || ep.MemberID == memberID) {
			out = append(out, ep)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return epSortKey(out[i]) > epSortKey(out[j]) // newest first
	})
	return out
}

func epSortKey(ep model.FeverEpisode) int64 {
	if ep.StartedAt != 0 {
		return ep.StartedAt
	}
	return ep.UpdatedAt
}

func (s *Store) TemperatureRecords(memberID, episodeID string) []model.TemperatureRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.TemperatureRecord
	for _, r := range s.data.TempRecords {
		if r.DeletedAt == nil && (memberID == "" || r.MemberID == memberID) && (episodeID == "" || r.EpisodeID == episodeID) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecordedAt < out[j].RecordedAt
	})
	return out
}

func (s *Store) MedicationRecords(memberID, episodeID string) []model.MedicationRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []model.MedicationRecord
	for _, r := range s.data.MedicationRecords {
		if r.DeletedAt == nil && (memberID == "" || r.MemberID == memberID) && (episodeID == "" || r.EpisodeID == episodeID) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecordedAt < out[j].RecordedAt
	})
	return out
}

// LatestTempAndMed returns the latest temperature reading and latest antipyretic medication for memberID.
func (s *Store) LatestTempAndMed(memberID string) (*model.TemperatureRecord, *model.MedicationRecord) {
	temps := s.TemperatureRecords(memberID, "")
	meds := s.MedicationRecords(memberID, "")
	var lastT *model.TemperatureRecord
	var lastM *model.MedicationRecord
	if len(temps) > 0 {
		t := temps[len(temps)-1]
		lastT = &t
	}
	for i := len(meds) - 1; i >= 0; i-- {
		if meds[i].MedType != model.MedPhysical {
			m := meds[i]
			lastM = &m
			break
		}
	}
	if lastM == nil && len(meds) > 0 {
		m := meds[len(meds)-1]
		lastM = &m
	}
	return lastT, lastM
}

// FormatElapsed formats a short Chinese elapsed duration from unixMs to now.
func FormatElapsed(unixMs int64, now time.Time) string {
	if unixMs <= 0 {
		return "暂无"
	}
	d := now.Sub(time.UnixMilli(unixMs))
	if d < 0 {
		d = 0
	}
	mins := int(d.Minutes())
	if mins < 1 {
		return "刚刚"
	}
	if mins < 60 {
		return itoa(mins) + "分钟前"
	}
	hours := mins / 60
	remM := mins % 60
	if hours < 24 {
		if remM == 0 {
			return itoa(hours) + "小时前"
		}
		return itoa(hours) + "小时" + itoa(remM) + "分前"
	}
	days := hours / 24
	return itoa(days) + "天前"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

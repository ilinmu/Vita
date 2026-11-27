// Package model defines Vita's shared domain entities, sync envelopes, and API DTOs.
package model

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Gender represents biological sex used for reference growth curves.
type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
)

// TempSite identifies where a body temperature measurement was taken.
type TempSite string

const (
	TempSiteEar      TempSite = "ear"
	TempSiteArmpit   TempSite = "armpit"
	TempSiteForehead TempSite = "forehead"
	TempSiteOral     TempSite = "oral"
)

// SiteLabel returns a concise Chinese label for a measurement site.
func SiteLabel(s TempSite) string {
	switch s {
	case TempSiteEar:
		return "耳温"
	case TempSiteArmpit:
		return "腋温"
	case TempSiteForehead:
		return "额温"
	case TempSiteOral:
		return "口温"
	default:
		return "耳温"
	}
}

// MedType categorizes antipyretic medications or physical cooling actions.
type MedType string

const (
	MedIbuprofen     MedType = "ibuprofen"
	MedAcetaminophen MedType = "acetaminophen"
	MedPhysical      MedType = "physical"
	MedOther         MedType = "other"
)

// MedTypeLabel returns a concise Chinese label for a medication/cooling type.
func MedTypeLabel(m MedType) string {
	switch m {
	case MedIbuprofen:
		return "布洛芬"
	case MedAcetaminophen:
		return "对乙酰氨基酚"
	case MedPhysical:
		return "物理降温"
	case MedOther:
		return "其他处理"
	default:
		return "其他处理"
	}
}

// EntityType identifies syncable record types.
type EntityType string

const (
	EntityMember             EntityType = "member"
	EntityGrowthRecord       EntityType = "growth_record"
	EntityCustomMetric       EntityType = "custom_metric"
	EntityCustomMetricRecord EntityType = "custom_metric_record"
	EntityFeverEpisode       EntityType = "fever_episode"
	EntityTemperatureRecord  EntityType = "temperature_record"
	EntityMedicationRecord   EntityType = "medication_record"
)

// NewUUID generates a random RFC 4122 v4 UUID string.
func NewUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// NowMS returns the current Unix timestamp in milliseconds.
func NowMS() int64 {
	return time.Now().UnixMilli()
}

// Member represents a tracked person in the family.
type Member struct {
	ID        string `json:"id"`
	FamilyID  string `json:"family_id"`
	Name      string `json:"name"`
	Gender    Gender `json:"gender"`
	BirthDate string `json:"birth_date"` // YYYY-MM-DD
	ColorHex  string `json:"color_hex"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt *int64 `json:"deleted_at,omitempty"`
}

// ParseBirthDate parses BirthDate in the local timezone.
func (m Member) ParseBirthDate() (time.Time, error) {
	return time.ParseInLocation("2006-01-02", strings.TrimSpace(m.BirthDate), time.Local)
}

// AgeMonthsAt returns the exact age in months (using 30.4375 days/month) at t.
func (m Member) AgeMonthsAt(t time.Time) float64 {
	bd, err := m.ParseBirthDate()
	if err != nil {
		return 0
	}
	days := t.Sub(bd).Hours() / 24.0
	if days < 0 {
		return 0
	}
	return days / 30.4375
}

// IsChildAt reports whether the member is under 18 years old (216 months) at t.
func (m Member) IsChildAt(t time.Time) bool {
	bd, err := m.ParseBirthDate()
	if err != nil {
		return true
	}
	return m.AgeMonthsAt(t) < 216.0 && !t.Before(bd)
}

// AgeLabelAt formats a human-friendly Chinese age string at t.
func (m Member) AgeLabelAt(t time.Time) string {
	bd, err := m.ParseBirthDate()
	if err != nil {
		return "年龄未知"
	}
	if t.Before(bd) {
		return "未出生"
	}
	years := t.Year() - bd.Year()
	months := int(t.Month()) - int(bd.Month())
	days := t.Day() - bd.Day()
	if days < 0 {
		months--
		prevMonth := time.Date(t.Year(), t.Month(), 0, 0, 0, 0, 0, t.Location())
		days += prevMonth.Day()
	}
	if months < 0 {
		years--
		months += 12
	}
	switch {
	case years == 0 && months == 0:
		return fmt.Sprintf("%d天", days)
	case years == 0:
		if days > 0 {
			return fmt.Sprintf("%d个月%d天", months, days)
		}
		return fmt.Sprintf("%d个月", months)
	case years < 6:
		if months > 0 {
			return fmt.Sprintf("%d岁%d个月", years, months)
		}
		return fmt.Sprintf("%d岁", years)
	default:
		return fmt.Sprintf("%d岁", years)
	}
}

// GrowthRecord stores a growth measurement entry.
type GrowthRecord struct {
	ID         string   `json:"id"`
	FamilyID   string   `json:"family_id"`
	MemberID   string   `json:"member_id"`
	RecordedAt int64    `json:"recorded_at"`
	HeightCM   *float64 `json:"height_cm,omitempty"`
	WeightKG   *float64 `json:"weight_kg,omitempty"`
	HeadCircCM *float64 `json:"head_circ_cm,omitempty"`
	Note       string   `json:"note,omitempty"`
	UpdatedAt  int64    `json:"updated_at"`
	DeletedAt  *int64   `json:"deleted_at,omitempty"`
}

// BMI computes kg / (m^2) when both HeightCM (>0) and WeightKG (>0) are present.
func (g GrowthRecord) BMI() *float64 {
	if g.HeightCM == nil || g.WeightKG == nil || *g.HeightCM <= 0 || *g.WeightKG <= 0 {
		return nil
	}
	hm := *g.HeightCM / 100.0
	v := *g.WeightKG / (hm * hm)
	return &v
}

// CustomMetric defines a user-created numerical metric (e.g. 脚长 cm, 视力).
type CustomMetric struct {
	ID        string `json:"id"`
	FamilyID  string `json:"family_id"`
	Name      string `json:"name"`
	Unit      string `json:"unit"`
	ColorHex  string `json:"color_hex"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt *int64 `json:"deleted_at,omitempty"`
}

// CustomMetricRecord stores one data point for a CustomMetric.
type CustomMetricRecord struct {
	ID         string  `json:"id"`
	FamilyID   string  `json:"family_id"`
	MemberID   string  `json:"member_id"`
	MetricID   string  `json:"metric_id"`
	RecordedAt int64   `json:"recorded_at"`
	Value      float64 `json:"value"`
	Note       string  `json:"note,omitempty"`
	UpdatedAt  int64   `json:"updated_at"`
	DeletedAt  *int64  `json:"deleted_at,omitempty"`
}

// FeverEpisode groups temperature & medication logs into a single illness course.
type FeverEpisode struct {
	ID        string `json:"id"`
	FamilyID  string `json:"family_id"`
	MemberID  string `json:"member_id"`
	Title     string `json:"title"`
	StartedAt int64  `json:"started_at"`
	EndedAt   *int64 `json:"ended_at,omitempty"`
	Note      string `json:"note,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
	DeletedAt *int64 `json:"deleted_at,omitempty"`
}

// TemperatureRecord stores a single body temperature reading.
type TemperatureRecord struct {
	ID         string   `json:"id"`
	FamilyID   string   `json:"family_id"`
	MemberID   string   `json:"member_id"`
	EpisodeID  string   `json:"episode_id,omitempty"`
	RecordedAt int64    `json:"recorded_at"`
	TempC      float64  `json:"temp_c"`
	Site       TempSite `json:"site"`
	Symptom    string   `json:"symptom,omitempty"`
	Note       string   `json:"note,omitempty"`
	UpdatedAt  int64    `json:"updated_at"`
	DeletedAt  *int64   `json:"deleted_at,omitempty"`
}

// MedicationRecord stores an antipyretic dose or physical cooling intervention.
type MedicationRecord struct {
	ID         string  `json:"id"`
	FamilyID   string  `json:"family_id"`
	MemberID   string  `json:"member_id"`
	EpisodeID  string  `json:"episode_id,omitempty"`
	RecordedAt int64   `json:"recorded_at"`
	MedType    MedType `json:"med_type"`
	Name       string  `json:"name"`
	Dosage     string  `json:"dosage,omitempty"`
	Note       string  `json:"note,omitempty"`
	UpdatedAt  int64   `json:"updated_at"`
	DeletedAt  *int64  `json:"deleted_at,omitempty"`
}

// SyncChange is one entity mutation exchanged during incremental sync.
type SyncChange struct {
	EntityType EntityType      `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	UpdatedAt  int64           `json:"updated_at"`
	DeletedAt  *int64          `json:"deleted_at,omitempty"`
	Payload    json.RawMessage `json:"payload"`
	ServerSeq  int64           `json:"server_seq,omitempty"`
}

// SyncRequest carries local pending changes and the client's last known server sequence.
type SyncRequest struct {
	SinceSeq int64        `json:"since_seq"`
	Changes  []SyncChange `json:"changes"`
}

// SyncResponse returns the latest server sequence and all changes since SinceSeq.
type SyncResponse struct {
	ServerSeq int64        `json:"server_seq"`
	Changes   []SyncChange `json:"changes"`
}

// AuthRequest is used for /api/v1/auth/register and /api/v1/auth/login.
type AuthRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	FamilyName string `json:"family_name,omitempty"`
	InviteCode string `json:"invite_code,omitempty"`
}

// FamilyInfo describes a family space a user belongs to.
type FamilyInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	InviteCode string `json:"invite_code"`
	Role       string `json:"role"`
}

// UserInfo summarizes the authenticated user.
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// AuthResponse is returned by register/login/me endpoints.
type AuthResponse struct {
	Token    string       `json:"token,omitempty"`
	User     UserInfo     `json:"user"`
	Families []FamilyInfo `json:"families"`
}

// CreateFamilyRequest creates a new family space.
type CreateFamilyRequest struct {
	Name string `json:"name"`
}

// JoinFamilyRequest joins an existing family space by invite code.
type JoinFamilyRequest struct {
	InviteCode string `json:"invite_code"`
}

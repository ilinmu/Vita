// Package server implements Vita's lightweight SQLite-backed sync & family auth HTTP server.
package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ilinmu/vita/internal/model"
	_ "modernc.org/sqlite"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
)

// DB wraps SQLite persistence for users, families, and LWW sync entities.
type DB struct {
	sql     *sql.DB
	writeMu sync.Mutex // serializes monotonic sync sequence allocation
}

// OpenDB opens or initializes the SQLite database at dsn (file path or ":memory:").
func OpenDB(dsn string) (*DB, error) {
	if dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
		if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite handles concurrent readers best with single writer connection pool or busy_timeout
	db.SetMaxOpenConns(1)

	pragmas := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA foreign_keys=ON;`,
		`PRAGMA busy_timeout=5000;`,
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("pragma %s: %w", p, err)
		}
	}
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &DB{sql: db}, nil
}

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS families (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	invite_code TEXT UNIQUE NOT NULL,
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS family_members (
	family_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL,
	joined_at INTEGER NOT NULL,
	PRIMARY KEY (family_id, user_id),
	FOREIGN KEY (family_id) REFERENCES families(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS sync_entities (
	family_id TEXT NOT NULL,
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	updated_at INTEGER NOT NULL,
	deleted_at INTEGER,
	payload TEXT NOT NULL,
	seq INTEGER NOT NULL,
	PRIMARY KEY (family_id, entity_type, entity_id),
	FOREIGN KEY (family_id) REFERENCES families(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_sync_entities_family_seq ON sync_entities(family_id, seq);
`
	_, err := db.Exec(schema)
	return err
}

func (d *DB) Close() error {
	return d.sql.Close()
}

func generateInviteCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var out [6]byte
	for i := range out {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		out[i] = chars[n.Int64()]
	}
	return string(out[:])
}

func (d *DB) UserCount(ctx context.Context) (int, error) {
	var n int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (d *DB) CreateUser(ctx context.Context, username, passwordHash string) (model.UserInfo, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	u := model.UserInfo{ID: model.NewUUID(), Username: username}
	_, err := d.sql.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		u.ID, u.Username, passwordHash, model.NowMS(),
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return model.UserInfo{}, ErrConflict
		}
		return model.UserInfo{}, err
	}
	return u, nil
}

func (d *DB) FindUserByUsername(ctx context.Context, username string) (model.UserInfo, string, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	var u model.UserInfo
	var hash string
	err := d.sql.QueryRowContext(ctx,
		`SELECT id, username, password_hash FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UserInfo{}, "", ErrNotFound
	}
	return u, hash, err
}

func (d *DB) FindUserByID(ctx context.Context, userID string) (model.UserInfo, error) {
	var u model.UserInfo
	err := d.sql.QueryRowContext(ctx,
		`SELECT id, username FROM users WHERE id = ?`, userID,
	).Scan(&u.ID, &u.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UserInfo{}, ErrNotFound
	}
	return u, err
}

func (d *DB) CreateFamily(ctx context.Context, userID, name string) (model.FamilyInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "我的家庭"
	}
	f := model.FamilyInfo{
		ID:         model.NewUUID(),
		Name:       name,
		InviteCode: generateInviteCode(),
		Role:       "owner",
	}
	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return model.FamilyInfo{}, err
	}
	defer tx.Rollback()
	now := model.NowMS()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO families (id, name, invite_code, created_at) VALUES (?, ?, ?, ?)`,
		f.ID, f.Name, f.InviteCode, now,
	); err != nil {
		return model.FamilyInfo{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO family_members (family_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		f.ID, userID, f.Role, now,
	); err != nil {
		return model.FamilyInfo{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.FamilyInfo{}, err
	}
	return f, nil
}

func (d *DB) InviteCodeExists(ctx context.Context, code string) (bool, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return false, nil
	}
	var n int
	err := d.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM families WHERE invite_code = ?`, code).Scan(&n)
	return n > 0, err
}

func (d *DB) JoinFamilyByCode(ctx context.Context, userID, code string) (model.FamilyInfo, error) {
	code = strings.TrimSpace(strings.ToUpper(code))
	var f model.FamilyInfo
	err := d.sql.QueryRowContext(ctx,
		`SELECT id, name, invite_code FROM families WHERE invite_code = ?`, code,
	).Scan(&f.ID, &f.Name, &f.InviteCode)
	if errors.Is(err, sql.ErrNoRows) {
		return model.FamilyInfo{}, ErrNotFound
	}
	if err != nil {
		return model.FamilyInfo{}, err
	}
	f.Role = "member"
	_, err = d.sql.ExecContext(ctx,
		`INSERT OR IGNORE INTO family_members (family_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		f.ID, userID, f.Role, model.NowMS(),
	)
	if err != nil {
		return model.FamilyInfo{}, err
	}
	return f, nil
}

func (d *DB) UserFamilies(ctx context.Context, userID string) ([]model.FamilyInfo, error) {
	rows, err := d.sql.QueryContext(ctx, `
SELECT f.id, f.name, f.invite_code, fm.role
FROM families f
JOIN family_members fm ON fm.family_id = f.id
WHERE fm.user_id = ?
ORDER BY fm.joined_at ASC
`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FamilyInfo
	for rows.Next() {
		var f model.FamilyInfo
		if err := rows.Scan(&f.ID, &f.Name, &f.InviteCode, &f.Role); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (d *DB) IsFamilyMember(ctx context.Context, familyID, userID string) (bool, error) {
	var n int
	err := d.sql.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM family_members WHERE family_id = ? AND user_id = ?`,
		familyID, userID,
	).Scan(&n)
	return n > 0, err
}

// Sync applies incoming changes with LWW semantics and returns all changes with seq > sinceSeq.
func (d *DB) Sync(ctx context.Context, familyID, userID string, req model.SyncRequest) (model.SyncResponse, error) {
	ok, err := d.IsFamilyMember(ctx, familyID, userID)
	if err != nil {
		return model.SyncResponse{}, err
	}
	if !ok {
		return model.SyncResponse{}, ErrForbidden
	}

	d.writeMu.Lock()
	defer d.writeMu.Unlock()

	tx, err := d.sql.BeginTx(ctx, nil)
	if err != nil {
		return model.SyncResponse{}, err
	}
	defer tx.Rollback()

	var maxSeq int64
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM sync_entities WHERE family_id = ?`, familyID,
	).Scan(&maxSeq); err != nil {
		return model.SyncResponse{}, err
	}

	for _, ch := range req.Changes {
		if ch.EntityID == "" || ch.EntityType == "" {
			continue
		}
		var existingUpdated int64
		err := tx.QueryRowContext(ctx,
			`SELECT updated_at FROM sync_entities WHERE family_id = ? AND entity_type = ? AND entity_id = ?`,
			familyID, string(ch.EntityType), ch.EntityID,
		).Scan(&existingUpdated)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return model.SyncResponse{}, err
		}
		if err == nil && ch.UpdatedAt < existingUpdated {
			// Server has newer version: keep server's version
			continue
		}
		maxSeq++
		var delVal any
		if ch.DeletedAt != nil {
			delVal = *ch.DeletedAt
		}
		payloadStr := string(ch.Payload)
		if payloadStr == "" {
			payloadStr = "{}"
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sync_entities (family_id, entity_type, entity_id, updated_at, deleted_at, payload, seq)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(family_id, entity_type, entity_id) DO UPDATE SET
	updated_at = excluded.updated_at,
	deleted_at = excluded.deleted_at,
	payload = excluded.payload,
	seq = excluded.seq
`, familyID, string(ch.EntityType), ch.EntityID, ch.UpdatedAt, delVal, payloadStr, maxSeq); err != nil {
			return model.SyncResponse{}, err
		}
	}

	rows, err := tx.QueryContext(ctx, `
SELECT entity_type, entity_id, updated_at, deleted_at, payload, seq
FROM sync_entities
WHERE family_id = ? AND seq > ?
ORDER BY seq ASC
`, familyID, req.SinceSeq)
	if err != nil {
		return model.SyncResponse{}, err
	}
	defer rows.Close()

	var changes []model.SyncChange
	for rows.Next() {
		var et, eid, payload string
		var updatedAt, seq int64
		var deletedAt sql.NullInt64
		if err := rows.Scan(&et, &eid, &updatedAt, &deletedAt, &payload, &seq); err != nil {
			return model.SyncResponse{}, err
		}
		var delPtr *int64
		if deletedAt.Valid {
			v := deletedAt.Int64
			delPtr = &v
		}
		changes = append(changes, model.SyncChange{
			EntityType: model.EntityType(et),
			EntityID:   eid,
			UpdatedAt:  updatedAt,
			DeletedAt:  delPtr,
			Payload:    json.RawMessage(payload),
			ServerSeq:  seq,
		})
	}
	if err := rows.Err(); err != nil {
		return model.SyncResponse{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.SyncResponse{}, err
	}
	return model.SyncResponse{
		ServerSeq: maxSeq,
		Changes:   changes,
	}, nil
}

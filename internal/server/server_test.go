package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ilinmu/vita/internal/localstore"
	"github.com/ilinmu/vita/internal/model"
	"github.com/ilinmu/vita/internal/syncer"
)

type handlerRT struct {
	h http.Handler
}

func (rt handlerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rt.h.ServeHTTP(rec, r)
	res := rec.Result()
	res.Body = io.NopCloser(rec.Body)
	return res, nil
}

func TestTwoPhoneSyncAndRegistrationGate(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	srv := New(db, Options{
		JWTSecret:           []byte("test-secret-key-at-least-32-bytes-long"),
		AllowPublicRegister: false,
	})
	rt := handlerRT{h: srv}

	ctx := context.Background()

	// 1. First owner registers freely (0 users initially)
	phone1Client := syncer.NewClient("http://lan.local", "https://pub.local")
	phone1Client.SetTransport(rt)
	auth1, err := phone1Client.Register(ctx, model.AuthRequest{
		Username:   "mom",
		Password:   "password123",
		FamilyName: "小核桃家",
	})
	if err != nil {
		t.Fatalf("owner register failed: %v", err)
	}
	if len(auth1.Families) != 1 || auth1.Families[0].InviteCode == "" {
		t.Fatalf("expected family with invite code, got %+v", auth1.Families)
	}
	inviteCode := auth1.Families[0].InviteCode

	// 2. Stranger without invite code is rejected when AllowPublicRegister=false
	_, err = phone1Client.Register(ctx, model.AuthRequest{
		Username: "stranger",
		Password: "password123",
	})
	var apiErr *syncer.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected 403 for stranger, got %v", err)
	}

	// 3. Spouse registers WITH invite code
	phone2Client := syncer.NewClient("", "https://pub.local")
	phone2Client.SetTransport(rt)
	auth2, err := phone2Client.Register(ctx, model.AuthRequest{
		Username:   "dad",
		Password:   "password123",
		InviteCode: inviteCode,
	})
	if err != nil {
		t.Fatalf("invited spouse register failed: %v", err)
	}
	if auth2.Families[0].ID != auth1.Families[0].ID {
		t.Fatalf("expected spouse to join same family")
	}

	// 4. Two offline stores sync bidirectionally
	st1, _ := localstore.Open(t.TempDir())
	_ = st1.UpdateConfig(func(c *localstore.Config) {
		c.LANURL = "http://lan.local"
		c.PublicURL = "https://pub.local"
		c.AuthToken = auth1.Token
		c.UserID = auth1.User.ID
		c.Username = auth1.User.Username
		c.FamilyID = auth1.Families[0].ID
	})
	m, _ := st1.SaveMember(model.Member{Name: "小核桃", Gender: model.GenderMale, BirthDate: "2024-06-01"})
	if err := phone1Client.Sync(ctx, st1); err != nil {
		t.Fatalf("phone1 sync: %v", err)
	}

	st2, _ := localstore.Open(t.TempDir())
	_ = st2.UpdateConfig(func(c *localstore.Config) {
		c.PublicURL = "https://pub.local"
		c.AuthToken = auth2.Token
		c.UserID = auth2.User.ID
		c.Username = auth2.User.Username
		c.FamilyID = auth2.Families[0].ID
	})
	if err := phone2Client.Sync(ctx, st2); err != nil {
		t.Fatalf("phone2 initial sync: %v", err)
	}
	if len(st2.Members()) != 1 || st2.Members()[0].Name != "小核桃" {
		t.Fatalf("expected phone2 to receive member, got %+v", st2.Members())
	}

	// Phone 2 records a fever temperature & syncs back to Phone 1
	_, _ = st2.SaveTemperatureRecord(model.TemperatureRecord{
		MemberID: m.ID,
		TempC:    38.6,
		Site:     model.TempSiteEar,
	})
	if err := phone2Client.Sync(ctx, st2); err != nil {
		t.Fatalf("phone2 push temp: %v", err)
	}
	if err := phone1Client.Sync(ctx, st1); err != nil {
		t.Fatalf("phone1 pull temp: %v", err)
	}
	temps := st1.TemperatureRecords(m.ID, "")
	if len(temps) != 1 || temps[0].TempC != 38.6 {
		t.Fatalf("expected phone1 to receive 38.6 temp, got %+v", temps)
	}
}

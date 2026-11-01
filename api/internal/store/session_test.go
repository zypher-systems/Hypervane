package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"hypervane/internal/auth"
)

// testStore needs a real database, because these tests exist to catch SQL that
// only fails when it runs. Without one they skip.
func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("HYPERVANE_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("HYPERVANE_TEST_DATABASE_URL is not set")
	}
	st, err := New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

func TestDeleteSessionByTokenHash(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	uid, err := auth.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	sid, err := auth.NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, uid, "session-test-"+uid, "!"); err != nil {
		t.Fatal(err)
	}
	hash := HashToken("token-" + uid)
	if err := st.CreateSession(ctx, Session{ID: sid, UserID: uid, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SessionByTokenHash(ctx, hash); err != nil {
		t.Fatal(err)
	}
	id, err := st.DeleteSessionByTokenHash(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	if id != sid {
		t.Fatalf("deleted %q, want %q", id, sid)
	}
	if _, err := st.SessionByTokenHash(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session survived: %v", err)
	}
	if _, err := st.DeleteSessionByTokenHash(ctx, hash); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestListAuditFilter(t *testing.T) {
	st := testStore(t)
	ctx := context.Background()
	stamp := time.Now().Format("150405.000000000")
	connA, connB, alice, bob := "a-"+stamp, "b-"+stamp, "alice-"+stamp, "bob-"+stamp
	for _, ev := range []AuditEvent{
		{Username: alice, ConnectionID: connA},
		{Username: bob, ConnectionID: connA},
		{Username: alice, ConnectionID: connB},
	} {
		ev.At, ev.Method, ev.Path, ev.Status = time.Now(), "POST", "/x", 200
		if err := st.InsertAudit(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		filter AuditFilter
		want   int
	}{
		{AuditFilter{ConnectionID: connA}, 2},
		{AuditFilter{ConnectionID: connA, Username: alice}, 1},
		{AuditFilter{ConnectionID: connB, Username: bob}, 0},
	}
	for _, c := range cases {
		rows, err := st.ListAudit(ctx, 200, c.filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != c.want {
			t.Fatalf("%+v: %d rows", c.filter, len(rows))
		}
	}
	rows, err := st.ListAudit(ctx, 500, AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 3 {
		t.Fatalf("no filter returned %d rows", len(rows))
	}
}

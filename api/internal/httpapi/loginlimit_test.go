package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func attemptFor(t *testing.T, username, addr string) loginAttempt {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = addr
	return newLoginAttempt(req, loginRequest{Username: username})
}

func TestLoginLimiterPairLimit(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	a := attemptFor(t, "admin", "192.0.2.1:1")
	for i := 0; i < loginMaxPair; i++ {
		if ok, _ := l.take(a, now); !ok {
			t.Fatalf("attempt %d refused", i)
		}
	}
	ok, wait := l.take(a, now)
	if ok {
		t.Fatal("over the limit")
	}
	if wait <= 0 || wait > loginWindow {
		t.Fatalf("wait %v", wait)
	}
	if ok, _ := l.take(a, now.Add(loginWindow+time.Second)); !ok {
		t.Fatal("limit outlived its window")
	}
}

func TestLoginLimiterIgnoresPortAndCase(t *testing.T) {
	a := attemptFor(t, "Admin", "192.0.2.1:1")
	b := attemptFor(t, "admin", "192.0.2.1:2")
	if a != b {
		t.Fatalf("%q != %q", a, b)
	}
}

// Rotating addresses, as a forged header would, still hits the account's limit.
func TestLoginLimiterAccountLimitAcrossAddresses(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < loginMaxAccount; i++ {
		if ok, _ := l.take(attemptFor(t, "admin", "192.0.2."+strconv.Itoa(i)+":1"), now); !ok {
			t.Fatalf("attempt %d refused", i)
		}
	}
	if ok, _ := l.take(attemptFor(t, "admin", "198.51.100.1:1"), now); ok {
		t.Fatal("account limit did not hold")
	}
}

func TestLoginLimiterAddressLimitAcrossAccounts(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	for i := 0; i < loginMaxAddress; i++ {
		if ok, _ := l.take(attemptFor(t, "user"+strconv.Itoa(i), "192.0.2.1:1"), now); !ok {
			t.Fatalf("attempt %d refused", i)
		}
	}
	if ok, _ := l.take(attemptFor(t, "someone-else", "192.0.2.1:1"), now); ok {
		t.Fatal("address limit did not hold")
	}
}

func TestLoginLimiterForgiveAndSucceed(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	a := attemptFor(t, "admin", "192.0.2.1:1")
	for i := 0; i < loginMaxPair*3; i++ {
		if ok, _ := l.take(a, now); !ok {
			t.Fatal("outage attempts must not add up")
		}
		l.forgive(a)
	}
	for i := 0; i < loginMaxPair-1; i++ {
		l.take(a, now)
	}
	l.take(a, now)
	l.succeed(a)
	if ok, _ := l.take(a, now); !ok {
		t.Fatal("a correct password should clear the account")
	}
	if got := len(l.fails[a.address]); got != loginMaxPair {
		t.Fatalf("address history %d", got)
	}
}

func TestLoginLimiterHoldsUnderConcurrency(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	a := attemptFor(t, "admin", "192.0.2.1:1")
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _ := l.take(a, now); ok {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if int(allowed.Load()) != loginMaxPair {
		t.Fatalf("%d attempts allowed", allowed.Load())
	}
}

func TestLoginLimiterSweepDropsOnlyExpiredKeys(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	l.fails = map[string][]time.Time{}
	for i := 0; i < loginMaxKeys+10; i++ {
		l.fails[strconv.Itoa(i)] = []time.Time{now}
	}
	l.sweep(now)
	if len(l.fails) != loginMaxKeys+10 {
		t.Fatalf("live keys were dropped: %d left", len(l.fails))
	}
	l.sweep(now.Add(loginWindow + time.Minute))
	if len(l.fails) != 0 {
		t.Fatalf("%d expired keys kept", len(l.fails))
	}
}

// Filling the table must never be a way to erase a lockout.
func TestLoginLimiterKeepsLiveLockoutsAtTheCap(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	var victims []loginAttempt
	for v := 0; v < 25; v++ {
		victim := attemptFor(t, "admin"+strconv.Itoa(v), "192.0.2."+strconv.Itoa(v+1)+":1")
		victims = append(victims, victim)
		for i := 0; i < loginMaxPair; i++ {
			if ok, _ := l.take(victim, now); !ok {
				t.Fatalf("attempt %d refused", i)
			}
		}
	}
	for i := 0; i < loginMaxKeys*4; i++ {
		l.fails["filler"+strconv.Itoa(i)] = []time.Time{now}
	}
	now = now.Add(2 * time.Second)
	for _, victim := range victims {
		if ok, _ := l.take(victim, now); ok {
			t.Fatal("the cap erased a live lockout")
		}
	}
}

// An account that can't be tracked can't be limited, so it has to wait.
func TestLoginLimiterRefusesWhatItCannotTrack(t *testing.T) {
	var l loginLimiter
	now := time.Unix(1_700_000_000, 0)
	l.fails = map[string][]time.Time{}
	for i := 0; i < loginMaxKeys; i++ {
		l.fails["filler"+strconv.Itoa(i)] = []time.Time{now}
	}
	stranger := attemptFor(t, "stranger", "192.0.2.9:1")
	now = now.Add(2 * time.Second)
	ok, wait := l.take(stranger, now)
	if ok || wait <= 0 {
		t.Fatalf("untracked account allowed: %v %v", ok, wait)
	}
	if len(l.fails) > loginMaxKeys {
		t.Fatalf("table grew to %d", len(l.fails))
	}
	now = now.Add(loginWindow + time.Second)
	if ok, _ := l.take(stranger, now); !ok {
		t.Fatal("expired entries did not free the table")
	}
}

package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// authBodyLimit caps the unauthenticated sign-in and setup bodies.
	authBodyLimit = 64 << 10

	loginWindow = 10 * time.Minute

	// One address guessing one account is the ordinary attack and gets the
	// tightest limit. Addresses can be shared or forged, so the account also
	// has a limit of its own, set high enough that a stranger can't easily
	// keep its owner out. The address limit bounds spraying across accounts.
	loginMaxPair    = 8
	loginMaxAccount = 30
	loginMaxAddress = 60

	loginMaxKeys = 50_000
)

// loginAttempt names what one sign-in counts against.
type loginAttempt struct {
	pair, account, address string
}

func newLoginAttempt(r *http.Request, req loginRequest) loginAttempt {
	account := strings.ToLower(req.Username)
	if realm := strings.ToLower(req.Realm); realm != "" && realm != "local" {
		if !strings.Contains(account, "@") {
			account += "@" + realm
		}
	} else {
		account = "local\x00" + account
	}
	address := r.RemoteAddr
	if host, _, err := net.SplitHostPort(address); err == nil {
		address = host
	}
	return loginAttempt{
		pair:    "pair\x00" + address + "\x00" + account,
		account: "account\x00" + account,
		address: "address\x00" + address,
	}
}

func (a loginAttempt) limits() []struct {
	key string
	max int
} {
	return []struct {
		key string
		max int
	}{{a.pair, loginMaxPair}, {a.account, loginMaxAccount}, {a.address, loginMaxAddress}}
}

// loginLimiter counts failed sign-ins in memory, so a restart clears it and each
// API instance counts alone. take charges an attempt before the password is
// checked, so simultaneous guesses can't all slip past the limit; forgive and
// succeed undo the charge.
type loginLimiter struct {
	mu    sync.Mutex
	fails map[string][]time.Time
	swept time.Time
}

// take charges one attempt, or reports how long to wait if a limit is reached.
func (l *loginLimiter) take(a loginAttempt, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fails == nil {
		l.fails = map[string][]time.Time{}
	}
	l.sweep(now)
	cut := now.Add(-loginWindow)
	var wait time.Duration
	for _, lim := range a.limits() {
		kept := l.fails[lim.key]
		for len(kept) > 0 && !kept[0].After(cut) {
			kept = kept[1:]
		}
		l.fails[lim.key] = kept
		if len(kept) >= lim.max {
			if w := kept[len(kept)-lim.max].Add(loginWindow).Sub(now); w > wait {
				wait = w
			}
		}
	}
	if wait > 0 {
		return false, wait
	}
	for _, lim := range a.limits() {
		l.fails[lim.key] = append(l.fails[lim.key], now)
	}
	return true, 0
}

// forgive refunds an attempt that said nothing about the password.
func (l *loginLimiter) forgive(a loginAttempt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, lim := range a.limits() {
		l.refund(lim.key)
	}
}

// succeed clears the account after a correct password. The address keeps its
// history, or anyone with a working login could reset it between guesses at
// other accounts.
func (l *loginLimiter) succeed(a loginAttempt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, a.pair)
	delete(l.fails, a.account)
	l.refund(a.address)
}

func (l *loginLimiter) refund(key string) {
	if n := len(l.fails[key]); n > 0 {
		l.fails[key] = l.fails[key][:n-1]
	}
}

// sweep keeps the map from growing with every name an attacker tries.
func (l *loginLimiter) sweep(now time.Time) {
	if now.Sub(l.swept) < time.Minute && len(l.fails) < loginMaxKeys {
		return
	}
	l.swept = now
	cut := now.Add(-loginWindow)
	for key, times := range l.fails {
		if len(times) == 0 || !times[len(times)-1].After(cut) {
			delete(l.fails, key)
		}
	}
	for key := range l.fails {
		if len(l.fails) < loginMaxKeys {
			break
		}
		delete(l.fails, key)
	}
}

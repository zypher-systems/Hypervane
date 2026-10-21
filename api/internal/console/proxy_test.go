package console

import (
	"testing"

	"hypervane/internal/pve"
)

func TestNormalizeKind(t *testing.T) {
	cases := map[string]Kind{
		"":       KindVNC,
		"vnc":    KindVNC,
		"VNC":    KindVNC,
		"term":   KindTerm,
		"serial": KindTerm,
		"shell":  KindTerm,
		"spice":  KindSpice,
	}
	for in, want := range cases {
		if got := NormalizeKind(in); got != want {
			t.Fatalf("NormalizeKind(%q)=%s want %s", in, got, want)
		}
	}
}

func TestConsoleUser(t *testing.T) {
	tok, err := pve.New(pve.Options{BaseURL: "https://pve.local", Token: "root@pam!hv=deadbeef"})
	if err != nil {
		t.Fatal(err)
	}
	if got := consoleUser(tok, ""); got != "root@pam" {
		t.Fatalf("token user %q", got)
	}
	if got := consoleUser(tok, "dusty@pve"); got != "dusty@pve" {
		t.Fatalf("explicit user %q", got)
	}
	if got := consoleUser(tok, "root@pam!Hypervane"); got != "root@pam!Hypervane" {
		t.Fatalf("proxy user kept %q", got)
	}

	tick, err := pve.New(pve.Options{BaseURL: "https://pve.local", Ticket: "PVE:dusty@pam:1710000000:sig"})
	if err != nil {
		t.Fatal(err)
	}
	if got := consoleUser(tick, ""); got != "dusty@pam" {
		t.Fatalf("ticket user %q", got)
	}
}

func TestTicketUser(t *testing.T) {
	if got := ticketUser("PVE:dusty@pam:1:sig"); got != "dusty@pam" {
		t.Fatalf("got %q", got)
	}
	if got := ticketUser("PVEVNC:1:sig"); got != "" {
		t.Fatalf("vnc ticket should not parse as user, got %q", got)
	}
}

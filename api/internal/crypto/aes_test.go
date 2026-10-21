package crypto

import "testing"

func TestSealOpen(t *testing.T) {
	box, err := NewBox("test-master-key-please-change")
	if err != nil {
		t.Fatal(err)
	}
	ct, nonce, err := box.Seal([]byte("PVEAPIToken=secret"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Open(ct, nonce)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "PVEAPIToken=secret" {
		t.Fatalf("got %q", plain)
	}
}

package console

import (
	"bytes"
	"crypto/des"
	"math/bits"
	"testing"
)

func TestVNCAuthResponseLength(t *testing.T) {
	got := vncAuthResponse(bytes.Repeat([]byte{1}, 16), "ticket")
	if len(got) != 16 {
		t.Fatalf("len %d", len(got))
	}
	if bytes.Equal(got, make([]byte, 16)) {
		t.Fatal("expected ciphertext")
	}
}

func TestVNCAuthResponseRoundTrip(t *testing.T) {
	challenge := []byte("1234567890abcdef")
	password := "ticket01"
	resp := vncAuthResponse(challenge, password)
	key := make([]byte, 8)
	copy(key, password)
	for i := range key {
		key[i] = bits.Reverse8(key[i])
	}
	block, err := des.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, 16)
	block.Decrypt(out[:8], resp[:8])
	block.Decrypt(out[8:], resp[8:])
	if !bytes.Equal(out, challenge) {
		t.Fatalf("round trip %q", out)
	}
}

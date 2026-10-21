package console

import (
	"crypto/des"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/gorilla/websocket"
)

func vncAuthResponse(challenge []byte, password string) []byte {
	key := make([]byte, 8)
	copy(key, password)
	for i := range key {
		key[i] = bits.Reverse8(key[i])
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return make([]byte, 16)
	}
	out := make([]byte, 16)
	block.Encrypt(out[:8], challenge[:8])
	block.Encrypt(out[8:], challenge[8:])
	return out
}

type wsStream struct {
	c   *websocket.Conn
	buf []byte
}

func (s *wsStream) Read(p []byte) (int, error) {
	for len(s.buf) == 0 {
		_, data, err := s.c.ReadMessage()
		if err != nil {
			return 0, err
		}
		s.buf = data
	}
	n := copy(p, s.buf)
	s.buf = s.buf[n:]
	return n, nil
}

func wsWrite(c *websocket.Conn, p []byte) error {
	return c.WriteMessage(websocket.BinaryMessage, p)
}

func readN(s *wsStream, n int) ([]byte, error) {
	out := make([]byte, n)
	_, err := io.ReadFull(s, out)
	return out, err
}

func relayVNCAuth(browser, upstream *websocket.Conn, password string) error {
	up := &wsStream{c: upstream}
	br := &wsStream{c: browser}

	sver, err := readN(up, 12)
	if err != nil {
		return fmt.Errorf("vnc server version: %w", err)
	}
	if err := wsWrite(browser, sver); err != nil {
		return err
	}
	cver, err := readN(br, 12)
	if err != nil {
		return fmt.Errorf("vnc client version: %w", err)
	}
	if err := wsWrite(upstream, cver); err != nil {
		return err
	}

	if string(sver) >= "RFB 003.007" {
		if err := vncAuth38(up, br, browser, upstream, password); err != nil {
			return err
		}
	} else {
		if err := vncAuth33(up, br, browser, upstream, password); err != nil {
			return err
		}
	}
	if len(up.buf) > 0 {
		if err := wsWrite(browser, append([]byte(nil), up.buf...)); err != nil {
			return err
		}
		up.buf = nil
	}
	if len(br.buf) > 0 {
		if err := wsWrite(upstream, append([]byte(nil), br.buf...)); err != nil {
			return err
		}
		br.buf = nil
	}
	return nil
}

func vncAuth38(up, br *wsStream, browser, upstream *websocket.Conn, password string) error {
	nbuf, err := readN(up, 1)
	if err != nil {
		return err
	}
	n := int(nbuf[0])
	if n == 0 {
		reason, _ := readN(up, 4)
		if len(reason) == 4 {
			ln := int(binary.BigEndian.Uint32(reason))
			msg, _ := readN(up, ln)
			return fmt.Errorf("vnc rejected: %s", msg)
		}
		return fmt.Errorf("vnc rejected")
	}
	types, err := readN(up, n)
	if err != nil {
		return err
	}
	hasVNC := false
	hasNone := false
	for _, t := range types {
		if t == 2 {
			hasVNC = true
		}
		if t == 1 {
			hasNone = true
		}
	}
	if hasVNC && password != "" {
		if err := wsWrite(upstream, []byte{2}); err != nil {
			return err
		}
		if err := completeVNCAuth(up, upstream, password); err != nil {
			return err
		}
		if err := wsWrite(browser, []byte{1, 1}); err != nil {
			return err
		}
		if _, err := readN(br, 1); err != nil {
			return err
		}
		return wsWrite(browser, []byte{0, 0, 0, 0})
	}
	if hasNone {
		sec := append([]byte{byte(n)}, types...)
		if err := wsWrite(browser, sec); err != nil {
			return err
		}
		sel, err := readN(br, 1)
		if err != nil {
			return err
		}
		if err := wsWrite(upstream, sel); err != nil {
			return err
		}
		res, err := readN(up, 4)
		if err != nil {
			return err
		}
		return wsWrite(browser, res)
	}
	return fmt.Errorf("vnc has no usable security type")
}

func vncAuth33(up, br *wsStream, browser, upstream *websocket.Conn, password string) error {
	raw, err := readN(up, 4)
	if err != nil {
		return err
	}
	sec := binary.BigEndian.Uint32(raw)
	if sec == 2 && password != "" {
		if err := completeVNCAuth(up, upstream, password); err != nil {
			return err
		}
		return wsWrite(browser, []byte{0, 0, 0, 1})
	}
	if err := wsWrite(browser, raw); err != nil {
		return err
	}
	sel, err := readN(br, 4)
	if err != nil {
		return err
	}
	return wsWrite(upstream, sel)
}

func completeVNCAuth(up *wsStream, upstream *websocket.Conn, password string) error {
	challenge, err := readN(up, 16)
	if err != nil {
		return fmt.Errorf("vnc challenge: %w", err)
	}
	if err := wsWrite(upstream, vncAuthResponse(challenge, password)); err != nil {
		return err
	}
	res, err := readN(up, 4)
	if err != nil {
		return err
	}
	if binary.BigEndian.Uint32(res) != 0 {
		return fmt.Errorf("vnc authentication failed")
	}
	return nil
}

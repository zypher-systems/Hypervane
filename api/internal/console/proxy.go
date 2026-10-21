package console

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"hypervane/internal/pve"
)

type Kind string

const (
	KindVNC   Kind = "vnc"
	KindTerm  Kind = "term"
	KindSpice Kind = "spice"
)

// Proxy relays a browser websocket to a Proxmox console. cmd (node terminals
// only) runs one of Proxmox's shell commands, such as "upgrade", instead of a
// login shell.
func Proxy(ctx context.Context, client *pve.Client, browser *websocket.Conn, node, guestKind string, vmid int, kind Kind, cmd string) error {
	var (
		wsPath   string
		query    url.Values
		termUser string
		termTick string
		vncTick  string
	)
	switch kind {
	case KindVNC:
		if guestKind == "node" {
			proxy, err := client.NodeVNCShell(ctx, node)
			if err != nil {
				return err
			}
			wsPath = fmt.Sprintf("/nodes/%s/vncwebsocket", url.PathEscape(node))
			query = url.Values{"port": {proxy.Port}, "vncticket": {proxy.Ticket}}
			vncTick = proxy.Ticket
			break
		}
		proxy, err := client.VNCProxy(ctx, node, guestKind, vmid)
		if err != nil {
			return err
		}
		wsPath = fmt.Sprintf("/nodes/%s/%s/%d/vncwebsocket", url.PathEscape(node), guestKind, vmid)
		query = url.Values{"port": {proxy.Port}, "vncticket": {proxy.Ticket}}
		vncTick = proxy.Ticket
	case KindTerm:
		var (
			proxy pve.TermProxy
			err   error
		)
		if guestKind == "node" {
			if cmd != "" {
				proxy, err = client.NodeCommandTermProxy(ctx, node, cmd)
			} else {
				proxy, err = client.NodeTermProxy(ctx, node)
			}
			wsPath = fmt.Sprintf("/nodes/%s/vncwebsocket", url.PathEscape(node))
		} else {
			proxy, err = client.TermProxy(ctx, node, guestKind, vmid)
			wsPath = fmt.Sprintf("/nodes/%s/%s/%d/vncwebsocket", url.PathEscape(node), guestKind, vmid)
		}
		if err != nil {
			return err
		}
		query = url.Values{
			"port":      {proxy.Port},
			"vncticket": {proxy.Ticket},
		}
		termUser = consoleUser(client, proxy.User)
		termTick = proxy.Ticket
	case KindSpice:
		if guestKind == "node" {
			proxy, err := client.NodeSpiceShell(ctx, node)
			if err != nil {
				return err
			}
			wsPath = fmt.Sprintf("/nodes/%s/vncwebsocket", url.PathEscape(node))
			query = url.Values{"port": {proxy.Port}, "vncticket": {proxy.Ticket}}
			break
		}
		proxy, err := client.SpiceProxy(ctx, node, vmid)
		if err != nil {
			return err
		}
		wsPath = fmt.Sprintf("/nodes/%s/qemu/%d/vncwebsocket", url.PathEscape(node), vmid)
		query = url.Values{"port": {proxy.Port}, "vncticket": {proxy.Ticket}}
	default:
		return fmt.Errorf("unknown console kind %s", kind)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 8 * time.Second,
		TLSClientConfig:  tlsConfigFrom(client),
	}
	header := http.Header{}
	dummy, _ := http.NewRequest(http.MethodGet, "http://pve.local", nil)
	client.Authorize(dummy)
	for k, vals := range dummy.Header {
		header[k] = vals
	}
	upstream, _, err := dialer.DialContext(ctx, client.WebsocketURL(wsPath, query), header)
	if err != nil {
		return fmt.Errorf("dial pve console: %w", err)
	}
	defer upstream.Close()

	if kind == KindTerm && termTick != "" {
		auth := termUser + ":" + termTick + "\n"
		if err := upstream.WriteMessage(websocket.BinaryMessage, []byte(auth)); err != nil {
			return fmt.Errorf("term auth: %w", err)
		}
	}
	if kind == KindVNC && vncTick != "" {
		if err := relayVNCAuth(browser, upstream, vncTick); err != nil {
			return err
		}
	}

	errc := make(chan error, 2)
	go pipe(browser, upstream, errc)
	go pipe(upstream, browser, errc)
	return <-errc
}

func consoleUser(c *pve.Client, user string) string {
	if user != "" {
		return user
	}
	if c != nil {
		if tok := c.Token(); tok != "" {
			if i := strings.Index(tok, "!"); i > 0 {
				return tok[:i]
			}
			return tok
		}
		if u := ticketUser(c.Ticket()); u != "" {
			return u
		}
	}
	return "root@pam"
}

func ticketUser(ticket string) string {
	parts := strings.Split(ticket, ":")
	if len(parts) >= 2 && (parts[0] == "PVE" || parts[0] == "PVEAUTH") {
		return parts[1]
	}
	return ""
}

func pipe(dst, src *websocket.Conn, errc chan error) {
	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		if err := dst.WriteMessage(mt, data); err != nil {
			errc <- err
			return
		}
	}
}

func tlsConfigFrom(c *pve.Client) *tls.Config {
	if t, ok := c.HTTP().Transport.(*http.Transport); ok && t.TLSClientConfig != nil {
		return t.TLSClientConfig.Clone()
	}
	return &tls.Config{InsecureSkipVerify: true} //nolint:gosec
}

func NormalizeKind(s string) Kind {
	if strings.EqualFold(s, "term") || strings.EqualFold(s, "serial") || strings.EqualFold(s, "shell") {
		return KindTerm
	}
	if strings.EqualFold(s, "spice") {
		return KindSpice
	}
	return KindVNC
}

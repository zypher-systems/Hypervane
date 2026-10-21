package pve

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const TicketTTL = 2 * time.Hour

type TicketAuth struct {
	Username string
	Ticket   string
	CSRF     string
	NeedTFA  bool
}

func (c *Client) CreateTicket(ctx context.Context, username, password, otp, tfaChallenge string) (TicketAuth, error) {
	form := url.Values{
		"username": {username},
		"password": {password},
	}
	if otp != "" {
		form.Set("password", otp)
	}
	if tfaChallenge != "" {
		form.Set("tfa-challenge", tfaChallenge)
	}
	return c.postTicket(ctx, form)
}

func (c *Client) RefreshTicket(ctx context.Context, username, ticket string) (TicketAuth, error) {
	return c.CreateTicket(ctx, username, ticket, "", "")
}

func (c *Client) postTicket(ctx context.Context, form url.Values) (TicketAuth, error) {
	full, err := url.Parse(c.base.String() + "/access/ticket")
	if err != nil {
		return TicketAuth{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, full.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return TicketAuth{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.http.Do(req)
	if err != nil {
		return TicketAuth{}, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return TicketAuth{}, err
	}
	if res.StatusCode >= 400 {
		return TicketAuth{}, parseAPIError(res.StatusCode, raw)
	}
	var envelope struct {
		Data struct {
			Username            string          `json:"username"`
			Ticket              string          `json:"ticket"`
			CSRFPreventionToken string          `json:"CSRFPreventionToken"`
			NeedTFA             json.RawMessage `json:"NeedTFA"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return TicketAuth{}, err
	}
	return TicketAuth{
		Username: envelope.Data.Username,
		Ticket:   envelope.Data.Ticket,
		CSRF:     envelope.Data.CSRFPreventionToken,
		NeedTFA:  needTFA(envelope.Data.NeedTFA),
	}, nil
}

func needTFA(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return n != 0
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	s := strings.Trim(string(raw), `"`)
	return s != "" && s != "0"
}

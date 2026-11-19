package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the production gateway. Plain HTTP: confidentiality comes from the envelope.
const DefaultBaseURL = "http://gw.amnezia.org:80/"

// DefaultTimeout per request, as in the official client.
const DefaultTimeout = 12 * time.Second

// maxResponse caps the response body.
const maxResponse = 4 << 20

// DialContext dials TCP connections; set it to route the gateway call through a proxy.
type DialContext func(ctx context.Context, network, addr string) (net.Conn, error)

// Client posts to the gateway.
type Client struct {
	BaseURL string
	// Dial, when set, is used for every connection (e.g. through the user's own server).
	Dial    DialContext
	Timeout time.Duration
}

func (c *Client) httpClient() *http.Client {
	t := &http.Transport{
		Proxy:                 nil, // never pick up system proxy settings implicitly
		ResponseHeaderTimeout: c.timeout(),
		DisableKeepAlives:     true,
	}
	if c.Dial != nil {
		t.DialContext = c.Dial
	} else {
		t.DialContext = (&net.Dialer{Timeout: c.timeout()}).DialContext
	}
	return &http.Client{Transport: t, Timeout: c.timeout()}
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}

func (c *Client) url(path string) string {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// Post sends payload (marshalled to JSON) to path, encrypted to pub, and returns the decrypted JSON answer.
// Answers with http_status >= 300 become *APIError. An answer that cannot be decrypted is ErrWrongKey.
func (c *Client) Post(ctx context.Context, pub *rsa.PublicKey, path string, payload any) (json.RawMessage, error) {
	pt, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	sess, err := NewSession()
	if err != nil {
		return nil, err
	}
	body, err := sess.Seal(pub, pt)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url(path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-Request-ID", newUUID())
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	// The HTTP status is not reliable: always try to decrypt.
	doc, err := sess.Open(raw)
	if err != nil || !json.Valid(doc) {
		return nil, ErrWrongKey
	}
	if err := checkStatus(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func checkStatus(doc []byte) error {
	var head struct {
		HTTPStatus *int   `json:"http_status"`
		Message    string `json:"message"`
		Captcha
	}
	if err := json.Unmarshal(doc, &head); err != nil {
		return nil // not an object: let the caller decide
	}
	if head.HTTPStatus == nil || *head.HTTPStatus < 300 { // 201 is success
		return nil
	}
	e := &APIError{Status: *head.HTTPStatus, Message: head.Message}
	hasCaptcha := head.ID != "" || head.ImageBase64 != ""
	e.Kind = classify(e.Status, e.Message, hasCaptcha)
	if hasCaptcha {
		c := head.Captcha
		e.Captcha = &c
	}
	return e
}

// PostAny tries each key in order and returns the answer plus the index of the key that worked.
// Only ErrWrongKey moves on to the next key; any other error is returned immediately.
func (c *Client) PostAny(ctx context.Context, keys []*rsa.PublicKey, path string, payload any) (json.RawMessage, int, error) {
	if len(keys) == 0 {
		return nil, -1, errors.New("no gateway public key")
	}
	for i, k := range keys {
		doc, err := c.Post(ctx, k, path, payload)
		if errors.Is(err, ErrWrongKey) {
			continue
		}
		return doc, i, err
	}
	return nil, -1, ErrWrongKey
}

func newUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NewUUID returns a random RFC 4122 v4 UUID.
func NewUUID() string { return newUUID() }

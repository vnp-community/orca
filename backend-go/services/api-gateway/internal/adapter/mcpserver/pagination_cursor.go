package mcpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// ErrInvalidCursor is returned for any cursor that is malformed, forged,
// expired, for another list kind or for another session.
var ErrInvalidCursor = errors.New("mcpserver: invalid cursor")

const cursorTTL = 10 * time.Minute

type cursorPayload struct {
	K string `json:"k"` // list kind, e.g. "tools"
	O int    `json:"o"` // offset of the next item
	S string `json:"s"` // MCP session id the cursor was issued to
	E int64  `json:"e"` // expiry, unix seconds
}

// CursorCodec issues and verifies HMAC-SHA256 signed opaque cursors. keys[0]
// signs; every key verifies, so rotation (current + previous) keeps in-flight
// cursors valid. All replicas must share the same keys.
type CursorCodec struct {
	keys [][]byte
	now  func() time.Time
}

// NewCursorCodec requires at least one non-empty key.
func NewCursorCodec(keys ...[]byte) (*CursorCodec, error) {
	var ks [][]byte
	for _, k := range keys {
		if len(k) > 0 {
			ks = append(ks, k)
		}
	}
	if len(ks) == 0 {
		return nil, errors.New("mcpserver: cursor codec needs at least one key")
	}
	return &CursorCodec{keys: ks, now: time.Now}, nil
}

func (c *CursorCodec) sign(key, payload []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(payload)
	return m.Sum(nil)
}

// Encode returns an opaque cursor for the item at offset of list kind.
func (c *CursorCodec) Encode(kind, sessionID string, offset int) string {
	p, _ := json.Marshal(cursorPayload{K: kind, O: offset, S: sessionID, E: c.now().Add(cursorTTL).Unix()})
	enc := base64.RawURLEncoding
	return enc.EncodeToString(p) + "." + enc.EncodeToString(c.sign(c.keys[0], p))
}

// Decode verifies cursor and returns its offset. An empty cursor means the
// first page (offset 0).
func (c *CursorCodec) Decode(kind, sessionID, cursor string) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	body, sig, ok := strings.Cut(cursor, ".")
	if !ok {
		return 0, ErrInvalidCursor
	}
	enc := base64.RawURLEncoding
	p, err1 := enc.DecodeString(body)
	s, err2 := enc.DecodeString(sig)
	if err1 != nil || err2 != nil {
		return 0, ErrInvalidCursor
	}
	valid := false
	for _, k := range c.keys {
		if hmac.Equal(s, c.sign(k, p)) {
			valid = true
		}
	}
	if !valid {
		return 0, ErrInvalidCursor
	}
	var cp cursorPayload
	if json.Unmarshal(p, &cp) != nil || cp.K != kind || cp.S != sessionID || cp.O < 0 || c.now().Unix() > cp.E {
		return 0, ErrInvalidCursor
	}
	return cp.O, nil
}

// Paginate slices items by the cursor and returns the next cursor ("" on the
// last page). An invalid cursor yields a JSON-RPC -32602 error, as MCP
// requires. Reuse it for every list method (tools, resources, prompts).
func Paginate[T any](c *CursorCodec, kind, sessionID, cursor string, items []T, pageSize int) ([]T, string, error) {
	off, err := c.Decode(kind, sessionID, cursor)
	if err != nil || off > len(items) {
		return nil, "", &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "invalid cursor"}
	}
	if pageSize <= 0 {
		return nil, "", fmt.Errorf("mcpserver: page size must be positive")
	}
	end := off + pageSize
	if end >= len(items) {
		return append([]T{}, items[off:]...), "", nil
	}
	return append([]T{}, items[off:end]...), c.Encode(kind, sessionID, end), nil
}

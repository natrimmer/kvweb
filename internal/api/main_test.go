package api_test

import (
	"encoding/json"
	"testing"

	"github.com/natrimmer/kvweb/internal/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// keyResponse mirrors the JSON that GET /api/key/{key} returns. Value stays raw
// because its shape depends on the key's type.
type keyResponse struct {
	Key        string          `json:"key"`
	Type       string          `json:"type"`
	Value      json.RawMessage `json:"value"`
	TTL        int64           `json:"ttl"`
	Memory     int64           `json:"memory"`
	Length     int64           `json:"length"`
	Encoding   string          `json:"encoding"`
	Pagination *pagination     `json:"pagination"`
}

type pagination struct {
	Page       int64           `json:"page"`
	PageSize   int64           `json:"pageSize"`
	Total      int64           `json:"total"`
	HasMore    bool            `json:"hasMore"`
	NextCursor json.RawMessage `json:"nextCursor"`
}

type hashPair struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

type zMember struct {
	Member string  `json:"member"`
	Score  float64 `json:"score"`
}

type geoMember struct {
	Member    string  `json:"member"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

type streamEntry struct {
	ID     string            `json:"id"`
	Fields map[string]string `json:"fields"`
}

type keyMeta struct {
	Key  string `json:"key"`
	Type string `json:"type"`
	TTL  int64  `json:"ttl"`
}

type prefixEntry struct {
	Prefix  string `json:"prefix"`
	Count   int    `json:"count"`
	IsLeaf  bool   `json:"isLeaf"`
	FullKey string `json:"fullKey"`
	Type    string `json:"type"`
}

// decodeValue unmarshals a key response's value into v.
func decodeValue[T any](t *testing.T, resp keyResponse) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(resp.Value, &v); err != nil {
		t.Fatalf("could not decode the %q value: %v\nraw: %s", resp.Type, err, resp.Value)
	}
	return v
}

// getKey fetches a key and decodes the envelope.
func getKey(t *testing.T, h *testenv.Harness, key string, query ...string) keyResponse {
	t.Helper()
	path := h.KeyPath(key)
	if len(query) > 0 && query[0] != "" {
		path += "?" + query[0]
	}
	var resp keyResponse
	h.With(t).Get(path).ExpectOK().Decode(&resp)
	return resp
}

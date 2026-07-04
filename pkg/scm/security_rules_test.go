package scm_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/scm"
)

// testServer wires an auth handler plus a mux for the API, and points the
// package vars at it.
func testServer(t *testing.T, mux http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(mux)
	oldBase, oldAuth := scm.BaseURL, scm.AuthURL
	scm.BaseURL = srv.URL
	scm.AuthURL = srv.URL + "/auth"
	t.Cleanup(func() {
		srv.Close()
		scm.BaseURL, scm.AuthURL = oldBase, oldAuth
	})
	return srv
}

func newTestClient(t *testing.T) *scm.Client {
	t.Helper()
	c, err := scm.New(context.Background(), "cid", "sec", "123", "Mobile Users", false)
	require.NoError(t, err)
	return c
}

func TestListSecurityRulesPaginates(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/config/security/v1/security-rules", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Mobile Users", r.URL.Query().Get("folder"))
		require.Equal(t, "pre", r.URL.Query().Get("position"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		var page []map[string]interface{}
		if offset == 0 {
			page = []map[string]interface{}{{"id": "1", "name": "r1"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": page, "total": 1, "offset": offset, "limit": 200,
		})
	})

	testServer(t, mux)
	c := newTestClient(t)

	rules, err := c.ListSecurityRules("pre")
	require.NoError(t, err)
	require.Len(t, rules, 1)
	require.Equal(t, "r1", rules[0]["name"])
}

func TestListSecurityRulesMultiPage(t *testing.T) {
	const total = 250 // two pages: 200 (offset 0) + 50 (offset 200)
	var offsetsSeen []int
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/config/security/v1/security-rules", func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		offsetsSeen = append(offsetsSeen, offset)
		count := total - offset
		if count > 200 {
			count = 200
		}
		if count < 0 {
			count = 0
		}
		page := make([]map[string]interface{}, 0, count)
		for i := 0; i < count; i++ {
			page = append(page, map[string]interface{}{"id": strconv.Itoa(offset + i), "name": "r" + strconv.Itoa(offset+i)})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": page, "total": total, "offset": offset, "limit": 200,
		})
	})

	testServer(t, mux)
	c := newTestClient(t)

	rules, err := c.ListSecurityRules("pre")
	require.NoError(t, err)
	require.Len(t, rules, total, "all pages must be collected")
	require.Equal(t, []int{0, 200}, offsetsSeen, "offset must advance by page length, then stop")
	require.Equal(t, "0", rules[0]["id"])
	require.Equal(t, "249", rules[total-1]["id"])
}

func TestGetAndUpdateSecurityRule(t *testing.T) {
	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/config/security/v1/security-rules/abc", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": "abc", "folder": "Mobile Users", "name": "r", "action": "allow",
			})
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusOK)
		}
	})

	testServer(t, mux)
	c := newTestClient(t)

	obj, err := c.GetSecurityRule("abc")
	require.NoError(t, err)
	require.Equal(t, "allow", obj["action"])

	obj["action"] = "deny"
	require.NoError(t, c.UpdateSecurityRule("abc", obj))
	require.Equal(t, "deny", gotBody["action"])
	_, hasID := gotBody["id"]
	_, hasFolder := gotBody["folder"]
	require.False(t, hasID, "id must be stripped from PUT body")
	require.False(t, hasFolder, "folder must be stripped from PUT body")
}

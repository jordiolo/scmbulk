package scm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"scmbulk/pkg/scm"
)

func TestNewAuthenticates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		require.Equal(t, "tsg_id:123", r.Form.Get("scope"))
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "cid", user)
		require.Equal(t, "sec", pass)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tok-123"})
	}))
	defer srv.Close()

	old := scm.AuthURL
	scm.AuthURL = srv.URL
	defer func() { scm.AuthURL = old }()

	c, err := scm.New(context.Background(), "cid", "sec", "123", "Mobile Users", false)
	require.NoError(t, err)
	require.Equal(t, "tok-123", c.Token())
}

func TestNewAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad creds", http.StatusUnauthorized)
	}))
	defer srv.Close()

	old := scm.AuthURL
	scm.AuthURL = srv.URL
	defer func() { scm.AuthURL = old }()

	_, err := scm.New(context.Background(), "cid", "sec", "123", "f", false)
	require.Error(t, err)
}

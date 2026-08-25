package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPlugin(config *configuration) *Plugin {
	p := &Plugin{}
	p.setConfiguration(config)
	p.router = p.initRouter()
	return p
}

func TestGetConfigRequiresAuth(t *testing.T) {
	p := newTestPlugin(&configuration{})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	p.ServeHTTP(&plugin.Context{}, w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Result().StatusCode)
}

func TestGetConfigReturnsNormalizedConfig(t *testing.T) {
	raw := &configuration{
		RedmineURL:               " https://redmine.example.com/ ",
		DefaultProjectIdentifier: "toolbox ",
		DefaultTrackerID:         "1",
	}
	p := newTestPlugin(raw.normalized())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	r.Header.Set("Mattermost-User-ID", "user-1")
	p.ServeHTTP(&plugin.Context{}, w, r)

	result := w.Result()
	defer result.Body.Close()
	require.Equal(t, http.StatusOK, result.StatusCode)

	var got clientConfig
	require.NoError(t, json.NewDecoder(result.Body).Decode(&got))
	assert.Equal(t, clientConfig{
		RedmineURL:        "https://redmine.example.com",
		ProjectIdentifier: "toolbox",
		TrackerID:         "1",
	}, got)
}

func TestGetConfigEmptyWhenUnconfigured(t *testing.T) {
	p := newTestPlugin(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	r.Header.Set("Mattermost-User-ID", "user-1")
	p.ServeHTTP(&plugin.Context{}, w, r)

	var got clientConfig
	require.NoError(t, json.NewDecoder(w.Result().Body).Decode(&got))
	assert.Equal(t, clientConfig{}, got)
}

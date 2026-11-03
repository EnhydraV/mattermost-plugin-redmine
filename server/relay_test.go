package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	relayBot      = &model.User{Id: "bot-id", Username: "redmine", IsBot: true}
	relayCharlie  = &model.User{Id: "charlie-id", Username: "charlie", Email: "charlie@example.com"}
	relayJuliette = &model.User{Id: "juliette-id", Username: "juliette"}
	relayTeam     = &model.Team{Id: "team-id", Name: "dev"}
	relayChannel  = &model.Channel{Id: "channel-id", TeamId: "team-id", Name: "redmine"}
	relayNotFound = model.NewAppError("test", "not_found", nil, "", http.StatusNotFound)
)

const relayChannelBody = `{
	"target": {"team": "dev", "channel": "redmine"},
	"actor": {"username": "charlie", "email": "charlie@example.com"},
	"post": {"message": "Hello", "props": {"attachments": [{"fallback": "Bug #1", "fields": [{"title": "Status", "value": "New", "short": true}]}]}}
}`

func newRelayPlugin(t *testing.T) (*Plugin, *plugintest.API) {
	t.Helper()
	api := &plugintest.API{}
	p := &Plugin{}
	p.SetAPI(api)
	p.setConfiguration(&configuration{RelayUsernames: "redmine", RelayMatchByEmail: true})
	p.router = p.initRouter()
	t.Cleanup(func() { api.AssertExpectations(t) })
	return p, api
}

func serveRelay(p *Plugin, method, path, userID, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID != "" {
		r.Header.Set("Mattermost-User-Id", userID)
	}
	w := httptest.NewRecorder()
	p.ServeHTTP(nil, w, r)
	return w
}

func decodeRelayResponse(t *testing.T, w *httptest.ResponseRecorder) relayResponse {
	t.Helper()
	var response relayResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	return response
}

func expectRelayChannel(api *plugintest.API) {
	api.On("GetTeamByName", "dev").Return(relayTeam, nil)
	api.On("GetChannelByName", "team-id", "redmine", false).Return(relayChannel, nil)
}

func TestRelayUnauthenticatedRequestIsRejected(t *testing.T) {
	p, _ := newRelayPlugin(t)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "", relayChannelBody)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRelayOtherAccountIsForbidden(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "charlie-id").Return(relayCharlie, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "charlie-id", relayChannelBody)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRelayIsDisabledWithoutAllowedAccount(t *testing.T) {
	p, api := newRelayPlugin(t)
	p.setConfiguration(&configuration{})
	api.On("GetUser", "bot-id").Return(relayBot, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRelayInvalidBodyIsRejected(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", `{"target": {"team": "dev"}, "post": {"message": "Hello"}}`)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRelayPostsAsActorWhenActorCanPost(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(relayCharlie, nil)
	expectRelayChannel(api)
	api.On("HasPermissionToChannel", "charlie-id", "channel-id", model.PermissionCreatePost).Return(true)
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		attachments := post.Attachments()
		return post.UserId == "charlie-id" &&
			post.ChannelId == "channel-id" &&
			post.Message == "Hello" &&
			len(attachments) == 1 &&
			attachments[0].Fallback == "Bug #1" &&
			len(attachments[0].Fields) == 1 &&
			bool(attachments[0].Fields[0].Short)
	})).Return(&model.Post{Id: "post-id", UserId: "charlie-id"}, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, relayResponse{PostID: "post-id", UserID: "charlie-id", AsActor: true}, decodeRelayResponse(t, w))
}

func TestRelayFallsBackToBotWhenActorCannotPost(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(relayCharlie, nil)
	expectRelayChannel(api)
	api.On("HasPermissionToChannel", "charlie-id", "channel-id", model.PermissionCreatePost).Return(false)
	api.On("HasPermissionToChannel", "bot-id", "channel-id", model.PermissionCreatePost).Return(true)
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.UserId == "bot-id"
	})).Return(&model.Post{Id: "post-id", UserId: "bot-id"}, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.False(t, decodeRelayResponse(t, w).AsActor)
}

func TestRelayRefusedWhenNobodyCanPost(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(relayCharlie, nil)
	expectRelayChannel(api)
	api.On("HasPermissionToChannel", mock.Anything, "channel-id", model.PermissionCreatePost).Return(false)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	assert.Equal(t, http.StatusForbidden, w.Code)
	api.AssertNotCalled(t, "CreatePost", mock.Anything)
}

func TestRelayActorIsFoundByEmail(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(nil, relayNotFound)
	api.On("GetUserByEmail", "charlie@example.com").Return(relayCharlie, nil)
	expectRelayChannel(api)
	api.On("HasPermissionToChannel", "charlie-id", "channel-id", model.PermissionCreatePost).Return(true)
	api.On("CreatePost", mock.Anything).Return(&model.Post{Id: "post-id", UserId: "charlie-id"}, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.True(t, decodeRelayResponse(t, w).AsActor)
}

func TestRelayEmailMatchingCanBeDisabled(t *testing.T) {
	p, api := newRelayPlugin(t)
	p.setConfiguration(&configuration{RelayUsernames: "redmine"})
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(nil, relayNotFound)
	expectRelayChannel(api)
	api.On("HasPermissionToChannel", "bot-id", "channel-id", model.PermissionCreatePost).Return(true)
	api.On("CreatePost", mock.Anything).Return(&model.Post{Id: "post-id", UserId: "bot-id"}, nil)

	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", relayChannelBody)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	api.AssertNotCalled(t, "GetUserByEmail", mock.Anything)
}

func TestRelayDirectMessageIsSentByActor(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)
	api.On("GetUserByUsername", "charlie").Return(relayCharlie, nil)
	api.On("GetUserByUsername", "juliette").Return(relayJuliette, nil)
	api.On("GetDirectChannel", "charlie-id", "juliette-id").Return(&model.Channel{Id: "dm-id"}, nil)
	api.On("CreatePost", mock.MatchedBy(func(post *model.Post) bool {
		return post.UserId == "charlie-id" && post.ChannelId == "dm-id"
	})).Return(&model.Post{Id: "post-id", UserId: "charlie-id"}, nil)

	body := `{"target": {"username": "Juliette"}, "actor": {"username": "charlie"}, "post": {"message": "Hello"}}`
	w := serveRelay(p, http.MethodPost, "/api/v1/posts", "bot-id", body)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestRelayStatusReturnsPluginVersion(t *testing.T) {
	p, api := newRelayPlugin(t)
	api.On("GetUser", "bot-id").Return(relayBot, nil)

	w := serveRelay(p, http.MethodGet, "/api/v1/status", "bot-id", "")

	require.Equal(t, http.StatusOK, w.Code)
	var status map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &status))
	assert.Equal(t, map[string]string{"id": "com.cubedesigners.redmine", "version": manifest.Version}, status)
}

func TestRelayUnknownRouteIsNotFound(t *testing.T) {
	p, _ := newRelayPlugin(t)

	w := serveRelay(p, http.MethodGet, "/api/v1/unknown", "bot-id", "")

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRelayAllowedUsernames(t *testing.T) {
	cfg := (&configuration{RelayUsernames: " redmine, @Redmine-Preprod ,"}).normalized()

	assert.True(t, cfg.isRelayAllowed("redmine"))
	assert.True(t, cfg.isRelayAllowed("redmine-preprod"))
	assert.False(t, cfg.isRelayAllowed("victor"))
	assert.False(t, (&configuration{}).isRelayAllowed(""))
}

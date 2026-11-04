package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testReplySecret = "secret-of-romeo"

var (
	notificationRoot = &model.Post{Id: "root-1", ChannelId: "channel-id", Props: model.StringInterface{redmineIssueProp: float64(42)}}
	juliette         = &model.User{Id: "juliette-id", Username: "juliette", Email: "juliette@example.com"}
)

// newRepliesPlugin branche le plugin sur un faux Redmine qui répond status et note la requête reçue.
func newRepliesPlugin(t *testing.T, status int, answer string) (*Plugin, *plugintest.API, *replyRequest) {
	t.Helper()
	received := &replyRequest{}
	redmine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, repliesPath, r.URL.Path)
		assert.Equal(t, testReplySecret, r.Header.Get(replySecretHeader))
		assert.NoError(t, json.NewDecoder(r.Body).Decode(received))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(answer))
	}))
	t.Cleanup(redmine.Close)

	api := &plugintest.API{}
	p := &Plugin{}
	p.SetAPI(api)
	p.setConfiguration(&configuration{RedmineURL: redmine.URL, RedmineReplySecret: testReplySecret})
	t.Cleanup(func() { api.AssertExpectations(t) })
	return p, api, received
}

func reply(message string) *model.Post {
	return &model.Post{Id: "reply-1", RootId: "root-1", ChannelId: "channel-id", UserId: "juliette-id", Message: message}
}

func TestReplyBecomesANoteAndLeavesTheThread(t *testing.T) {
	p, api, received := newRepliesPlugin(t, http.StatusCreated, `{"journal_id": 7}`)
	api.On("GetPost", "root-1").Return(notificationRoot, nil)
	api.On("GetUser", "juliette-id").Return(juliette, nil)
	api.On("DeletePost", "reply-1").Return(nil)

	p.MessageHasBeenPosted(nil, reply("Fixed on my side"))

	assert.Equal(t, replyRequest{IssueID: "42", Username: "juliette", Email: "juliette@example.com",
		Message: "Fixed on my side", RootID: "root-1", ChannelID: "channel-id"}, *received)
}

func TestRefusedReplyStaysAndWarnsItsAuthor(t *testing.T) {
	p, api, _ := newRepliesPlugin(t, http.StatusForbidden, `{"message": "not allowed to add notes to this issue"}`)
	api.On("GetPost", "root-1").Return(notificationRoot, nil)
	api.On("GetUser", "juliette-id").Return(juliette, nil)
	api.On("SendEphemeralPost", "juliette-id", mock.MatchedBy(func(post *model.Post) bool {
		return post.RootId == "root-1" && post.ChannelId == "channel-id"
	})).Return(&model.Post{})

	p.MessageHasBeenPosted(nil, reply("Fixed on my side"))

	api.AssertNotCalled(t, "DeletePost", mock.Anything)
}

func TestReplyToAnOrdinaryThreadIsIgnored(t *testing.T) {
	p, api, _ := newRepliesPlugin(t, http.StatusCreated, `{}`)
	api.On("GetPost", "root-1").Return(&model.Post{Id: "root-1"}, nil)

	p.MessageHasBeenPosted(nil, reply("Just chatting"))

	api.AssertNotCalled(t, "DeletePost", mock.Anything)
}

func TestNotificationsAndRootPostsAreIgnored(t *testing.T) {
	p, api, _ := newRepliesPlugin(t, http.StatusCreated, `{}`)

	notification := reply("Issue updated")
	notification.AddProp(redmineIssueProp, float64(42))
	p.MessageHasBeenPosted(nil, notification)
	p.MessageHasBeenPosted(nil, &model.Post{Id: "post-2", ChannelId: "channel-id", Message: "New topic"})

	api.AssertNotCalled(t, "GetPost", mock.Anything)
}

func TestRepliesAreIgnoredWithoutSecret(t *testing.T) {
	p, api, _ := newRepliesPlugin(t, http.StatusCreated, `{}`)
	p.setConfiguration(&configuration{RedmineURL: "https://redmine.example.com"})

	p.MessageHasBeenPosted(nil, reply("Fixed"))

	api.AssertNotCalled(t, "GetPost", mock.Anything)
}

func TestReplyWithFilesStaysInTheThread(t *testing.T) {
	p, api, _ := newRepliesPlugin(t, http.StatusCreated, `{}`)
	api.On("GetPost", "root-1").Return(notificationRoot, nil)
	api.On("GetUser", "juliette-id").Return(juliette, nil)
	api.On("SendEphemeralPost", "juliette-id", mock.Anything).Return(&model.Post{})

	withFile := reply("See the screenshot")
	withFile.FileIds = model.StringArray{"file-1"}
	p.MessageHasBeenPosted(nil, withFile)

	api.AssertNotCalled(t, "DeletePost", mock.Anything)
}

func TestIssueIDOf(t *testing.T) {
	for value, want := range map[any]string{float64(42): "42", 7: "7", int64(9): "9", "12": "12", json.Number("5"): "5",
		float64(1.5): "", "abc": "", nil: "", -3: ""} {
		assert.Equal(t, want, issueIDOf(value), "value %v", value)
	}
}

func TestReplyRefusalMessages(t *testing.T) {
	require.Contains(t, replyRefusal(http.StatusUnprocessableEntity, "no Redmine account for this Mattermost user"), "aucun compte Redmine")
	require.Contains(t, replyRefusal(http.StatusBadGateway, ""), "502")
}

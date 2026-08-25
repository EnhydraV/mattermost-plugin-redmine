package command

import (
	"errors"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"
)

type fakeStore struct {
	projects map[string]string
	fail     bool
}

func (f *fakeStore) GetProject(channelID string) (string, error) {
	if f.fail {
		return "", errors.New("kv down")
	}
	return f.projects[channelID], nil
}

func (f *fakeStore) SetProject(channelID, identifier string) error {
	if f.fail {
		return errors.New("kv down")
	}
	f.projects[channelID] = identifier
	return nil
}

func (f *fakeStore) DeleteProject(channelID string) error {
	if f.fail {
		return errors.New("kv down")
	}
	delete(f.projects, channelID)
	return nil
}

func newHandler(store *fakeStore, admin bool, defaultProject string) *Handler {
	return NewHandler(Deps{
		Store:            store,
		CanManageChannel: func(_, _ string) bool { return admin },
		DefaultProject:   func() string { return defaultProject },
	})
}

func run(h *Handler, command string) *model.CommandResponse {
	return h.Handle(&model.CommandArgs{Command: command, UserId: "u1", ChannelId: "c1"})
}

func TestValidProjectIdentifier(t *testing.T) {
	assert.True(t, ValidProjectIdentifier("toolbox"))
	assert.True(t, ValidProjectIdentifier("mon-projet_2"))
	assert.False(t, ValidProjectIdentifier("Toolbox"))
	assert.False(t, ValidProjectIdentifier("123"))
	assert.False(t, ValidProjectIdentifier("a b"))
	assert.False(t, ValidProjectIdentifier(""))
}

func TestLinkAsChannelAdmin(t *testing.T) {
	store := &fakeStore{projects: map[string]string{}}
	resp := run(newHandler(store, true, ""), "/redmine link toolbox")

	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
	assert.Contains(t, resp.Text, "`toolbox`")
	assert.Equal(t, "toolbox", store.projects["c1"])
}

func TestLinkRefusedForNonAdmin(t *testing.T) {
	store := &fakeStore{projects: map[string]string{}}
	resp := run(newHandler(store, false, ""), "/redmine link toolbox")

	assert.Contains(t, resp.Text, "administrateurs")
	assert.Empty(t, store.projects)
}

func TestLinkRejectsInvalidIdentifier(t *testing.T) {
	store := &fakeStore{projects: map[string]string{}}
	resp := run(newHandler(store, true, ""), "/redmine link Tool box")

	assert.Contains(t, resp.Text, "pas un identifiant")
	assert.Empty(t, store.projects)
}

func TestLinkWithoutIdentifier(t *testing.T) {
	resp := run(newHandler(&fakeStore{projects: map[string]string{}}, true, ""), "/redmine link")
	assert.Contains(t, resp.Text, "manque")
}

func TestUnlink(t *testing.T) {
	store := &fakeStore{projects: map[string]string{"c1": "toolbox"}}
	resp := run(newHandler(store, true, ""), "/redmine unlink")

	assert.Contains(t, resp.Text, "plus lié")
	assert.Empty(t, store.projects)
}

func TestStatus(t *testing.T) {
	linked := &fakeStore{projects: map[string]string{"c1": "toolbox"}}
	assert.Contains(t, run(newHandler(linked, false, "def"), "/redmine status").Text, "`toolbox`")

	empty := &fakeStore{projects: map[string]string{}}
	assert.Contains(t, run(newHandler(empty, false, "def"), "/redmine status").Text, "`def`")
	assert.Contains(t, run(newHandler(empty, false, ""), "/redmine status").Text, "aucun projet par défaut")
}

func TestStoreErrorIsReported(t *testing.T) {
	resp := run(newHandler(&fakeStore{fail: true}, true, ""), "/redmine status")
	assert.Contains(t, resp.Text, "kv down")
}

func TestUsageOnUnknownSubcommand(t *testing.T) {
	assert.Contains(t, run(newHandler(&fakeStore{}, true, ""), "/redmine plop").Text, "Commandes disponibles")
	assert.Contains(t, run(newHandler(&fakeStore{}, true, ""), "/redmine").Text, "Commandes disponibles")
}

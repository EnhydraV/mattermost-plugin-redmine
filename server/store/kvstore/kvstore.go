package kvstore

import (
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

const channelProjectKeyPrefix = "redmine_channel_"

// ChannelProjects persiste le mapping canal Mattermost -> identifiant de projet Redmine.
type ChannelProjects interface {
	GetProject(channelID string) (string, error)
	SetProject(channelID, projectIdentifier string) error
	DeleteProject(channelID string) error
}

type Client struct {
	client *pluginapi.Client
}

func NewChannelProjects(client *pluginapi.Client) ChannelProjects {
	return Client{client: client}
}

func channelProjectKey(channelID string) string {
	return channelProjectKeyPrefix + channelID
}

// GetProject renvoie "" sans erreur quand aucun projet n'est lié.
func (kv Client) GetProject(channelID string) (string, error) {
	var project string
	if err := kv.client.KV.Get(channelProjectKey(channelID), &project); err != nil {
		return "", errors.Wrap(err, "failed to get channel project")
	}
	return project, nil
}

func (kv Client) SetProject(channelID, projectIdentifier string) error {
	if _, err := kv.client.KV.Set(channelProjectKey(channelID), projectIdentifier); err != nil {
		return errors.Wrap(err, "failed to set channel project")
	}
	return nil
}

func (kv Client) DeleteProject(channelID string) error {
	if err := kv.client.KV.Delete(channelProjectKey(channelID)); err != nil {
		return errors.Wrap(err, "failed to delete channel project")
	}
	return nil
}

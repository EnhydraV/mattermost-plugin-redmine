package main

import (
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"

	"github.com/EnhydraV/mattermost-plugin-redmine/server/command"
	"github.com/EnhydraV/mattermost-plugin-redmine/server/store/kvstore"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// client is the Mattermost server API client.
	client *pluginapi.Client

	// channelProjects est le mapping canal -> projet Redmine, en KV.
	channelProjects kvstore.ChannelProjects

	// commandHandler traite /redmine.
	commandHandler *command.Handler

	// router is the HTTP router for handling API requests.
	router *mux.Router

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration
}

// OnActivate is invoked when the plugin is activated. If an error is returned, the plugin will be deactivated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	p.channelProjects = kvstore.NewChannelProjects(p.client)
	p.router = p.initRouter()

	if err := command.Register(p.client); err != nil {
		return errors.Wrap(err, "failed to register /redmine command")
	}
	p.commandHandler = command.NewHandler(command.Deps{
		Store:            p.channelProjects,
		CanManageChannel: p.canManageChannel,
		DefaultProject: func() string {
			return p.getConfiguration().DefaultProjectIdentifier
		},
	})

	return nil
}

// ExecuteCommand handles /redmine.
func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	return p.commandHandler.Handle(args), nil
}

// canManageChannel : admins de canal (et donc admins système) uniquement.
func (p *Plugin) canManageChannel(userID, channelID string) bool {
	return p.API.HasPermissionToChannel(userID, channelID, model.PermissionManageChannelRoles)
}

// projectForChannel applique l'ordre du §6 : mapping du canal, puis projet par défaut, sinon "".
func (p *Plugin) projectForChannel(channelID string) string {
	if channelID != "" && p.channelProjects != nil {
		project, err := p.channelProjects.GetProject(channelID)
		if err != nil {
			p.API.LogWarn("Failed to read channel project mapping", "channel_id", channelID, "error", err.Error())
		} else if project != "" {
			return project
		}
	}
	return p.getConfiguration().DefaultProjectIdentifier
}

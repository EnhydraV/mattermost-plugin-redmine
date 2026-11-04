package main

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
)

// configuration reflète le settings_schema de plugin.json. Seul RedmineReplySecret est un secret.
type configuration struct {
	RedmineURL               string
	DefaultProjectIdentifier string
	DefaultTrackerID         string

	// RelayUsernames liste les comptes (le bot de Redmine) autorisés à publier par le relais.
	RelayUsernames string
	// RelayMatchByEmail cherche l'utilisateur par email quand son login Redmine ne correspond à aucun compte.
	RelayMatchByEmail bool

	// RedmineReplySecret est partagé avec Redmine (REDMINE_MATTERMOST_REPLY_SECRET) : vide, les
	// réponses aux notifications restent dans Mattermost.
	RedmineReplySecret string
}

// Clone shallow copies the configuration.
func (c *configuration) Clone() *configuration {
	clone := *c
	return &clone
}

// normalized renvoie une copie aux valeurs nettoyées (espaces, slash final).
func (c *configuration) normalized() *configuration {
	clone := c.Clone()
	clone.RedmineURL = strings.TrimRight(strings.TrimSpace(clone.RedmineURL), "/")
	clone.DefaultProjectIdentifier = strings.TrimSpace(clone.DefaultProjectIdentifier)
	clone.DefaultTrackerID = strings.TrimSpace(clone.DefaultTrackerID)
	clone.RelayUsernames = strings.TrimSpace(clone.RelayUsernames)
	clone.RedmineReplySecret = strings.TrimSpace(clone.RedmineReplySecret)
	return clone
}

// isRelayAllowed indique si le compte appelant fait partie des comptes autorisés à utiliser le relais.
func (c *configuration) isRelayAllowed(username string) bool {
	for name := range strings.SplitSeq(c.RelayUsernames, ",") {
		name = strings.TrimPrefix(strings.TrimSpace(name), "@")
		if name != "" && strings.EqualFold(name, username) {
			return true
		}
	}
	return false
}

func (p *Plugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

func (p *Plugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}
		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *Plugin) OnConfigurationChange() error {
	configuration := new(configuration)

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	p.setConfiguration(configuration.normalized())

	return nil
}

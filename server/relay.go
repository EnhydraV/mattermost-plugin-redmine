package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// Relais des notifications du plugin Redmine redmine_mattermost : Redmine appelle ces routes avec
// le jeton de son bot, et le post est publié au nom de l'utilisateur qui a agi dans Redmine.
// Mattermost ne notifie personne de ses propres posts : l'utilisateur ne l'est donc pas de ce
// qu'il vient de faire. Le plugin ne parle toujours pas à Redmine, c'est Redmine qui l'appelle.

const relayMaxBodyBytes = 1 << 20

// relayRequest est le corps de POST /api/v1/posts envoyé par Redmine.
type relayRequest struct {
	Target relayTarget `json:"target"`
	Actor  *relayActor `json:"actor,omitempty"`
	Post   relayPost   `json:"post"`
}

// relayTarget désigne un canal (équipe + nom) ou un utilisateur pour un message direct.
type relayTarget struct {
	Team     string `json:"team,omitempty"`
	Channel  string `json:"channel,omitempty"`
	Username string `json:"username,omitempty"`
}

// relayActor identifie l'utilisateur Redmine à l'origine de l'événement.
type relayActor struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

type relayPost struct {
	Message string         `json:"message"`
	Props   map[string]any `json:"props,omitempty"`
}

type relayResponse struct {
	PostID  string `json:"post_id"`
	UserID  string `json:"user_id"`
	AsActor bool   `json:"as_actor"`
}

type httpError struct {
	status  int
	message string
}

func (e *httpError) Error() string {
	return e.message
}

func newHTTPError(status int, format string, args ...any) *httpError {
	return &httpError{status: status, message: fmt.Sprintf(format, args...)}
}

func (r *relayRequest) validate() *httpError {
	if strings.TrimSpace(r.Post.Message) == "" && len(r.Post.Props) == 0 {
		return newHTTPError(http.StatusBadRequest, "post.message or post.props is required")
	}
	if r.Target.Username == "" && (r.Target.Team == "" || r.Target.Channel == "") {
		return newHTTPError(http.StatusBadRequest, "target needs a username, or a team and a channel")
	}
	return nil
}

// authorizeRelay n'accepte que les requêtes authentifiées par le jeton d'un compte autorisé.
func (p *Plugin) authorizeRelay(r *http.Request) (*model.User, *httpError) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		return nil, newHTTPError(http.StatusUnauthorized, "authentication required")
	}
	user, appErr := p.API.GetUser(userID)
	if appErr != nil || user == nil || !p.getConfiguration().isRelayAllowed(user.Username) {
		return nil, newHTTPError(http.StatusForbidden, "this account is not allowed to use the Redmine relay")
	}
	return user, nil
}

func (p *Plugin) handleRelayStatus(w http.ResponseWriter, r *http.Request) {
	if _, herr := p.authorizeRelay(r); herr != nil {
		writeError(w, herr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"id": manifest.Id, "version": manifest.Version})
}

func (p *Plugin) handleRelayPost(w http.ResponseWriter, r *http.Request) {
	requester, herr := p.authorizeRelay(r)
	if herr != nil {
		writeError(w, herr)
		return
	}

	var req relayRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, relayMaxBodyBytes)).Decode(&req); err != nil {
		writeError(w, newHTTPError(http.StatusBadRequest, "invalid JSON body"))
		return
	}
	if herr = req.validate(); herr != nil {
		writeError(w, herr)
		return
	}

	post, herr := buildRelayPost(req.Post)
	if herr != nil {
		writeError(w, herr)
		return
	}

	actor := p.findActor(req.Actor)
	channel, poster, herr := p.resolveDestination(req.Target, requester, actor)
	if herr != nil {
		writeError(w, herr)
		return
	}

	post.UserId = poster.Id
	post.ChannelId = channel.Id
	created, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Failed to create the Redmine post", "error", appErr.Error())
		writeError(w, newHTTPError(statusOf(appErr), "%s", appErr.Message))
		return
	}

	writeJSON(w, http.StatusCreated, relayResponse{
		PostID:  created.Id,
		UserID:  created.UserId,
		AsActor: actor != nil && poster.Id == actor.Id,
	})
}

// buildRelayPost recopie le message et ses props ; les attachments sont convertis dans le type du serveur.
func buildRelayPost(source relayPost) (*model.Post, *httpError) {
	post := &model.Post{Message: source.Message}
	for key, value := range source.Props {
		if key != model.PostPropsAttachments {
			post.AddProp(key, value)
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, newHTTPError(http.StatusBadRequest, "invalid attachments")
		}
		var attachments []*model.MessageAttachment
		if err := json.Unmarshal(raw, &attachments); err != nil {
			return nil, newHTTPError(http.StatusBadRequest, "invalid attachments")
		}
		post.AddProp(model.PostPropsAttachments, attachments)
	}
	return post, nil
}

// findActor cherche le compte Mattermost actif correspondant à l'utilisateur Redmine.
func (p *Plugin) findActor(actor *relayActor) *model.User {
	if actor == nil {
		return nil
	}

	var user *model.User
	if name := normalizeName(actor.Username); name != "" {
		user, _ = p.API.GetUserByUsername(name)
	}
	if user == nil && strings.TrimSpace(actor.Email) != "" && p.getConfiguration().RelayMatchByEmail {
		user, _ = p.API.GetUserByEmail(strings.TrimSpace(actor.Email))
	}
	if user == nil || user.DeleteAt != 0 || user.IsBot {
		return nil
	}
	return user
}

// resolveDestination choisit le canal et l'auteur du post : l'utilisateur s'il peut publier, sinon le bot.
func (p *Plugin) resolveDestination(target relayTarget, requester, actor *model.User) (*model.Channel, *model.User, *httpError) {
	if name := normalizeName(target.Username); name != "" {
		recipient, appErr := p.API.GetUserByUsername(name)
		if appErr != nil || recipient == nil {
			return nil, nil, newHTTPError(http.StatusNotFound, "user %s not found", name)
		}
		poster := requester
		if actor != nil && actor.Id != recipient.Id {
			poster = actor
		}
		channel, appErr := p.API.GetDirectChannel(poster.Id, recipient.Id)
		if appErr != nil {
			return nil, nil, newHTTPError(statusOf(appErr), "%s", appErr.Message)
		}
		return channel, poster, nil
	}

	teamName := normalizeName(target.Team)
	team, appErr := p.API.GetTeamByName(teamName)
	if appErr != nil || team == nil {
		return nil, nil, newHTTPError(http.StatusNotFound, "team %s not found", teamName)
	}
	channelName := normalizeName(target.Channel)
	channel, appErr := p.API.GetChannelByName(team.Id, channelName, false)
	if appErr != nil || channel == nil {
		return nil, nil, newHTTPError(http.StatusNotFound, "channel %s/%s not found", teamName, channelName)
	}

	if actor != nil && p.API.HasPermissionToChannel(actor.Id, channel.Id, model.PermissionCreatePost) {
		return channel, actor, nil
	}
	// Le bot garde les droits qu'il aurait avec l'API REST : pas de publication dans un canal qui lui est fermé
	if p.API.HasPermissionToChannel(requester.Id, channel.Id, model.PermissionCreatePost) {
		return channel, requester, nil
	}
	return nil, nil, newHTTPError(http.StatusForbidden, "neither the user nor the bot can post in %s/%s", teamName, channelName)
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(name), "@"))
}

func statusOf(appErr *model.AppError) int {
	if appErr.StatusCode != 0 {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, herr *httpError) {
	writeJSON(w, herr.status, map[string]string{"message": herr.message})
}

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

const (
	// Images collées dans une note : 5 fichiers de 10 Mo au plus, transmis en base64.
	relayMaxFiles     = 5
	relayMaxFileBytes = 10 << 20
	// Le base64 grossit les fichiers d'un tiers ; 1 Mo pour le reste du post.
	relayMaxBodyBytes = 1<<20 + relayMaxFiles*relayMaxFileBytes*4/3
)

// relayRequest est le corps de POST /api/v1/posts envoyé par Redmine.
type relayRequest struct {
	Target relayTarget `json:"target"`
	Actor  *relayActor `json:"actor,omitempty"`
	Post   relayPost   `json:"post"`
	// BotMessage remplace le message quand le post part au nom du bot : le message
	// destiné à l'utilisateur ne nomme pas l'auteur, celui du bot le nomme.
	BotMessage string `json:"bot_message,omitempty"`
	// ReplyTo place le post dans le fil d'où vient la note, s'il vise le canal de ce fil.
	ReplyTo *relayReplyTo `json:"reply_to,omitempty"`
	// Files sont téléversés dans le canal de destination et joints au post.
	Files []relayFile `json:"files,omitempty"`
}

// relayFile : Data arrive en base64, décodé par encoding/json.
type relayFile struct {
	Name string `json:"name"`
	Data []byte `json:"data"`
}

type relayReplyTo struct {
	RootID    string `json:"root_id"`
	ChannelID string `json:"channel_id"`
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
	if len(r.Files) > relayMaxFiles {
		return newHTTPError(http.StatusBadRequest, "at most %d files", relayMaxFiles)
	}
	for _, file := range r.Files {
		if strings.TrimSpace(file.Name) == "" || len(file.Data) == 0 || len(file.Data) > relayMaxFileBytes {
			return newHTTPError(http.StatusBadRequest, "each file needs a name and at most %d bytes", relayMaxFileBytes)
		}
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
	if poster.Id == requester.Id && req.BotMessage != "" {
		post.Message = req.BotMessage
	}
	if req.ReplyTo != nil && req.ReplyTo.RootID != "" && req.ReplyTo.ChannelID == channel.Id {
		post.RootId = req.ReplyTo.RootID
	}
	if post.FileIds, herr = p.uploadRelayFiles(channel.Id, req.Files); herr != nil {
		writeError(w, herr)
		return
	}
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

// uploadRelayFiles téléverse les fichiers dans le canal. Un fichier téléversé par un plugin n'a pas
// de créateur ("nouser") : Mattermost accepte de le joindre au post de l'acteur comme à celui du bot.
// Un échec renvoie 422 : Redmine publie alors par le bot, avec des liens vers les images.
func (p *Plugin) uploadRelayFiles(channelID string, files []relayFile) (model.StringArray, *httpError) {
	if len(files) == 0 {
		return nil, nil
	}
	ids := make(model.StringArray, 0, len(files))
	for _, file := range files {
		info, appErr := p.API.UploadFile(file.Data, channelID, file.Name)
		if appErr != nil {
			p.API.LogError("Failed to upload a Redmine image", "file", file.Name, "error", appErr.Error())
			return nil, newHTTPError(http.StatusUnprocessableEntity, "upload of %s failed: %s", file.Name, appErr.Message)
		}
		ids = append(ids, info.Id)
	}
	return ids, nil
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

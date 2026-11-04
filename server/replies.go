package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// Réponses aux notifications de ticket : une réponse dans le fil d'une notification de
// redmine_mattermost devient une note du ticket, écrite par l'utilisateur Redmine qui
// correspond à l'auteur. Acceptée par Redmine, elle est retirée du fil, où la notification de
// la note prend sa place ; refusée, elle reste et son auteur reçoit un message éphémère.

const (
	redmineIssueProp   = "redmine_issue_id"
	repliesPath        = "/mattermost/replies"
	replySecretHeader  = "X-Redmine-Mattermost-Secret"
	replyMaxErrorBytes = 4096
)

// replyHTTPClient appelle Redmine ; remplacé dans les tests.
var replyHTTPClient = &http.Client{Timeout: 15 * time.Second}

type replyRequest struct {
	IssueID   string `json:"issue_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	RootID    string `json:"root_id"`
	ChannelID string `json:"channel_id"`
}

// MessageHasBeenPosted transmet à Redmine les réponses aux notifications de ticket.
func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	cfg := p.getConfiguration()
	if cfg.RedmineURL == "" || cfg.RedmineReplySecret == "" {
		return
	}
	// Nos propres notifications (dont celle de la note) portent la propriété : pas de boucle
	if post.RootId == "" || post.Type != "" || post.GetProp(redmineIssueProp) != nil {
		return
	}

	root, appErr := p.API.GetPost(post.RootId)
	if appErr != nil || root == nil {
		return
	}
	issueID := issueIDOf(root.GetProp(redmineIssueProp))
	if issueID == "" {
		return
	}

	author, appErr := p.API.GetUser(post.UserId)
	if appErr != nil || author == nil || author.IsBot {
		return
	}
	if len(post.FileIds) > 0 {
		p.warnAuthor(post, issueID, "les fichiers joints ne sont pas transmis, ajoute-les dans Redmine")
		return
	}

	refusal := p.sendReply(cfg, replyRequest{
		IssueID:   issueID,
		Username:  author.Username,
		Email:     author.Email,
		Message:   post.Message,
		RootID:    post.RootId,
		ChannelID: post.ChannelId,
	})
	if refusal != "" {
		p.warnAuthor(post, issueID, refusal)
		return
	}
	if appErr := p.API.DeletePost(post.Id); appErr != nil {
		p.API.LogWarn("Reply added to Redmine but not removed from the thread", "post_id", post.Id, "error", appErr.Error())
	}
}

// sendReply renvoie "" quand Redmine a ajouté la note, sinon la raison, lisible par l'auteur.
func (p *Plugin) sendReply(cfg *configuration, reply replyRequest) string {
	body, err := json.Marshal(reply)
	if err != nil {
		return "réponse illisible"
	}
	req, err := http.NewRequest(http.MethodPost, cfg.RedmineURL+repliesPath, bytes.NewReader(body))
	if err != nil {
		return "URL de Redmine invalide dans la configuration du plugin"
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(replySecretHeader, cfg.RedmineReplySecret)

	resp, err := replyHTTPClient.Do(req)
	if err != nil {
		p.API.LogWarn("Redmine unreachable for a reply", "error", err.Error())
		return "Redmine ne répond pas"
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusCreated {
		return ""
	}

	var answer struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, replyMaxErrorBytes)).Decode(&answer)
	return replyRefusal(resp.StatusCode, answer.Message)
}

func replyRefusal(status int, message string) string {
	switch status {
	case http.StatusForbidden:
		return "tu n'as pas le droit d'ajouter une note à ce ticket"
	case http.StatusUnprocessableEntity:
		if strings.Contains(message, "no Redmine account") {
			return "aucun compte Redmine ne correspond à ton nom d'utilisateur ou à ton email"
		}
		if message != "" {
			return "Redmine refuse la note (" + message + ")"
		}
	case http.StatusNotFound:
		return "ticket introuvable, ou réponses non activées côté Redmine"
	}
	return fmt.Sprintf("Redmine a répondu %d", status)
}

func (p *Plugin) warnAuthor(post *model.Post, issueID, reason string) {
	p.API.SendEphemeralPost(post.UserId, &model.Post{
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
		Message:   fmt.Sprintf("Réponse non ajoutée au ticket Redmine #%s : %s. Elle reste dans le fil.", issueID, reason),
	})
}

// issueIDOf accepte les formes que prend un nombre dans les props (JSON : float64).
func issueIDOf(value any) string {
	switch v := value.(type) {
	case float64:
		if v > 0 && v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
	case int:
		if v > 0 {
			return strconv.Itoa(v)
		}
	case int64:
		if v > 0 {
			return strconv.FormatInt(v, 10)
		}
	case json.Number:
		return issueIDOf(v.String())
	case string:
		if id, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && id > 0 {
			return strconv.FormatInt(id, 10)
		}
	}
	return ""
}

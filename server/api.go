package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// clientConfig est ce que le webapp a besoin de connaître pour construire l'URL du formulaire Redmine.
type clientConfig struct {
	RedmineURL        string `json:"redmine_url"`
	ProjectIdentifier string `json:"project_identifier"`
	TrackerID         string `json:"tracker_id"`
}

func (p *Plugin) initRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(p.MattermostAuthorizationRequired)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.HandleFunc("/config", p.handleGetConfig).Methods(http.MethodGet)

	return router
}

// ServeHTTP délègue au routeur.
func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

// MattermostAuthorizationRequired rejette toute requête sans utilisateur Mattermost authentifié.
func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleGetConfig renvoie la configuration applicable au canal passé en query (?channel_id=...).
func (p *Plugin) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := p.getConfiguration()

	response := clientConfig{
		RedmineURL:        config.RedmineURL,
		ProjectIdentifier: p.projectForChannel(r.URL.Query().Get("channel_id")),
		TrackerID:         config.DefaultTrackerID,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		p.API.LogError("Failed to write config response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

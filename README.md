# mattermost-plugin-redmine

Ajoute une entrée **« Créer un ticket Redmine »** au menu « … » d'un message Mattermost.
Le clic ouvre, dans un nouvel onglet, le formulaire natif de création de ticket Redmine,
prérempli avec le contenu du message (sujet, description, permalien vers le message).

Le plugin **n'appelle jamais l'API Redmine** : pas de clé API, pas d'authentification côté
plugin, pas de mapping d'identité. L'utilisateur arrive dans Redmine avec sa propre session,
le ticket est créé par lui et ses permissions s'appliquent nativement. C'est un choix
d'architecture délibéré, voir `HANDOFF.md`.

## Configuration (Console système > Plugins > Redmine)

| Réglage | Rôle |
|---|---|
| `RedmineURL` | URL de base de l'instance Redmine (sans slash final) |
| `DefaultProjectIdentifier` | Projet présélectionné quand le canal n'est lié à aucun projet ; vide = sélecteur de projet Redmine |
| `DefaultTrackerID` | Tracker présélectionné (obligatoire dès qu'un champ personnalisé est prérempli) |

## Développement

Prérequis : Go (version de `go.mod`), Node (version de `.nvmrc`), npm.

```
make dist          # bundle installable dans dist/
make test          # tests Go + webapp
make check-style   # golangci-lint, eslint, tsc
make deploy        # déploiement sur un serveur local (voir build/pluginctl)
```

Pour le déploiement local, définir `MM_SERVICESETTINGS_SITEURL` et `MM_ADMIN_TOKEN`
(ou `MM_ADMIN_USERNAME` / `MM_ADMIN_PASSWORD`) avant `make deploy`.

## Structure

- `server/` : Go minimal, expose la configuration (et le mapping canal → projet) au webapp via `GET /plugins/com.cubedesigners.redmine/api/v1/config`.
- `webapp/src/redmine/buildIssueUrl.ts` : construction de l'URL préremplie et budget de troncature (module pur, testé).
- `webapp/src/index.tsx` : enregistrement de l'action de menu post.

Basé sur [mattermost-plugin-starter-template](https://github.com/mattermost/mattermost-plugin-starter-template).

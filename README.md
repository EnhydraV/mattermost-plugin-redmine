# mattermost-plugin-redmine

Ajoute une entrée **« Créer un ticket Redmine »** au menu **Actions du message** d'un message
Mattermost (l'icône à gauche du « … » dans la barre de survol ; Mattermost ne l'affiche que si un
plugin y a enregistré une action).
Le clic ouvre, dans un nouvel onglet, le formulaire natif de création de ticket Redmine,
prérempli avec le contenu du message : sujet (première ligne), description (corps du
message, auteur, canal et permalien vers le message).

Le plugin **n'appelle jamais l'API Redmine** : pas de clé API, pas d'authentification côté
plugin, pas de mapping d'identité. L'utilisateur arrive dans Redmine avec sa propre session,
le ticket est créé par lui et ses permissions s'appliquent nativement. C'est un choix
d'architecture délibéré, détaillé dans `HANDOFF.md`.

## Prérequis

- Mattermost 9.0 ou plus récent (`min_server_version` du `plugin.json`).
- Redmine configuré en **Markdown ou CommonMark** (Administration > Paramètres > Formatage du
  texte). En Textile, le corps du message ressortira mal formaté.
- Les utilisateurs doivent avoir une session Redmine ouverte dans leur navigateur (sinon
  Redmine leur demande de se connecter, puis les redirige vers le formulaire).

## Installation

1. Récupérer le bundle `com.cubedesigners.redmine-<version>.tar.gz` (release GitHub ou `make dist`).
2. Console système > Plugins > Gestion des plugins > Téléverser le plugin.
3. Activer le plugin, puis le configurer (ci-dessous).

## Configuration (Console système > Plugins > Redmine)

| Réglage | Rôle |
|---|---|
| **URL de Redmine** | URL de base de l'instance, sans slash final. Obligatoire. |
| **Projet par défaut** | Identifiant du projet présélectionné quand le canal n'est lié à aucun projet. Vide : Redmine affiche son sélecteur de projet. |
| **Tracker par défaut (ID)** | ID numérique du tracker présélectionné. Optionnel. |

L'identifiant de projet est celui qui figure dans l'URL Redmine
(`https://redmine.example.com/projects/<identifiant>`).

## Utilisation

### Créer un ticket depuis un message

Survoler un message > icône **Actions du message** (à gauche du « … ») > **Créer un ticket
Redmine**. Un onglet s'ouvre sur le formulaire
Redmine, prérempli. Tout reste modifiable avant validation : sujet, description, tracker,
assigné, priorité, champs personnalisés, pièces jointes.

Le projet retenu est, dans l'ordre : le projet lié au canal, sinon le projet par défaut,
sinon aucun (Redmine demande alors de choisir).

Les messages très longs sont tronqués dans la description pour tenir dans une URL raisonnable
(2000 caractères) ; le permalien vers le message d'origine est toujours conservé, le contexte
complet reste donc à un clic.

### Lier un canal à un projet

Commande slash, réservée aux administrateurs du canal (et administrateurs système) :

```
/redmine link <identifiant-projet>   lier ce canal à un projet Redmine
/redmine unlink                      retirer le lien
/redmine status                      afficher le projet lié (ou le repli appliqué)
```

### Langue

L'interface (libellé du menu, messages d'erreur) suit la langue de l'utilisateur Mattermost :
français ou anglais, anglais par défaut. Les textes de la console système et des commandes
slash sont en français (le format `plugin.json` n'est pas localisable).

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

### Structure

- `server/` : Go minimal. Expose la configuration au webapp
  (`GET /plugins/com.cubedesigners.redmine/api/v1/config?channel_id=`), gère `/redmine` et
  le mapping canal → projet en KV (`server/store/kvstore`). Ne parle jamais à Redmine.
- `webapp/src/redmine/buildIssueUrl.ts` : construction de l'URL préremplie et budget de
  troncature (module pur, testé).
- `webapp/src/redmine/` : contexte du message (`postContext.ts`), appel de configuration
  (`fetchConfig.ts`), ouverture de l'onglet (`openIssueForm.ts`), i18n (`i18n.ts`, `messages.ts`).
- `webapp/src/index.tsx` : enregistrement de l'action de menu post.

### Release

Semver automatique par [Conventional Commits](https://www.conventionalcommits.org/) :

- `commitlint.yml` vérifie le format des messages sur chaque PR (`feat:`, `fix:`, `docs:`,
  `chore:`, `refactor:`, `ci:`, `test:`, `perf:` ; `feat!:` ou `BREAKING CHANGE:` pour un majeur).
- `release-please.yml` lit les commits poussés sur `master`, ouvre et maintient une PR de release
  (version calculée, `CHANGELOG.md`, `version.txt`). Au merge de cette PR, il pose le tag
  `vX.Y.Z`, crée la release GitHub, puis construit et y attache le bundle `dist/*.tar.gz`.
- Avant 1.0.0, un `feat` incrémente le mineur et un `fix` le correctif (`bump-minor-pre-major`).

`ci.yml` (workflow réutilisable `mattermost/actions-workflows`) lint, teste et construit à
chaque push et PR. Aucun tag n'est à poser à la main.

Basé sur [mattermost-plugin-starter-template](https://github.com/mattermost/mattermost-plugin-starter-template).

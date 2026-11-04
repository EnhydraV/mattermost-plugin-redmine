# mattermost-plugin-redmine

Ajoute une entrée **« Créer un ticket Redmine »** au menu **Actions du message** d'un message
Mattermost (l'icône à gauche du « … » dans la barre de survol ; Mattermost ne l'affiche que si un
plugin y a enregistré une action).
Le clic ouvre, dans un nouvel onglet, le formulaire natif de création de ticket Redmine,
prérempli avec le contenu du message : sujet (première ligne), description (corps du
message, auteur, canal et permalien vers le message).

Pour cette action, le plugin **n'appelle jamais l'API Redmine** : pas de clé API, pas
d'authentification côté plugin, pas de mapping d'identité. L'utilisateur arrive dans Redmine avec
sa propre session, le ticket est créé par lui et ses permissions s'appliquent nativement. C'est
un choix d'architecture délibéré, détaillé dans `HANDOFF.md`.

Le plugin sert aussi de **relais** au plugin Redmine
[redmine_mattermost](https://github.com/EnhydraV/redmine_mattermost), qui publie les
notifications de Redmine dans Mattermost : avec le relais, chaque notification est publiée au
nom de l'utilisateur qui a agi dans Redmine, qui n'est donc pas notifié de sa propre action.
C'est Redmine qui appelle le plugin ; le plugin ne parle toujours pas à Redmine.

## Prérequis

- Mattermost 12.0 ou plus récent (`min_server_version` du `plugin.json`).
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
| **Relais : compte bot de Redmine** | Nom d'utilisateur du bot dont Redmine utilise le jeton (plusieurs noms séparés par des virgules). Vide : relais désactivé. |
| **Relais : chercher les utilisateurs par email** | Quand aucun utilisateur n'a pour nom d'utilisateur le login Redmine, le chercher par email. Activé par défaut. |
| **Réponses : secret partagé avec Redmine** | Même valeur que `REDMINE_MATTERMOST_REPLY_SECRET` côté Redmine. Vide : les réponses aux notifications restent dans Mattermost. |

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

### Relayer les notifications de Redmine

1. Configurer d'abord le plugin Redmine `redmine_mattermost` avec son compte bot (voir son
   README) et vérifier que les notifications arrivent au nom du bot.
2. Dans la console système, renseigner **Relais : compte bot de Redmine** avec le nom
   d'utilisateur de ce bot.
3. Vérifier que le relais répond au jeton du bot :

   ```
   curl -s -H "Authorization: Bearer <jeton du bot>" https://chat.example.com/plugins/com.cubedesigners.redmine/api/v1/status
   ```

   Réponse attendue : `{"id":"com.cubedesigners.redmine","version":"<version>"}`. Un 403
   signale un nom de bot absent du réglage.
4. Dans Redmine, cocher **Publier au nom de l'utilisateur** dans la configuration du plugin.

L'utilisateur Mattermost retenu est celui dont le nom d'utilisateur est le login Redmine, ou à
défaut celui qui a la même adresse email. Le post est publié à son nom s'il peut écrire dans le
canal ; les messages directs partent de lui vers le destinataire. Sinon le bot publie, comme il
le fait quand le relais ne répond pas.

Le relais permet au jeton du bot de publier au nom de tout utilisateur autorisé à écrire dans le
canal visé : ce jeton doit rester hors de la base Redmine (variable d'environnement
`REDMINE_MATTERMOST_BOT_TOKEN`). Un canal peut rester affiché comme non lu pour l'auteur du post,
les posts créés par un plugin ne marquant pas le canal comme vu.

Routes, appelées par Redmine avec le jeton du bot :

- `POST /plugins/com.cubedesigners.redmine/api/v1/posts` : `{target: {team, channel} | {username}, actor: {username, email}, post: {message, props}}`, réponse `{post_id, user_id, as_actor}` ;
- `GET /plugins/com.cubedesigners.redmine/api/v1/status` : identifiant et version du plugin.

### Répondre à une notification de ticket

Une réponse écrite dans le fil d'une notification de ticket devient une note de ce ticket, au nom
de l'utilisateur Redmine qui a le même nom d'utilisateur, ou à défaut la même adresse email, et
qui a le droit d'ajouter des notes. Redmine l'accepte : la réponse est retirée du fil, et la
notification de la note y prend sa place. Redmine la refuse (compte introuvable, droit manquant) :
la réponse reste, et son auteur reçoit un message éphémère qui explique pourquoi. Une réponse avec
des fichiers joints n'est pas transmise.

Mise en place : générer un secret (`openssl rand -hex 32`), le donner à Redmine dans la variable
`REDMINE_MATTERMOST_REPLY_SECRET` et au plugin dans **Réponses : secret partagé avec Redmine**.
Le plugin appelle `POST {URL de Redmine}/mattermost/replies` avec l'en-tête
`X-Redmine-Mattermost-Secret`. Les modifications et suppressions ultérieures d'une réponse ne sont
pas répercutées.

### Langue

L'interface (libellé du menu, messages d'erreur) suit la langue de l'utilisateur Mattermost :
français ou anglais, anglais par défaut. Les textes de la console système et des commandes
slash sont en français (le format `plugin.json` n'est pas localisable).

## Développement

Prérequis : Go (version de `go.mod`), Node (version de `.nvmrc`), npm. Le `Makefile` impose la
chaîne Go 1.26.7 (`GOTOOLCHAIN`, téléchargée au besoin) : grpc, tiré par l'API plugin de
Mattermost 12, ne compile pas en Go 1.27. Hors `make`, préfixer les commandes Go de
`GOTOOLCHAIN=go1.26.7`.

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
- `server/relay.go` : relais des notifications de Redmine (`/api/v1/posts`, `/api/v1/status`).
- `server/replies.go` : hook `MessageHasBeenPosted`, réponses aux notifications envoyées à Redmine.
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

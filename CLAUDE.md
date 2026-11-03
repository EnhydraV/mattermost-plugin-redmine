# mattermost-plugin-redmine

Lire `HANDOFF.md` en entier avant toute proposition : il fixe l'architecture (URL préremplie,
jamais d'API Redmine ; relais des notifications appelé par Redmine) et le journal de session en
fin de fichier.

## Environnement

- Go : chaîne 1.26.7, imposée par `GOTOOLCHAIN` en tête du `Makefile`. Hors `make`, préfixer
  les commandes Go de `GOTOOLCHAIN=go1.26.7` : avec l'API plugin de Mattermost 12 (grpc 1.81),
  la compilation échoue en Go 1.27 (Go 1.27 de l'image superclaude dans `/usr/local/go`).
- golangci-lint : `GOTOOLCHAIN=go1.26.7 GOBIN=$PWD/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.9.0`,
  puis `./bin/golangci-lint run ./...`. Les 3 signalements `gofmt` sur `build/` viennent du CRLF
  du worktree (le contenu du dépôt est conforme).
- Node : `.nvmrc` (24.x), npm. Deps webapp : `cd webapp && npm ci`.
- Module Go : `github.com/EnhydraV/mattermost-plugin-redmine`. Id plugin : `com.cubedesigners.redmine`.
- Build complet : `make dist`. Tests Go : `go test ./server/...`. Webapp : `npm run check-types`,
  `npm run lint`, `npx jest` dans `webapp/`.
- Checkout Windows : les fichiers du worktree sont en CRLF (`git ls-files --eol`). `npm run lint`
  remonte donc des milliers de `linebreak-style` sans rapport avec le code ; ignorer, ne pas
  normaliser sans demande explicite. Préserver les fins de ligne existantes quand on édite.

## Conventions

- TypeScript strict, SCSS. Une responsabilité par fichier.
- `webapp/src/redmine/buildIssueUrl.ts` reste pur (aucune dépendance React ni Mattermost) et
  testé : cas nominal, message vide, message très long (10 000 caractères), unicode et
  caractères spéciaux dans le sujet, absence de projet.
- `registerPostDropdownMenuAction` uniquement (jamais `registerPostDropdownMenuComponent`).
- Commentaires en français, avec parcimonie ; identifiants en anglais.
- Commits conventionnels en français, une fonctionnalité par branche.

## Points §8 du HANDOFF à établir (non tranchés)

1. Version de Mattermost de l'instance cible : 12.0 minimum, fixé par Vincent le 2026-11-03
   (`min_server_version` 12.0.0, API plugin `server/public` v0.4.3).
2. Formatage du texte de Redmine (Textile / Markdown / CommonMark) : inconnu.
3. `large_client_header_buffers` de nginx devant Redmine : inconnu (budget URL 2000 car. par défaut).

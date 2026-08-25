# mattermost-plugin-redmine

Lire `HANDOFF.md` en entier avant toute proposition : il fixe l'architecture (URL préremplie,
jamais d'API Redmine) et le journal de session en fin de fichier.

## Environnement

- Go : version de `go.mod`. Dans le conteneur superclaude, Go n'est pas dans l'image : il est
  installé dans `/home/node/sdk/go` (persistant), à ajouter au PATH :
  `export PATH=/home/node/sdk/go/bin:$PATH`.
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

1. Version de Mattermost de l'instance cible : inconnue.
2. Formatage du texte de Redmine (Textile / Markdown / CommonMark) : inconnu.
3. `large_client_header_buffers` de nginx devant Redmine : inconnu (budget URL 2000 car. par défaut).

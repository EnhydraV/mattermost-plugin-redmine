# Handoff — `mattermost-plugin-redmine`

Document de démarrage pour Claude Code. À lire intégralement avant d'écrire la première ligne.

---

## 1. Objectif

Ajouter une entrée **« Créer un ticket Redmine »** au menu « … » d'un message Mattermost. Le clic ouvre, dans un nouvel onglet, le **formulaire natif de création de ticket Redmine**, prérempli avec le contenu du message.

L'utilisateur ajuste ensuite tout manuellement dans Redmine : sujet, description, tracker, assigné, priorité, champs personnalisés, pièces jointes. Le plugin ne crée rien lui-même — il transporte du contexte et ouvre une porte.

**Ce que ce choix élimine, et qu'il ne faut surtout pas réintroduire :**

- Aucune clé API Redmine. Aucune authentification côté plugin.
- Aucun appel à l'API REST Redmine. Le plugin ne parle jamais à Redmine côté serveur.
- Aucune modale de saisie côté Mattermost.
- Aucun mapping d'identité Mattermost → Redmine.

L'utilisateur arrive dans son propre navigateur avec sa session Redmine : le ticket est authentiquement créé par lui, et ses permissions projet s'appliquent nativement. C'est la propriété qui justifie toute cette architecture. **Toute proposition d'« améliorer » le plugin en créant le ticket via l'API détruit cette propriété et doit être refusée.**

---

## 2. Base de fork

```
https://github.com/mattermost/mattermost-plugin-starter-template
```

**Pas** `mattermost-plugin-github`. Ce dernier avait de l'intérêt pour sa modale de création et son client API — deux choses dont ce design n'a plus besoin. Le forker imposerait de supprimer l'essentiel de son code.

Le plugin GitHub reste utile en **lecture seule**, comme référence sur un seul point : la façon dont il enregistre son action de menu post côté webapp. Le lire, pas le copier.

Volume cible : environ 200 à 300 lignes au total, webapp compris.

---

## 3. Mécanisme central : le préremplissage par URL

Redmine construit son formulaire de nouveau ticket à partir des paramètres `issue[...]` passés en query string.

```
{RedmineURL}/projects/{identifier}/issues/new
  ?issue[tracker_id]=1
  &issue[subject]=...
  &issue[description]=...
```

Paramètres exploitables : `tracker_id`, `subject`, `description`, `assigned_to_id`, `priority_id`, `category_id`, `fixed_version_id`, et `custom_field_values[<id>]`.

### Piège documenté à respecter

Les champs personnalisés **dépendent du tracker**. Si `issue[tracker_id]` n'est pas fourni explicitement, Redmine ne résout aucun champ personnalisé et **supprime silencieusement** les `custom_field_values` de l'URL. Règle : dès qu'on préremplit un champ personnalisé, on fournit obligatoirement `tracker_id`.

### Encodage

`encodeURIComponent` sur les **valeurs** uniquement. Les crochets des noms de paramètres peuvent rester littéraux, Rails les interprète correctement dans les deux formes. Ne pas encoder la structure `issue[...]` elle-même.

---

## 4. La contrainte dimensionnante : la longueur d'URL

C'est le seul vrai problème de ce design, et il doit être traité dès le premier jet — pas ajouté après coup.

Un message Mattermost peut faire plusieurs milliers de caractères. Passé en query string, il se heurte aux limites de ligne de requête du navigateur, de nginx, puis de Rails. Le comportement en cas de dépassement est un 414 ou une troncature silencieuse, les deux étant mauvais.

**Budget à appliquer** : viser une URL totale sous **2000 caractères**, marge de sécurité comprise.

**Stratégie de troncature**, dans cet ordre :

1. Le **permalien du message est prioritaire et toujours présent**, quelle que soit la troncature. C'est le filet de sécurité : même si la description est amputée, le contexte complet reste à un clic.
2. Le sujet est tronqué à 120 caractères.
3. La description reçoit le budget restant. Si le message dépasse, tronquer et ajouter un marqueur explicite du type `[…] message tronqué, voir le lien ci-dessus`.
4. Si même tronqué on dépasse le budget, se rabattre sur une description ne contenant que le bloc de provenance.

Ne jamais laisser partir une URL au-delà du budget en espérant que ça passe.

---

## 5. Contenu prérempli

### Sujet

Première ligne du message, tronquée à 120 caractères. Si le message commence par une ligne vide ou du markdown de structure, prendre la première ligne non vide.

### Description

```
<corps du message, éventuellement tronqué>

---
Depuis Mattermost — @<username> dans ~<channel_display_name>
<SiteURL>/<team_name>/pl/<post_id>
```

Le format de permalien Mattermost est `{SiteURL}/{team_name}/pl/{post_id}`.

### Point de vigilance : format de texte

Redmine peut être configuré en **Textile** ou en **Markdown/CommonMark** (*Administration → Paramètres → Formatage du texte*). Le message Mattermost est en Markdown.

**Vérifier le réglage de l'instance cible avant de coder** (cf. §8). Si Redmine est en Textile, le markdown ressortira en soupe illisible : il faudra soit convertir, soit encadrer le corps du message dans un bloc `<pre>` pour au moins préserver la lisibilité brute.

---

## 6. Le projet cible

L'URL exige un identifiant de projet. Trois sources, dans l'ordre :

1. **Mapping canal → projet**, stocké en KV : `redmine_channel_<channel_id>` → identifiant de projet. Défini par `/redmine link <identifiant-projet>`, réservé aux administrateurs de canal.
2. **Projet par défaut** défini en configuration plugin.
3. **Aucun des deux** : ouvrir `{RedmineURL}/issues/new` sans projet. Redmine affiche alors un sélecteur de projet. Comportement dégradé mais parfaitement fonctionnel — c'est le repli, pas une erreur.

Ne pas bloquer l'utilisateur si le mapping est absent. Le cas 3 doit marcher.

---

## 7. Découpage technique

### Côté serveur (Go, minimal)

Sa seule raison d'exister est d'exposer la configuration et le mapping au webapp. Il ne parle jamais à Redmine.

- Configuration plugin : `RedmineURL`, `DefaultProjectIdentifier`, `DefaultTrackerID` (optionnel).
- `GET /api/v1/config` → renvoie l'URL Redmine et le projet applicable au canal demandé.
- Commande slash `/redmine link|unlink|status`, écriture KV.

### Côté webapp (TypeScript + React)

- Enregistrement de l'action de menu post via **`registerPostDropdownMenuAction`** — surtout pas `registerPostDropdownMenuComponent`, retiré à partir de Mattermost v11.
- Construction de l'URL, application du budget de troncature du §4.
- `window.open(url, '_blank', 'noopener,noreferrer')`.

La construction d'URL et la troncature vont dans un module isolé et pur (`src/redmine/buildIssueUrl.ts`), sans dépendance React. C'est la seule logique non triviale du plugin, elle doit être testable unitairement.

---

## 8. À établir avant la première ligne de code

Ces trois points changent des décisions d'implémentation. Les consigner dans `CLAUDE.md`.

1. **Version de Mattermost** de l'instance cible → confirme le hook de menu post à utiliser.
2. **Formatage du texte** configuré dans Redmine : Textile, Markdown ou CommonMark → détermine le traitement du corps de message (§5).
3. **`large_client_header_buffers` de nginx** devant Redmine → confirme ou resserre le budget d'URL du §4.

---

## 9. Jalons

| Jalon | Contenu | Critère de fin |
|---|---|---|
| **M0** | Fork du starter template, renommage, build | `make dist` OK, bundle installable |
| **M1** | Config plugin (`RedmineURL`), `buildIssueUrl` avec sujet + description + permalien + troncature, tests unitaires du module | Le module produit une URL correcte et bornée pour un message de 10 000 caractères |
| **M2** | Action de menu post, `window.open`, repli §6 cas 3 | Un clic ouvre le formulaire Redmine prérempli |
| **M3** | Mapping canal → projet, `/redmine link`, endpoint config | Le bon projet est présélectionné selon le canal |
| **M4** | i18n fr/en, README, CI Forgejo, release du bundle | Installable par un collègue sans assistance |

M1 avant M2 : la logique testable d'abord, le câblage UI ensuite.

---

## 10. Extensions envisageables — **pas en v1**

Listées pour cadrer le périmètre, à ne pas implémenter sans arbitrage explicite.

- **Rattacher un message à un ticket existant.** Le préremplissage d'une note via `/issues/:id/edit?issue[notes]=...` n'est pas documenté et **doit être testé manuellement** avant toute décision. S'il ne fonctionne pas, cette fonctionnalité impose l'API REST et sort du périmètre de ce design.
- **Réaction emoji** posée automatiquement sur le message source comme marqueur « traité ». Impossible sans savoir si le ticket a effectivement été créé — or le plugin perd la main dès l'ouverture de l'onglet. À laisser manuel.
- **Enrichissement des liens Redmine** dans les messages. Fonctionnalité distincte, déjà couverte par `moddi3/mattermost-plugin-redmine-link`.
- **Notifications Redmine → Mattermost.** Déjà couvert par `redmine_messenger` côté Redmine. Ne pas réimplémenter.

---

## 11. Conventions du dépôt

À écrire dans `CLAUDE.md` dès M0 :

- TypeScript strict, SCSS pour les styles.
- Architecture multi-fichiers, une responsabilité par fichier.
- `buildIssueUrl` reste pur et testé : cas nominal, message vide, message très long, caractères spéciaux et unicode dans le sujet, absence de mapping projet.
- Commits conventionnels, une fonctionnalité par branche.

---

## 12. Amorce pour Claude Code

> Je crée `mattermost-plugin-redmine` à partir de `mattermost/mattermost-plugin-starter-template`.
> Lis `HANDOFF.md` en entier avant de proposer quoi que ce soit.
> Le plugin n'appelle jamais l'API Redmine : il construit une URL de formulaire prérempli et l'ouvre. C'est délibéré, ne le remets pas en cause.
> Commence par M0 puis M1 — c'est-à-dire le module `buildIssueUrl` et ses tests, avant tout câblage UI.
> Avant de coder, propose-moi la signature du module et la liste des cas de test, que je valide.

---

## Journal de session

### 2026-08-25 : M0 terminé

- Template renommé : module Go `github.com/EnhydraV/mattermost-plugin-redmine`, id plugin
  `com.cubedesigners.redmine`, `min_server_version` 9.0.0, icône `assets/redmine-icon.svg`.
- Code de démo supprimé (commande hello, job périodique, kvstore template, mocks, cible `mock`).
- Configuration plugin posée (`RedmineURL`, `DefaultProjectIdentifier`, `DefaultTrackerID`),
  normalisée côté serveur (trim, slash final). Endpoint `GET /api/v1/config` renvoyant
  `{redmine_url, project_identifier, tracker_id}` ; le paramètre `channel_id` est prévu mais
  ignoré jusqu'à M3. Tests Go (3) verts.
- `make dist` OK, bundle `dist/com.cubedesigners.redmine-0.0.0+<hash>.tar.gz`.
- `CLAUDE.md` créé (environnement, conventions, questions §8 ouvertes).
- Go absent de l'image superclaude : installé dans `/home/node/sdk/go` (Go 1.27.0).

### 2026-08-25 : M1 terminé

- `webapp/src/redmine/buildIssueUrl.ts` : module pur, signature validée telle que ci-dessous.
  Troncature par points de code (pas d'emoji coupé), recherche dichotomique du plus long
  préfixe dont la forme encodée tient dans le budget, garde-fou `MIN_BODY_BUDGET = 40`
  (en dessous, repli direct sur la provenance seule). Sujet : première ligne non vide,
  débarrassée du markdown de structure (`#`, `>`, listes, fences).
- 11 tests jest verts (`npx jest src/redmine`), tsc OK, eslint OK hors `linebreak-style`.
- Décision : Redmine supposé en Markdown/CommonMark, aucun traitement du corps.
- Limite connue : avec un permalien pathologiquement long (> ~1800 car.), même le repli
  dépasse le budget ; la priorité au permalien l'emporte, conformément au §4.
- Prochaine étape : M2 (action de menu post, `window.open`, récupération de la config via
  `/api/v1/config`, repli sans projet).

### Signature validée de `buildIssueUrl`

```ts
// webapp/src/redmine/buildIssueUrl.ts, module pur
export type IssueUrlInput = {
    redmineUrl: string;              // sans slash final
    projectIdentifier?: string;      // absent -> /issues/new sans projet (§6 cas 3)
    trackerId?: string;
    message: string;                 // texte brut du post
    permalink: string;               // {SiteURL}/{team}/pl/{post_id}
    authorUsername: string;
    channelDisplayName: string;
    maxUrlLength?: number;           // défaut 2000
};
export function buildIssueUrl(input: IssueUrlInput): string;
```

Constantes : `SUBJECT_MAX_LENGTH = 120`, `URL_BUDGET = 2000`,
marqueur `[…] message tronqué, voir le lien ci-dessus`.

Cas de test prévus :
1. nominal : sujet = première ligne, description = corps + bloc de provenance + permalien, projet et tracker présents ;
2. message vide : sujet vide, description = bloc de provenance seul, URL valide ;
3. première ligne vide ou markdown de structure (`#`, `>`, `-`) : sujet = première ligne utile, nettoyée ;
4. sujet > 120 caractères : tronqué à 120 ;
5. message de 10 000 caractères : URL ≤ 2000, permalien intact, marqueur de troncature présent ;
6. troncature insuffisante (permalien et provenance très longs) : repli description = provenance seule, toujours ≤ budget ;
7. unicode et caractères spéciaux (`&`, `=`, `#`, `%`, emoji, accents) : valeurs encodées, `issue[...]` littéral ;
8. sans projet : `{redmineUrl}/issues/new?...` ;
9. sans tracker : pas de paramètre `issue[tracker_id]` ;
10. `maxUrlLength` personnalisé respecté.

Questions §8 toujours ouvertes (Mattermost version, formatage Redmine, nginx).

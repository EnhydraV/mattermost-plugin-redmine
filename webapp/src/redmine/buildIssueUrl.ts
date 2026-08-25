// Construction de l'URL du formulaire Redmine « nouveau ticket » prérempli.
// Module pur : aucune dépendance React ni Mattermost, testé unitairement.

export type IssueUrlInput = {

    /** URL de base de Redmine, sans slash final. */
    redmineUrl: string;

    /** Identifiant du projet ; absent : Redmine affiche un sélecteur de projet. */
    projectIdentifier?: string;

    /** ID numérique du tracker à présélectionner. */
    trackerId?: string;

    /** Texte brut du message Mattermost. */
    message: string;

    /** Permalien du message : {SiteURL}/{team}/pl/{post_id}. Toujours conservé. */
    permalink: string;

    authorUsername: string;
    channelDisplayName: string;

    /** Longueur maximale de l'URL produite (défaut : URL_BUDGET). */
    maxUrlLength?: number;
};

export const SUBJECT_MAX_LENGTH = 120;
export const URL_BUDGET = 2000;
export const TRUNCATION_MARKER = '[…] message tronqué, voir le lien ci-dessus';

// En dessous de ce budget (forme encodée), un corps tronqué n'apporte rien : repli direct.
const MIN_BODY_BUDGET = 40;

// Préfixes de markdown de structure à ignorer pour choisir la ligne de sujet.
const STRUCTURE_PREFIX = /^(?:#{1,6}\s+|>\s*|[-*+]\s+|\d+\.\s+|```.*)/;

export function buildIssueUrl(input: IssueUrlInput): string {
    const maxUrlLength = input.maxUrlLength ?? URL_BUDGET;
    const subject = extractSubject(input.message);
    const provenance = buildProvenance(input);
    const body = input.message.trim();

    // Étape 1 : message complet.
    const full = assemble(input, subject, body ? `${body}\n\n${provenance}` : provenance);
    if (full.length <= maxUrlLength) {
        return full;
    }

    // Étape 2 : corps tronqué au budget restant, marqueur explicite.
    const withoutBody = assemble(input, subject, `\n\n${TRUNCATION_MARKER}\n\n${provenance}`);
    const remaining = maxUrlLength - withoutBody.length;
    if (remaining >= MIN_BODY_BUDGET) {
        const truncatedBody = truncateToEncodedLength(body, remaining);
        if (truncatedBody) {
            return assemble(input, subject, `${truncatedBody}\n\n${TRUNCATION_MARKER}\n\n${provenance}`);
        }
    }

    // Étape 3 : repli, provenance seule (le permalien reste prioritaire).
    return assemble(input, subject, provenance);
}

/** Première ligne non vide qui n'est pas du markdown de structure, tronquée. */
export function extractSubject(message: string): string {
    const lines = message.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    const candidate = lines.find((line) => !STRUCTURE_PREFIX.test(line) || line.replace(STRUCTURE_PREFIX, '').trim()) ?? '';
    const cleaned = candidate.replace(STRUCTURE_PREFIX, '').trim();
    return truncateChars(cleaned, SUBJECT_MAX_LENGTH);
}

function buildProvenance(input: IssueUrlInput): string {
    return `---\nDepuis Mattermost, @${input.authorUsername} dans ~${input.channelDisplayName}\n${input.permalink}`;
}

function assemble(input: IssueUrlInput, subject: string, description: string): string {
    const projectPath = input.projectIdentifier ? `/projects/${encodeURIComponent(input.projectIdentifier)}` : '';
    const base = `${input.redmineUrl}${projectPath}/issues/new`;

    const params: string[] = [];
    if (input.trackerId) {
        params.push(`issue[tracker_id]=${encodeURIComponent(input.trackerId)}`);
    }
    params.push(`issue[subject]=${encodeURIComponent(subject)}`);
    params.push(`issue[description]=${encodeURIComponent(description)}`);

    return `${base}?${params.join('&')}`;
}

/** Tronque par points de code (pas par unités UTF-16) pour ne pas casser un emoji. */
function truncateChars(text: string, max: number): string {
    const chars = Array.from(text);
    return chars.length <= max ? text : chars.slice(0, max).join('');
}

/** Plus long préfixe de `text` dont la forme encodée tient dans `budget` caractères. */
function truncateToEncodedLength(text: string, budget: number): string {
    const chars = Array.from(text);
    let low = 0;
    let high = chars.length;
    while (low < high) {
        const mid = Math.ceil((low + high) / 2);
        if (encodeURIComponent(chars.slice(0, mid).join('')).length <= budget) {
            low = mid;
        } else {
            high = mid - 1;
        }
    }
    return chars.slice(0, low).join('').trimEnd();
}

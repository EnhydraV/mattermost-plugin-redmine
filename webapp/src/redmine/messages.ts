// Chaînes d'interface, sans react-intl : le plugin n'a pas de composant React à part le libellé.
export type MessageKey = 'menu.label' | 'error.notConfigured' | 'error.unreachable';

export type Messages = Record<MessageKey, string>;

export const messages: Record<'en' | 'fr', Messages> = {
    en: {
        'menu.label': 'Create a Redmine issue',
        'error.notConfigured': 'The Redmine plugin is not configured: the Redmine URL is missing in the System Console.',
        'error.unreachable': 'Unable to reach the Redmine plugin. Try again or contact an administrator.',
    },
    fr: {
        'menu.label': 'Créer un ticket Redmine',
        'error.notConfigured': 'Le plugin Redmine n\'est pas configuré : l\'URL de Redmine manque dans la console système.',
        'error.unreachable': 'Impossible de joindre le plugin Redmine. Réessaie ou contacte un administrateur.',
    },
};

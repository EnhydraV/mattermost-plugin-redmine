import type {GlobalState} from '@mattermost/types/store';

import {getCurrentUserLocale} from 'mattermost-redux/selectors/entities/i18n';

import {messages} from './messages';
import type {MessageKey} from './messages';

export const DEFAULT_LOCALE = 'en';

/** Ramène « fr-FR », « fr_CA »... à une langue supportée ; anglais par défaut. */
export function resolveLanguage(locale: string | undefined): keyof typeof messages {
    const language = (locale ?? '').toLowerCase().split(/[-_]/)[0];
    return language in messages ? (language as keyof typeof messages) : DEFAULT_LOCALE;
}

export function translate(locale: string | undefined, key: MessageKey): string {
    return messages[resolveLanguage(locale)][key];
}

export function translateForState(state: GlobalState, key: MessageKey): string {
    return translate(getCurrentUserLocale(state, DEFAULT_LOCALE), key);
}

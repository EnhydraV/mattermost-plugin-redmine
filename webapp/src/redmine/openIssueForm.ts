import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import {buildIssueUrl} from './buildIssueUrl';
import {fetchConfig} from './fetchConfig';
import {translateForState} from './i18n';
import {getPostContext, getSiteUrl} from './postContext';

/**
 * Ouvre l'onglet de façon synchrone, avant l'appel réseau, pour ne pas être
 * bloqué par les anti-popups ; l'URL finale est posée ensuite.
 */
export function openDetachedTab(): Window | null {
    const tab = window.open('about:blank', '_blank');
    if (tab) {
        tab.opener = null;
    }
    return tab;
}

export function navigateTab(tab: Window | null, url: string): void {
    if (tab && !tab.closed) {
        tab.location.replace(url);
        return;
    }
    window.open(url, '_blank', 'noopener,noreferrer');
}

export async function openIssueForm(store: Store<GlobalState>, postId: string): Promise<void> {
    const state = store.getState();
    const context = getPostContext(state, postId);
    if (!context) {
        return;
    }

    const tab = openDetachedTab();
    try {
        const config = await fetchConfig(getSiteUrl(state), context.channelId);
        if (!config.redmine_url) {
            tab?.close();
            // eslint-disable-next-line no-alert -- erreur de configuration rare, pas de modale Mattermost disponible ici
            window.alert(translateForState(state, 'error.notConfigured'));
            return;
        }

        navigateTab(tab, buildIssueUrl({
            redmineUrl: config.redmine_url,
            projectIdentifier: config.project_identifier || undefined,
            trackerId: config.tracker_id || undefined,
            message: context.message,
            permalink: context.permalink,
            authorUsername: context.authorUsername,
            channelDisplayName: context.channelDisplayName,
        }));
    } catch (error) {
        tab?.close();

        // eslint-disable-next-line no-console
        console.error('[redmine] impossible d\'ouvrir le formulaire', error);
        // eslint-disable-next-line no-alert -- erreur de configuration rare, pas de modale Mattermost disponible ici
        window.alert(translateForState(state, 'error.unreachable'));
    }
}

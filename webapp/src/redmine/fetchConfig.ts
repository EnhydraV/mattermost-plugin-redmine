import manifest from 'manifest';

import type {PluginConfig} from './types';

/** Préfixe de chemin quand Mattermost est servi sous un sous-répertoire. */
export function pluginApiBase(siteUrl: string | undefined): string {
    let prefix = '';
    if (siteUrl) {
        try {
            prefix = new URL(siteUrl).pathname.replace(/\/$/, '');
        } catch {
            prefix = '';
        }
    }
    return `${prefix}/plugins/${manifest.id}/api/v1`;
}

export async function fetchConfig(siteUrl: string | undefined, channelId: string): Promise<PluginConfig> {
    const url = `${pluginApiBase(siteUrl)}/config?channel_id=${encodeURIComponent(channelId)}`;
    const response = await fetch(url, {
        method: 'GET',
        credentials: 'same-origin',
        headers: {'X-Requested-With': 'XMLHttpRequest'},
    });
    if (!response.ok) {
        throw new Error(`Redmine plugin config: HTTP ${response.status}`);
    }
    return response.json() as Promise<PluginConfig>;
}

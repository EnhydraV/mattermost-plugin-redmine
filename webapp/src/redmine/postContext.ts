import type {GlobalState} from '@mattermost/types/store';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentTeam, getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemMessage} from 'mattermost-redux/utils/post_utils';

import type {PostContext} from './types';

export function getSiteUrl(state: GlobalState): string {
    const configured = getConfig(state).SiteURL;
    return (configured || window.location.origin).replace(/\/$/, '');
}

/** Vrai si le message peut donner lieu à un ticket (pas un message système). */
export function isEligiblePost(state: GlobalState, postId: string): boolean {
    const post = getPost(state, postId);
    return Boolean(post) && !isSystemMessage(post);
}

/** Rassemble ce que buildIssueUrl attend, ou null si le post est inconnu du store. */
export function getPostContext(state: GlobalState, postId: string): PostContext | null {
    const post = getPost(state, postId);
    if (!post) {
        return null;
    }

    const channel = getChannel(state, post.channel_id);
    const author = getUser(state, post.user_id);

    // Les messages directs et de groupe n'ont pas d'équipe : le permalien passe par l'équipe courante.
    const team = (channel?.team_id ? getTeam(state, channel.team_id) : undefined) ?? getCurrentTeam(state);
    const teamName = team?.name ?? '';

    return {
        message: post.message ?? '',
        permalink: `${getSiteUrl(state)}/${teamName}/pl/${post.id}`,
        authorUsername: author?.username ?? post.user_id,
        channelDisplayName: channel?.display_name || channel?.name || post.channel_id,
        channelId: post.channel_id,
    };
}

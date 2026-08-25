import type {GlobalState} from '@mattermost/types/store';

import {getPostContext, isEligiblePost} from './postContext';

function makeState(overrides: {channelTeamId?: string; postType?: string} = {}): GlobalState {
    return {
        entities: {
            general: {config: {SiteURL: 'https://mm.example.com/'}},
            posts: {posts: {
                p1: {id: 'p1', channel_id: 'c1', user_id: 'u1', message: 'Hello', type: overrides.postType ?? ''},
            }},
            channels: {channels: {
                c1: {id: 'c1', team_id: overrides.channelTeamId ?? 't1', display_name: 'Dev Toolbox', name: 'dev-toolbox'},
            }},
            users: {profiles: {u1: {id: 'u1', username: 'vincent'}}},
            teams: {currentTeamId: 't-current',
                teams: {
                    t1: {id: 't1', name: 'cube'},
                    't-current': {id: 't-current', name: 'current-team'},
                }},
        },
    } as unknown as GlobalState;
}

describe('getPostContext', () => {
    test('builds permalink from the channel team', () => {
        const context = getPostContext(makeState(), 'p1');
        expect(context).toEqual({
            message: 'Hello',
            permalink: 'https://mm.example.com/cube/pl/p1',
            authorUsername: 'vincent',
            channelDisplayName: 'Dev Toolbox',
            channelId: 'c1',
        });
    });

    test('direct message channel without team uses the current team', () => {
        const context = getPostContext(makeState({channelTeamId: ''}), 'p1');
        expect(context?.permalink).toBe('https://mm.example.com/current-team/pl/p1');
    });

    test('unknown post returns null', () => {
        expect(getPostContext(makeState(), 'nope')).toBeNull();
    });
});

describe('isEligiblePost', () => {
    test('regular post is eligible, system post is not', () => {
        expect(isEligiblePost(makeState(), 'p1')).toBe(true);
        expect(isEligiblePost(makeState({postType: 'system_join_channel'}), 'p1')).toBe(false);
        expect(isEligiblePost(makeState(), 'nope')).toBe(false);
    });
});

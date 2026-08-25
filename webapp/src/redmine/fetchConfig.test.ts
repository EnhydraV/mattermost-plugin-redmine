import manifest from 'manifest';

import {pluginApiBase} from './fetchConfig';

describe('pluginApiBase', () => {
    test('root site URL', () => {
        expect(pluginApiBase('https://mm.example.com')).toBe(`/plugins/${manifest.id}/api/v1`);
    });

    test('site served under a subpath', () => {
        expect(pluginApiBase('https://example.com/chat/')).toBe(`/chat/plugins/${manifest.id}/api/v1`);
    });

    test('missing or invalid site URL falls back to root', () => {
        expect(pluginApiBase(undefined)).toBe(`/plugins/${manifest.id}/api/v1`);
        expect(pluginApiBase('not a url')).toBe(`/plugins/${manifest.id}/api/v1`);
    });
});

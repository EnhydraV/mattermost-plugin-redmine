import {
    buildIssueUrl,
    extractSubject,
    SUBJECT_MAX_LENGTH,
    TRUNCATION_MARKER,
    URL_BUDGET,
} from './buildIssueUrl';

const base = {
    redmineUrl: 'https://redmine.example.com',
    projectIdentifier: 'toolbox',
    trackerId: '1',
    permalink: 'https://mm.example.com/cube/pl/abc123def456',
    authorUsername: 'vincent',
    channelDisplayName: 'Dev Toolbox',
};

function params(url: string): URLSearchParams {
    return new URL(url).searchParams;
}

describe('buildIssueUrl', () => {
    test('nominal: subject from first line, description with body, provenance and permalink', () => {
        const url = buildIssueUrl({...base, message: 'Le bouton export plante\n\nStack trace ci-dessous.'});
        const parsed = new URL(url);

        expect(parsed.origin + parsed.pathname).toBe('https://redmine.example.com/projects/toolbox/issues/new');
        expect(url).toContain('issue[tracker_id]=1');
        expect(params(url).get('issue[subject]')).toBe('Le bouton export plante');
        expect(params(url).get('issue[description]')).toBe(
            'Le bouton export plante\n\nStack trace ci-dessous.\n\n---\nDepuis Mattermost, @vincent dans ~Dev Toolbox\nhttps://mm.example.com/cube/pl/abc123def456',
        );
    });

    test('empty message: empty subject, description is provenance only', () => {
        const url = buildIssueUrl({...base, message: ''});
        expect(params(url).get('issue[subject]')).toBe('');
        expect(params(url).get('issue[description]')).toBe(
            '---\nDepuis Mattermost, @vincent dans ~Dev Toolbox\nhttps://mm.example.com/cube/pl/abc123def456',
        );
    });

    test('leading blank lines and structural markdown are skipped for the subject', () => {
        expect(extractSubject('\n\n## Titre du bug\ncorps')).toBe('Titre du bug');
        expect(extractSubject('> citation\nreste')).toBe('citation');
        expect(extractSubject('- item un\n- item deux')).toBe('item un');
        expect(extractSubject('```\ncode\n```')).toBe('code');
    });

    test('subject is truncated to 120 characters', () => {
        const long = 'a'.repeat(300);
        expect(extractSubject(long)).toHaveLength(SUBJECT_MAX_LENGTH);
        expect(params(buildIssueUrl({...base, message: long})).get('issue[subject]')).toHaveLength(SUBJECT_MAX_LENGTH);
    });

    test('10 000 characters message: URL within budget, permalink intact, marker present', () => {
        const message = 'Ligne de log assez longue pour remplir. '.repeat(250);
        expect(message.length).toBeGreaterThanOrEqual(10000);

        const url = buildIssueUrl({...base, message});
        const description = params(url).get('issue[description]') ?? '';

        expect(url.length).toBeLessThanOrEqual(URL_BUDGET);
        expect(url.length).toBeGreaterThan(URL_BUDGET - 100);
        expect(description).toContain(TRUNCATION_MARKER);
        expect(description.endsWith(base.permalink)).toBe(true);
        expect(description.startsWith('Ligne de log')).toBe(true);
    });

    test('fallback to provenance only when even a truncated body does not fit', () => {
        const permalink = 'https://mm.example.com/cube/pl/' + 'x'.repeat(1550);
        const url = buildIssueUrl({...base, permalink, message: 'Un message normal. '.repeat(15)});
        const description = params(url).get('issue[description]') ?? '';

        expect(url.length).toBeLessThanOrEqual(URL_BUDGET);
        expect(description).not.toContain('Un message normal.');
        expect(description).not.toContain(TRUNCATION_MARKER);
        expect(description.endsWith(permalink)).toBe(true);
    });

    test('unicode and special characters are encoded, issue[...] keys stay literal', () => {
        const message = 'Café & thé = 100% #1 🚀\nligne 2';
        const url = buildIssueUrl({...base, message});

        expect(url).toContain('?issue[tracker_id]=1&issue[subject]=');
        expect(url).toContain('&issue[description]=');
        expect(url.split('&')).toHaveLength(3);
        expect(url).not.toMatch(/[ #]/);
        expect(params(url).get('issue[subject]')).toBe('Café & thé = 100% #1 🚀');
        expect(params(url).get('issue[description]')).toContain(message);
    });

    test('without project: /issues/new at Redmine root', () => {
        const url = buildIssueUrl({...base, projectIdentifier: undefined, message: 'x'});
        expect(url.startsWith('https://redmine.example.com/issues/new?')).toBe(true);
    });

    test('without tracker: no issue[tracker_id] parameter', () => {
        const url = buildIssueUrl({...base, trackerId: undefined, message: 'x'});
        expect(url).not.toContain('tracker_id');
        expect(url).toContain('?issue[subject]=');
    });

    test('custom maxUrlLength is honoured', () => {
        const message = 'mot '.repeat(200);
        const url = buildIssueUrl({...base, message, maxUrlLength: 600});
        expect(url.length).toBeLessThanOrEqual(600);
        expect(params(url).get('issue[description]')).toContain(TRUNCATION_MARKER);
    });

    test('truncation does not split a surrogate pair', () => {
        const message = '🚀'.repeat(2000);
        const description = params(buildIssueUrl({...base, message})).get('issue[description]') ?? '';
        expect(description).not.toMatch(/[\uD800-\uDBFF](?![\uDC00-\uDFFF])/);
    });
});

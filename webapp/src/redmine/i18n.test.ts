import {resolveLanguage, translate} from './i18n';

describe('i18n', () => {
    test('resolves regional variants and falls back to English', () => {
        expect(resolveLanguage('fr')).toBe('fr');
        expect(resolveLanguage('fr-FR')).toBe('fr');
        expect(resolveLanguage('fr_CA')).toBe('fr');
        expect(resolveLanguage('de')).toBe('en');
        expect(resolveLanguage(undefined)).toBe('en');
    });

    test('translates in both languages', () => {
        expect(translate('fr', 'menu.label')).toBe('Créer un ticket Redmine');
        expect(translate('en', 'menu.label')).toBe('Create a Redmine issue');
    });
});

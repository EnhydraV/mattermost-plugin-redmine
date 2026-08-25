// Conventional Commits, sujets en français : la casse du sujet n'est pas contrainte.
export default {
    extends: ['@commitlint/config-conventional'],
    rules: {
        'subject-case': [0],
        'header-max-length': [2, 'always', 100],
        'body-max-line-length': [1, 'always', 120],
    },
};

import React from 'react';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import {translateForState} from '../redmine/i18n';

type Props = {
    store: Store<GlobalState>;
};

// Lit la locale au rendu (à chaque ouverture du menu) plutôt qu'à l'initialisation du
// plugin, moment où l'utilisateur courant n'est pas forcément encore chargé.
export default function MenuLabel({store}: Props): JSX.Element {
    return <>{translateForState(store.getState(), 'menu.label')}</>;
}

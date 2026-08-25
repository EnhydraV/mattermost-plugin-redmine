import manifest from 'manifest';
import React from 'react';
import type {Store} from 'redux';

import type {GlobalState} from '@mattermost/types/store';

import type {PluginRegistry} from 'types/mattermost-webapp';

import MenuLabel from './components/menu_label';
import {openIssueForm} from './redmine/openIssueForm';
import {isEligiblePost} from './redmine/postContext';

export default class Plugin {
    public async initialize(registry: PluginRegistry, store: Store<GlobalState>) {
        registry.registerPostDropdownMenuAction(
            <MenuLabel store={store}/>,
            (postId: string) => {
                openIssueForm(store, postId);
            },
            (postId: string) => isEligiblePost(store.getState(), postId),
        );
    }
}

declare global {
    interface Window {
        registerPlugin(pluginId: string, plugin: Plugin): void;
    }
}

window.registerPlugin(manifest.id, new Plugin());

// Réponse de GET /api/v1/config côté serveur.
export type PluginConfig = {
    redmine_url: string;
    project_identifier: string;
    tracker_id: string;
};

// Ce que le webapp extrait du store à propos du message source.
export type PostContext = {
    message: string;
    permalink: string;
    authorUsername: string;
    channelDisplayName: string;
    channelId: string;
};

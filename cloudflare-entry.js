import { onRequest, onRequestOptions } from './functions/api/[[path]].js';

// Adapt the existing Pages API to a Git-connected Cloudflare Worker.
export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);
        if (url.pathname.startsWith('/api/')) {
            if (request.method === 'OPTIONS') return onRequestOptions();
            const path = url.pathname.slice('/api/'.length).split('/').filter(Boolean);
            return onRequest({ request, env, params: { path }, waitUntil: ctx.waitUntil.bind(ctx) });
        }
        return env.ASSETS.fetch(request);
    }
};

import { auth } from '$lib/stores/auth.svelte';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const ssr = false;

export const load: LayoutLoad = ({ url }) => {
    if (url.pathname === '/login') return;

    if (auth.token === '') {
        redirect(302, '/login')
    }
}
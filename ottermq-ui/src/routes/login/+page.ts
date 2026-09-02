import { auth } from '$lib/stores/auth.svelte';
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const ssr = false;

export const load: PageLoad = () => {
    if (auth.token !== '') {
        redirect(302, '/')
    }
}
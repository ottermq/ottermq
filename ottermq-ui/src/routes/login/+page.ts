import { goto } from '$app/navigation';
import { auth } from '$lib/stores/auth.svelte';
import type { PageLoad } from './$types';

export const ssr = false;

export const load: PageLoad = ({ }) => {
    if (auth.token !== '') {
        goto('/')
    }
}
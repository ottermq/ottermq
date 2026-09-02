import { api } from '$lib/services/api';

export interface VhostsData {
    vhosts: VHost[]
}
export interface VHost {
    name: string;
    state: string;
    unconfirmed_count: number;
    prefetch_count: number;
    unacked_count: number;
}
function getInitialVhost() {
    if (typeof localStorage === 'undefined') {
        return ''
    }

    return localStorage.getItem('selected_vhost') ?? '';
}
const initialVhost = getInitialVhost();


const defaultVhost = '/';
export const vhosts = $state({
    items: [] as VHost[],
    selected: initialVhost || defaultVhost,
    loading: false,
    error: '',
})

const names = $derived(vhosts?.items.map(v => v.name))

export async function fetchVhosts(): Promise<VhostsData | null> {
    vhosts.loading = true;
    vhosts.error = '';
    let data: VhostsData | null = null
    try {
        data = await (await api.get('/api/vhosts')).json();
        vhosts.items = Array.isArray(data?.vhosts) ? data.vhosts : []
        setSelectedVhost(getInitialVhost())
    } catch (err) {
        console.log('Failed to fetch: ', err);
    } finally {
        vhosts.loading = false;
    }
    return data;
}

export function setSelectedVhost(selected: string) {
    vhosts.selected = selected;
    if (!names.includes(vhosts.selected)) {
        vhosts.selected = names.includes(defaultVhost)
            ? defaultVhost
            : (names[0] ?? defaultVhost);
    }
    localStorage.setItem('selected_vhost', vhosts.selected)
}

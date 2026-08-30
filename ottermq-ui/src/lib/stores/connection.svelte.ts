import { api } from "$lib/services/api";
export interface ConnectionData {
    vhost: string,
    name: string,
    user_name: string,
    state: string,
    ssl: boolean,
    protocol: string,
    channels: number,
    last_heartbeat: string,
    connected_at: string,
}

export async function getConnections(): Promise<ConnectionData[] | null> {
    try {
        const response = await api.get('/api/connections')
        const data = await response.json();
        return Array.isArray(data?.connections) ? data.connections : null;
    } catch (err) {
        console.error('Failed to fetch: ', err)
    }
    return null;
}
import { api } from "$lib/services/api";

export interface ChannelData {
    vhost: string;
    connection_name: string;
    number: number;
    user: string;
    state: string;
    // Details
    unconfirmed_count: number;
    prefetch_count: number;
    unacked_count: number;
    // Stats
    publish_rate: number;
    confirm_rate: number;
    unrotable_rate: number;
    deliver_rate: number;
    ack_rate: number;
}


export async function getChannels(): Promise<ChannelData[] | null> {
    try {
        const response = await api.get('/api/channels')
        const data = await response.json();
        return Array.isArray(data?.channels) ? data.channels : null;
    } catch (err) {
        console.error('Failed to fetch: ', err)
    }
    return null;
}
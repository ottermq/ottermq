import { api } from '$lib/services/api';

export interface BrokerData {
    product: string;
    version: string;
    go_version: string;
}
export interface OverviewData {
    broker: BrokerData;
    node: {
        name: string;
        uptime: number;
        gorotines: number;
        fd_used: number;
        fd_limit: number;
        memory_usage: number;
    }
    message_stats: {
        messages_total: number;
        messages_ready: number;
        messages_unacknowledged: number;
    };
    object_totals: {
        consumers: number;
    };
}


export async function fetchOverviewData() {
    let data;
    try {
        const response = await api.get('/api/overview');
        data = await response.json();
    } catch (err) {
        console.error('Failed to fetch: ', err);
    }
    return data;
}

export async function fetchBrokerData(): Promise<BrokerData | null> {
    try {
        const response = await api.get('/api/overview/broker');
        return await response.json();
    } catch (err) {
        console.error('Failed to fetch: ', err);
    }
    return null;
}
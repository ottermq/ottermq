import { api } from "$lib/services/api";
export interface QueueData {
    vhost: string,
    name: string,

    messages: number,
    messages_ready: number,
    messages_unacked: number,
    messages_persistent: number,
    messages_total: number,

    consumers: number;

    durable: boolean;
    auto_delete: boolean;
    exclusive: boolean;
    arguments?: Record<string, unknown>;

    dead_letter_exchange?: string;
    dead_letter_routing_key?: string;

    message_ttl?: number;
    max_lenght?: number; // AKA QLL

    max_priority?: number;

    state: string;

    owner_connection: string;
    persistence_enabled: boolean;
}

export async function getQueues(): Promise<QueueData[] | null> {
    try {
        const response = await api.get('/api/queues')
        const data = await response.json();
        return Array.isArray(data?.queues) ? data.queues : null;
    } catch (err) {
        console.error('Failed to fetch: ', err);
    }
    return null;
}
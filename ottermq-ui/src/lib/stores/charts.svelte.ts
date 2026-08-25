import { api } from "$lib/services/api";

export interface TimeSeries{
    timestamp: Date
    value: number
}

export interface MessageStatsTimeSeries {
    ready: TimeSeries[]
    unacked: TimeSeries[]
    total: TimeSeries[]
}

export interface MessageRatesTimeSeries {
    publish: TimeSeries[]
    deliver_auto_ack: TimeSeries[]
    deliver_manual_ack: TimeSeries[]
    ack: TimeSeries[]
}

export interface ChartsData {
    message_stats: MessageStatsTimeSeries
    message_rates: MessageRatesTimeSeries
}

export async function fetchChartData() :Promise<ChartsData | null>{
    try {
        const response = await api.get('/api/overview/charts')
        return await response.json()
    } catch (err) {
        console.error('Failed to fetch: ', err);
    }
    return null;
}
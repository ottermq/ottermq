<script lang="ts">
	import StatCard from '$lib/components/StatCard.svelte';
	import TimeSeriesChart from '$lib/components/TimeSeriesChart.svelte';
	import { fetchChartData, type ChartsData } from '$lib/stores/charts.svelte';
	import type { OverviewData } from '$lib/stores/overview.svelte';
	import { fetchOverviewData as fetchData } from '$lib/stores/overview.svelte';
	import {formatBytes} from '$lib/utils'
	let data = $state<OverviewData | null>(null);

	async function getData() {
		data = await fetchData();
	}

	$effect(() => {
		getData();
		getChats();
		const interval = setInterval(() => {
			(getData(), getChats());
		}, 5000);
		return () => clearInterval(interval);
	});

	let charts = $state<ChartsData | null>(null);

	async function getChats() {
		charts = await fetchChartData();
	}

	const uptimeFormatter = (secs:number) => {
		if (secs == 0) return '-'
		const d = Math.floor(secs / 86400)
		const h = Math.floor((secs % 86400) / 3600)
		const m = Math.floor((secs % 3600) / 60)
		const s = Math.floor(secs % 60 )
		return `${d}d ${h}h ${m}m ${s}`
	}
	let uptime = $derived(uptimeFormatter(data?.broker.uptime_secs ?? 0))


</script>

<h1>Overview</h1>

<div class="stats">
	<StatCard
		title="Total Messages"
		value={data?.message_stats.messages_total ?? 0}
		color="var(--color-series-8)"
	/>
	<StatCard
		title="Ready"
		value={data?.message_stats.messages_ready ?? 0}
		color="var(--color-series-4)"
	/>
	<StatCard
		title="Unacknowledged"
		value={data?.message_stats.messages_unacknowledged ?? 0}
		color="var(--color-series-9)"
	/>
	<StatCard
		title="Consumers"
		value={data?.object_totals.consumers ?? 0}
		color="var(--color-series-6)"
	/>
</div>
<div class="chart-block">
	<TimeSeriesChart
		title="Queued Messages"
		yAxisTitle="Messages"
		format={(v) => Math.round(v).toLocaleString()}
		series={[
			{ name: 'Ready', points: charts?.message_stats.ready ?? [], color: '--color-series-1' },
			{ name: 'Unacked', points: charts?.message_stats.unacked ?? [], color: '--color-series-2' },
			{ name: 'Total', points: charts?.message_stats.total ?? [], color: '--color-series-3' }
		]}
	/>
	<TimeSeriesChart
		title="Messages Rates"
		yAxisTitle="Messages/s"
		format={(val: number) => val.toFixed(1)}
		unit="msg/s"
		decimals={1}
		tooltipDecimals={2}
		series={[
			{ name: 'Publish', points: charts?.message_rates.publish ?? [], color: '--color-series-1' },
			{
				name: 'Deliver (auto ack)',
				points: charts?.message_rates.deliver_auto_ack ?? [],
				color: '--color-series-8'
			},
			{
				name: 'Deliver (manual ack)',
				points: charts?.message_rates.deliver_manual_ack ?? [],
				color: '--color-series-5'
			},
			{ name: 'Ack', points: charts?.message_rates.ack ?? [], color: '--color-series-4' }
		]}
	/>
</div>
<div class="stats">

	<div class="stats-card">
		<h6>Global counts</h6>
		{#if data}
		<table>
			<tbody>
				{#each Object.entries(data.object_totals) as [label, value] (label)}
				<tr>
					<td class="label">{label}</td>
					<td class="value">{value}</td>
				</tr>
				{/each}
			</tbody>
		</table>
		{/if}
	</div>
	
	<div class="stats-card">
		<h6>Node Statistics</h6>
		{#if data}
		<table>
			<tbody>
				<tr><td class="label">Name</td><td class="value">{data.node.name}</td></tr>
				<tr><td class="label">Uptime</td><td class="value">{uptime}</td></tr>
				<tr><td class="label">Gorotines</td><td class="value">{data.node.goroutines}</td></tr>
				<tr><td class="label">File Descriptors</td><td class="value"><span>{data.node.fd_used} / {data.node.fd_limit} </span></td></tr>
				<tr><td class="label">Memory</td><td class="value"><span> {formatBytes(data.node.memory_usage)} / {formatBytes(data.node.memory_limit)}</span></td></tr>
			</tbody>
		</table>
		{/if}
	</div>
</div>

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
		gap: 16px;
	}

	h1 {
		font-size: 34px;
		font-weight: 600;
	}

	.chart-block {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
		gap: 16px;
		margin-top: 16px;
	}
	/* charts are child components, so their root elements need :global() */
	.chart-block > :global(*) {
		min-width: 0;
	}

	.stats-card {
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 16px 20px;
		margin-top: 16px;
	}

	.stats-card h6 {
		margin: 0 0 10px;
		font-size: large;
		font-weight: 600;
		letter-spacing: 0.03em;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	td {
		padding: 7px 4px;
	}

	tr {
		font-size: 14px;
		font-weight: 600;
	}

	/* tbody tr:not(:last-child) td {
		border-bottom: 1px solid var(--color-border);
	} */

	.label {
		color: var(--color-text-muted-hi-contrast);
		text-transform: capitalize;
	}

	.value {
		text-align: right;
		font-variant-numeric: tabular-nums;
		color: var(--color-text);
	}
</style>

<script lang="ts">
	import StatCard from '$lib/components/StatCard.svelte';
	import type { OverviewData } from '$lib/stores/overview.svelte';
	import { fetchOverviewData as fetchData } from '$lib/stores/overview.svelte';
	import { fetchChartData, type ChartsData } from '$lib/stores/charts.svelte';
	import TimeSeriesChart from '$lib/components/TimeSeriesChart.svelte';
	let data = $state<OverviewData | null>(null);

	async function getData() {
		data = await fetchData();
	}

	$effect(() => {
		getData();
		getChats();
		const interval = setInterval(()=>{
			getData(),
			getChats()
		}, 5000);
		return () => clearInterval(interval);
	});

	let charts = $state<ChartsData|null>(null);

	async function getChats() {
		charts = await fetchChartData();
	}
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
		format={(v)=>Math.round(v).toLocaleString()}
		series={[
			{ name: 'Ready', 	points: charts?.message_stats.ready		??[], color: '--color-series-1' },
			{ name: 'Unacked', 	points: charts?.message_stats.unacked	??[], color: '--color-series-2' },
			{ name: 'Total', 	points: charts?.message_stats.total		??[], color: '--color-series-3' },
		]}
	/>
	<TimeSeriesChart 
		title="Messages Rates"
		yAxisTitle="Messages/s"
		format={(val:number) => val.toFixed(1)}
		unit="msg/s"
		decimals={1}
		tooltipDecimals={2}
		series={[
			{ name: 'Publish', 	points: charts?.message_rates.publish	??[], color: '--color-series-1' },
			{ name: 'Deliver (auto ack)', 	points: charts?.message_rates.deliver_auto_ack	??[], color: '--color-series-8' },
			{ name: 'Deliver (manual ack)', 	points: charts?.message_rates.deliver_manual_ack	??[], color: '--color-series-5' },
			{ name: 'Ack', 	points: charts?.message_rates.ack	??[], color: '--color-series-4' },
		]}
	/>
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
		gap: 20px;
	}
	/* charts are child components, so their root elements need :global() */
	.chart-block > :global(*) {
		min-width: 0;
	}
</style>

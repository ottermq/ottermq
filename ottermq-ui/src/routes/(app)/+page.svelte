<!-- routes/+page.svelte -->
<script lang="ts">
	import MessageStatsChart from '$lib/components/MessageStatsChart.svelte';
	import StatCard from '$lib/components/StatCard.svelte';
	import type { OverviewData } from '$lib/stores/overview.svelte';
	import { fetchOverviewData as fetchData } from '$lib/stores/overview.svelte';
	import { fetchChartData, type ChartsData } from '$lib/stores/charts.svelte';
	import MessageRateChart from '$lib/components/MessageRateChart.svelte';
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
	<MessageStatsChart
		chartData={charts?.message_stats!}
	/>
	<MessageRateChart
		chartData={charts?.message_rates!}
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

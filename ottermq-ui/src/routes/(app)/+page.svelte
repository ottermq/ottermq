<!-- routes/+page.svelte -->
<script lang="ts">
	import MessageStatsChart from '$lib/components/MessageStatsChart.svelte';
	import StatCard from '$lib/components/StatCard.svelte';
	import type { OverviewData } from '$lib/stores/overview.svelte';
	import { fetchOverviewData as fetchData } from '$lib/stores/overview.svelte';
	import { fetchChartData, type ChartsData } from '$lib/stores/charts.svelte';

	let data = $state<OverviewData | null>(null);

	async function getData() {
		data = await fetchData();
	}

	$effect(() => {
		getData();
		getChats();
		const interval = setInterval(getData, 5000);
		return () => clearInterval(interval);
	});

	let charts = $state<ChartsData|null>(null);

	async function getChats() {
		charts = await fetchChartData();
	}
</script>

<h1>Overview</h1>

<div class="stats">
	{#if data}
		<StatCard title="Total Messages" value={data.message_stats.messages_total} color="blue" />
		<StatCard title="Ready" value={data.message_stats.messages_ready} color="green" />
		<StatCard
			title="Unacknowledged"
			value={data.message_stats.messages_unacknowledged}
			color="orange"
		/>
		<StatCard title="Consumers" value={data.object_totals.consumers} color="black" />
	{:else}
		<StatCard title="Total Messages" value={0} color="blue" />
		<StatCard title="Ready" value={0} color="green" />
		<StatCard title="Unacknowledged" value={0} color="orange" />
		<StatCard title="Consumers" value={0} color="black" />
	{/if}
</div>
<div class=charts>
	<MessageStatsChart
		chartData={charts?.message_stats!}
	/>
		
</div>

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
	}
</style>

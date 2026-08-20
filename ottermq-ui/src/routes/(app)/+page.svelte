<!-- routes/+page.svelte -->
<script lang="ts">
	import StatCard from '$lib/components/StatCard.svelte';
	import { api } from '$lib/services/api';

	interface OverviewData {
		message_stats: {
			messages_total: number;
			messages_ready: number;
			messages_unacknowledged: number;
		};
		object_totals: {
			consumers: number;
		};
	}

	let data = $state<OverviewData | null>(null);

	async function loadData() {
		try {
			const response = await api.get('/api/overview');
			data = await response.json();
		} catch (err) {
			console.error('Failed to fetch:', err);
		}
	}

	$effect(() => {
		loadData();
		const interval = setInterval(loadData, 5000);
		return () => clearInterval(interval);
	});
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

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
	}
</style>

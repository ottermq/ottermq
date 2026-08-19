<!-- routes/+page.svelte -->
<script lang="ts">
	import StatCard from '$lib/components/StatCard.svelte';
	import { api } from '$lib/services/api';

	let data: any;

	async function loadData() {
		try {
			const response = await api.get('/api/overview');
			data = await response.json();
		} catch (err) {
			console.error('Failed to fetch:', err);
		}
	}

	loadData();
</script>

<h1>Overview</h1>

<div class="stats">
	<StatCard title="Total Messages" value={0} color="blue" />
	<StatCard title="Ready" value={0} color="green" />
	<StatCard title="Unacknowledged" value={0} color="orange" />
	<StatCard title="Consumers" value={0} color="black" />
</div>

<style>
	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
	}
</style>

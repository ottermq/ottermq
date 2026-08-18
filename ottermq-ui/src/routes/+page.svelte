<!-- routes/+page.svelte -->
<script>
	import StatCard from '$lib/components/StatCard.svelte';
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

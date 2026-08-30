<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import '../layout.css';

	let { children } = $props();
</script>

<div class="shell">
	<Sidebar />
	<main>
		{@render children()}
	</main>
</div>

<style>
	.shell {
		display: flex;
		height: 100vh;
		overflow: hidden;
	}

	main {
		flex: 1;
		min-width: 0;
		overflow-y: auto;
	}
</style>

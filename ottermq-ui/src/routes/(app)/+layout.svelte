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
		min-height: 100vh;
	}
</style>

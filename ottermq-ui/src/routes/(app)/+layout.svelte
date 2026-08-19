<script lang="ts">
	import Header from '$lib/components/Header.svelte';
	import '../layout.css';

	let { children } = $props();
</script>

<Header />

<main>
	{@render children()}
</main>

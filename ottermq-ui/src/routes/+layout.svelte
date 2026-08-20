<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.css';
	import './layout.css';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>OtterMQ</title>
</svelte:head>

{@render children()}

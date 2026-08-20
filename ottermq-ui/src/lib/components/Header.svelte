<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { auth, logout } from '$lib/stores/auth.svelte';
	import type { OverviewData } from '$lib/stores/overview.svelte';
	import { fetchOverviewData } from '$lib/stores/overview.svelte';

	const menu = [
		{ label: 'OVERVIEW', href: '/' },
		{ label: 'CONNECTIONS', href: '/connections' },
		{ label: 'CHANNELS', href: '/channels' },
		{ label: 'EXCHANGES', href: '/exchanges' },
		{ label: 'QUEUES', href: '/queues' },
		{ label: 'ADMIN', href: '/admin' }
	];

	const username = auth.username || 'annonymous';

	function handleLogout() {
		logout();
		goto('/login');
	}

	let overview = $state<OverviewData | null>(null);

	async function getData() {
		overview = await fetchOverviewData();
	}

	$effect(() => {
		getData();
	});

	const version = $derived(overview?.broker.version ?? '');

	const goVersion = $derived(getGoVersion());
	function getGoVersion() {
		const full = overview?.broker.go_version;
		if (!full) return '';
		const parts = full.split(' ');
		let ver = parts.length >= 2 ? parts[0] : full;
		if (ver.startsWith('go')) {
			ver = ver.slice(2);
		}
		return `Go ${ver}`;
	}
</script>

<header>
	<div class="flex">
		<div class="text-xl text-white">OtterMQ</div>
		<div class="version">
			<span>{version}</span>
			<span>{goVersion}</span>
		</div>
		<div>User:<strong>{username}</strong></div>
		<div class="logout">
			<button onclick={handleLogout}>LOGOUT</button>
		</div>
	</div>

	<nav>
		{#each menu as { label, href }}
			<a {href} class:active={page.url.pathname === href}>
				{label}
			</a>
		{/each}
	</nav>
</header>

<style>
	@reference "tailwindcss";
	header {
		background: #1976d2;
		color: white;
		padding-top: 10px;
		padding-left: 10px;
	}

	nav {
		display: flex;
		gap: 28px;
		padding: 12px 14px 0;
	}

	nav a {
		color: white;
		text-decoration: none;
		font-weight: bold;
		font-size: 13px;
		padding-bottom: 8px;
		border-bottom: 2px solid transparent;
	}

	nav a.active {
		border-bottom-color: white;
	}

	.version {
		margin-left: 15px;
		margin-right: 10px;
	}

	.version span {
		background-color: #50a0e2;
		font-size: 14px;
		color: white;
		margin-left: 5px;
		padding: 2px;
		border-color: transparent;
		border-width: 1px;
		border-radius: 4px;
	}
</style>

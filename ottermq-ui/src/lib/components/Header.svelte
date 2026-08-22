<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { auth, logout } from '$lib/stores/auth.svelte';
	import type { BrokerData } from '$lib/stores/overview.svelte';
	import { fetchBrokerData } from '$lib/stores/overview.svelte';
	import { fetchVhosts, vhosts } from '$lib/stores/vhosts.svelte';

	const menu = [
		{ label: 'OVERVIEW', href: resolve('/') },
		{ label: 'CONNECTIONS', href: resolve('/connections') },
		{ label: 'CHANNELS', href: resolve('/channels') },
		{ label: 'EXCHANGES', href: resolve('/exchanges') },
		{ label: 'QUEUES', href: resolve('/queues') },
		{ label: 'ADMIN', href: resolve('/admin') }
	];

	const username = auth.username || 'annonymous';

	let broker = $state<BrokerData | null>(null);
	const version = $derived(broker?.version ?? '');
	const goVersion = $derived(getGoVersion());
	const vhost = $derived(vhosts.selected);

	async function getData() {
		broker = await fetchBrokerData();
	}

	function getGoVersion() {
		const full = broker?.go_version;
		if (!full) return '';
		const parts = full.split(' ');
		let ver = parts.length >= 2 ? parts[0] : full;
		if (ver.startsWith('go')) {
			ver = ver.slice(2);
		}
		return `Go ${ver}`;
	}

	function handleLogout() {
		logout();
		goto(resolve('/login'));
	}

	async function getVhosts() {
		await fetchVhosts();
	}

	$effect(() => {
		getData();
		getVhosts();
	});
</script>

<header>
	<div class="flex">
		<div class="text-xl text-white">OtterMQ</div>
		{#if version}
			<div class="version">
				<span>{version}</span>
				{#if goVersion}
					<span>{goVersion}</span>
				{/if}
			</div>
		{/if}
		{#if vhost}
			<div class="vhost">
				<span>VHost</span>
				<span>{vhost}</span>
			</div>
		{/if}
		<div>User:<strong>{username}</strong></div>
		<div class="logout">
			<button onclick={handleLogout}>LOGOUT</button>
		</div>
	</div>

	<nav>
		{#each menu as { label, href } (href)}
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

	.vhost {
		margin-left: 20px;
		margin-right: 10px;
		border: 1px solid azure;
		padding: 1px 4px;
	}
	.vhost span {
		padding: 0px 2px 0px 2px;
	}
</style>

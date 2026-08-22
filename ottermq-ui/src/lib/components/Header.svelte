<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { auth, logout } from '$lib/stores/auth.svelte';
	import type { BrokerData } from '$lib/stores/overview.svelte';
	import { fetchBrokerData } from '$lib/stores/overview.svelte';
	import { fetchVhosts, vhosts } from '$lib/stores/vhosts.svelte';
	import NavBar from './NavBar.svelte';

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
	<div class="topbar">
		<div class="brand-group">
			<div class="brand">OtterMQ</div>
			{#if version}
				<div class="version">
					<span>{version}</span>
					{#if goVersion}
						<span>{goVersion}</span>
					{/if}
				</div>
			{/if}
		</div>
		<div class="actions">
			{#if vhost}
				<div class="vhost">
					<span>VHost</span>
					<span>{vhost}</span>
				</div>
			{/if}
			<div class="user">
				<p>User:</p>
				<strong>{username}</strong>
			</div>
			<button class="logout" onclick={handleLogout}>logout</button>
		</div>
	</div>

	<NavBar />
</header>

<style>
	header {
		background: #1976d2;
		color: white;
	}

	.topbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 10px 14px;
	}

	.brand-group {
		display: flex;
		align-items: center;
		gap: 25px;
	}

	.brand {
		font-size: 2.125rem;
		font-weight: 600;
	}

	.version {
		display: flex;
		gap: 8px;
	}

	.version span {
		background-color: #ffffff4d;
		font-size: 0.75rem;
		font-weight: 300;
		color: white;
		padding: 1px 4px 2px;
		border-color: transparent;
		border-width: 1px;
		border-radius: 4px;
	}

	.actions {
		display: flex;
		align-items: center;
		gap: 10px;
	}

	.user {
		display: flex;
		gap: 4px;
		padding-right: 4px;
	}

	.vhost {
		border: 1px solid #ffffff6d;
		padding: 8px 8px 8px 4px;
		border-radius: 5px;
	}
	.vhost span {
		padding: 0px 4px 0px 4px;
	}

	.logout {
		padding: 8px 16px;
		background: transparent;
		border: none;
		color: white;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		text-transform: uppercase;
		border: 1px solid #ffffff2e;
		border-radius: 5px;
	}
	.logout:hover {
		background-color: #ffffff2e;
	}
</style>

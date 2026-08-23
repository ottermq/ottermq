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

<aside class="rail">
	<div class="rail-brand">
		<div class="name">OtterMQ</div>
		{#if version}
			<div class="rail-badges">
				<span>{version}</span>
				{#if goVersion}<span>{goVersion}</span>
				{/if}
			</div>
		{/if}
	</div>

	<nav class="rail-nav">
		{#each menu as { label, href } (href)}
			<a {href} class:active={page.url.pathname === href}>{label}</a>
		{/each}
	</nav>

	<div class="rail-foot">
		{#if vhost}
			<div class="rail-vhost">
				<span class="k">VHost</span>
				<span class="v">{vhost}</span>
			</div>
		{/if}
		<div class="rail-user">
			<span>User: <strong>{username}</strong></span>
			<button class="rail-logout" onclick={handleLogout}>Logout</button>
		</div>
	</div>
</aside>

<style>
	.rail {
		width: 220px;
		flex-shrink: 0;
		background: var(--color-surface-raised);
		color: var(--color-surface-raised-contrast);
		display: flex;
		flex-direction: column;
		padding: 18px 0;
	}

	.rail-brand {
		padding: 0 18px 16px;
	}
	.rail-brand .name {
		font-size: 1.25rem;
		font-weight: 600;
	}

	.rail-badges {
		display: flex;
		gap: 5px;
		margin-top: 8px;
		flex-wrap: wrap;
	}
	.rail-badges span {
		font-size: 0.62rem;
		font-weight: 500;
		padding: 2px 6px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--color-surface-raised-contrast) 14%, transparent);
		opacity: 0.85;
	}

	.rail-nav {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 1px;
		padding: 10px;
		border-top: 1px solid color-mix(in srgb, var(--color-surface-raised-contrast) 12%, transparent);
		border-bottom: 1px solid
			color-mix(in srgb, var(--color-surface-raised-contrast) 12%, transparent);
	}
	.rail-nav a {
		display: block;
		text-decoration: none;
		color: var(--color-surface-raised-contrast);
		font-size: 0.8rem;
		font-weight: 600;
		padding: 9px 12px;
		border-radius: 6px;
		border-left: 3px solid transparent;
		opacity: 0.62;
	}
	.rail-nav a.active {
		opacity: 1;
		background: color-mix(in srgb, var(--color-accent) 20%, transparent);
		border-left-color: var(--color-accent);
	}

	.rail-foot {
		padding: 14px 18px 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.rail-vhost {
		display: flex;
		align-items: center;
		justify-content: space-between;
		border: 1px solid color-mix(in srgb, var(--color-surface-raised-contrast) 22%, transparent);
		border-radius: 6px;
		padding: 7px 10px;
		font-size: 0.76rem;
	}
	.rail-vhost .k {
		opacity: 0.65;
		font-size: 0.68rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.rail-user {
		display: flex;
		align-items: center;
		justify-content: space-between;
		font-size: 0.78rem;
		opacity: 0.9;
	}

	.rail-logout {
		font-size: 0.72rem;
		font-weight: 600;
		letter-spacing: 0.03em;
		text-transform: uppercase;
		background: transparent;
		border: none;
		color: var(--color-accent);
		cursor: pointer;
	}
</style>

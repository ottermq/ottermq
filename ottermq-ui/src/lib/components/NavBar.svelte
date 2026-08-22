<script lang="ts">
	import { resolve } from '$app/paths';
	import { page } from '$app/state';

	const menu = [
		{ label: 'OVERVIEW', href: resolve('/') },
		{ label: 'CONNECTIONS', href: resolve('/connections') },
		{ label: 'CHANNELS', href: resolve('/channels') },
		{ label: 'EXCHANGES', href: resolve('/exchanges') },
		{ label: 'QUEUES', href: resolve('/queues') },
		{ label: 'ADMIN', href: resolve('/admin') }
	];
</script>

<nav>
	{#each menu as { label, href } (href)}
		<a {href} class:active={page.url.pathname === href}>
			{label}
		</a>
	{/each}
</nav>

<style>
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
</style>

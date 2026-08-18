<script lang="ts">
    import { page } from '$app/state'
    const menu = [
		{ label: 'OVERVIEW', href: '/'},
		{ label: 'CONNECTIONS', href: '/connections'},
		{ label: 'CHANNELS', href: '/channels'},
		{ label: 'EXCHANGES', href: '/exchanges'},
		{ label: 'QUEUES', href: '/queues'},
		{ label: 'ADMIN', href: '/admin'}
	]
</script>

<header>
	<div class="brand">
		<strong>OtterMQ</strong>
	</div>

	<nav>
		{#each menu as {label, href}}		
			<a 
				href={href}
				class:active={page.url.pathname === href}
			>
				{label}
			</a>
		{/each}
	</nav>
</header>

<style>
    header {
    background: #1976d2;
    color: white;
}

.brand {
    padding: 12px 14px 10px;
    font-size: 28px;
}

nav {
    display: flex;
    gap: 28px;
    padding: 12px 14px 0;
}

nav a {
    color: white;
    text-decoration:none;
    font-weight: bold;
    font-size: 13px;
    padding-bottom: 8px;
    border-bottom: 2px solid transparent;
}

nav a.active {
    border-bottom-color: white;
}
</style>
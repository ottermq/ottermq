<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth, login } from '$lib/stores/auth.svelte';

	let username = $state('');
	let password = $state('');

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		try {
			await login(username, password);
			goto('/');
		} catch {}
	}
</script>

<div class="flex items-center justify-center">
	<div class="card">
		<div class="card-section">
			<div class="text-h6">Sign in</div>
		</div>

		<hr />
		<div class="card-section">
			<form onsubmit={handleSubmit}>
				<label>
					Username
					<input name="username" type="text" bind:value={username} />
				</label>
				<label>
					Password
					<input name="password" type="password" bind:value={password} />
				</label>
				<button>Log in</button>
				{#if auth.error}
					<div class="text-negative text-caption">{auth.error}</div>
				{/if}
			</form>
		</div>
	</div>
</div>

<style>
	@import 'tailwindcss';
	.card {
		border-width: 2px;
		border-radius: 8px;
		border-color: gray;
		width: 360px;
		max-width: 95vw;
		padding: 10px;
	}

	button {
		margin-top: 10px;
		padding: 4px;
		border-width: 1px;
		border-color: black;
	}
</style>

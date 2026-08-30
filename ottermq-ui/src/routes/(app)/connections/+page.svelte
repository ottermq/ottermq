<script lang="ts">
	import type { ConnectionData } from '$lib/stores/connection.svelte';
	import { getConnections } from '$lib/stores/connection.svelte';
	import { stateColor } from '$lib/utils';

	let connections = $state<ConnectionData[] | null>(null);
	async function getConnectionList() {
		connections = await getConnections();
	}

	$effect(() => {
		getConnectionList();
		const interval = setInterval(getConnectionList, 5000);
		return () => clearInterval(interval);
	});

	function heartbeatDeltaSeconds(last: string) {
		const lastDate = new Date(last).getTime();
		const now = Date.now();
		return Math.floor((now - lastDate) / 1000);
	}

	function formatHeartbeatDelta(totalSeconds: number) {
		if (totalSeconds < 60) {
			return `${totalSeconds}s`;
		}
		const m = Math.floor(totalSeconds / 60);
		const s = totalSeconds % 60;
		return `${m}m${String(s).padStart(2, '0')}s`;
	}

	function formatTime(date: Date) {
		const h = String(date.getHours()).padStart(2, '0');
		const m = String(date.getMinutes()).padStart(2, '0');
		const s = String(date.getSeconds()).padStart(2, '0');
		return `${h}:${m}:${s}`;
	}
	function formatDate(date: Date) {
		const y = date.getFullYear();
		const m = String(date.getMonth() + 1).padStart(2, '0');
		const d = String(date.getDate()).padStart(2, '0');
		return `${y}-${m}-${d}`;
	}
</script>

<h1>Connections</h1>
<div class="table-card">
	<table>
		<thead>
			<tr>
				<th>Vhost</th>
				<th>Name</th>
				<th>User</th>
				<th>State</th>
				<th>SSL</th>
				<th>Protocol</th>
				<th>Channels</th>
				<th>Heartbeat</th>
				<th>Connected At</th>
			</tr>
		</thead>
		<tbody>
			{#each connections as c (c.vhost + c.name)}
				<tr>
					<td>{c.vhost}</td>
					<td>{c.name}</td>
					<td>{c.user_name}</td>
					<td class="state"
						><span class="small-square small-square--{stateColor(c.state)}"></span> {c.state}</td
					>
					<td class="state"><span>{c.ssl ? '●' : '○'}</span></td>
					<td>{c.protocol}</td>
					<td class="num">{c.channels}</td>
					<td class="num">{formatHeartbeatDelta(heartbeatDeltaSeconds(c.last_heartbeat))}</td>
					<td>
						<div class="show-time">{formatTime(new Date(c.connected_at))}</div>
						<div class="show-date">{formatDate(new Date(c.connected_at))}</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.table-card {
		border: 1px solid var(--color-border);
		border-radius: 4px;
		margin-top: 16px;
		overflow-x: auto;
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th,
	td {
		padding: 8px 16px;
	}

	th {
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.03em;
		color: var(--color-text-muted);
		border-bottom: 1px solid var(--color-border);
	}

	thead th:not(:last-child) {
		border-right: 1px solid var(--color-border);
	}

	tbody tr:not(:last-child) td {
		border-bottom: 1px solid var(--color-border);
	}

	tbody td:not(:last-child) {
		border-right: 1px solid var(--color-border);
	}

	.num {
		text-align: right;
		font-variant-numeric: tabular-nums;
	}

	.state {
		text-align: center;
	}

	.show-time {
		font-size: 1em;
		margin-bottom: 2px;
	}
	.show-date {
		font-size: 0.8em;
		color: var(--color-text-muted);
	}
</style>

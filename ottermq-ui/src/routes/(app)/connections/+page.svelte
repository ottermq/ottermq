<script lang="ts">
	import { usePolledList } from '$lib/pooling.svelte';
	import { getConnections } from '$lib/stores/connections.svelte';
	import { stateColor } from '$lib/utils';

	const connections = usePolledList(getConnections);

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
			{#each connections.items as c (c.vhost + c.name)}
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
	.show-time {
		font-size: 1em;
		margin-bottom: 2px;
	}
	.show-date {
		font-size: 0.8em;
		color: var(--color-text-muted);
	}
</style>

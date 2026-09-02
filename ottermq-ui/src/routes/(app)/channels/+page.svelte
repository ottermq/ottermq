<script lang="ts">
	import { usePolledList } from '$lib/pooling.svelte';
	import { getChannels } from '$lib/stores/channels.svelte';
	import { stateColor } from '$lib/utils';

	const channels = usePolledList(getChannels);
</script>

<h1>Channels</h1>
<div class="table-card">
	<table>
		<thead>
			<tr>
				<th>Vhost</th>
				<th>Connection</th>
				<th>Channels</th>
				<th>User</th>
				<th>State</th>
				<th>Prefetch</th>
				<th>Unconfirmed</th>
				<th>Publish</th>
				<th>Unroutable(drop)</th>
				<th>Deliver/s</th>
				<th>Ack/s</th>
			</tr>
		</thead>
		<tbody>
			{#each channels.items as i (i.vhost + i.connection_name + i.number)}
				<tr>
					<td>{i.vhost}</td>
					<td>{i.connection_name}</td>
					<td>{i.number}</td>
					<td>{i.user}</td>
					<td class="state"
						><span class="small-square small-square--{stateColor(i.state)}"></span> {i.state}</td
					>
					<td class="num">{i.prefetch_count}</td>
					<td class="num">{i.unconfirmed_count}</td>
					<td class="num">{i.publish_rate}</td>
					<td class="num">{i.unrotable_rate}</td>
					<td class="num">{i.deliver_rate}</td>
					<td class="num">{i.ack_rate}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

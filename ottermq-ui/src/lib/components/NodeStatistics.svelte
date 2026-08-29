<script lang="ts">

    interface Props {
		uptime_secs: number;
        nodes: NodeStatistics[];
    }

    interface NodeStatistics {
        name: string;
		goroutines: number;
        fd_used: number;
        fd_limit: number;
        memory_usage: number;
        memory_limit: number;
    }

    let {
		uptime_secs,
		nodes,
	}: Props = $props();


	const uptimeFormatter = (secs:number) => {
		if (secs == 0) return '-'
		const d = Math.floor(secs / 86400)
		const h = Math.floor((secs % 86400) / 3600)
		const m = Math.floor((secs % 3600) / 60)
		const s = Math.floor(secs % 60 )
		return `${d}d ${h}h ${m}m ${s}s`
	}
 	let uptime = $derived(uptimeFormatter(uptime_secs ?? 0))
</script>

<div class="node_statistics">
	<h6>Node Statistics</h6>
	{#if nodes }
	<table>
		<thead>
			<tr>
				<td>
					Name
				</td>
				
				<td>
					Gorotines
				</td>
				<td>
					File Descriptors
				</td>
				<td>
					Memory Used
				</td>
			</tr>
		</thead>
		<tbody>
		{#each nodes as node}
			
		<tr>
			<td>
				{node.name}
			</td>
			
			<td>
				{node.goroutines}
			</td>
			<td>
				<span>{node.fd_used} / {node.fd_limit} </span>
			</td>
			<td>
				<span> {node.memory_usage} </span>
			</td>
		</tr>
		{/each}
		</tbody>
	</table>
	{/if}
</div>
            
<style>
		.node_statistics {
		border: 1px solid var(--color-border);
		border-radius: 4px;
		padding: 16px 20px;
		margin-top: 16px;
	}

	.node_statistics {
		font-variant-numeric: tabular-nums;
		color: var(--color-text);
	}
</style>
<script lang="ts">
	import deleteIcon from '$lib/icons/delete.svg?raw';
	import { usePolledList } from '$lib/pooling.svelte';
	import type { QueueData } from '$lib/stores/queues.svelte';
	import { getQueues } from '$lib/stores/queues.svelte';
	import { stateColor } from '$lib/utils';

	const queues = usePolledList(getQueues);

	interface FeatureBadge {
		label: string;
		test: (q: QueueData) => boolean;
	}

	const featureBadges: FeatureBadge[] = [
		{ label: 'D', test: (q) => q.durable },
		{ label: 'AD', test: (q) => q.auto_delete },
		{ label: 'Args', test: (q) => q.arguments !== undefined }
	];
</script>

<h1>Queues</h1>

<div class="table-card">
	<table>
		<thead>
			<tr>
				<th>Virtual Host</th>
				<th>Name</th>
				<th>State</th>
				<th>Ready</th>
				<th>Unacked</th>
				<th>Total</th>
				<th>Consumers</th>
				<th>Features</th>
				<th>Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each queues.items as q (q.vhost + q.name)}
				<tr>
					<td>{q.vhost}</td>
					<td>{q.name}</td>
					<td class="state"
						><span class="small-square small-square--{stateColor(q.state)}"></span> {q.state}</td
					>
					<td class="num">{q.messages_ready}</td>
					<td class="num">{q.messages_unacked}</td>
					<td class="num">{q.messages_total}</td>
					<td class="num">{q.consumers}</td>
					<td class="features">
						<div class="badges">
							{#each featureBadges.filter((b) => b.test(q)) as badge (badge.label)}
								<span class="badge">{badge.label}</span>
							{/each}
						</div>
					</td>
					<td class="action">
						<button class="row-action" aria-label="Delete queue">
							<!-- deleteIcon is a static, build-time-bundled asset, not user/API data -->
							<!-- eslint-disable-next-line svelte/no-at-html-tags -->
							{@html deleteIcon}
						</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.action,
	.features {
		text-align: center;
	}
	.features .badges {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 4px;
	}

	.features .badge {
		font-size: 0.62rem;
		font-weight: 500;
		padding: 2px 6px;
		border-radius: 4px;
		background: color-mix(in srgb, var(--color-surface-raised) 14%, transparent);
		opacity: 0.85;
	}

	.row-action {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		padding: 4px;
		border-radius: 4px;
		color: var(--color-text-muted);
		cursor: pointer;
	}

	.row-action :global(svg) {
		width: 18px;
		height: 18px;
	}

	.row-action:hover {
		/* background: var(--color-border); */
		color: var(--color-text);
	}
</style>

<script lang="ts">
	import deleteIcon from '$lib/icons/delete.svg?raw';
</script>

<h1>Queues</h1>

<div class="table-card">
	<table>
		<thead>
			<tr>
				<th>Virtual Host</th>
				<th>Name</th>
				<th>Status</th>
				<th>Ready</th>
				<th>Unacked</th>
				<th>Total</th>
				<th>Consumers</th>
				<th>D</th>
				<th>AD</th>
				<th>Actions</th>
			</tr>
		</thead>
		<tbody>
			<tr>
				<td>/</td>
				<td>bonjour</td>
				<td class="status"><span class="small-square small-square--green"></span> online</td>
				<td class="num">0</td>
				<td class="num">0</td>
				<td class="num">0</td>
				<td class="num">0</td>
				<td class="flag">x</td>
				<td class="flag">x</td>
				<td class="action">
					<button class="row-action" aria-label="Delete queue">
						<!-- deleteIcon is a static, build-time-bundled asset, not user/API data -->
						<!-- eslint-disable-next-line svelte/no-at-html-tags -->
						{@html deleteIcon}
					</button>
				</td>
			</tr>
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

	.flag,
	.action,
	.status {
		text-align: center;
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

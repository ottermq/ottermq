<script lang="ts">
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
						<svg
							xmlns="http://www.w3.org/2000/svg"
							height="24px"
							viewBox="0 -960 960 960"
							width="24px"
							fill="currentColor"
						>
							<path
								d="M280-120q-33 0-56.5-23.5T200-200v-520h-40v-80h200v-40h240v40h200v80h-40v520q0 33-23.5 56.5T680-120H280Zm400-600H280v520h400v-520ZM360-280h80v-360h-80v360Zm160 0h80v-360h-80v360ZM280-720v520-520Z"
							/>
						</svg>
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
</style>

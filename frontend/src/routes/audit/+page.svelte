<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet } from '$lib/api';
	import type { AuditEntry } from '$lib/types';

	let entries = $state<AuditEntry[]>([]);
	let loading = $state(true);
	let error = $state('');

	let subject = $state('');
	let entityType = $state('');
	let expanded = $state<number | null>(null);

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: '200' });
			if (subject) params.set('subject', subject);
			if (entityType) params.set('entity_type', entityType);
			entries = (await apiGet<AuditEntry[]>(`/api/audit?${params.toString()}`)) ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<div class="topbar"><h1>Audit Log</h1></div>

<div class="toolbar">
	<input placeholder="Filter by subject…" bind:value={subject} onkeydown={(e) => e.key === 'Enter' && load()} />
	<input placeholder="Filter by entity type…" bind:value={entityType} onkeydown={(e) => e.key === 'Enter' && load()} />
	<button class="btn secondary" onclick={load}>Filter</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

<div class="card">
	{#if loading}
		<p class="muted">Loading…</p>
	{:else}
		<table>
			<thead>
				<tr><th>When</th><th>Subject</th><th>Actor</th><th>Entity</th></tr>
			</thead>
			<tbody>
				{#each entries as e}
					<tr
						role="button"
						tabindex="0"
						onclick={() => (expanded = expanded === e.id ? null : e.id)}
						onkeydown={(ev) => (ev.key === 'Enter' || ev.key === ' ') && (ev.preventDefault(), (expanded = expanded === e.id ? null : e.id))}
					>
						<td class="muted">{new Date(e.occurred_at).toLocaleString()}</td>
						<td><span class="badge">{e.subject}</span></td>
						<td class="muted">{e.actor || '—'}</td>
						<td class="muted">{e.entity_type}{e.entity_id ? ` · ${e.entity_id.slice(0, 8)}` : ''}</td>
					</tr>
					{#if expanded === e.id}
						<tr>
							<td colspan="4">
								<pre style="margin:0; white-space:pre-wrap; font-size:12px; color:var(--muted)">{JSON.stringify(e.payload, null, 2)}</pre>
							</td>
						</tr>
					{/if}
				{/each}
				{#if entries.length === 0}
					<tr><td colspan="4" class="muted">No audit entries yet.</td></tr>
				{/if}
			</tbody>
		</table>
	{/if}
</div>

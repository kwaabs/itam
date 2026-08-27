<script lang="ts">
	import type { FieldDefinition } from '$lib/types';

	let {
		fields,
		values = $bindable()
	}: { fields: FieldDefinition[]; values: Record<string, unknown> } = $props();

	function kind(f: FieldDefinition): string {
		return f.data_type?.value_kind ?? 'text';
	}

	function set(key: string, v: unknown) {
		values[key] = v;
	}

	function optValue(o: unknown): string {
		if (o && typeof o === 'object' && 'value' in o) return String((o as { value: unknown }).value);
		return String(o);
	}
	function optLabel(o: unknown): string {
		if (o && typeof o === 'object' && 'label' in o) return String((o as { label: unknown }).label);
		return String(o);
	}
</script>

{#each fields as f}
	<div class="field">
		<label>
			{f.label}{#if f.required}<span style="color:var(--danger)"> *</span>{/if}
			{#if f.unit}<span class="muted"> ({f.unit.symbol})</span>{/if}
		</label>

		{#if kind(f) === 'number'}
			<input
				type="number"
				value={(values[f.key] as number) ?? ''}
				oninput={(e) => set(f.key, e.currentTarget.value === '' ? null : Number(e.currentTarget.value))}
			/>
		{:else if kind(f) === 'bool'}
			<select
				value={String(values[f.key] ?? 'false')}
				onchange={(e) => set(f.key, e.currentTarget.value === 'true')}
			>
				<option value="true">Yes</option>
				<option value="false">No</option>
			</select>
		{:else if kind(f) === 'date'}
			<input
				type="date"
				value={(values[f.key] as string) ?? ''}
				oninput={(e) => set(f.key, e.currentTarget.value || null)}
			/>
		{:else if kind(f) === 'enum'}
			<select
				value={(values[f.key] as string) ?? ''}
				onchange={(e) => set(f.key, e.currentTarget.value || null)}
			>
				<option value="">—</option>
				{#each f.enum_options as o}
					<option value={optValue(o)}>{optLabel(o)}</option>
				{/each}
			</select>
		{:else}
			<input
				type="text"
				value={(values[f.key] as string) ?? ''}
				oninput={(e) => set(f.key, e.currentTarget.value)}
			/>
		{/if}

		{#if f.help_text}<div class="muted" style="font-size:12px; margin-top:4px">{f.help_text}</div>{/if}
	</div>
{/each}

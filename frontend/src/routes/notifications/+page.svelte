<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiPut, apiDelete } from '$lib/api';
	import { me, can } from '$lib/me';
	import type {
		NotificationChannel,
		ScheduledCheck,
		NotificationLogEntry,
		AutomationRule
	} from '$lib/types';

	let tab = $state<'channels' | 'checks' | 'routing' | 'log'>('channels');
	let channels = $state<NotificationChannel[]>([]);
	let checks = $state<ScheduledCheck[]>([]);
	let rules = $state<AutomationRule[]>([]);
	let log = $state<NotificationLogEntry[]>([]);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	const manage = $derived(can($me, 'notification.manage'));
	const canRoute = $derived(can($me, 'metadata.manage'));

	const CH_TYPES = [
		{ id: 'slack', label: 'Slack (incoming webhook)' },
		{ id: 'teams', label: 'Microsoft Teams (webhook)' },
		{ id: 'webhook', label: 'Generic webhook' },
		{ id: 'email', label: 'Email (SMTP)' }
	];
	const CHECK_KINDS = [
		{ id: 'license_expiry', label: 'License expiring' },
		{ id: 'warranty_expiry', label: 'Warranty expiring' },
		{ id: 'asset_stale', label: 'Asset not seen recently' },
		{ id: 'data_quality', label: 'Data quality gaps' },
		{ id: 'sql', label: 'Custom SQL query' }
	];

	// forms
	let showChForm = $state(false);
	let chForm = $state({ key: '', name: '', type: 'slack', url: '', host: '', port: '587', from: '', to: '', username: '', password: '' });
	let showCkForm = $state(false);
	let ckForm = $state({ key: '', name: '', kind: 'license_expiry', days: '30', gap_threshold: '5', query: '', event_subject: 'itam.license.expiring', interval_hours: '24' });
	let showRuleForm = $state(false);
	let ruleForm = $state({ key: '', name: '', on_subject: '', channel: '', template: '' });
	let saving = $state(false);

	async function load() {
		loading = true;
		error = '';
		try {
			channels = (await apiGet<NotificationChannel[]>('/api/notifications/channels')) ?? [];
			checks = (await apiGet<ScheduledCheck[]>('/api/notifications/checks')) ?? [];
			log = (await apiGet<NotificationLogEntry[]>('/api/notifications/log')) ?? [];
			if (canRoute) {
				const all = (await apiGet<AutomationRule[]>('/api/metadata/automation-rules')) ?? [];
				rules = all.filter((r) => (r.action as { type?: string })?.type === 'notify');
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		} finally {
			loading = false;
		}
	}
	onMount(load);

	function flash(msg: string) {
		notice = msg;
		setTimeout(() => (notice = ''), 4000);
	}

	function channelConfig(): Record<string, string> {
		if (chForm.type === 'email') {
			return { host: chForm.host, port: chForm.port, from: chForm.from, to: chForm.to, username: chForm.username, password: chForm.password };
		}
		return { url: chForm.url };
	}

	async function createChannel() {
		saving = true;
		error = '';
		try {
			await apiPost('/api/notifications/channels', {
				key: chForm.key, name: chForm.name, type: chForm.type, enabled: true, config: channelConfig()
			});
			showChForm = false;
			chForm = { key: '', name: '', type: 'slack', url: '', host: '', port: '587', from: '', to: '', username: '', password: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create channel';
		} finally {
			saving = false;
		}
	}

	async function toggleChannel(c: NotificationChannel) {
		try {
			await apiPut(`/api/notifications/channels/${c.id}`, { ...c, enabled: !c.enabled });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function testChannel(c: NotificationChannel) {
		error = '';
		try {
			await apiPost(`/api/notifications/channels/${c.id}/test`);
			flash(`Test notification sent to "${c.name}".`);
			await load();
		} catch (e) {
			error = `Test failed: ${e instanceof Error ? e.message : e}`;
			await load();
		}
	}
	async function removeChannel(c: NotificationChannel) {
		if (!confirm(`Delete channel "${c.name}"?`)) return;
		try {
			await apiDelete(`/api/notifications/channels/${c.id}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	function checkParams(): Record<string, unknown> {
		if (ckForm.kind === 'sql') return { query: ckForm.query };
		if (ckForm.kind === 'data_quality') {
			return { gap_threshold: Number(ckForm.gap_threshold) || 5, dedupe: false };
		}
		return { days: Number(ckForm.days) || 30 };
	}
	function defaultSubject(kind: string): string {
		return {
			license_expiry: 'itam.license.expiring',
			warranty_expiry: 'itam.warranty.expiring',
			asset_stale: 'itam.asset.stale',
			data_quality: 'itam.data_quality.gaps',
			sql: 'itam.custom.check'
		}[kind] ?? 'itam.custom.check';
	}
	$effect(() => {
		// keep the suggested subject in sync with the chosen kind
		ckForm.event_subject = defaultSubject(ckForm.kind);
	});

	async function createCheck() {
		saving = true;
		error = '';
		try {
			await apiPost('/api/notifications/checks', {
				key: ckForm.key, name: ckForm.name, kind: ckForm.kind,
				interval_seconds: (Number(ckForm.interval_hours) || 24) * 3600,
				params: checkParams(), event_subject: ckForm.event_subject, enabled: false
			});
			showCkForm = false;
			ckForm = { key: '', name: '', kind: 'license_expiry', days: '30', gap_threshold: '5', query: '', event_subject: 'itam.license.expiring', interval_hours: '24' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create check';
		} finally {
			saving = false;
		}
	}
	function warrantyPreset() {
		ckForm = {
			key: 'warranty_reminder_30',
			name: 'Warranty renewal reminder (30d)',
			kind: 'warranty_expiry',
			days: '30',
			gap_threshold: '5',
			query: '',
			event_subject: 'itam.warranty.expiring',
			interval_hours: '24'
		};
		showCkForm = true;
	}
	function dataQualityPreset() {
		ckForm = {
			key: 'data-quality-gaps',
			name: 'Data quality gaps exceed threshold',
			kind: 'data_quality',
			days: '30',
			gap_threshold: '5',
			query: '',
			event_subject: 'itam.data_quality.gaps',
			interval_hours: '168'
		};
		showCkForm = true;
	}

	async function toggleCheck(c: ScheduledCheck) {
		try {
			await apiPut(`/api/notifications/checks/${c.id}`, { ...c, enabled: !c.enabled });
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}
	async function runCheck(c: ScheduledCheck) {
		error = '';
		try {
			const res = await apiPost<{ count: number }>(`/api/notifications/checks/${c.id}/run`);
			flash(`"${c.name}" ran: ${res.count} match(es).`);
			await load();
		} catch (e) {
			error = `Run failed: ${e instanceof Error ? e.message : e}`;
		}
	}
	async function removeCheck(c: ScheduledCheck) {
		if (!confirm(`Delete check "${c.name}"?`)) return;
		try {
			await apiDelete(`/api/notifications/checks/${c.id}`);
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed';
		}
	}

	async function createRule() {
		saving = true;
		error = '';
		try {
			await apiPost('/api/metadata/automation-rules', {
				key: ruleForm.key, name: ruleForm.name, on_subject: ruleForm.on_subject,
				condition: {}, action: { type: 'notify', channel: ruleForm.channel, template: ruleForm.template },
				enabled: true, sort: 100
			});
			showRuleForm = false;
			ruleForm = { key: '', name: '', on_subject: '', channel: '', template: '' };
			await load();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to create rule';
		} finally {
			saving = false;
		}
	}

	function fmt(ts?: string | null): string {
		return ts ? new Date(ts).toLocaleString() : '—';
	}
	function statusClass(s?: string): string {
		if (s === 'sent' || s === 'ok' || s === 'success') return 'ok';
		if (s === 'error') return 'bad';
		return 'muted';
	}
	function intervalLabel(sec: number): string {
		if (sec % 86400 === 0) return `${sec / 86400}d`;
		if (sec % 3600 === 0) return `${sec / 3600}h`;
		return `${Math.round(sec / 60)}m`;
	}
</script>

<div class="page">
	<header class="head">
		<div>
			<h1>Notifications</h1>
			<p class="muted">Metadata-driven alerting. Define <strong>channels</strong> (where), <strong>scheduled checks</strong> (what to watch), and <strong>routing rules</strong> (which event goes to which channel). Nothing is hardcoded.</p>
		</div>
	</header>

	{#if error}<div class="banner bad">{error}</div>{/if}
	{#if notice}<div class="banner ok">{notice}</div>{/if}

	<div class="tabs">
		<button class="tab" class:active={tab === 'channels'} onclick={() => (tab = 'channels')}>Channels</button>
		<button class="tab" class:active={tab === 'checks'} onclick={() => (tab = 'checks')}>Scheduled checks</button>
		{#if canRoute}
			<button class="tab" class:active={tab === 'routing'} onclick={() => (tab = 'routing')}>Routing rules</button>
		{/if}
		<button class="tab" class:active={tab === 'log'} onclick={() => (tab = 'log')}>Delivery log</button>
	</div>

	{#if loading}
		<p class="muted">Loading…</p>
	{:else if tab === 'channels'}
		{#if manage}
			<div class="bar"><button class="btn" onclick={() => (showChForm = !showChForm)}>New channel</button></div>
		{/if}
		{#if showChForm}
			<div class="card form">
				<div class="grid cols-3">
					<div class="field"><label for="ch-name">Name</label><input id="ch-name" bind:value={chForm.name} placeholder="Ops Slack" /></div>
					<div class="field"><label for="ch-key">Key</label><input id="ch-key" bind:value={chForm.key} placeholder="ops_slack" /></div>
					<div class="field"><label for="ch-type">Type</label>
						<select id="ch-type" bind:value={chForm.type}>{#each CH_TYPES as t}<option value={t.id}>{t.label}</option>{/each}</select>
					</div>
				</div>
				{#if chForm.type === 'email'}
					<div class="grid cols-3" style="margin-top:10px">
						<div class="field"><label>SMTP host</label><input bind:value={chForm.host} placeholder="smtp.office365.com" /></div>
						<div class="field"><label>Port</label><input bind:value={chForm.port} /></div>
						<div class="field"><label>From</label><input bind:value={chForm.from} placeholder="itam@corp.com" /></div>
						<div class="field"><label>To (comma-separated)</label><input bind:value={chForm.to} placeholder="ops@corp.com" /></div>
						<div class="field"><label>Username</label><input bind:value={chForm.username} /></div>
						<div class="field"><label>Password</label><input type="password" bind:value={chForm.password} /></div>
					</div>
				{:else}
					<div class="field" style="margin-top:10px"><label>Webhook URL</label><input bind:value={chForm.url} placeholder="https://hooks.slack.com/services/…" /></div>
				{/if}
				<div class="actions">
					<button class="btn" onclick={createChannel} disabled={saving || !chForm.name || !chForm.key}>{saving ? 'Saving…' : 'Create channel'}</button>
					<button class="btn secondary" onclick={() => (showChForm = false)}>Cancel</button>
				</div>
			</div>
		{/if}
		<div class="card">
			<table>
				<thead><tr><th>Channel</th><th>Type</th><th>Status</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each channels as c}
						<tr>
							<td><strong>{c.name}</strong><div class="muted mono">{c.key}</div></td>
							<td><span class="badge">{c.type}</span></td>
							<td>{#if c.enabled}<span class="dot ok"></span> Enabled{:else}<span class="dot"></span> <span class="muted">Disabled</span>{/if}</td>
							{#if manage}
								<td style="text-align:right">
									<button class="btn secondary small" onclick={() => testChannel(c)}>Test</button>
									<button class="btn secondary small" onclick={() => toggleChannel(c)}>{c.enabled ? 'Disable' : 'Enable'}</button>
									<button class="btn danger small" onclick={() => removeChannel(c)}>Delete</button>
								</td>
							{/if}
						</tr>
					{/each}
					{#if channels.length === 0}<tr><td colspan={manage ? 4 : 3} class="muted">No channels yet.</td></tr>{/if}
				</tbody>
			</table>
		</div>
	{:else if tab === 'checks'}
		{#if manage}
			<div class="bar" style="display:flex; gap:8px">
				<button class="btn" onclick={() => (showCkForm = !showCkForm)}>New check</button>
				<button class="btn secondary" onclick={warrantyPreset}>+ Warranty reminder preset</button>
				<button class="btn secondary" onclick={dataQualityPreset}>+ Data quality preset</button>
			</div>
		{/if}
		<p class="muted small" style="margin:-4px 0 12px">
			<strong>Data quality gaps</strong> runs weekly by default when asset gaps exceed the threshold.
			Warranty reminders fire once per asset per expiry date.
		</p>
		{#if showCkForm}
			<div class="card form">
				<div class="grid cols-3">
					<div class="field"><label>Name</label><input bind:value={ckForm.name} placeholder="License expiring (30d)" /></div>
					<div class="field"><label>Key</label><input bind:value={ckForm.key} placeholder="license_expiry_30" /></div>
					<div class="field"><label>Kind</label>
						<select bind:value={ckForm.kind}>{#each CHECK_KINDS as k}<option value={k.id}>{k.label}</option>{/each}</select>
					</div>
					{#if ckForm.kind === 'sql'}
						<div class="field" style="grid-column:1 / -1"><label>SELECT query</label><textarea bind:value={ckForm.query} rows="3" placeholder="select id, name from core.assets where …"></textarea></div>
					{:else if ckForm.kind === 'data_quality'}
						<div class="field"><label>Gap threshold (assets)</label><input bind:value={ckForm.gap_threshold} /></div>
					{:else}
						<div class="field"><label>Threshold (days)</label><input bind:value={ckForm.days} /></div>
					{/if}
					<div class="field"><label>Run every (hours)</label><input bind:value={ckForm.interval_hours} /></div>
					<div class="field"><label>Emits event</label><input bind:value={ckForm.event_subject} /></div>
				</div>
				<div class="actions">
					<button class="btn" onclick={createCheck} disabled={saving || !ckForm.name || !ckForm.key}>{saving ? 'Saving…' : 'Create check'}</button>
					<button class="btn secondary" onclick={() => (showCkForm = false)}>Cancel</button>
				</div>
			</div>
		{/if}
		<div class="card">
			<table>
				<thead><tr><th>Check</th><th>Kind</th><th>Every</th><th>Emits</th><th>Last run</th><th>Status</th>{#if manage}<th></th>{/if}</tr></thead>
				<tbody>
					{#each checks as c}
						<tr>
							<td><strong>{c.name}</strong><div class="muted mono">{c.key}</div></td>
							<td><span class="badge">{c.kind}</span></td>
							<td class="muted">{intervalLabel(c.interval_seconds)}</td>
							<td class="muted mono">{c.event_subject}</td>
							<td class="muted">{fmt(c.last_run_at)}{#if c.last_run_at} · {c.last_count} hit(s){/if}</td>
							<td>
								<span class={statusClass(c.last_status)}>{c.last_status || '—'}</span>
								{#if !c.enabled}<span class="badge" style="margin-left:6px">paused</span>{/if}
							</td>
							{#if manage}
								<td style="text-align:right">
									<button class="btn secondary small" onclick={() => runCheck(c)}>Run now</button>
									<button class="btn secondary small" onclick={() => toggleCheck(c)}>{c.enabled ? 'Pause' : 'Enable'}</button>
									<button class="btn danger small" onclick={() => removeCheck(c)}>Delete</button>
								</td>
							{/if}
						</tr>
					{/each}
					{#if checks.length === 0}<tr><td colspan={manage ? 7 : 6} class="muted">No checks yet.</td></tr>{/if}
				</tbody>
			</table>
		</div>
	{:else if tab === 'routing'}
		{#if canRoute}
			<div class="bar"><button class="btn" onclick={() => (showRuleForm = !showRuleForm)}>New routing rule</button></div>
		{/if}
		{#if showRuleForm}
			<div class="card form">
				<div class="grid cols-2">
					<div class="field"><label>Name</label><input bind:value={ruleForm.name} placeholder="Notify ops of license expiry" /></div>
					<div class="field"><label>Key</label><input bind:value={ruleForm.key} placeholder="notify_license_expiry" /></div>
					<div class="field"><label>On event subject</label><input bind:value={ruleForm.on_subject} placeholder="itam.license.expiring" /></div>
					<div class="field"><label>Channel</label>
						<select bind:value={ruleForm.channel}>
							<option value="">Select channel…</option>
							{#each channels as c}<option value={c.key}>{c.name} ({c.type})</option>{/each}
						</select>
					</div>
					<div class="field" style="grid-column:1 / -1"><label>Message template</label><input bind:value={ruleForm.template} placeholder={'[{{subject}}] {{count}} licenses expiring soon'} /></div>
				</div>
				<div class="actions">
					<button class="btn" onclick={createRule} disabled={saving || !ruleForm.name || !ruleForm.key || !ruleForm.on_subject || !ruleForm.channel}>{saving ? 'Saving…' : 'Create rule'}</button>
					<button class="btn secondary" onclick={() => (showRuleForm = false)}>Cancel</button>
				</div>
				<p class="muted small" style="margin-top:8px">Template tokens: <code>{'{{subject}}'}</code>, <code>{'{{count}}'}</code>, <code>{'{{entity_id}}'}</code>, plus any event payload field.</p>
			</div>
		{/if}
		<div class="card">
			<table>
				<thead><tr><th>Rule</th><th>On subject</th><th>→ Channel</th><th>Enabled</th></tr></thead>
				<tbody>
					{#each rules as r}
						<tr>
							<td><strong>{r.name}</strong><div class="muted mono">{r.key}</div></td>
							<td class="mono muted">{r.on_subject}</td>
							<td class="mono">{(r.action as { channel?: string }).channel ?? '—'}</td>
							<td>{#if r.enabled}<span class="dot ok"></span>{:else}<span class="dot"></span>{/if}</td>
						</tr>
					{/each}
					{#if rules.length === 0}<tr><td colspan="4" class="muted">No routing rules yet. Create one to deliver events to a channel.</td></tr>{/if}
				</tbody>
			</table>
		</div>
	{:else if tab === 'log'}
		<div class="card">
			<table>
				<thead><tr><th>When</th><th>Channel</th><th>Subject</th><th>Status</th><th>Detail</th></tr></thead>
				<tbody>
					{#each log as e}
						<tr>
							<td class="muted">{fmt(e.created_at)}</td>
							<td class="mono">{e.channel_key || '—'}</td>
							<td class="mono muted">{e.subject || '—'}</td>
							<td class={statusClass(e.status)}>{e.status}</td>
							<td class="muted small">{e.error || (e.payload as { message?: string })?.message || ''}</td>
						</tr>
					{/each}
					{#if log.length === 0}<tr><td colspan="5" class="muted">No deliveries yet.</td></tr>{/if}
				</tbody>
			</table>
		</div>
	{/if}
</div>

<style>
	.page { padding: 22px 26px; max-width: 1120px; }
	.head h1 { margin: 0 0 4px; }
	.head p { margin: 0; max-width: 720px; }
	.tabs { display: flex; gap: 4px; border-bottom: 1px solid var(--border); margin: 16px 0; }
	.tab { background: none; border: none; padding: 9px 14px; color: var(--muted); cursor: pointer; border-bottom: 2px solid transparent; font-size: 14px; }
	.tab.active { color: var(--text); border-bottom-color: var(--primary); font-weight: 600; }
	.bar { margin-bottom: 12px; }
	.card { background: var(--surface); border: 1px solid var(--border); border-radius: 12px; padding: 16px; margin-bottom: 16px; }
	.card.form { padding: 18px; }
	.grid { display: grid; gap: 12px; }
	.cols-2 { grid-template-columns: 1fr 1fr; }
	.cols-3 { grid-template-columns: 1fr 1fr 1fr; }
	.field label { display: block; font-size: 12px; color: var(--muted); margin-bottom: 4px; }
	.field input, .field select, .field textarea { width: 100%; }
	.actions { display: flex; gap: 8px; margin-top: 14px; }
	table { width: 100%; border-collapse: collapse; }
	th, td { text-align: left; padding: 9px 10px; border-bottom: 1px solid var(--border); font-size: 14px; }
	.badge { background: var(--surface-2); border: 1px solid var(--border); border-radius: 6px; padding: 1px 8px; font-size: 12px; }
	.dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #888; margin-right: 4px; }
	.dot.ok { background: var(--success); }
	.muted { color: var(--muted); }
	.mono { font-family: ui-monospace, monospace; font-size: 12px; }
	.small { font-size: 12px; }
	.ok { color: var(--success); }
	.bad { color: var(--danger); }
	.banner { padding: 10px 14px; border-radius: 10px; margin-bottom: 14px; border: 1px solid var(--border); }
	.banner.bad { background: color-mix(in srgb, var(--danger) 12%, transparent); border-color: var(--danger); }
	.banner.ok { background: color-mix(in srgb, var(--success) 12%, transparent); border-color: var(--success); }
	.btn.small { padding: 4px 10px; font-size: 12px; }
</style>

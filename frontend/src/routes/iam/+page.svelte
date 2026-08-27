<script lang="ts">
	import { onMount } from 'svelte';
	import { apiGet, apiPost, apiDelete } from '$lib/api';
	import type { Role, Permission, UserProfile, RoleGrant, Location, OrgUnit } from '$lib/types';

	let tab = $state<'grants' | 'roles' | 'users'>('grants');
	let error = $state('');

	let roles = $state<Role[]>([]);
	let permissions = $state<Permission[]>([]);
	let users = $state<UserProfile[]>([]);
	let grants = $state<RoleGrant[]>([]);
	let locations = $state<Location[]>([]);
	let orgUnits = $state<OrgUnit[]>([]);

	let g = $state({ user_id: '', role_id: '', scope_type: 'global', scope_path: '' });

	const rolesById = $derived(new Map(roles.map((r) => [r.id, r])));
	const usersById = $derived(new Map(users.map((u) => [u.user_id, u])));

	async function load() {
		error = '';
		try {
			roles = await apiGet<Role[]>('/api/iam/roles');
			permissions = await apiGet<Permission[]>('/api/iam/permissions');
			users = await apiGet<UserProfile[]>('/api/iam/users');
			grants = await apiGet<RoleGrant[]>('/api/iam/grants');
			locations = await apiGet<Location[]>('/api/locations');
			orgUnits = await apiGet<OrgUnit[]>('/api/org-units');
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to load';
		}
	}

	onMount(load);

	function depth(path: string): number {
		return Math.max(0, path.split('.').length - 1);
	}

	async function createGrant(e: Event) {
		e.preventDefault();
		error = '';
		try {
			await apiPost('/api/iam/grants', {
				user_id: g.user_id,
				role_id: Number(g.role_id),
				scope_type: g.scope_type,
				scope_path: g.scope_type === 'global' ? '' : g.scope_path
			});
			g = { user_id: '', role_id: '', scope_type: 'global', scope_path: '' };
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to create grant';
		}
	}

	async function removeGrant(grant: RoleGrant) {
		if (!confirm('Revoke this grant?')) return;
		try {
			await apiDelete(`/api/iam/grants/${grant.id}`);
			await load();
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to revoke';
		}
	}

	function userLabel(id: string): string {
		const u = usersById.get(id);
		return u ? u.email : id;
	}
</script>

<div class="topbar"><h1>Access Control</h1></div>

<div class="tabs">
	<button type="button" class="tab" class:active={tab === 'grants'} onclick={() => (tab = 'grants')}>Grants</button>
	<button type="button" class="tab" class:active={tab === 'roles'} onclick={() => (tab = 'roles')}>Roles & Permissions</button>
	<button type="button" class="tab" class:active={tab === 'users'} onclick={() => (tab = 'users')}>Users</button>
</div>

{#if error}<p class="error">{error}</p>{/if}

{#if tab === 'grants'}
	<div class="card" style="margin-bottom:16px">
		<h3 style="margin-top:0">Grant a role</h3>
		<p class="muted" style="margin-top:-6px">
			A grant gives a user a role within a scope. Global applies everywhere; location/org-unit grants apply to the chosen subtree and everything beneath it.
		</p>
		<form onsubmit={createGrant}>
			<div class="grid cols-4">
				<div class="field">
					<label>User *</label>
					<select bind:value={g.user_id} required>
						<option value="">Select user…</option>
						{#each users as u}<option value={u.user_id}>{u.email}</option>{/each}
					</select>
				</div>
				<div class="field">
					<label>Role *</label>
					<select bind:value={g.role_id} required>
						<option value="">Select role…</option>
						{#each roles as r}<option value={String(r.id)}>{r.name}</option>{/each}
					</select>
				</div>
				<div class="field">
					<label>Scope</label>
					<select bind:value={g.scope_type}>
						<option value="global">Global</option>
						<option value="location">Location subtree</option>
						<option value="org_unit">Org unit subtree</option>
					</select>
				</div>
				<div class="field">
					<label>Scope target</label>
					{#if g.scope_type === 'location'}
						<select bind:value={g.scope_path}>
							<option value="">Select location…</option>
							{#each locations as l}<option value={l.path}>{'— '.repeat(depth(l.path))}{l.name}</option>{/each}
						</select>
					{:else if g.scope_type === 'org_unit'}
						<select bind:value={g.scope_path}>
							<option value="">Select org unit…</option>
							{#each orgUnits as o}<option value={o.path}>{'— '.repeat(depth(o.path))}{o.name}</option>{/each}
						</select>
					{:else}
						<input value="entire organization" disabled />
					{/if}
				</div>
			</div>
			<button class="btn" type="submit">Add grant</button>
		</form>
	</div>

	<div class="card">
		<h3 style="margin-top:0">Current grants</h3>
		<table>
			<thead><tr><th>User</th><th>Role</th><th>Scope</th><th></th></tr></thead>
			<tbody>
				{#each grants as gr}
					<tr>
						<td>{userLabel(gr.user_id)}</td>
						<td>{gr.role?.name ?? rolesById.get(gr.role_id)?.name ?? gr.role_id}</td>
						<td>
							{#if gr.scope_type === 'global'}
								<span class="badge">global</span>
							{:else}
								<span class="badge">{gr.scope_type}</span> <span class="muted">{gr.scope_path}</span>
							{/if}
						</td>
						<td style="text-align:right"><button class="btn danger small" onclick={() => removeGrant(gr)}>Revoke</button></td>
					</tr>
				{/each}
				{#if grants.length === 0}<tr><td colspan="4" class="muted">No grants yet.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{:else if tab === 'roles'}
	<div class="grid cols-2">
		<div class="card">
			<h3 style="margin-top:0">Roles</h3>
			<table>
				<thead><tr><th>Name</th><th>Key</th><th>Description</th></tr></thead>
				<tbody>
					{#each roles as r}
						<tr><td>{r.name}</td><td class="muted">{r.key}</td><td class="muted">{r.description}</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
		<div class="card">
			<h3 style="margin-top:0">Permissions</h3>
			<table>
				<thead><tr><th>Key</th><th>Description</th></tr></thead>
				<tbody>
					{#each permissions as p}
						<tr><td class="muted">{p.key}</td><td>{p.description}</td></tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>
{:else if tab === 'users'}
	<div class="card">
		<h3 style="margin-top:0">Users</h3>
		<p class="muted" style="margin-top:-6px">Users are provisioned automatically on first sign-in via GoTrue.</p>
		<table>
			<thead><tr><th>Email</th><th>Superuser</th><th>Active</th><th>User ID</th></tr></thead>
			<tbody>
				{#each users as u}
					<tr>
						<td>{u.email}</td>
						<td>{u.is_superuser ? '✓' : '—'}</td>
						<td>{u.is_active ? '✓' : '—'}</td>
						<td class="muted" style="font-size:12px">{u.user_id}</td>
					</tr>
				{/each}
				{#if users.length === 0}<tr><td colspan="4" class="muted">No users yet.</td></tr>{/if}
			</tbody>
		</table>
	</div>
{/if}

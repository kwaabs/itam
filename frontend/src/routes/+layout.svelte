<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { session } from '$lib/session';
	import { me, can } from '$lib/me';
	import { logout, captureOAuthRedirect } from '$lib/auth';
	import { apiGet } from '$lib/api';
	import { theme } from '$lib/theme';
	import Icon from '$lib/components/Icon.svelte';
	import type { Me } from '$lib/types';

	let { children } = $props();

	async function loadMe() {
		try {
			me.set(await apiGet<Me>('/api/me'));
		} catch {
			me.set(null);
		}
	}

	onMount(() => {
		captureOAuthRedirect();
		const unsub = session.subscribe((s) => {
			if (!s) {
				me.set(null);
				if ($page.url.pathname !== '/login') goto('/login');
			} else {
				loadMe();
			}
		});
		return unsub;
	});

	function handleLogout() {
		logout();
		goto('/login');
	}

	let menuOpen = $state(false);
	function initials(email?: string | null): string {
		if (!email) return '?';
		const name = email.split('@')[0];
		const parts = name.split(/[._-]+/).filter(Boolean);
		const chars = parts.length >= 2 ? parts[0][0] + parts[1][0] : name.slice(0, 2);
		return chars.toUpperCase();
	}

	type NavItem = {
		href?: string;
		label: string;
		icon: string;
		perm: string | string[] | null;
		children?: NavItem[];
	};
	type NavGroup = { label: string; items: NavItem[] };

	const groups: NavGroup[] = [
		{
			label: 'Overview',
			items: [
				{ href: '/', label: 'Dashboard', icon: 'dashboard', perm: null },
				{ href: '/reports', label: 'Reports', icon: 'report', perm: 'report.read' }
			]
		},
		{
			label: 'Inventory',
			items: [
				{ href: '/assets', label: 'Assets', icon: 'assets', perm: 'asset.read' },
				{ href: '/peripherals', label: 'Peripherals', icon: 'peripheral', perm: 'asset.read' },
				{ href: '/stock', label: 'Stock', icon: 'stock', perm: 'asset.read' },
				{ href: '/stores', label: 'Stores', icon: 'store', perm: 'hierarchy.read' },
				{ href: '/software', label: 'Software', icon: 'software', perm: 'software.read' },
				{ href: '/locations', label: 'Locations', icon: 'locations', perm: 'hierarchy.read' }
			]
		},
		{
			label: 'Infrastructure',
			items: [
				{
					label: 'Data Center',
					icon: 'datacenter',
					perm: 'dcim.read',
					children: [
						{ href: '/datacenter', label: 'Floor & Racks', icon: 'datacenter', perm: 'dcim.read' },
						{ href: '/capacity', label: 'Capacity', icon: 'gauge', perm: 'dcim.read' }
					]
				},
				{
					label: 'Compute & Storage',
					icon: 'virt',
					perm: ['virt.read', 'storage.read'],
					children: [
						{ href: '/virtualization', label: 'Virtualization', icon: 'virt', perm: 'virt.read' },
						{ href: '/storage', label: 'Storage', icon: 'storage', perm: 'storage.read' }
					]
				},
				{
					label: 'Networking',
					icon: 'network',
					perm: ['dcim.read', 'netcfg.read', 'ipam.read'],
					children: [
						{ href: '/network', label: 'Network', icon: 'network', perm: 'dcim.read' },
						{ href: '/netconfig', label: 'Firewall / Net Config', icon: 'shield', perm: 'netcfg.read' },
						{ href: '/ipam', label: 'IPAM', icon: 'ipam', perm: 'ipam.read' }
					]
				}
			]
		},
		{
			label: 'Operations',
			items: [
				{ href: '/procurement', label: 'Procurement', icon: 'procurement', perm: 'procurement.read' },
				{ href: '/vendors', label: 'Vendors', icon: 'vendor', perm: 'procurement.read' },
				{ href: '/costs', label: 'Costs', icon: 'cost', perm: 'cost.read' },
				{ href: '/import', label: 'Import', icon: 'import', perm: ['asset.write', 'procurement.manage'] },
				{ href: '/integrations', label: 'Integrations', icon: 'integrations', perm: 'integration.read' }
			]
		},
		{
			label: 'Directory',
			items: [
				{ href: '/org-units', label: 'Org Units', icon: 'org', perm: 'hierarchy.read' },
				{ href: '/people', label: 'People', icon: 'people', perm: 'hierarchy.read' }
			]
		},
		{
			label: 'Administration',
			items: [
				{ href: '/admin', label: 'Metadata', icon: 'metadata', perm: 'metadata.read' },
				{ href: '/notifications', label: 'Notifications', icon: 'bell', perm: 'notification.read' },
				{ href: '/iam', label: 'Access Control', icon: 'iam', perm: 'iam.manage' },
				{ href: '/audit', label: 'Audit Log', icon: 'audit', perm: 'audit.read' }
			]
		}
	];

	function allowed(perm: string | string[] | null): boolean {
		if (!perm) return true;
		const perms = Array.isArray(perm) ? perm : [perm];
		return perms.some((p) => can($me, p));
	}

	function isActive(href: string | undefined, path: string): boolean {
		if (!href) return false;
		return href === '/' ? path === '/' : path.startsWith(href);
	}

	// Visible children of a parent item, filtered by permission.
	function visibleChildren(item: NavItem): NavItem[] {
		return (item.children ?? []).filter((c) => allowed(c.perm));
	}
	// Is an item visible at all? Leaf: own perm. Parent: any visible child.
	function itemVisible(item: NavItem): boolean {
		if (item.children) return visibleChildren(item).length > 0;
		return allowed(item.perm);
	}
	function itemHasActive(item: NavItem, path: string): boolean {
		if (item.children) return visibleChildren(item).some((c) => isActive(c.href, path));
		return isActive(item.href, path);
	}

	// Flatten an item (and any children) into leaf links — used in rail mode.
	function leafLinks(items: NavItem[]): NavItem[] {
		const out: NavItem[] = [];
		for (const i of items) {
			if (i.children) out.push(...visibleChildren(i));
			else if (allowed(i.perm)) out.push(i);
		}
		return out;
	}

	// Collapsed groups (persisted). Active group always shows its items.
	let collapsed = $state<Record<string, boolean>>({});
	function toggleGroup(label: string) {
		collapsed = { ...collapsed, [label]: !collapsed[label] };
	}
	function groupHasActive(g: NavGroup, path: string): boolean {
		return g.items.some((i) => itemVisible(i) && itemHasActive(i, path));
	}

	// Expanded sub-menus (persisted).
	let subOpen = $state<Record<string, boolean>>({});
	function toggleSub(label: string) {
		subOpen = { ...subOpen, [label]: !subOpen[label] };
	}

	// Collapsed icon-only rail (persisted).
	let rail = $state(false);
	onMount(() => {
		rail = localStorage.getItem('sidebar-rail') === '1';
	});
	function toggleRail() {
		rail = !rail;
		localStorage.setItem('sidebar-rail', rail ? '1' : '0');
	}
</script>

{#if $page.url.pathname === '/login'}
	{@render children()}
{:else}
	<div class="app" class:rail>
		<header class="appbar">
			<div class="brand">
				<button class="rail-toggle" onclick={toggleRail} title={rail ? 'Expand menu' : 'Collapse menu'} aria-label="Toggle menu">
					<span class="bar"></span><span class="bar"></span><span class="bar"></span>
				</button>
				<span class="brand-text">IT<span>AM</span></span>
			</div>
			<div class="appbar-spacer"></div>
			<div class="appbar-right">
				<button class="user-btn" class:open={menuOpen} onclick={() => (menuOpen = !menuOpen)} aria-haspopup="menu" aria-expanded={menuOpen}>
					<span class="avatar">{initials($me?.email)}</span>
					<span class="user-meta">
						<span class="user-email">{$me?.email ?? ''}</span>
						<span class="user-role">{$me?.is_superuser ? 'Superuser' : 'User'}</span>
					</span>
					<span class="caret">▾</span>
				</button>
				{#if menuOpen}
					<button class="menu-backdrop" onclick={() => (menuOpen = false)} aria-label="Close menu" tabindex="-1"></button>
					<div class="user-menu" role="menu">
						<div class="menu-id">
							<div class="user-email">{$me?.email ?? ''}</div>
							<div class="muted" style="font-size:11px">{$me?.is_superuser ? 'Superuser' : 'User'}</div>
						</div>
						<div class="menu-sep"></div>
						<div class="menu-row">
							<span class="menu-label">Theme</span>
							<div class="theme-switch" role="group" aria-label="Theme">
								<button class:active={$theme === 'light'} title="Light" onclick={() => theme.set('light')}>☀</button>
								<button class:active={$theme === 'system'} title="System" onclick={() => theme.set('system')}>◐</button>
								<button class:active={$theme === 'dark'} title="Dark" onclick={() => theme.set('dark')}>☾</button>
							</div>
						</div>
						<div class="menu-sep"></div>
						<button class="menu-item" role="menuitem" onclick={() => { menuOpen = false; handleLogout(); }}>Sign out</button>
					</div>
				{/if}
			</div>
		</header>

		<aside class="sidebar">
			{#each groups as g}
				{@const visible = g.items.filter((i) => itemVisible(i))}
				{#if visible.length}
					{@const isCollapsed = !rail && collapsed[g.label] && !groupHasActive(g, $page.url.pathname)}
					<div class="nav-group" class:collapsed={isCollapsed}>
						{#if rail}
							<div class="rail-sep"></div>
						{:else}
							<button class="nav-group-header" onclick={() => toggleGroup(g.label)}>
								<span>{g.label}</span>
								<span class="chev">▾</span>
							</button>
						{/if}
						{#if rail}
							{#each leafLinks(visible) as item}
								<a
									class="nav-link"
									class:active={isActive(item.href, $page.url.pathname)}
									href={item.href}
									title={item.label}
								>
									<span class="nav-ico"><Icon name={item.icon} /></span>
									<span class="nav-label">{item.label}</span>
								</a>
							{/each}
						{:else if !isCollapsed}
							{#each visible as item}
								{#if item.children}
									{@const open = subOpen[item.label] || itemHasActive(item, $page.url.pathname)}
									<button
										class="nav-link nav-parent"
										class:active={itemHasActive(item, $page.url.pathname)}
										onclick={() => toggleSub(item.label)}
									>
										<span class="nav-ico"><Icon name={item.icon} /></span>
										<span class="nav-label">{item.label}</span>
										<span class="sub-chev" class:open>▾</span>
									</button>
									{#if open}
										<div class="nav-sub">
											{#each visibleChildren(item) as child}
												<a
													class="nav-link nav-child"
													class:active={isActive(child.href, $page.url.pathname)}
													href={child.href}
												>
													<span class="nav-ico"><Icon name={child.icon} /></span>
													<span class="nav-label">{child.label}</span>
												</a>
											{/each}
										</div>
									{/if}
								{:else}
									<a
										class="nav-link"
										class:active={isActive(item.href, $page.url.pathname)}
										href={item.href}
									>
										<span class="nav-ico"><Icon name={item.icon} /></span>
										<span class="nav-label">{item.label}</span>
									</a>
								{/if}
							{/each}
						{/if}
					</div>
				{/if}
			{/each}
		</aside>

		<main class="main">
			{@render children()}
		</main>
	</div>
{/if}

<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, loadMyAccess } from '$lib/iam/myAccess';
	import { goto } from '$app/navigation';
	import { visibleGroups, searchSettings } from '$lib/settings/sections';
	import {
		SEARCH_FIELD,
		SUBNAV_ASIDE,
		SUBNAV_CONTENT,
		SUBNAV_ITEM,
		SUBNAV_ITEM_OFF,
		SUBNAV_ITEM_ON
	} from '$lib/ui/classes';

	let { children }: { children: Snippet } = $props();

	// On wide screens the sub-navigation folds down to an icon rail, so wide tables and editors get the room. The choice is remembered per browser.
	const COLLAPSED_KEY = 'marmot.settings.nav.collapsed';
	let collapsed = $state(false);

	function setCollapsed(next: boolean) {
		collapsed = next;
		try {
			localStorage.setItem(COLLAPSED_KEY, next ? '1' : '0');
		} catch {
			// Private windows can refuse storage; the rail still works for this visit
		}
	}

	onMount(() => {
		loadMyAccess();
		try {
			collapsed = localStorage.getItem(COLLAPSED_KEY) === '1';
		} catch {
			collapsed = false;
		}
	});

	let groups = $derived(visibleGroups($myAccess));

	// Searching narrows the sub-navigation to matching sections, including the words a label does not say (SSO finds Sign-in, tokens find Service accounts).
	let query = $state('');
	let highlighted = $state(0);
	let searchInput: HTMLInputElement | undefined = $state();
	let results = $derived(searchSettings($myAccess, query));
	$effect(() => {
		void results;
		highlighted = 0;
	});

	function open(href: string) {
		query = '';
		goto(resolve(href as '/settings'));
	}

	function onSearchKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			highlighted = Math.min(highlighted + 1, results.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			highlighted = Math.max(highlighted - 1, 0);
		} else if (e.key === 'Enter' && results[highlighted]) {
			e.preventDefault();
			open(results[highlighted].href);
		} else if (e.key === 'Escape') {
			query = '';
		}
	}

	// "/" jumps to the settings search from anywhere in Settings, unless the caller is already typing
	function onWindowKeydown(e: KeyboardEvent) {
		const target = e.target as HTMLElement | null;
		const typing =
			target &&
			(target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));
		if (e.key === '/' && !typing && !e.metaKey && !e.ctrlKey) {
			e.preventDefault();
			if (collapsed) setCollapsed(false);
			setTimeout(() => searchInput?.focus());
		}
		// Cmd/Ctrl+B folds the sub-navigation, as in editors and most dashboards
		if (e.key.toLowerCase() === 'b' && (e.metaKey || e.ctrlKey) && !e.altKey && !typing) {
			e.preventDefault();
			setCollapsed(!collapsed);
		}
	}
	let path = $derived($page.url.pathname);

	function active(href: string): boolean {
		return href === '/settings'
			? path === '/settings'
			: path === href || path.startsWith(href + '/');
	}

	const ITEM = SUBNAV_ITEM;
	const ITEM_ON = SUBNAV_ITEM_ON;
	const ITEM_OFF = SUBNAV_ITEM_OFF;
	// In the rail each link is just its icon, centred, with the label kept for screen readers and the tooltip
	const RAIL = 'lg:justify-center lg:px-0';
</script>

<svelte:window onkeydown={onWindowKeydown} />

<div class="flex w-full flex-col lg:flex-row">
	<!-- The sub-navigation stays quieter than the content: no panel, just a column of links grouped by intent -->
	<aside
		class="{SUBNAV_ASIDE} transition-[width] duration-150 {collapsed ? 'lg:w-14 lg:px-2' : ''}"
	>
		<div class="mb-3 hidden items-center justify-between lg:flex">
			{#if !collapsed}
				<p class="px-2.5 text-sm font-semibold text-gray-900 dark:text-gray-100">
					{m.settings_title()}
				</p>
			{/if}
			<button
				type="button"
				onclick={() => setCollapsed(!collapsed)}
				class="rounded-md p-1.5 text-gray-400 transition hover:bg-white hover:text-gray-700 dark:hover:bg-gray-800 dark:hover:text-gray-200 {collapsed
					? 'mx-auto'
					: ''}"
				aria-label={collapsed ? m.settings_nav_expand() : m.settings_nav_collapse()}
				title="{collapsed ? m.settings_nav_expand() : m.settings_nav_collapse()} (⌘B)"
				aria-expanded={!collapsed}
			>
				<IconifyIcon
					icon={collapsed
						? 'material-symbols:left-panel-open-outline'
						: 'material-symbols:left-panel-close-outline'}
					class="h-4 w-4"
				/>
			</button>
		</div>
		<label class="{SEARCH_FIELD} mb-3 lg:mx-0.5 {collapsed ? 'lg:hidden' : ''}">
			<IconifyIcon icon="material-symbols:search" class="h-4 w-4 flex-shrink-0 text-gray-400" />
			<input
				bind:this={searchInput}
				bind:value={query}
				onkeydown={onSearchKeydown}
				type="search"
				placeholder={m.settings_search_placeholder()}
				aria-label={m.settings_search_placeholder()}
				aria-controls="settings-search-results"
				class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
			/>
			<kbd
				class="hidden rounded border border-gray-200 px-1.5 text-[10px] font-medium text-gray-400 dark:border-gray-700 lg:inline"
				>/</kbd
			>
		</label>
		{#if query.trim()}
			<ul
				id="settings-search-results"
				class="flex flex-col gap-0.5"
				aria-label={m.settings_search_results_aria()}
			>
				{#each results as result, i (result.href)}
					<li>
						<button
							type="button"
							onclick={() => open(result.href)}
							onmouseenter={() => (highlighted = i)}
							class="{ITEM} w-full text-left {i === highlighted ? ITEM_ON : ITEM_OFF}"
						>
							<IconifyIcon icon={result.icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
							<span class="min-w-0 flex-1 truncate">{result.label}</span>
							<span class="text-[11px] text-gray-400">{result.group}</span>
						</button>
					</li>
				{:else}
					<li class="px-2.5 py-1.5 text-sm text-gray-500 dark:text-gray-400">
						{m.settings_search_empty()}
					</li>
				{/each}
			</ul>
		{:else}
			<nav
				aria-label={m.settings_nav_aria()}
				class="flex gap-1 overflow-x-auto lg:flex-col lg:gap-0.5 lg:overflow-visible"
			>
				<a
					href={resolve('/settings')}
					class="{ITEM} {active('/settings') ? ITEM_ON : ITEM_OFF} {collapsed ? RAIL : ''}"
					aria-current={active('/settings') ? 'page' : undefined}
					title={collapsed ? m.settings_overview() : undefined}
				>
					<IconifyIcon
						icon="material-symbols:space-dashboard-outline"
						class="h-4 w-4 flex-shrink-0 text-gray-400"
					/>
					<span class={collapsed ? 'lg:sr-only' : ''}>{m.settings_overview()}</span>
				</a>
				{#each groups as group (group.id)}
					<p
						class="mt-4 hidden px-2.5 pb-1 text-[11px] font-medium uppercase tracking-wider text-gray-400 dark:text-gray-500 {collapsed
							? ''
							: 'lg:block'}"
					>
						{group.label()}
					</p>
					{#each group.items as item (item.id)}
						<a
							href={resolve(item.href)}
							class="{ITEM} {active(item.href) ? ITEM_ON : ITEM_OFF} {collapsed ? RAIL : ''}"
							aria-current={active(item.href) ? 'page' : undefined}
							title={collapsed ? item.label() : undefined}
						>
							<IconifyIcon icon={item.icon} class="h-4 w-4 flex-shrink-0 text-gray-400" />
							<span class={collapsed ? 'lg:sr-only' : ''}>{item.label()}</span>
						</a>
					{/each}
				{/each}
			</nav>
		{/if}
	</aside>

	<div class={SUBNAV_CONTENT}>
		{@render children()}
	</div>
</div>

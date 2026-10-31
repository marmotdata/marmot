<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { goto } from '$app/navigation';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, loadMyAccess } from '$lib/iam/myAccess';
	import { mmIcon, setCrumbs } from '$lib/mm';
	import {
		visibleGroups,
		searchSettings,
		sectionForPath,
		settingsCrumbs,
		type SettingsItem
	} from '$lib/settings/sections';
	import { settingsAttentionRows } from '$lib/settings/attention';

	let { children }: { children: Snippet } = $props();

	onMount(loadMyAccess);

	let groups = $derived(visibleGroups($myAccess));
	let path = $derived(page.url.pathname);
	let section = $derived(sectionForPath(path));
	let current = $derived(path === '/settings' ? 'overview' : (section?.id ?? ''));

	// A section's own index page gets "Settings › Section"; deeper pages (a team, a role) set their own trail
	$effect(() => {
		if (path === '/settings') setCrumbs([{ label: m.settings_title() }]);
		else if (section && path === section.href)
			setCrumbs(settingsCrumbs({ label: section.label() }));
	});

	// Each section counts the attention rows that point into it, from the same check the gear and the Overview read
	function attentionFor(item: SettingsItem): number {
		return $settingsAttentionRows.filter(
			(row) => row.href === item.href || row.href.startsWith(item.href + '/')
		).length;
	}

	// Searching narrows the sub-navigation to matching sections, including the words a label does not say (SSO finds Authentication, tokens find Service accounts)
	let query = $state('');
	let results = $derived(searchSettings($myAccess, query));

	function open(href: string) {
		query = '';
		goto(resolve(href as '/settings'));
	}
</script>

{#snippet navItem(id: string, href: string, icon: string, label: string, count: number)}
	<mm-list-item href={resolve(href as '/settings')} selected={current === id}>
		<mm-icon slot="start" name={icon}></mm-icon>{label}
		{#if count > 0}
			<mm-count slot="end" value={count} label={m.settings_nav_attention_label({ count })}
			></mm-count>
		{/if}
	</mm-list-item>
{/snippet}

<div class="mm-split mm-split--sidebar">
	<nav aria-label={m.settings_nav_aria()} class="settings-nav mm-stack">
		<mm-search-field
			label={m.settings_search_placeholder()}
			hide-label
			placeholder={m.settings_search_placeholder()}
			shortcut="/"
			global-shortcut
			value={query}
			onmm-input={(e: CustomEvent<{ value: string }>) => (query = e.detail.value)}
			onmm-search={(e: CustomEvent<{ value: string }>) => {
				e.preventDefault();
				if (results[0]) open(results[0].href);
			}}
		></mm-search-field>

		{#if query.trim()}
			{#if results.length > 0}
				<mm-list dense aria-label={m.settings_search_results_aria()}>
					{#each results as result (result.href)}
						<mm-list-item href={resolve(result.href as '/settings')} secondary={result.group}>
							<mm-icon slot="start" name={mmIcon(result.icon)}></mm-icon>{result.label}
						</mm-list-item>
					{/each}
				</mm-list>
			{:else}
				<p class="mm-caption m-0 px-space-4" role="status">{m.settings_search_empty()}</p>
			{/if}
		{:else}
			<mm-list dense>
				{@render navItem('overview', '/settings', 'tune', m.settings_overview(), 0)}
			</mm-list>
			{#each groups as group (group.id)}
				<div class="mm-stack" style="--mm-gap: var(--space-1)">
					<p class="mm-label m-0 px-space-4" id="settings-nav-{group.id}">{group.label()}</p>
					<mm-list dense aria-labelledby="settings-nav-{group.id}">
						{#each group.items as item (item.id)}
							{@render navItem(
								item.id,
								item.href,
								mmIcon(item.icon),
								item.label(),
								attentionFor(item)
							)}
						{/each}
					</mm-list>
				</div>
			{/each}
		{/if}
	</nav>

	<div class="mm-stack min-w-0">
		{@render children()}
	</div>
</div>

<style>
	/* Sticky keeps the sub-navigation beside long tables; once the split stacks, its grid area is only as tall as the nav so it stays put */
	.settings-nav {
		--mm-gap: var(--space-5);
		position: sticky;
		top: 0;
	}
</style>

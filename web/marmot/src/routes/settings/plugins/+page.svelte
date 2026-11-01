<script lang="ts">
	import { onMount } from 'svelte';
	import '@marmotdata/elements/toolbar.js';
	import '@marmotdata/elements/filter-pill.js';
	import { m } from '$lib/paraglide/messages';
	import { myAccess, holdsAtRoot } from '$lib/iam/myAccess';
	import { toasts } from '$lib/stores/toast';
	import { severityRank, worstAdvisory } from '$lib/plugins/advisories';
	import {
		checkPluginUpdates,
		getPluginUpdatesStatus,
		PluginUpdatesUnavailableError
	} from '$lib/plugins/api';
	import {
		chooseVersion,
		confirmRollback,
		confirmReset,
		setPinned,
		updateToLatest,
		type PluginChange
	} from '$lib/plugins/actions';
	import type { PluginUpdatesStatus, PluginVersionStatus } from '$lib/plugins/types';
	import type { PluginRowAction } from '$components/settings/plugins/pluginVersionsColumns';
	import PluginUpdatesHeader from '$components/settings/plugins/PluginUpdatesHeader.svelte';
	import PluginVersionsTable from '$components/settings/plugins/PluginVersionsTable.svelte';
	import PluginVersionPicker from '$components/settings/plugins/PluginVersionPicker.svelte';

	// The table's default order puts what needs attention first, worst first: a plugin with a
	// critical advisory outranks one with a low one, which outranks a plain update, which outranks
	// nothing to do. Everything tied on that is then just alphabetical.
	function severityOf(p: PluginVersionStatus): number {
		const worst = worstAdvisory(p.advisories);
		return worst ? severityRank(worst.severity) : -1;
	}

	const canManage = $derived(holdsAtRoot($myAccess, 'ingestion:manage'));

	let status = $state<PluginUpdatesStatus | null>(null);
	let loading = $state(true);
	let unavailable = $state(false);
	let loadError = $state('');
	let checking = $state(false);
	let search = $state('');
	let updatesOnly = $state(false);
	// The plugin a change is in flight for; while set, every row's actions are disabled so a second
	// change can never overlap it, here or from the version picker.
	let busyId = $state<string | null>(null);
	// Held by id, not the plugin object itself, so a reload while the picker is open (another tab,
	// or the index check finishing) shows it fresh data rather than what was there when it opened.
	let pickerPluginId = $state<string | null>(null);
	let pickerBusyVersion = $state<string | null>(null);

	let pickerPlugin = $derived(status?.plugins.find((p) => p.id === pickerPluginId) ?? null);

	async function load() {
		loading = true;
		loadError = '';
		unavailable = false;
		try {
			status = await getPluginUpdatesStatus();
		} catch (err) {
			if (err instanceof PluginUpdatesUnavailableError) unavailable = true;
			else loadError = err instanceof Error ? err.message : m.plugin_versions_load_error();
		} finally {
			loading = false;
		}
	}

	onMount(load);

	let needsAttentionCount = $derived(
		status ? status.plugins.filter((p) => p.update_available || p.advisories.length > 0).length : 0
	);

	let visiblePlugins = $derived.by(() => {
		if (!status) return [];
		const q = search.trim().toLowerCase();
		return status.plugins
			.filter((p) => !updatesOnly || p.update_available || p.advisories.length > 0)
			.filter((p) => !q || p.name.toLowerCase().includes(q) || p.id.toLowerCase().includes(q))
			.sort(
				(a, b) =>
					severityOf(b) - severityOf(a) ||
					Number(b.update_available) - Number(a.update_available) ||
					a.name.localeCompare(b.name)
			);
	});

	async function handleCheck() {
		checking = true;
		try {
			const res = await checkPluginUpdates();
			toasts.success(
				res.updated ? m.plugin_versions_check_success() : m.plugin_versions_check_up_to_date()
			);
			await load();
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : m.plugin_versions_check_error());
		} finally {
			checking = false;
		}
	}

	/**
	 * Runs a change, marking the plugin busy while it does. Reloads in `finally`, whether it
	 * succeeded, failed outright, or was saved but not installed yet (a 502), so the table always
	 * shows what the server actually has: the stored version, and a per-plugin error if there is one.
	 * A null change (declined, or nothing to do) is a no-op.
	 */
	async function apply(p: PluginVersionStatus, change: PluginChange | null) {
		if (!change) return;
		busyId = p.id;
		try {
			await change.run();
			toasts.success(change.successMessage);
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : change.errorMessage);
		} finally {
			busyId = null;
			await load();
		}
	}

	function handleAction(p: PluginVersionStatus, action: PluginRowAction) {
		switch (action) {
			case 'latest':
				apply(p, updateToLatest(p));
				break;
			case 'choose':
				pickerPluginId = p.id;
				break;
			case 'pin':
				apply(p, setPinned(p, true));
				break;
			case 'unpin':
				apply(p, setPinned(p, false));
				break;
			case 'rollback':
				confirmRollback(p).then((change) => apply(p, change));
				break;
			case 'reset':
				confirmReset(p).then((change) => apply(p, change));
				break;
		}
	}

	async function handleChooseVersion(version: string) {
		const p = pickerPlugin;
		if (!p) return;
		pickerBusyVersion = version;
		const change = chooseVersion(p, version);
		try {
			await change.run();
			toasts.success(change.successMessage);
			pickerPluginId = null;
		} catch (err) {
			toasts.error(err instanceof Error ? err.message : change.errorMessage);
		} finally {
			pickerBusyVersion = null;
			await load();
		}
	}
</script>

<svelte:head>
	<title>{m.settings_plugins()} · {m.settings_title()}</title>
</svelte:head>

<div class="mm-stack" style="--mm-gap: var(--space-8)">
	<mm-page-header heading={m.settings_plugins()}></mm-page-header>

	{#if unavailable}
		<mm-empty-state
			heading={m.plugin_versions_unavailable_heading()}
			description={m.plugin_versions_unavailable_body()}
		></mm-empty-state>
	{:else if loading}
		<mm-skeleton loading count={3}></mm-skeleton>
	{:else if loadError}
		<mm-callout tone="danger" heading={m.plugin_versions_load_error_heading()}>
			{loadError}
			<mm-button slot="actions" size="sm" icon="refresh" onclick={load}
				>{m.common_retry()}</mm-button
			>
		</mm-callout>
	{:else if status}
		<PluginUpdatesHeader {status} {canManage} {checking} onCheck={handleCheck} />

		<mm-section heading={m.plugin_versions_table_label()}>
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				<mm-toolbar label={m.plugin_versions_filter_label()}>
					<mm-search-field
						label={m.plugin_versions_search()}
						hide-label
						placeholder={m.plugin_versions_search()}
						shortcut="/"
						value={search}
						style="inline-size: min(18rem, 100%)"
						oninput={(e: Event) => (search = (e.currentTarget as HTMLInputElement).value)}
					></mm-search-field>
					{#if needsAttentionCount > 0}
						<mm-filter-pill
							pressed={updatesOnly}
							count={needsAttentionCount}
							onmm-change={(e: CustomEvent<{ pressed: boolean }>) =>
								(updatesOnly = e.detail.pressed)}
						>
							{m.plugin_versions_updates_only()}
						</mm-filter-pill>
					{/if}
				</mm-toolbar>

				{#if status.plugins.length === 0}
					<mm-empty-state
						compact
						heading={m.plugin_versions_none_heading()}
						description={m.plugin_versions_none_body()}
					></mm-empty-state>
				{:else}
					<PluginVersionsTable
						plugins={visiblePlugins}
						{canManage}
						disabled={busyId !== null}
						{busyId}
						onAction={handleAction}
					/>
				{/if}
			</div>
		</mm-section>
	{/if}
</div>

<PluginVersionPicker
	plugin={pickerPlugin}
	busyVersion={pickerBusyVersion}
	onChoose={handleChooseVersion}
	onClose={() => (pickerPluginId = null)}
/>

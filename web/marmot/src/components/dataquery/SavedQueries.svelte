<script lang="ts">
	import { onMount } from 'svelte';
	import '@marmotdata/elements/saved-query-card.js';
	import type { MmDialog } from '@marmotdata/elements/dialog.js';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage, toasts } from '$lib/stores/toast';
	import {
		createSavedQuery,
		deleteSavedQuery,
		listSavedQueries,
		markSavedQueryRun,
		type SavedQuery
	} from '$lib/savedqueries/api';

	// The workspace's current source and statement are what Save query keeps; opening a saved query hands it back to the workspace
	let {
		source,
		statement,
		providers,
		onOpen
	}: {
		source: string;
		statement: string;
		providers: Map<string, string>;
		onOpen: (source: string, sql: string, run: boolean) => void;
	} = $props();

	let saved = $state<SavedQuery[]>([]);
	let loading = $state(true);
	let error = $state('');
	let dialog = $state<MmDialog>();
	let name = $state('');
	let nameError = $state('');
	let saving = $state(false);

	let canSave = $derived(!!source && statement.trim().length > 0);

	async function load() {
		loading = true;
		error = '';
		try {
			saved = (await listSavedQueries()).queries;
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	function openSave() {
		name = '';
		nameError = '';
		dialog?.show();
	}

	async function save(e?: Event) {
		e?.preventDefault();
		if (!name.trim()) {
			nameError = m.savedqueries_name_required();
			return;
		}
		saving = true;
		try {
			const q = await createSavedQuery({ name: name.trim(), sql: statement, source });
			saved = [q, ...saved];
			dialog?.close();
			toasts.success(m.savedqueries_saved_toast());
		} catch (err) {
			nameError = getErrorMessage(err);
		} finally {
			saving = false;
		}
	}

	async function remove(q: SavedQuery) {
		try {
			await deleteSavedQuery(q.id);
			saved = saved.filter((s) => s.id !== q.id);
			toasts.success(m.savedqueries_deleted_toast());
		} catch (err) {
			toasts.error(getErrorMessage(err));
		}
	}

	function run(q: SavedQuery) {
		onOpen(q.source, q.sql, true);
		// Running moves it up the list; a failure here only leaves the order as it was
		markSavedQueryRun(q.id)
			.then(load)
			.catch(() => {});
	}

	onMount(load);
</script>

<section class="mm-stack saved" aria-labelledby="saved-queries-heading">
	<div class="mm-cluster head">
		<mm-heading id="saved-queries-heading" level={2} variant="panel-title"
			>{m.savedqueries_heading()}</mm-heading
		>
		<mm-button size="sm" variant="secondary" icon="bookmark" disabled={!canSave} onclick={openSave}
			>{m.savedqueries_save()}</mm-button
		>
	</div>
	{#if error}
		<mm-callout tone="danger" compact>{error}</mm-callout>
	{:else if !loading && saved.length === 0}
		<mm-text variant="dense" tone="secondary" block>{m.savedqueries_empty()}</mm-text>
	{:else}
		<div class="mm-stack list" role="list" aria-label={m.savedqueries_list_label()}>
			{#each saved as q (q.id)}
				<div role="listitem">
					<mm-saved-query-card
						name={q.name}
						sql={q.sql}
						source={q.source}
						provider={providers.get(q.source) ?? ''}
						last-run-at={q.last_run_at ?? ''}
						level={3}
						onmm-open-query={() => onOpen(q.source, q.sql, false)}
						onmm-run={() => run(q)}
					>
						<mm-menu-button
							slot="menu"
							icon="more-horiz"
							icon-only
							size="sm"
							variant="ghost"
							label={m.savedqueries_menu_label({ name: q.name })}
						>
							<mm-menu onmm-select={() => remove(q)}>
								<mm-menu-item value="delete" icon="delete">{m.savedqueries_delete()}</mm-menu-item>
							</mm-menu>
						</mm-menu-button>
					</mm-saved-query-card>
				</div>
			{/each}
		</div>
	{/if}
</section>

<mm-dialog bind:this={dialog} heading={m.savedqueries_save()} size="sm">
	<form class="mm-stack" onsubmit={save}>
		<mm-text-field
			label={m.savedqueries_name_label()}
			value={name}
			error={nameError}
			required
			oninput={(e: Event) => {
				name = (e.target as HTMLInputElement).value;
				nameError = '';
			}}
			onkeydown={(e: KeyboardEvent) => e.key === 'Enter' && save(e)}
		></mm-text-field>
	</form>
	<mm-button slot="footer" data-close="cancel" disabled={saving}>{m.common_cancel()}</mm-button>
	<mm-button slot="footer" variant="primary" loading={saving} onclick={() => save()}
		>{m.savedqueries_save()}</mm-button
	>
</mm-dialog>

<style>
	.saved {
		--mm-gap: var(--space-3);
	}

	.head {
		justify-content: space-between;
	}

	.list {
		--mm-gap: var(--space-2);
	}
</style>

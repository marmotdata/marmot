<script lang="ts">
	import { onMount, tick } from 'svelte';
	import '@marmotdata/elements/drawer.js';
	import { toast } from '@marmotdata/elements/toast.js';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';
	import {
		getLinkedAssets,
		getSuggestedAssets,
		linkAsset,
		unlinkAsset,
		type LinkedAsset
	} from '$lib/glossary/assets';

	let {
		termId,
		termName,
		onClose
	}: {
		termId: string;
		termName: string;
		/** Called once the drawer has closed, so the term page can reload its linked assets. */
		onClose: () => void;
	} = $props();

	let linked = $state<LinkedAsset[]>([]);
	let linkedLoading = $state(true);
	let ruleNames = $state<string[]>([]);
	let suggested = $state<LinkedAsset[]>([]);
	let suggestedLoading = $state(true);
	let search = $state('');
	let results = $state<LinkedAsset[]>([]);
	let searching = $state(false);
	let busy = $state<string | null>(null);
	let body = $state<HTMLElement | null>(null);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let request = 0;

	let linkedIds = $derived(new Set(linked.map((a) => a.id)));
	// Chosen assets live in their own list, so results only offer what is not linked yet
	let unlinkedResults = $derived(results.filter((a) => !linkedIds.has(a.id)));
	let unlinkedSuggestions = $derived(suggested.filter((a) => !linkedIds.has(a.id)));
	let searchingNow = $derived(!!search.trim());
	// Before anyone searches, the drawer offers assets that mention the term
	let offered = $derived(searchingNow ? unlinkedResults : unlinkedSuggestions);

	onMount(async () => {
		getSuggestedAssets(termId)
			.then((assets) => (suggested = assets))
			.finally(() => (suggestedLoading = false));
		try {
			// Only manual links are listed here: assets a rule links are managed on the rule
			const res = await getLinkedAssets(termId, 100, 'manual');
			linked = res.assets;
			ruleNames = res.groups
				.filter((g) => g.source === 'rule' && g.rule_name)
				.map((g) => g.rule_name as string);
		} catch {
			linked = [];
		} finally {
			linkedLoading = false;
		}
	});

	function onSearch(value: string) {
		search = value;
		clearTimeout(timer);
		timer = setTimeout(runSearch, 200);
	}

	async function runSearch() {
		const q = search.trim();
		const id = ++request;
		if (!q) {
			results = [];
			searching = false;
			return;
		}
		searching = true;
		try {
			const res = await fetchApi(`/assets/search?q=${encodeURIComponent(q)}&limit=20`);
			const data = res.ok ? await res.json() : { assets: [] };
			if (id === request) results = data.assets ?? [];
		} catch {
			if (id === request) results = [];
		} finally {
			if (id === request) searching = false;
		}
	}

	const secondary = (a: LinkedAsset) => [a.type, a.providers?.[0]].filter(Boolean).join(' · ');

	// After a row moves to the other list, keep focus in the list it left so keyboard users can carry on
	async function refocus(list: 'linked' | 'results', index: number) {
		await tick();
		const buttons = body?.querySelectorAll<HTMLElement>(`[data-list="${list}"] mm-button`);
		const next = buttons?.[Math.min(index, (buttons?.length ?? 1) - 1)];
		(next ?? body?.querySelector<HTMLElement>('mm-search-field'))?.focus();
	}

	async function link(asset: LinkedAsset, index = -1, announce = true) {
		busy = asset.id;
		try {
			await linkAsset(asset.id, termId);
			if (!linkedIds.has(asset.id)) linked = [...linked, asset];
			if (index >= 0) refocus('results', index);
			if (announce) {
				toast({
					tone: 'success',
					message: m.glossary_linked_toast({ asset: asset.name, term: termName }),
					action: m.glossary_undo(),
					duration: 8000,
					onAction: () => unlink(asset, -1, false)
				});
			}
		} catch {
			toast({ tone: 'danger', message: m.glossary_link_error() });
		} finally {
			busy = null;
		}
	}

	async function unlink(asset: LinkedAsset, index = -1, announce = true) {
		busy = asset.id;
		try {
			await unlinkAsset(asset.id, termId);
			linked = linked.filter((a) => a.id !== asset.id);
			if (index >= 0) refocus('linked', index);
			if (announce) {
				toast({
					tone: 'success',
					message: m.glossary_unlinked_toast({ asset: asset.name, term: termName }),
					action: m.glossary_undo(),
					duration: 8000,
					onAction: () => link(asset, -1, false)
				});
			}
		} catch {
			toast({ tone: 'danger', message: m.glossary_unlink_error() });
		} finally {
			busy = null;
		}
	}
</script>

<mm-drawer
	open
	dismissible
	heading={m.glossary_link_title()}
	description={termName}
	onmm-close={onClose}
>
	<div bind:this={body} class="mm-stack" style="--mm-gap: var(--space-6)">
		<mm-section
			heading={m.glossary_linked_manually()}
			description={ruleNames.length
				? m.glossary_link_rules_note({ rules: ruleNames.join(', ') })
				: ''}
		>
			{#if linkedLoading}
				<mm-skeleton loading shape="text" count={3}></mm-skeleton>
			{:else if linked.length === 0}
				<mm-text tone="secondary">{m.glossary_link_linked_empty()}</mm-text>
			{:else}
				<mm-list
					aria-label={m.glossary_linked_heading()}
					data-list="linked"
					style="--mm-list-inset: 0"
				>
					{#each linked as a, i (a.id)}
						<mm-list-item secondary={secondary(a)}>
							<mm-provider-logo
								slot="start"
								provider={a.providers?.[0] ?? a.type}
								size={20}
								decorative
							></mm-provider-logo>
							{a.name}
							<mm-button
								slot="end"
								size="sm"
								variant="ghost"
								icon="check"
								pressed="true"
								label={m.glossary_unlink_label({ name: a.name })}
								loading={busy === a.id}
								onclick={() => unlink(a, i)}>{m.glossary_linked_button()}</mm-button
							>
						</mm-list-item>
					{/each}
				</mm-list>
			{/if}
		</mm-section>

		<mm-section
			heading={searchingNow
				? m.glossary_link_results_heading()
				: m.glossary_link_suggested_heading()}
			description={searchingNow ? '' : m.glossary_link_suggested_hint({ term: termName })}
		>
			<div class="mm-stack" style="--mm-gap: var(--space-3)">
				<mm-search-field
					label={m.glossary_link_search_label()}
					hide-label
					placeholder={m.glossary_link_search()}
					value={search}
					onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
				></mm-search-field>
				{#if searchingNow && !searching && unlinkedResults.length === 0}
					<mm-empty-state
						compact
						heading={m.glossary_link_none()}
						description={m.glossary_link_none_hint()}
					></mm-empty-state>
				{:else if (searchingNow && searching && unlinkedResults.length === 0) || (!searchingNow && suggestedLoading)}
					<mm-skeleton loading shape="text" count={3}></mm-skeleton>
				{:else if !searchingNow && unlinkedSuggestions.length === 0}
					<mm-text tone="secondary">{m.glossary_link_suggested_none()}</mm-text>
				{:else}
					<mm-list
						aria-label={searchingNow
							? m.glossary_link_results_label()
							: m.glossary_link_suggested_heading()}
						data-list="results"
						style="--mm-list-inset: 0"
					>
						{#each offered as a, i (a.id)}
							<mm-list-item secondary={secondary(a)}>
								<mm-provider-logo
									slot="start"
									provider={a.providers?.[0] ?? a.type}
									size={20}
									decorative
								></mm-provider-logo>
								{a.name}
								<mm-button
									slot="end"
									size="sm"
									icon="add"
									label={m.glossary_link_button_label({ name: a.name })}
									loading={busy === a.id}
									onclick={() => link(a, i)}>{m.glossary_link_button()}</mm-button
								>
							</mm-list-item>
						{/each}
					</mm-list>
				{/if}
			</div>
		</mm-section>
	</div>
	<mm-button slot="footer" variant="primary" data-close="done">{m.glossary_link_done()}</mm-button>
</mm-drawer>

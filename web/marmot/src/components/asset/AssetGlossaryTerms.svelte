<script lang="ts">
	import { fetchApi } from '$lib/api';
	import { resolve } from '$app/paths';
	import type { Asset, AssetTerm, GlossaryTerm } from '$lib/assets/types';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	// The "What it means" blade section: glossary terms linked to the asset, grouped by how they got there so only manual links can be removed here.
	let { asset, editable = true }: { asset: Asset; editable?: boolean } = $props();

	let canManageAssets = $derived(editable && auth.hasPermission('assets', 'manage'));

	let terms: AssetTerm[] = $state([]);
	let pickerOpen = $state(false);
	let availableTerms: GlossaryTerm[] = $state([]);
	let termSearchQuery = $state('');
	let loadingTerms = $state(false);
	let savingTerm = $state(false);
	let searchTimer: ReturnType<typeof setTimeout> | undefined;

	let manualTerms = $derived(terms.filter((t) => t.source === 'user'));
	let ruleTerms = $derived(terms.filter((t) => t.source.startsWith('rule:')));
	let pluginTerms = $derived(
		terms.filter((t) => t.source !== 'user' && !t.source.startsWith('rule:'))
	);

	$effect(() => {
		if (asset?.id) {
			fetchAssetTerms();
		}
	});

	async function fetchAssetTerms() {
		try {
			const response = await fetchApi(`/assets/terms/${asset.id}`);
			if (response.ok) {
				const data = await response.json();
				terms = Array.isArray(data) ? data : [];
			}
		} catch (error) {
			console.error('Failed to fetch asset terms:', error);
		}
	}

	function onSearch(value: string) {
		termSearchQuery = value;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(fetchAvailableTerms, 200);
	}

	async function fetchAvailableTerms() {
		if (!termSearchQuery.trim()) {
			availableTerms = [];
			return;
		}
		loadingTerms = true;
		try {
			const response = await fetchApi(`/glossary/search?q=${encodeURIComponent(termSearchQuery)}`);
			if (response.ok) {
				const data = await response.json();
				availableTerms = data.terms || [];
			}
		} catch (error) {
			console.error('Failed to fetch glossary terms:', error);
		} finally {
			loadingTerms = false;
		}
	}

	async function addTerm(termId: string) {
		if (!asset?.id) return;
		savingTerm = true;
		try {
			const response = await fetchApi(`/assets/terms/${asset.id}`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ term_ids: [termId] })
			});
			if (response.ok) {
				const data = await response.json();
				terms = Array.isArray(data) ? data : [];
			}
		} catch (error) {
			console.error('Error adding term:', error);
		} finally {
			savingTerm = false;
		}
	}

	async function removeTerm(termId: string) {
		if (!asset?.id) return;
		try {
			const response = await fetchApi(`/assets/terms/${asset.id}`, {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ term_id: termId })
			});
			if (response.ok) {
				const data = await response.json();
				terms = Array.isArray(data) ? data : [];
			}
		} catch (error) {
			console.error('Error removing term:', error);
		}
	}

	function closePicker() {
		pickerOpen = false;
		termSearchQuery = '';
		availableTerms = [];
	}
</script>

{#if terms.length > 0 || canManageAssets}
	<mm-blade-section heading={m.asset_page_terms_heading()}>
		{#if canManageAssets}
			<mm-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="add"
				label={m.asset_page_link_term()}
				pressed={pickerOpen}
				onclick={() => (pickerOpen ? closePicker() : (pickerOpen = true))}
			></mm-button>
		{/if}
		<div class="mm-stack" style="--mm-gap: var(--space-3)">
			{#if pickerOpen}
				<div class="mm-stack" style="--mm-gap: var(--space-2)">
					<mm-search-field
						label={m.asset_page_find_term()}
						hide-label
						placeholder={m.asset_page_find_term()}
						value={termSearchQuery}
						autofocus
						onmm-input={(e: CustomEvent<{ value: string }>) => onSearch(e.detail.value)}
						onkeydown={(e: KeyboardEvent) => e.key === 'Escape' && closePicker()}
					></mm-search-field>
					{#if loadingTerms}
						<mm-skeleton count={2}></mm-skeleton>
					{:else if availableTerms.length > 0}
						<mm-list dense aria-label={m.asset_page_term_results()}>
							{#each availableTerms as t (t.id)}
								{@const added = terms.some((x) => x.term_id === t.id)}
								<mm-list-item secondary={t.definition}>
									{t.name}
									{#if added}
										<mm-text slot="end" variant="caption" tone="secondary"
											>{m.asset_page_term_linked()}</mm-text
										>
									{:else}
										<mm-button
											slot="end"
											size="sm"
											disabled={savingTerm}
											onclick={() => addTerm(t.id)}>{m.asset_page_link_button()}</mm-button
										>
									{/if}
								</mm-list-item>
							{/each}
						</mm-list>
					{:else if termSearchQuery.trim()}
						<mm-text variant="dense" tone="secondary" block>{m.asset_no_matching_terms()}</mm-text>
					{/if}
				</div>
			{/if}

			{#if terms.length === 0 && !pickerOpen}
				<mm-text variant="dense" tone="secondary" block>{m.asset_page_no_terms()}</mm-text>
			{/if}

			{#if manualTerms.length > 0}
				<div class="mm-cluster" style="--mm-gap: var(--space-2)">
					{#each manualTerms as t (t.term_id)}
						<mm-tag
							href={resolve(`/glossary/${t.term_id}`)}
							removable={canManageAssets}
							onmm-remove={() => removeTerm(t.term_id)}>{t.term_name}</mm-tag
						>
					{/each}
				</div>
			{/if}
			{#if ruleTerms.length > 0}
				<div class="mm-stack" style="--mm-gap: var(--space-1)">
					<mm-text variant="caption" tone="secondary">{m.asset_page_from_rules()}</mm-text>
					<div class="mm-cluster" style="--mm-gap: var(--space-2)">
						{#each ruleTerms as t (t.term_id)}
							<mm-tag href={resolve(`/glossary/${t.term_id}`)}>{t.term_name}</mm-tag>
						{/each}
					</div>
				</div>
			{/if}
			{#if pluginTerms.length > 0}
				<div class="mm-stack" style="--mm-gap: var(--space-1)">
					<mm-text variant="caption" tone="secondary">{m.asset_page_from_plugins()}</mm-text>
					<div class="mm-cluster" style="--mm-gap: var(--space-2)">
						{#each pluginTerms as t (t.term_id)}
							<mm-tag href={resolve(`/glossary/${t.term_id}`)}>{t.term_name}</mm-tag>
						{/each}
					</div>
				</div>
			{/if}
		</div>
	</mm-blade-section>
{/if}

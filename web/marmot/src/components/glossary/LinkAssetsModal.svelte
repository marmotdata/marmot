<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import IconifyIcon from '@iconify/svelte';
	import Modal from '$components/ui/Modal.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { fetchApi } from '$lib/api';
	import { m } from '$lib/paraglide/messages';

	interface FoundAsset {
		id: string;
		name: string;
		type: string;
		mrn?: string;
		providers?: string[];
	}

	let {
		termId,
		termName,
		linkedIds,
		onClose,
		onLinked
	}: {
		termId: string;
		termName: string;
		linkedIds: string[];
		onClose: () => void;
		onLinked: () => void;
	} = $props();

	let search = $state('');
	let results = $state<FoundAsset[]>([]);
	let searching = $state(false);
	let saving = $state(false);
	let error = $state('');
	const picked = new SvelteSet<string>();
	let timer: ReturnType<typeof setTimeout> | undefined;
	let request = 0;

	// Assets already carrying the term are shown ticked and cannot be picked again
	let already = $derived(new Set(linkedIds));

	function onSearch() {
		clearTimeout(timer);
		timer = setTimeout(runSearch, 200);
	}

	async function runSearch() {
		const q = search.trim();
		const id = ++request;
		if (!q) {
			results = [];
			return;
		}
		searching = true;
		try {
			const res = await fetchApi(`/assets/search?q=${encodeURIComponent(q)}&limit=12`);
			const data = res.ok ? await res.json() : { assets: [] };
			if (id === request) results = data.assets ?? [];
		} catch {
			if (id === request) results = [];
		} finally {
			if (id === request) searching = false;
		}
	}

	function toggle(asset: FoundAsset) {
		if (already.has(asset.id)) return;
		if (picked.has(asset.id)) picked.delete(asset.id);
		else picked.add(asset.id);
	}

	async function link() {
		if (picked.size === 0) return;
		saving = true;
		error = '';
		try {
			const outcomes = await Promise.all(
				[...picked].map((assetId) =>
					fetchApi(`/assets/terms/${encodeURIComponent(assetId)}`, {
						method: 'POST',
						body: JSON.stringify({ term_ids: [termId] })
					})
				)
			);
			if (outcomes.some((r) => !r.ok)) throw new Error(m.glossary_link_error());
			onLinked();
			onClose();
		} catch (e) {
			error = e instanceof Error ? e.message : m.glossary_link_error();
		} finally {
			saving = false;
		}
	}
</script>

<Modal
	title={m.glossary_link_title()}
	subtitle={m.glossary_link_subtitle({ name: termName })}
	{onClose}
	width="xl"
	dirty={picked.size > 0}
>
	<label
		class="flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-3 py-2 focus-within:border-earthy-terracotta-500 focus-within:ring-2 focus-within:ring-earthy-terracotta-500/30 dark:border-gray-600 dark:bg-gray-800"
	>
		<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
		<!-- svelte-ignore a11y_autofocus -->
		<input
			bind:value={search}
			oninput={onSearch}
			placeholder={m.glossary_link_search()}
			aria-label={m.glossary_link_search()}
			autofocus
			class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
		/>
		{#if searching}
			<IconifyIcon
				icon="material-symbols:progress-activity"
				class="h-4 w-4 animate-spin text-gray-400"
			/>
		{/if}
	</label>

	<div class="mt-3 max-h-80 min-h-[8rem] overflow-y-auto">
		{#if !search.trim()}
			<p class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
				{m.glossary_link_hint()}
			</p>
		{:else if results.length === 0 && !searching}
			<p class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
				{m.glossary_link_none()}
			</p>
		{:else}
			<ul class="space-y-0.5">
				{#each results as asset (asset.id)}
					{@const isLinked = already.has(asset.id)}
					{@const isPicked = picked.has(asset.id)}
					<li>
						<button
							type="button"
							onclick={() => toggle(asset)}
							disabled={isLinked}
							aria-pressed={isPicked}
							class="flex w-full items-center gap-3 rounded-lg px-2.5 py-2 text-left transition disabled:cursor-default {isPicked
								? 'bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20'
								: 'hover:bg-gray-50 dark:hover:bg-gray-700/40'}"
						>
							<span
								class="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded border {isPicked ||
								isLinked
									? 'border-earthy-terracotta-600 bg-earthy-terracotta-600 text-white'
									: 'border-gray-300 dark:border-gray-600'} {isLinked ? 'opacity-50' : ''}"
							>
								{#if isPicked || isLinked}
									<IconifyIcon icon="material-symbols:check" class="h-3 w-3" />
								{/if}
							</span>
							<Icon name={asset.providers?.[0] ?? asset.type} size="xs" showLabel={false} />
							<span class="min-w-0 flex-1">
								<span class="block truncate text-sm font-medium text-gray-900 dark:text-gray-100"
									>{asset.name}</span
								>
								<span class="block truncate text-xs text-gray-500 dark:text-gray-400"
									>{asset.type}{asset.providers?.[0] ? ` · ${asset.providers[0]}` : ''}</span
								>
							</span>
							{#if isLinked}
								<span class="text-xs text-gray-400">{m.glossary_link_already()}</span>
							{/if}
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if error}
		<p
			class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200"
		>
			{error}
		</p>
	{/if}

	{#snippet footer(close)}
		<span class="mr-auto text-sm text-gray-500 dark:text-gray-400">
			{m.glossary_link_selected({ count: picked.size })}
		</span>
		<button
			type="button"
			onclick={close}
			class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
			>{m.common_cancel()}</button
		>
		<button
			type="button"
			onclick={link}
			disabled={saving || picked.size === 0}
			class="inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:opacity-50"
		>
			{m.glossary_link_button()}
		</button>
	{/snippet}
</Modal>

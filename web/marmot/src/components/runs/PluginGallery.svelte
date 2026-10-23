<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { resolve } from '$app/paths';
	import Icon from '$components/ui/Icon.svelte';
	import { m } from '$lib/paraglide/messages';
	import {
		PANEL,
		SEARCH_FIELD,
		FILTER_PILL,
		FILTER_PILL_ON,
		FILTER_PILL_OFF
	} from '$lib/ui/classes';

	interface Plugin {
		id: string;
		name: string;
		description?: string;
		icon?: string;
		category?: string;
		supports_query?: boolean;
	}

	interface Props {
		plugins: Plugin[];
		// How many pipelines already use each plugin, by plugin id
		connected: Record<string, number>;
		canAdd: boolean;
	}

	let { plugins, connected, canAdd }: Props = $props();

	// Every plugin Marmot can connect, searchable and filterable; picking one opens the create flow with it chosen
	type NewRoute = `/pipelines/new?${string}`;

	let search = $state('');
	let queryOnly = $state(false);
	let category = $state('');

	let categories = $derived(
		[...new Set(plugins.map((p) => p.category).filter((c): c is string => !!c))].sort()
	);
	let queryCount = $derived(plugins.filter((p) => p.supports_query).length);
	let shown = $derived.by(() => {
		const q = search.trim().toLowerCase();
		return plugins
			.filter((p) => !queryOnly || p.supports_query)
			.filter((p) => !category || p.category === category)
			.filter(
				(p) =>
					!q ||
					p.name.toLowerCase().includes(q) ||
					p.id.includes(q) ||
					p.description?.toLowerCase().includes(q)
			)
			.sort((a, b) => a.name.localeCompare(b.name));
	});

	const PILL = FILTER_PILL + ' capitalize';
	const PILL_ON = FILTER_PILL_ON;
	const PILL_OFF = FILTER_PILL_OFF;
</script>

<section id="available" class="scroll-mt-20" aria-labelledby="available-heading">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
		<div>
			<h2 id="available-heading" class="text-base font-semibold text-gray-900 dark:text-gray-100">
				{m.plugins_available_heading()}
				<span class="ml-1 text-sm font-normal tabular-nums text-gray-400">{plugins.length}</span>
			</h2>
			<p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{m.plugins_available_body()}</p>
		</div>
		<label class="{SEARCH_FIELD} w-full sm:w-72">
			<IconifyIcon icon="material-symbols:search" class="h-4 w-4 text-gray-400" />
			<input
				bind:value={search}
				type="search"
				placeholder={m.pipelines_search_plugins_placeholder()}
				aria-label={m.pipelines_search_plugins_placeholder()}
				class="min-w-0 flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-gray-100"
			/>
		</label>
	</div>

	<div class="mt-4 flex flex-wrap items-center gap-2" role="group">
		<button
			type="button"
			onclick={() => {
				queryOnly = false;
				category = '';
			}}
			aria-pressed={!queryOnly && !category}
			class="{PILL} {!queryOnly && !category ? PILL_ON : PILL_OFF}">{m.plugins_filter_all()}</button
		>
		{#if queryCount > 0}
			<button
				type="button"
				onclick={() => (queryOnly = !queryOnly)}
				aria-pressed={queryOnly}
				class="{PILL} {queryOnly ? PILL_ON : PILL_OFF}"
			>
				<IconifyIcon icon="material-symbols:bolt" class="h-3.5 w-3.5" />
				{m.plugins_filter_queryable()}
			</button>
		{/if}
		{#each categories as c (c)}
			<button
				type="button"
				onclick={() => (category = category === c ? '' : c)}
				aria-pressed={category === c}
				class="{PILL} {category === c ? PILL_ON : PILL_OFF}">{c}</button
			>
		{/each}
	</div>

	{#if shown.length === 0}
		<p class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
			{m.pipelines_no_plugins_match({ query: search })}
		</p>
	{:else}
		<ul class="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
			{#each shown as plugin (plugin.id)}
				{@const uses = connected[plugin.id] ?? 0}
				<li>
					<svelte:element
						this={canAdd ? 'a' : 'div'}
						href={canAdd
							? resolve(
									`/pipelines/new?plugin=${plugin.id}${plugin.supports_query ? '&query=1' : ''}` as NewRoute
								)
							: undefined}
						class="{PANEL} group flex h-full items-start gap-3 p-4 transition {canAdd
							? 'hover:-translate-y-px hover:border-earthy-terracotta-300 hover:shadow-sm dark:hover:border-earthy-terracotta-700'
							: ''}"
					>
						<span
							class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg border border-gray-100 bg-gray-50 dark:border-gray-700 dark:bg-gray-900"
						>
							<Icon name={plugin.icon || plugin.id} size="sm" showLabel={false} />
						</span>
						<span class="min-w-0 flex-1">
							<span class="flex items-center gap-1.5">
								<span class="truncate text-sm font-semibold text-gray-900 dark:text-gray-100"
									>{plugin.name}</span
								>
								{#if plugin.supports_query}
									<IconifyIcon
										icon="material-symbols:bolt"
										class="h-4 w-4 flex-shrink-0 text-earthy-terracotta-600"
										aria-label={m.runs_queryable_badge()}
									/>
								{/if}
							</span>
							{#if plugin.description}
								<span class="mt-0.5 line-clamp-2 block text-xs text-gray-500 dark:text-gray-400"
									>{plugin.description}</span
								>
							{/if}
							{#if uses > 0}
								<span
									class="mt-2 inline-flex items-center gap-1 text-[11px] font-medium text-earthy-green-700 dark:text-earthy-green-300"
								>
									<IconifyIcon icon="material-symbols:check-circle-outline" class="h-3.5 w-3.5" />
									{m.plugins_connected_uses({ count: uses })}
								</span>
							{/if}
						</span>
						{#if canAdd}
							<IconifyIcon
								icon="material-symbols:add"
								class="h-5 w-5 flex-shrink-0 text-gray-300 transition group-hover:text-earthy-terracotta-600"
							/>
						{/if}
					</svelte:element>
				</li>
			{/each}
		</ul>
	{/if}
</section>

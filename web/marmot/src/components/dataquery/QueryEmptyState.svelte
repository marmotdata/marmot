<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	interface Plugin {
		id: string;
		name: string;
		supports_query?: boolean;
	}

	// The edit page reads ?query=1 to switch querying on, so the link carries it
	type EnableRoute = `/pipelines/${string}/edit?query=1`;
	type NewRoute = `/pipelines/new?${string}`;

	const CHIP =
		'inline-flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-2 py-1 text-xs font-medium text-gray-700 transition dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200';

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		queryable?: boolean;
	}

	// Shown when no source answers queries: it names the plugins that can, points at the pipelines that could be switched on and otherwise offers to create one, so the next step is always a click away
	const canManage =
		auth.hasPermission('ingestion', 'manage') && auth.hasPermission('access', 'manage');

	let capable = $state<Plugin[]>([]);
	let candidates = $state<Pipeline[]>([]);
	let loaded = $state(false);

	onMount(async () => {
		try {
			const [pluginsRes, pipelinesRes] = await Promise.all([
				fetchApi('/plugins'),
				canManage ? fetchApi('/ingestion/schedules?limit=100') : Promise.resolve(null)
			]);
			const plugins: Plugin[] = pluginsRes.ok ? ((await pluginsRes.json()).plugins ?? []) : [];
			capable = plugins.filter((p) => p.supports_query);
			if (pipelinesRes?.ok) {
				const ids = new Set(capable.map((p) => p.id));
				const pipelines: Pipeline[] = (await pipelinesRes.json()).schedules ?? [];
				candidates = pipelines.filter((p) => ids.has(p.plugin_id) && !p.queryable);
			}
		} catch {
			// The empty state still explains itself without the plugin list
		} finally {
			loaded = true;
		}
	});
</script>

<div class="flex flex-1 items-center justify-center p-8">
	<div class="w-full max-w-lg">
		<span
			class="flex h-11 w-11 items-center justify-center rounded-xl bg-earthy-terracotta-100 text-earthy-terracotta-700 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-300"
		>
			<IconifyIcon icon="material-symbols:terminal" class="h-6 w-6" />
		</span>
		<h2 class="mt-4 text-lg font-semibold text-gray-900 dark:text-gray-100">
			{candidates.length > 0 ? m.dataquery_empty_title_enable() : m.dataquery_empty_title()}
		</h2>
		<p class="mt-1.5 text-sm text-gray-600 dark:text-gray-400">{m.dataquery_empty_body()}</p>

		{#if capable.length > 0}
			<div class="mt-4 flex flex-wrap items-center gap-2">
				<span class="text-xs font-medium text-gray-500 dark:text-gray-400"
					>{m.dataquery_empty_works_with()}</span
				>
				{#each capable as plugin (plugin.id)}
					{#if canManage}
						<a
							href={resolve(`/pipelines/new?query=1&plugin=${plugin.id}` as NewRoute)}
							class="{CHIP} hover:border-earthy-terracotta-300 hover:bg-earthy-terracotta-50 dark:hover:border-earthy-terracotta-700 dark:hover:bg-earthy-terracotta-900/20"
						>
							<Icon name={plugin.id} size="xs" showLabel={false} />
							{plugin.name}
						</a>
					{:else}
						<span class={CHIP}>
							<Icon name={plugin.id} size="xs" showLabel={false} />
							{plugin.name}
						</span>
					{/if}
				{/each}
			</div>
		{/if}

		{#if !loaded}
			<div class="mt-6 h-10 w-40 animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800"></div>
		{:else if canManage}
			{#if candidates.length > 0}
				<ul
					class="mt-6 divide-y divide-gray-100 rounded-xl border border-gray-200 bg-white dark:divide-gray-700 dark:border-gray-700 dark:bg-gray-800"
				>
					{#each candidates as pipeline (pipeline.id)}
						<li class="flex items-center gap-3 px-4 py-3">
							<Icon name={pipeline.plugin_id} size="xs" showLabel={false} />
							<span
								class="min-w-0 flex-1 truncate text-sm font-medium text-gray-900 dark:text-gray-100"
								>{pipeline.name}</span
							>
							<a
								href={resolve(`/pipelines/${pipeline.id}/edit?query=1` as EnableRoute)}
								class="inline-flex items-center gap-1 rounded-lg bg-earthy-terracotta-600 px-3 py-1.5 text-xs font-semibold text-white transition hover:bg-earthy-terracotta-500"
							>
								<IconifyIcon icon="material-symbols:toggle-on-outline" class="h-4 w-4" />
								{m.dataquery_empty_turn_on()}
							</a>
						</li>
					{/each}
				</ul>
				<a
					href={resolve('/pipelines/new?query=1')}
					class="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
				>
					<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
					{m.dataquery_empty_create()}
				</a>
			{:else}
				<a
					href={resolve('/pipelines/new?query=1')}
					class="mt-6 inline-flex items-center gap-2 rounded-lg bg-earthy-terracotta-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-earthy-terracotta-500"
				>
					<IconifyIcon icon="material-symbols:add" class="h-5 w-5" />
					{m.dataquery_empty_create()}
				</a>
			{/if}
		{:else}
			<p
				class="mt-6 flex items-start gap-2 rounded-lg bg-gray-100 px-3 py-2.5 text-sm text-gray-700 dark:bg-gray-800 dark:text-gray-300"
			>
				<IconifyIcon
					icon="material-symbols:info-outline"
					class="mt-0.5 h-4 w-4 flex-shrink-0 text-gray-400"
				/>
				{m.dataquery_empty_ask_admin()}
			</p>
		{/if}
	</div>
</div>

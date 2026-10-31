<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';

	interface Plugin {
		id: string;
		supports_query?: boolean;
	}

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		queryable?: boolean;
	}

	// The edit page reads ?query=1 to switch querying on, so the link carries it
	type EnableRoute = `/pipelines/${string}/edit?query=1`;

	// Shown when no source answers queries: it points at the pipelines that could be switched on and otherwise offers to create one, so the next step is always a click away
	const canManage =
		auth.hasPermission('ingestion', 'manage') && auth.hasPermission('access', 'manage');

	let candidates = $state<Pipeline[]>([]);

	onMount(async () => {
		if (!canManage) return;
		try {
			const [pluginsRes, pipelinesRes] = await Promise.all([
				fetchApi('/plugins'),
				fetchApi('/ingestion/schedules?limit=100')
			]);
			if (!pluginsRes.ok || !pipelinesRes.ok) return;
			const plugins: Plugin[] = (await pluginsRes.json()).plugins ?? [];
			const capable = new Set(plugins.filter((p) => p.supports_query).map((p) => p.id));
			const pipelines: Pipeline[] = (await pipelinesRes.json()).schedules ?? [];
			candidates = pipelines.filter((p) => capable.has(p.plugin_id) && !p.queryable);
		} catch {
			// The empty state still says what to do without the pipeline list
		}
	});
</script>

<div class="empty mm-stack">
	<mm-empty-state
		level={1}
		heading={candidates.length > 0 ? m.dataquery_empty_title_enable() : m.dataquery_empty_title()}
		description={canManage ? m.dataquery_empty_body() : m.dataquery_empty_ask_admin()}
	>
		{#if canManage}
			<mm-button
				slot="action"
				variant={candidates.length > 0 ? 'secondary' : 'primary'}
				icon="add"
				href={resolve('/pipelines/new?query=1')}>{m.dataquery_empty_create()}</mm-button
			>
		{/if}
	</mm-empty-state>

	{#if candidates.length > 0}
		<section class="mm-stack candidates" aria-labelledby="query-candidates">
			<mm-heading id="query-candidates" level={2} variant="panel-title"
				>{m.dataquery_empty_candidates_heading()}</mm-heading
			>
			<mm-list dense aria-labelledby="query-candidates">
				{#each candidates as pipeline (pipeline.id)}
					<mm-list-item>
						<mm-provider-logo slot="start" provider={pipeline.plugin_id} size={20} decorative
						></mm-provider-logo>
						{pipeline.name}
						<mm-button
							slot="end"
							size="sm"
							variant="secondary"
							icon="toggle-on"
							href={resolve(`/pipelines/${pipeline.id}/edit?query=1` as EnableRoute)}
							>{m.dataquery_empty_turn_on()}</mm-button
						>
					</mm-list-item>
				{/each}
			</mm-list>
		</section>
	{/if}
</div>

<style>
	.empty {
		--mm-gap: var(--space-6);
		min-block-size: 100%;
		justify-content: center;
		align-items: center;
		padding: var(--space-8) var(--space-4);
		box-sizing: border-box;
	}

	.candidates {
		--mm-gap: var(--space-2);
		inline-size: min(100%, 32rem);
	}
</style>

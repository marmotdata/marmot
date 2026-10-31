<script lang="ts">
	import '@marmotdata/elements/kpi-tile.js';
	import '@marmotdata/elements/activity-feed.js';
	import '@marmotdata/elements/run-strip.js';
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { setCrumbs } from '$lib/mm';
	import ConnectDrawer, { type ConnectTab } from '$components/home/ConnectDrawer.svelte';
	import SetupChecklist, { type SetupStep } from '$components/home/SetupChecklist.svelte';
	import QueryBlade from '$components/home/QueryBlade.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { userProfile } from '$lib/stores/user';
	import { setupHidden, setupRemoved, skippedSteps } from '$lib/stores/setup.svelte';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { getQueryStats, listQueryHistory } from '$lib/dataquery/api';
	import { assetRouteParams, subjectParts } from '$lib/dataquery/format';
	import type { QueryHistory, QueryRecord, QueryStats } from '$lib/dataquery/types';
	import {
		listAgentSummaries,
		listRecent,
		listStaleAssets,
		type AgentSummary,
		type RecentItem,
		type StaleAsset
	} from '$lib/home/api';
	import { listSavedQueries, savedQueryHref, type SavedQuery } from '$lib/savedqueries/api';

	interface AssetSummary {
		types: Record<string, { count?: number }>;
		providers: Record<string, number>;
	}

	interface MyAsset {
		id: string;
		name: string;
		type: string;
		providers?: string[];
		mrn?: string;
	}

	interface ViewedAsset {
		asset_id: string;
		asset_name: string;
		asset_type: string;
		asset_provider: string;
		count: number;
	}

	type RunState = 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';

	interface PipelineRun {
		id: string;
		schedule_id?: string;
		status: RunState;
		started_at?: string;
		finished_at?: string;
		created_at: string;
	}

	interface Pipeline {
		id: string;
		name: string;
		plugin_id: string;
		last_run_at?: string;
		last_run_status?: string;
	}

	setCrumbs([{ label: m.nav_home() }]);

	const canQuery = auth.hasPermission('assets', 'query');
	const canViewAccess = auth.hasPermission('access', 'view');
	const canManageIngestion = auth.hasPermission('ingestion', 'manage');
	const isAdmin = auth.hasRole('admin');

	let loading = $state(true);
	let unreachable = $state(false);
	let summary = $state<AssetSummary>({ types: {}, providers: {} });
	let myAssets = $state<MyAsset[]>([]);
	let viewed = $state<ViewedAsset[]>([]);
	let glossaryTotal = $state(0);
	let productsTotal = $state(0);
	let usersTotal = $state(1);
	let history = $state<QueryHistory | null>(null);
	let agentQueries = $state(0);
	let failedRuns = $state(0);
	let day = $state<QueryStats | null>(null);
	let yesterday = $state<QueryStats | null>(null);
	let week = $state<QueryStats | null>(null);
	let pipelines = $state<Pipeline[]>([]);
	let runs = $state<PipelineRun[]>([]);
	let recents = $state<RecentItem[]>([]);
	let savedQueries = $state<SavedQuery[]>([]);
	let agentSummaries = $state<AgentSummary[] | null>(null);
	let stale = $state<StaleAsset[]>([]);
	let staleTotal = $state(0);

	let connect = $state<ConnectTab | null>(null);
	let detailId = $state<string | null>(null);
	let confirmRemove = $state(false);

	let recent = $derived<QueryRecord[]>(history?.queries ?? []);
	let queriesTotal = $derived(history?.total ?? 0);
	// Agent assets are cataloged from agents themselves, so they do not count as a connected source
	let sourceAssets = $derived(
		Object.entries(summary.types).reduce(
			(n, [type, t]) => (type === 'Agent' ? n : n + (t.count ?? 0)),
			0
		)
	);
	let agentCount = $derived(summary.types['Agent']?.count ?? 0);

	let done = $derived<Record<SetupStep, boolean>>({
		source: sourceAssets > 0,
		// An agent counts once one is catalogued or any query has come through MCP or the CLI, which is how coding agents reach Marmot
		agent: agentCount > 0 || agentQueries > 0,
		query: queriesTotal > 0,
		context: glossaryTotal + productsTotal > 0,
		team: usersTotal > 1
	});
	let setupComplete = $derived(
		(Object.keys(done) as SetupStep[]).every((k) => done[k] || skippedSteps.steps.includes(k))
	);
	// Setup and Pipelines show only before the catalog is in use; the guide goes when hidden, removed or settled
	let onboarding = $derived(
		!loading && !unreachable && !setupHidden.value && !setupRemoved.value && !setupComplete
	);
	let blank = $derived(sourceAssets === 0);

	// Agents are the service accounts that query; people are left to the activity feed. Each row opens the audit log filtered to that agent for those who may read it.
	let agents = $derived(
		agentSummaries
			? agentSummaries.map((a) => ({
					key: a.principal_id,
					name: a.name || subjectParts(a.audit_subject).name,
					total: a.queries_7d,
					href: canViewAccess
						? `${resolve('/query-audit')}?${new URLSearchParams({ caller: `${a.principal_type}:${a.principal_id}`, range: '7d' })}`
						: ''
				}))
			: (week?.top_callers ?? [])
					.filter((c) => c.principal_type === 'service_account')
					.slice(0, 4)
					.map((c) => ({
						key: c.key,
						name: subjectParts(c.label).name,
						total: c.total,
						href: ''
					}))
	);
	let activeAgentsToday = $derived(
		(day?.top_callers ?? []).filter((c) => c.principal_type === 'service_account').length
	);
	let spark = $derived((day?.series ?? []).map((p) => p.succeeded + p.failed + p.denied));
	let deniedToday = $derived(day?.totals?.denied ?? 0);

	interface JumpItem {
		key: string;
		name: string;
		caption: string;
		href: string;
		provider?: string;
		icon?: string;
	}

	const KIND_ICONS: Record<string, string> = {
		glossary_term: 'menu-book',
		data_product: 'package-2',
		saved_query: 'terminal'
	};

	function kindLabel(r: RecentItem): string {
		if (r.kind === 'glossary_term') return m.home_jump_kind_term();
		if (r.kind === 'data_product') return m.home_jump_kind_product();
		if (r.kind === 'saved_query') return m.home_jump_kind_saved();
		return r.type || m.home_jump_kind_asset();
	}

	// Relative times sit mid-caption, so "Just now" drops its capital there
	function ago(date: string): string {
		const text = formatRelativeTime(date);
		return text === m.common_just_now() ? text.toLocaleLowerCase() : text;
	}

	function recentHref(r: RecentItem): string {
		if (r.kind === 'glossary_term') return resolve('/glossary/[[id]]', { id: r.id });
		if (r.kind === 'data_product') return resolve('/products/[id]', { id: r.id });
		const route = r.mrn ? assetRouteParams(r.mrn) : null;
		return route ? resolve('/discover/[type]/[service]/[name]', route) : '';
	}

	// What the viewer opened or ran last, newest first; saved queries count from when they were last run, viewed or saved
	let recentItems = $derived.by<JumpItem[]>(() => {
		const viewedAt = new Map(
			recents.filter((r) => r.kind === 'saved_query').map((r) => [r.id, r.viewed_at])
		);
		const dated: { at: number; item: JumpItem }[] = recents
			.filter((r) => r.kind !== 'saved_query')
			.map((r) => ({
				at: new Date(r.viewed_at).getTime(),
				item: {
					key: `${r.kind}:${r.id}`,
					name: r.name,
					caption: m.home_jump_viewed({
						kind: kindLabel(r),
						when: ago(r.viewed_at)
					}),
					href: recentHref(r),
					provider: r.kind === 'asset' ? r.provider?.toLowerCase() || undefined : undefined,
					icon: KIND_ICONS[r.kind] ?? 'table'
				}
			}));
		for (const q of savedQueries) {
			const viewed = viewedAt.get(q.id);
			const ran = q.last_run_at ? new Date(q.last_run_at).getTime() : 0;
			const seen = viewed ? new Date(viewed).getTime() : 0;
			const at = Math.max(ran, seen, new Date(q.updated_at).getTime());
			let caption = m.home_jump_saved({ when: ago(q.updated_at) });
			if (ran && ran >= seen) caption = m.home_jump_ran({ when: ago(q.last_run_at!) });
			else if (seen)
				caption = m.home_jump_viewed({ kind: m.home_jump_kind_saved(), when: ago(viewed!) });
			dated.push({
				at,
				item: {
					key: `saved_query:${q.id}`,
					name: q.name,
					caption,
					href: savedQueryHref(resolve('/query'), q),
					icon: KIND_ICONS.saved_query
				}
			});
		}
		return dated
			.sort((a, b) => b.at - a.at)
			.slice(0, 5)
			.map((d) => d.item);
	});

	// Before the viewer has opened anything, owned assets then what people open most, so they still have somewhere to go back to
	let jumpItems = $derived.by<JumpItem[]>(() => {
		if (recentItems.length > 0) return recentItems;
		const owned = myAssets.map((a) => ({
			key: a.id,
			name: a.name,
			caption: a.type,
			provider: a.providers?.[0]?.toLowerCase() || undefined,
			href: resolve('/discover/[type]/[service]/[name]', myAssetParams(a))
		}));
		const popular = viewed
			.filter((v) => !myAssets.some((a) => a.id === v.asset_id))
			.map((v) => ({
				key: v.asset_id,
				name: v.asset_name,
				caption: v.asset_type,
				provider: v.asset_provider?.toLowerCase() || undefined,
				href: resolve('/discover/[type]/[service]/[name]', {
					type: v.asset_type,
					service: v.asset_provider,
					name: v.asset_name
				})
			}));
		return [...owned, ...popular].slice(0, 5);
	});

	interface Attention {
		key: string;
		label: string;
		count?: number;
		secondary?: string;
		href: string;
		icon: string;
		provider?: string;
	}

	const STALE_SHOWN = 3;

	function staleDetail(a: StaleAsset): string {
		if (a.last_refreshed_at && a.pipeline_name)
			return m.home_attention_stale_detail({
				when: formatRelativeTime(a.last_refreshed_at),
				pipeline: a.pipeline_name
			});
		if (a.pipeline_name) return m.home_attention_stale_pipeline({ pipeline: a.pipeline_name });
		return '';
	}

	// Stale assets are named one by one so each links to its page; any beyond the first few are one row that opens the full list in Insights
	let staleAttention = $derived<Attention[]>([
		...stale.slice(0, STALE_SHOWN).map((a) => {
			const route = assetRouteParams(a.mrn);
			return {
				key: `stale:${a.id}`,
				label: m.home_attention_stale({ name: a.name }),
				secondary: staleDetail(a),
				href: route ? resolve('/discover/[type]/[service]/[name]', route) : '',
				icon: 'schedule',
				provider: a.providers?.[0]?.toLowerCase() || undefined
			};
		}),
		...(staleTotal > STALE_SHOWN
			? [
					{
						key: 'stale:more',
						label: m.home_attention_stale_more(),
						count: staleTotal - STALE_SHOWN,
						href: `${resolve('/insights')}?view=catalog`,
						icon: 'schedule'
					}
				]
			: [])
	]);
	let countedAttention = $derived<Attention[]>(
		[
			...(canQuery
				? [
						{
							key: 'denied',
							label: m.home_attention_denied(),
							count: deniedToday,
							href: canViewAccess
								? `${resolve('/query-audit')}?outcome=denied&range=24h`
								: resolve('/query'),
							icon: 'block'
						}
					]
				: []),
			{
				key: 'runs',
				label: m.home_attention_failed_runs(),
				count: failedRuns,
				href: resolve('/runs'),
				icon: 'sync-problem'
			}
		].filter((a) => a.count > 0)
	);

	let attention = $derived<Attention[]>([...countedAttention, ...staleAttention]);

	// Each pipeline with its recent runs, oldest first, for the run strip
	let pipelineRows = $derived(
		pipelines.map((p) => ({
			...p,
			strip: runs
				.filter((r) => r.schedule_id === p.id)
				.slice(0, 20)
				.reverse()
				.map((r) => ({
					id: r.id,
					status: stripStatus(r.status),
					startedAt: r.started_at ?? r.created_at,
					durationMs:
						r.started_at && r.finished_at
							? new Date(r.finished_at).getTime() - new Date(r.started_at).getTime()
							: undefined
				}))
		}))
	);

	let firstName = $derived.by(() => {
		const profile = $userProfile;
		if (!profile) return null;
		const name = profile.name?.split(' ')[0] || profile.display_name || profile.username;
		return name ? name.charAt(0).toUpperCase() + name.slice(1) : null;
	});

	let greeting = $derived.by(() => {
		const hour = new Date().getHours();
		const part = hour < 12 ? 'morning' : hour < 18 ? 'afternoon' : 'evening';
		if (firstName) {
			if (part === 'morning') return m.home_greeting_morning({ name: firstName });
			if (part === 'afternoon') return m.home_greeting_afternoon({ name: firstName });
			return m.home_greeting_evening({ name: firstName });
		}
		if (part === 'morning') return m.home_greeting_morning_anonymous();
		if (part === 'afternoon') return m.home_greeting_afternoon_anonymous();
		return m.home_greeting_evening_anonymous();
	});

	function stripStatus(s: RunState): 'succeeded' | 'failed' | 'running' | 'queued' | 'skipped' {
		if (s === 'succeeded' || s === 'failed' || s === 'running') return s;
		if (s === 'cancelled') return 'skipped';
		return 'queued';
	}

	function myAssetParams(a: MyAsset) {
		const parts = (a.mrn ?? '').replace('mrn://', '').split('/');
		if (parts.length >= 3)
			return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
		return { type: a.type, service: a.providers?.[0] ?? '', name: a.name };
	}

	// The first table a query read names it in the feed, falling back to its source
	function activityTarget(q: QueryRecord): { name: string; href: string } {
		const mrn = q.referenced_mrns?.[0];
		if (!mrn) return { name: q.source_name, href: '' };
		const context = q.context?.find((c) => c.mrn === mrn);
		const route = context?.asset_id ? assetRouteParams(mrn) : null;
		return {
			name: context?.name || mrn.split('/').pop() || mrn,
			href: route ? resolve('/discover/[type]/[service]/[name]', route) : ''
		};
	}

	async function json<T>(path: string): Promise<T | null> {
		try {
			const res = await fetchApi(path);
			return res.ok ? ((await res.json()) as T) : null;
		} catch {
			return null;
		}
	}

	async function load() {
		const now = new Date();
		const monthAgo = new Date(now.getTime() - 30 * 86_400_000);
		const dayAgo = new Date(now.getTime() - 86_400_000);
		const twoDaysAgo = new Date(now.getTime() - 2 * 86_400_000);
		const weekAgo = new Date(now.getTime() - 7 * 86_400_000);
		const [
			sum,
			mine,
			top,
			glossary,
			products,
			users,
			hist,
			stats,
			prevStats,
			weekStats,
			failed,
			schedules,
			latestRuns,
			agentHistory,
			recentViews,
			saved,
			agentRows,
			staleAssets
		] = await Promise.all([
			json<AssetSummary>('/assets/summary'),
			json<{ assets?: MyAsset[] }>('/assets/my-assets?limit=5'),
			json<ViewedAsset[] | { assets?: ViewedAsset[] }>(
				`/metrics/top-assets?start=${monthAgo.toISOString()}&end=${now.toISOString()}&limit=5`
			),
			json<{ total: number }>('/glossary/list?limit=1'),
			json<{ total: number }>('/products/list?limit=1'),
			json<{ total: number }>('/users?limit=1'),
			canQuery ? listQueryHistory({ limit: 8 }).catch(() => null) : Promise.resolve(null),
			canQuery ? getQueryStats(dayAgo).catch(() => null) : Promise.resolve(null),
			canQuery ? getQueryStats(twoDaysAgo, dayAgo).catch(() => null) : Promise.resolve(null),
			canQuery ? getQueryStats(weekAgo).catch(() => null) : Promise.resolve(null),
			json<{ total: number }>('/ingestion/runs?status=failed&limit=1'),
			json<{ schedules?: Pipeline[] }>('/ingestion/schedules?limit=5'),
			json<{ runs?: PipelineRun[] }>('/ingestion/runs?limit=100'),
			canQuery
				? listQueryHistory({ client: 'mcp,cli', limit: 1 }).catch(() => null)
				: Promise.resolve(null),
			listRecent(10).catch(() => []),
			canQuery ? listSavedQueries(20).catch(() => null) : Promise.resolve(null),
			canQuery ? listAgentSummaries(4).catch(() => null) : Promise.resolve(null),
			listStaleAssets(STALE_SHOWN).catch(() => null)
		]);
		// The asset summary backs every view, so without it the API is taken to be unreachable
		unreachable = sum === null;
		summary = { types: sum?.types ?? {}, providers: sum?.providers ?? {} };
		myAssets = mine?.assets ?? [];
		viewed = Array.isArray(top) ? top : (top?.assets ?? []);
		glossaryTotal = glossary?.total ?? 0;
		productsTotal = products?.total ?? 0;
		usersTotal = users?.total ?? 1;
		history = hist;
		agentQueries = agentHistory?.total ?? 0;
		day = stats;
		yesterday = prevStats;
		week = weekStats;
		failedRuns = failed?.total ?? 0;
		pipelines = schedules?.schedules ?? [];
		runs = latestRuns?.runs ?? [];
		recents = recentViews;
		savedQueries = saved?.queries ?? [];
		agentSummaries = agentRows;
		stale = staleAssets?.assets ?? [];
		staleTotal = staleAssets?.total ?? 0;
		loading = false;
	}

	function retry() {
		loading = true;
		load();
	}

	onMount(() => {
		userProfile.load();
		load();
		// Coming back to the tab, after connecting an agent or running a pipeline elsewhere, refreshes the page so finished steps tick off without a reload
		const onVisible = () => {
			if (document.visibilityState === 'visible') load();
		};
		document.addEventListener('visibilitychange', onVisible);
		return () => document.removeEventListener('visibilitychange', onVisible);
	});
</script>

<svelte:head>
	<title>{m.home_page_title()}</title>
</svelte:head>

{#snippet kpis()}
	<mm-kpi-row label={m.home_kpi_last_day()}>
		{#if canQuery}
			<mm-kpi-tile
				label={m.home_kpi_queries()}
				value={day?.totals?.queries ?? 0}
				{spark}
				delta={day && yesterday ? day.totals.queries - yesterday.totals.queries : undefined}
				context={day && yesterday ? m.home_kpi_on_yesterday() : m.home_kpi_last_day()}
				error={day ? '' : m.home_kpi_error()}
			></mm-kpi-tile>
			<mm-kpi-tile label={m.home_kpi_agents()} value={activeAgentsToday}></mm-kpi-tile>
			<mm-kpi-tile
				label={m.home_kpi_refused()}
				value={deniedToday}
				delta={day && yesterday ? deniedToday - yesterday.totals.denied : undefined}
				trend="bad"
				context={day && yesterday ? m.home_kpi_on_yesterday() : ''}
			></mm-kpi-tile>
		{/if}
		<mm-kpi-tile label={m.home_kpi_assets()} value={sourceAssets}></mm-kpi-tile>
	</mm-kpi-row>
{/snippet}

{#snippet activity()}
	{#if !canQuery}
		<mm-panel heading={m.home_activity_heading()} icon="history" padding="default">
			<mm-text variant="dense" tone="secondary" block>{m.home_needs_query()}</mm-text>
		</mm-panel>
	{:else}
		<mm-panel
			heading={m.home_activity_heading()}
			icon="history"
			link-href={recent.length > 0 ? resolve(canViewAccess ? '/query-audit' : '/query') : ''}
			link-label={m.home_activity_link()}
		>
			<mm-activity-feed
				error={history ? '' : m.home_activity_error()}
				empty-heading={m.home_activity_empty_heading()}
				empty-description={m.home_activity_empty()}
				onmm-retry={retry}
			>
				{#each recent as q (q.id)}
					{@const target = activityTarget(q)}
					<mm-activity-item
						interactive
						selected={detailId === q.id}
						actor={subjectParts(q.audit_subject).name}
						actor-kind={q.principal_type === 'user' ? 'person' : 'agent'}
						verb={m.home_activity_verb()}
						target={target.name}
						target-href={target.href}
						time={q.started_at}
						status={q.decision === 'denied' ? 'denied' : q.status === 'failed' ? 'failed' : ''}
						onmm-activate={() => (detailId = q.id)}
					></mm-activity-item>
				{/each}
			</mm-activity-feed>
		</mm-panel>
	{/if}
{/snippet}

{#snippet jumpBackIn()}
	{#if jumpItems.length > 0}
		<mm-panel heading={m.home_jump_heading()} icon="history">
			<mm-list aria-label={m.home_jump_heading()}>
				{#each jumpItems as a (a.key)}
					<mm-list-item href={a.href || undefined} secondary={a.caption}>
						{#if a.provider}
							<mm-provider-logo slot="start" provider={a.provider} size="20" decorative
							></mm-provider-logo>
						{:else}
							<mm-icon slot="start" name={a.icon ?? 'table'} aria-hidden="true"></mm-icon>
						{/if}
						{a.name}
					</mm-list-item>
				{/each}
			</mm-list>
		</mm-panel>
	{/if}
{/snippet}

{#snippet needsAttention()}
	{#if attention.length === 0}
		<mm-panel heading={m.home_attention_heading()} icon="flag" padding="default">
			<mm-text variant="dense" tone="secondary" block>{m.home_attention_clear()}</mm-text>
		</mm-panel>
	{:else}
		<mm-panel heading={m.home_attention_heading()} icon="flag" count={attention.length}>
			<mm-list>
				{#each attention as a (a.key)}
					<mm-list-item href={a.href || undefined} secondary={a.secondary || undefined}>
						{#if a.provider}
							<mm-provider-logo slot="start" provider={a.provider} size="20" decorative
							></mm-provider-logo>
						{:else}
							<mm-icon slot="start" name={a.icon} aria-hidden="true"></mm-icon>
						{/if}
						{a.label}
						{#if a.count !== undefined}
							<mm-text slot="end" variant="dense" numeric>{formatNumber(a.count)}</mm-text>
						{/if}
					</mm-list-item>
				{/each}
			</mm-list>
		</mm-panel>
	{/if}
{/snippet}

{#snippet agentsPanel()}
	{#if canQuery}
		{#if agents.length === 0}
			<mm-panel heading={m.home_agents_heading()} icon="smart-toy" empty>
				<mm-empty-state
					slot="empty"
					compact
					seed={11}
					heading={m.home_agents_empty_heading()}
					description={m.home_agents_empty()}
				>
					<mm-button slot="action" icon="smart-toy" onclick={() => (connect = 'agent')}
						>{m.home_step_agent_action()}</mm-button
					>
				</mm-empty-state>
			</mm-panel>
		{:else}
			<mm-panel
				heading={m.home_agents_heading()}
				icon="smart-toy"
				link-href={resolve('/insights')}
				link-label={m.home_agents_link()}
			>
				<mm-list>
					{#each agents as a (a.key)}
						<mm-list-item
							href={a.href || undefined}
							secondary={m.home_agents_queries({ count: formatNumber(a.total) })}
						>
							<mm-avatar slot="start" kind="agent" name={a.name} decorative></mm-avatar>
							{a.name}
						</mm-list-item>
					{/each}
				</mm-list>
			</mm-panel>
		{/if}
	{/if}
{/snippet}

{#snippet pipelinesPanel()}
	{#if pipelineRows.length === 0}
		<mm-panel heading={m.home_pipelines_heading()} icon="extension" padding="default">
			<mm-text variant="dense" tone="secondary" block>{m.home_pipelines_empty()}</mm-text>
		</mm-panel>
	{:else}
		<mm-panel
			heading={m.home_pipelines_heading()}
			icon="extension"
			link-href={resolve('/runs')}
			link-label={m.home_pipelines_link()}
		>
			<mm-list>
				{#each pipelineRows as p (p.id)}
					<mm-list-item
						href={resolve('/runs/[id]', { id: p.id })}
						secondary={p.last_run_at ? formatRelativeTime(p.last_run_at) : ''}
					>
						<mm-provider-logo slot="start" provider={p.plugin_id} size="20" decorative
						></mm-provider-logo>
						{p.name}
						<span slot="end" class="mm-cluster" style="--mm-gap: var(--space-3); flex-wrap: nowrap">
							{#if p.strip.length > 0}
								<span class="hidden sm:inline">
									<mm-run-strip
										runs={p.strip}
										label={m.home_pipelines_strip_label({ name: p.name })}
										live={p.last_run_status === 'running'}
									></mm-run-strip>
								</span>
							{/if}
							{#if p.last_run_status === 'failed'}
								<mm-status-pill state="failed"></mm-status-pill>
							{:else if p.last_run_status === 'running'}
								<mm-status-pill state="running" live></mm-status-pill>
							{/if}
						</span>
					</mm-list-item>
				{/each}
			</mm-list>
		</mm-panel>
	{/if}
{/snippet}

{#if loading}
	<div class="mm-stack">
		<mm-page-header heading={greeting}></mm-page-header>
		<mm-kpi-row label={m.home_kpi_last_day()}>
			{#each [m.home_kpi_queries(), m.home_kpi_agents(), m.home_kpi_refused(), m.home_kpi_assets()] as label (label)}
				<mm-kpi-tile {label} loading></mm-kpi-tile>
			{/each}
		</mm-kpi-row>
		<div class="mm-split">
			<mm-panel heading={m.home_activity_heading()} icon="history" loading></mm-panel>
			<mm-panel heading={m.home_attention_heading()} icon="flag" loading></mm-panel>
		</div>
	</div>
{:else if unreachable}
	<div class="mm-stack">
		<mm-page-header heading={greeting}></mm-page-header>
		<mm-callout tone="danger" heading={m.home_unreachable_heading()}>
			{m.home_unreachable_body()}
			<mm-button slot="actions" size="sm" icon="refresh" onclick={retry}>{m.home_retry()}</mm-button
			>
		</mm-callout>
	</div>
{:else if onboarding}
	<div class="mm-stack">
		<!-- The welcome and setup sit on a faint contour field, the one decoration Home allows, and only before setup is done -->
		<section class="welcome">
			<mm-topography seed={7} fade="edges"></mm-topography>
			<div class="mm-stack">
				<mm-page-header
					heading={firstName ? m.home_welcome({ name: firstName }) : m.home_welcome_anonymous()}
					description={blank ? m.home_welcome_empty() : m.home_welcome_first_run()}
				>
					<mm-button slot="actions" icon="link" onclick={() => (connect = 'agent')}
						>{m.home_connect_button()}</mm-button
					>
				</mm-page-header>
				<div class="max-w-[44rem]">
					<SetupChecklist
						{done}
						onConnectAgent={() => (connect = 'agent')}
						onHide={() => setupHidden.set(true)}
						onRemove={() => (confirmRemove = true)}
						{canManageIngestion}
						{canQuery}
						{isAdmin}
					/>
				</div>
			</div>
		</section>
		<!-- Before the catalog is in use, panels that can only say "nothing yet" stay hidden -->
		{#if !blank}
			{@render kpis()}
			<div class="mm-split">
				<div class="mm-stack">{@render activity()}</div>
				<div class="mm-stack">{@render pipelinesPanel()}</div>
			</div>
		{/if}
	</div>
{:else}
	<div class="mm-stack">
		<mm-page-header heading={greeting}>
			{#if setupHidden.value && !setupComplete && !setupRemoved.value}
				<mm-menu-button slot="actions" icon="more-horiz" icon-only label={m.home_more_actions()}>
					<mm-menu onmm-select={() => setupHidden.set(false)}>
						<mm-menu-item value="setup" icon="checklist">{m.home_setup_show()}</mm-menu-item>
					</mm-menu>
				</mm-menu-button>
			{/if}
			<mm-button slot="actions" icon="link" onclick={() => (connect = 'agent')}
				>{m.home_connect_button()}</mm-button
			>
		</mm-page-header>
		{@render kpis()}
		<div class="mm-split">
			<div class="mm-stack">
				{@render jumpBackIn()}
				{@render activity()}
			</div>
			<div class="mm-stack">
				{@render needsAttention()}
				{@render agentsPanel()}
			</div>
		</div>
	</div>
{/if}

<mm-confirm-dialog
	open={confirmRemove}
	heading={m.home_setup_remove_title()}
	body={m.home_setup_remove_body()}
	confirm-label={m.home_setup_remove_confirm()}
	tone="default"
	onmm-confirm={() => setupRemoved.set(true)}
	onmm-close={() => (confirmRemove = false)}
></mm-confirm-dialog>

{#if connect}
	<ConnectDrawer tab={connect} onClose={() => (connect = null)} />
{/if}

{#if detailId}
	<QueryBlade queryId={detailId} onClose={() => (detailId = null)} />
{/if}

<style>
	.welcome {
		position: relative;
		isolation: isolate;
		padding: var(--space-6);
		margin: calc(-1 * var(--space-6));
		margin-block-end: 0;
	}
</style>

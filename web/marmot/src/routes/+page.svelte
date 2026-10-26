<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import IconifyIcon from '@iconify/svelte';
	import Icon from '$components/ui/Icon.svelte';
	import ConnectBlade, { type ConnectTab } from '$components/home/ConnectBlade.svelte';
	import SetupChecklist, { type SetupStep } from '$components/home/SetupChecklist.svelte';
	import MarmotFlow from '$components/home/MarmotFlow.svelte';
	import QueryDetail from '$components/dataquery/QueryDetail.svelte';
	import Sparkline from '$components/charts/Sparkline.svelte';
	import { fetchApi } from '$lib/api';
	import { auth } from '$lib/stores/auth';
	import { userProfile } from '$lib/stores/user';
	import { setupHidden, setupRemoved, skippedSteps } from '$lib/stores/setup.svelte';
	import Modal from '$components/ui/Modal.svelte';
	import KpiTile from '$components/ui/KpiTile.svelte';
	import { PANEL, LABEL, BTN_PRIMARY, BTN_SECONDARY, BTN_DANGER } from '$lib/ui/classes';
	import { m } from '$lib/paraglide/messages';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { getQueryStats, listQueryHistory } from '$lib/dataquery/api';
	import { outcome, subjectParts } from '$lib/dataquery/format';
	import type { QueryRecord, QueryStats } from '$lib/dataquery/types';

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

	interface PipelineRun {
		id: string;
		pipeline_name: string;
		status: 'pending' | 'claimed' | 'running' | 'succeeded' | 'failed' | 'cancelled';
		started_at?: string;
		finished_at?: string;
		created_at: string;
		assets_created: number;
		assets_updated: number;
	}

	interface ViewedAsset {
		asset_id: string;
		asset_name: string;
		asset_type: string;
		asset_provider: string;
		count: number;
	}

	// The pages the homepage links into; none take route parameters, so resolve() accepts them with or without a query string
	type HomeRoute =
		| `${'/settings' | '/settings/system' | '/settings/members' | '/query-audit' | '/discover' | '/glossary' | '/insights' | '/query' | '/runs' | '/products/new'}`
		| `${'/settings' | '/discover' | '/runs'}?${string}`;

	const TASK =
		'inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-sm transition hover:border-gray-300 hover:bg-gray-50 hover:text-gray-900 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700';

	const canQuery = auth.hasPermission('assets', 'query');
	const canViewAccess = auth.hasPermission('access', 'view');
	const canManageIngestion = auth.hasPermission('ingestion', 'manage');
	const isAdmin = auth.hasRole('admin');

	let loading = $state(true);
	let summary = $state<AssetSummary>({ types: {}, providers: {} });
	let myAssets = $state<MyAsset[]>([]);
	let viewed = $state<ViewedAsset[]>([]);
	let glossaryTotal = $state(0);
	let productsTotal = $state(0);
	let usersTotal = $state(1);
	let recent = $state<QueryRecord[]>([]);
	let queriesTotal = $state(0);
	let agentQueries = $state(0);
	let deniedToday = $state(0);
	let failedRuns = $state(0);
	let day = $state<QueryStats | null>(null);
	let week = $state<QueryStats | null>(null);
	let runs = $state<PipelineRun[]>([]);

	let connect = $state<ConnectTab | null>(null);
	let detailId = $state<string | null>(null);
	let setupStep = $state<SetupStep | null>(null);

	// Agent assets are cataloged from agents themselves, so they do not count as a connected source
	let sourceAssets = $derived(
		Object.entries(summary.types).reduce(
			(n, [type, t]) => (type === 'Agent' ? n : n + (t.count ?? 0)),
			0
		)
	);
	let agentCount = $derived(summary.types['Agent']?.count ?? 0);
	let blank = $derived(!loading && sourceAssets === 0);

	let done = $derived<Record<SetupStep, boolean>>({
		source: sourceAssets > 0,
		// An agent counts once one is catalogued or any query has come through MCP or the CLI, which is how coding agents reach Marmot
		agent: agentCount > 0 || agentQueries > 0,
		query: queriesTotal > 0,
		context: glossaryTotal + productsTotal > 0,
		team: usersTotal > 1
	});
	// Skipped optional steps count as settled so the Setup pill stops asking about things the team does not use
	let setupDone = $derived(
		(Object.keys(done) as SetupStep[]).filter((k) => done[k] || skippedSteps.steps.includes(k))
			.length
	);
	let setupComplete = $derived(setupDone === 5);
	// The onboarding surface outlives the first source so the remaining steps stay in view; it goes when hidden or when every step is done or skipped
	let onboarding = $derived(
		!loading && !setupHidden.value && !setupRemoved.value && !setupComplete
	);
	let confirmClose = $state(false);

	let topProviders = $derived(
		Object.entries(summary.providers)
			.filter(([name]) => name !== 'Marmot')
			.sort((a, b) => b[1] - a[1])
			.slice(0, 5)
	);
	// Agents are the service accounts that query; people are left to the activity feed
	let agents = $derived(
		(week?.top_callers ?? []).filter((c) => c.principal_type === 'service_account').slice(0, 5)
	);
	let activeAgentsToday = $derived(
		(day?.top_callers ?? []).filter((c) => c.principal_type === 'service_account').length
	);
	let hourly = $derived(
		(day?.series ?? []).map((p) => ({
			label: new Date(p.bucket).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
			total: p.succeeded + p.failed + p.denied,
			denied: p.denied
		}))
	);
	let agentMax = $derived(Math.max(1, ...agents.map((a) => a.total)));

	// The common jobs people come to Marmot for, each one click from home; only the ones the viewer may do are offered
	interface Task {
		id: string;
		label: string;
		body: string;
		icon: string;
		route?: HomeRoute;
		connectTab?: ConnectTab;
	}
	const isGlossaryManager = auth.hasPermission('glossary', 'manage');
	const canManageAssets = auth.hasPermission('assets', 'manage');
	const tasks: Task[] = [
		...(canQuery
			? [
					{
						id: 'query',
						label: m.home_task_query(),
						body: m.home_task_query_body(),
						icon: 'material-symbols:terminal',
						route: '/query' as HomeRoute
					}
				]
			: []),
		{
			id: 'agent',
			label: m.home_task_agent(),
			body: m.home_task_agent_body(),
			icon: 'material-symbols:smart-toy-outline',
			connectTab: 'agent' as ConnectTab
		},
		...(canManageIngestion
			? [
					{
						id: 'source',
						label: m.home_task_source(),
						body: m.home_task_source_body(),
						icon: 'material-symbols:database-outline',
						connectTab: 'source' as ConnectTab
					}
				]
			: []),
		...(isGlossaryManager
			? [
					{
						id: 'term',
						label: m.home_task_term(),
						body: m.home_task_term_body(),
						icon: 'material-symbols:menu-book-outline',
						route: '/glossary' as HomeRoute
					}
				]
			: []),
		...(canManageAssets
			? [
					{
						id: 'product',
						label: m.home_task_product(),
						body: m.home_task_product_body(),
						icon: 'material-symbols:inventory-2-outline',
						route: '/products/new' as HomeRoute
					}
				]
			: [])
	];

	// What the blank homepage previews: the same tiles and panels the populated dashboard shows, empty
	const previewKpis = [
		{ label: m.home_kpi_assets() },
		{ label: m.home_kpi_queries() },
		{ label: m.home_kpi_agents() },
		{ label: m.home_kpi_refused() }
	];
	const previewPanels = [
		{
			title: m.home_activity_heading(),
			icon: 'material-symbols:bolt-outline',
			caption: m.home_preview_activity()
		},
		{
			title: m.home_agents_heading(),
			icon: 'material-symbols:smart-toy-outline',
			caption: m.home_preview_agents()
		},
		{
			title: m.home_pipelines_heading(),
			icon: 'material-symbols:account-tree-outline',
			caption: m.home_preview_pipelines()
		}
	];

	let attentionTotal = $derived(deniedToday + failedRuns);

	let firstName = $derived.by(() => {
		const profile = $userProfile;
		if (!profile) return null;
		const name = profile.name?.split(' ')[0] || profile.display_name || profile.username;
		return name ? name.charAt(0).toUpperCase() + name.slice(1) : null;
	});

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
		const weekAgo = new Date(now.getTime() - 7 * 86_400_000);
		const [
			sum,
			mine,
			top,
			glossary,
			products,
			users,
			history,
			stats,
			weekStats,
			failed,
			latestRuns,
			agentHistory
		] = await Promise.all([
			json<AssetSummary>('/assets/summary'),
			json<{ assets?: MyAsset[] }>('/assets/my-assets?limit=6'),
			json<ViewedAsset[] | { assets?: ViewedAsset[] }>(
				`/metrics/top-assets?start=${monthAgo.toISOString()}&end=${now.toISOString()}&limit=6`
			),
			json<{ total: number }>('/glossary/list?limit=1'),
			json<{ total: number }>('/products/list?limit=1'),
			json<{ total: number }>('/users?limit=1'),
			canQuery ? listQueryHistory({ limit: 8 }).catch(() => null) : Promise.resolve(null),
			canQuery ? getQueryStats(dayAgo).catch(() => null) : Promise.resolve(null),
			canQuery ? getQueryStats(weekAgo).catch(() => null) : Promise.resolve(null),
			json<{ total: number }>('/ingestion/runs?status=failed&limit=1'),
			json<{ runs?: PipelineRun[] }>('/ingestion/runs?limit=5'),
			canQuery
				? listQueryHistory({ client: 'mcp,cli', limit: 1 }).catch(() => null)
				: Promise.resolve(null)
		]);
		summary = { types: sum?.types ?? {}, providers: sum?.providers ?? {} };
		myAssets = mine?.assets ?? [];
		viewed = Array.isArray(top) ? top : (top?.assets ?? []);
		glossaryTotal = glossary?.total ?? 0;
		productsTotal = products?.total ?? 0;
		usersTotal = users?.total ?? 1;
		recent = history?.queries ?? [];
		queriesTotal = history?.total ?? 0;
		agentQueries = agentHistory?.total ?? 0;
		day = stats;
		week = weekStats;
		deniedToday = stats?.totals?.denied ?? 0;
		failedRuns = failed?.total ?? 0;
		runs = latestRuns?.runs ?? [];
		loading = false;
	}

	function myAssetParams(a: MyAsset) {
		const parts = (a.mrn ?? '').replace('mrn://', '').split('/');
		if (parts.length >= 3)
			return { type: parts[0], service: parts[1], name: parts.slice(2).join('/') };
		return { type: a.type, service: a.providers?.[0] ?? '', name: a.name };
	}

	function oneLine(text: string): string {
		return text.replace(/\s+/g, ' ').trim();
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

{#snippet moduleHeader(title: string, icon: string, linkText?: string, route?: HomeRoute)}
	<div class="flex items-center justify-between">
		<h2 class="flex items-center gap-2 text-sm font-semibold text-gray-900 dark:text-gray-100">
			<IconifyIcon {icon} class="h-4 w-4 text-gray-400" />
			{title}
		</h2>
		{#if linkText && route}
			<a
				href={resolve(route)}
				class="text-xs font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
				>{linkText}</a
			>
		{/if}
	</div>
{/snippet}

{#snippet attentionRow(icon: string, label: string, count: number, route: HomeRoute, tone: string)}
	<a
		href={resolve(route)}
		class="flex items-center gap-3 rounded-lg px-2 py-2 transition hover:bg-gray-50 dark:hover:bg-gray-700/40"
	>
		<span
			class="flex h-8 w-8 items-center justify-center rounded-lg {count > 0
				? tone
				: 'bg-gray-100 text-gray-400 dark:bg-gray-700'}"
		>
			<IconifyIcon {icon} class="h-4 w-4" />
		</span>
		<span class="flex-1 text-sm text-gray-700 dark:text-gray-300">{label}</span>
		<span
			class="text-sm font-semibold tabular-nums {count > 0
				? 'text-gray-900 dark:text-gray-100'
				: 'text-gray-400'}">{formatNumber(count)}</span
		>
	</a>
{/snippet}

<div class="w-full space-y-6 px-4 py-6 sm:px-6 lg:px-8">
	{#if !loading && !onboarding}
		<div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
			<div class="max-w-3xl">
				<h1 class="text-2xl font-semibold tracking-tight text-gray-900 dark:text-gray-100">
					{firstName ? m.home_greeting({ name: firstName }) : m.home_greeting_anonymous()}
				</h1>
				<p class="mt-1 text-sm text-gray-600 dark:text-gray-400">{m.home_hero_title()}</p>
			</div>
			<div class="flex flex-shrink-0 items-center gap-2">
				{#if !setupComplete && !setupRemoved.value}
					<button type="button" onclick={() => setupHidden.set(false)} class={BTN_SECONDARY}>
						<span class="flex gap-0.5" aria-hidden="true">
							{#each Array(5) as _, i (i)}
								<span
									class="h-1.5 w-3 rounded-full {i < setupDone
										? 'bg-earthy-green-500'
										: 'bg-gray-200 dark:bg-gray-600'}"
								></span>
							{/each}
						</span>
						{m.home_setup_pill({ done: setupDone, total: 5 })}
					</button>
				{/if}
				<button type="button" onclick={() => (connect = 'agent')} class={BTN_PRIMARY}>
					<IconifyIcon icon="material-symbols:add-link" class="h-5 w-5" />
					{m.home_connect_button()}
				</button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="h-80 animate-pulse rounded-2xl bg-gray-100 dark:bg-gray-800"></div>
		<div class="h-32 animate-pulse rounded-xl bg-gray-100 dark:bg-gray-800"></div>
	{:else}
		{#if onboarding}
			<!-- Onboarding is one surface: the message and the checklist on the left, a picture of what Marmot does on the right; it stays until it is hidden or every step is settled, data or not -->
			<section
				class="hero relative overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
			>
				<div
					class="relative grid items-center gap-10 p-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] lg:p-12"
				>
					<div class="max-w-xl">
						<h1
							class="text-3xl font-semibold tracking-tight text-gray-900 sm:text-4xl dark:text-gray-100"
						>
							{m.home_blank_title()}
						</h1>
						<p class="mt-4 text-base text-gray-600 dark:text-gray-400">{m.home_blank_body()}</p>
						<div class="mt-10">
							<SetupChecklist
								{done}
								bind:selected={setupStep}
								onDismiss={() => setupHidden.set(true)}
								onRemove={() => (confirmClose = true)}
								{canManageIngestion}
								{canQuery}
								{isAdmin}
							/>
						</div>
					</div>
					<div class="hidden lg:block">
						<MarmotFlow />
					</div>
				</div>
			</section>
		{/if}

		{#if blank}
			<!-- A faded preview of the dashboard this page becomes, so a blank instance shows where things will appear rather than empty space -->
			<section class="ghost space-y-4" aria-hidden="true">
				<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
					{#each previewKpis as kpi (kpi.label)}
						<div class="{PANEL} p-4">
							<p class="text-xs font-medium text-gray-500 dark:text-gray-400">{kpi.label}</p>
							<p class="mt-1 text-2xl font-semibold tabular-nums text-gray-300 dark:text-gray-600">
								0
							</p>
							<svg
								class="mt-3 h-6 w-full"
								viewBox="0 0 100 24"
								preserveAspectRatio="none"
								aria-hidden="true"
							>
								<path
									d="M0 20 L100 20"
									class="stroke-gray-200 dark:stroke-gray-700"
									stroke-width="2"
									stroke-dasharray="3 4"
									vector-effect="non-scaling-stroke"
								/>
							</svg>
						</div>
					{/each}
				</div>
				<div class="grid gap-4 lg:grid-cols-3">
					{#each previewPanels as panel (panel.title)}
						<div class="{PANEL} p-5">
							{@render moduleHeader(panel.title, panel.icon)}
							<ul class="mt-4 space-y-3" aria-hidden="true">
								{#each [70, 55, 80, 45] as width, i (i)}
									<li class="flex items-center gap-3">
										<span class="h-7 w-7 flex-shrink-0 rounded-lg bg-gray-100 dark:bg-gray-700/60"
										></span>
										<span class="flex-1 space-y-1.5">
											<span
												class="block h-2 rounded-full bg-gray-100 dark:bg-gray-700/60"
												style="width: {width}%"
											></span>
											<span
												class="block h-2 rounded-full bg-gray-100/70 dark:bg-gray-700/40"
												style="width: {width - 25}%"
											></span>
										</span>
									</li>
								{/each}
							</ul>
							<p class="mt-4 text-xs text-gray-500 dark:text-gray-400">{panel.caption}</p>
						</div>
					{/each}
				</div>
			</section>
		{:else}
			<!-- Quick ways into the common jobs -->
			<div
				class="-mt-2 flex flex-wrap items-center gap-2"
				role="group"
				aria-label={m.home_tasks_label()}
			>
				{#each tasks as task (task.id)}
					{#if task.route}
						<a href={resolve(task.route)} class={TASK} title={task.body}>
							<IconifyIcon icon={task.icon} class="h-4 w-4 text-gray-500 dark:text-gray-400" />
							{task.label}
						</a>
					{:else}
						<button
							type="button"
							onclick={() => (connect = task.connectTab ?? 'agent')}
							class={TASK}
							title={task.body}
						>
							<IconifyIcon icon={task.icon} class="h-4 w-4 text-gray-500 dark:text-gray-400" />
							{task.label}
						</button>
					{/if}
				{/each}
			</div>

			<!-- The pulse: how much is happening and how much of it was refused -->
			<div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
				{#if canQuery}
					<a
						href={resolve('/query')}
						class="{PANEL} block p-4 transition hover:border-gray-300 dark:hover:border-gray-600"
					>
						<div class={LABEL}>
							{m.home_kpi_queries()}
						</div>
						<div class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-gray-100">
							{formatNumber(day?.totals?.queries ?? 0)}
						</div>
						<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
							{m.home_kpi_last_day()}
						</div>
						<Sparkline points={hourly.map((h) => ({ label: h.label, value: h.total }))} />
					</a>
					<KpiTile
						href={resolve('/query')}
						label={m.home_kpi_agents()}
						value={formatNumber(activeAgentsToday)}
						sub={m.home_kpi_agents_sub({ count: formatNumber(agentCount) })}
					/>
					<a
						href={resolve(canViewAccess ? '/query-audit' : '/query')}
						class="{PANEL} block p-4 transition hover:border-gray-300 dark:hover:border-gray-600"
					>
						<div class={LABEL}>
							{m.home_kpi_refused()}
						</div>
						<div
							class="mt-1 text-2xl font-semibold tabular-nums {deniedToday > 0
								? 'text-red-700 dark:text-red-300'
								: 'text-gray-900 dark:text-gray-100'}"
						>
							{formatNumber(deniedToday)}
						</div>
						<div class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
							{m.home_kpi_last_day()}
						</div>
						<Sparkline
							points={hourly.map((h) => ({ label: h.label, value: h.denied }))}
							tone="critical"
						/>
					</a>
				{/if}
				<KpiTile
					href={resolve('/insights')}
					label={m.home_kpi_assets()}
					value={formatNumber(sourceAssets)}
					sub={m.home_kpi_assets_sub({ count: formatNumber(topProviders.length) })}
				>
					<div class="flex flex-wrap gap-1.5">
						{#each topProviders.slice(0, 4) as [name] (name)}
							<span
								class="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-[11px] text-gray-600 dark:bg-gray-700 dark:text-gray-300"
							>
								<Icon name={name.toLowerCase()} size="xs" showLabel={false} />
								{name}
							</span>
						{/each}
					</div>
				</KpiTile>
			</div>

			<!-- What happened, and what needs a look -->
			<div class="grid gap-5 lg:grid-cols-3">
				<section class="{PANEL} p-5 lg:col-span-2">
					{@render moduleHeader(
						m.home_activity_heading(),
						'material-symbols:bolt-outline',
						canQuery ? m.home_activity_link() : undefined,
						canViewAccess ? '/query-audit' : '/query'
					)}
					{#if !canQuery}
						<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">{m.home_needs_query()}</p>
					{:else if recent.length === 0}
						<div class="mt-6 flex flex-col items-start gap-3">
							<p class="text-sm text-gray-500 dark:text-gray-400">{m.home_activity_empty()}</p>
							<button
								type="button"
								onclick={() => (connect = 'agent')}
								class="inline-flex items-center gap-1.5 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
							>
								<IconifyIcon icon="material-symbols:smart-toy-outline" class="h-4 w-4" />
								{m.home_step_agent_action()}
							</button>
						</div>
					{:else}
						<ul class="mt-3 divide-y divide-gray-100 dark:divide-gray-700/60">
							{#each recent as q (q.id)}
								{@const who = subjectParts(q.audit_subject)}
								{@const o = outcome(q)}
								<li>
									<button
										type="button"
										onclick={() => (detailId = q.id)}
										class="flex w-full items-center gap-3 py-2.5 text-left transition hover:bg-gray-50 dark:hover:bg-gray-700/30"
									>
										<span
											class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full {q.principal_type ===
											'user'
												? 'bg-earthy-blue-100 text-earthy-blue-800 dark:bg-earthy-blue-900/40 dark:text-earthy-blue-200'
												: 'bg-earthy-terracotta-100 text-earthy-terracotta-800 dark:bg-earthy-terracotta-900/40 dark:text-earthy-terracotta-200'}"
										>
											<IconifyIcon
												icon={q.principal_type === 'user'
													? 'material-symbols:person-outline'
													: 'material-symbols:smart-toy-outline'}
												class="h-4 w-4"
											/>
										</span>
										<span class="min-w-0 flex-1">
											<span class="block truncate text-sm text-gray-900 dark:text-gray-100">
												<span class="font-medium">{who.name}</span>
												<span class="text-gray-500 dark:text-gray-400">
													· {q.source_name} · {formatRelativeTime(q.started_at)}</span
												>
											</span>
											<span
												class="block truncate font-mono text-xs text-gray-500 dark:text-gray-400"
												>{oneLine(q.statement)}</span
											>
										</span>
										<span
											class="flex-shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium {o.classes}"
											>{o.label}</span
										>
									</button>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<div class="space-y-5">
					{#if canQuery}
						<section class="{PANEL} p-5">
							{@render moduleHeader(
								m.home_agents_heading(),
								'material-symbols:smart-toy-outline',
								m.home_agents_link(),
								'/query'
							)}
							{#if agents.length === 0}
								<div class="mt-4 flex flex-col items-start gap-2">
									<p class="text-sm text-gray-500 dark:text-gray-400">{m.home_agents_empty()}</p>
									<button
										type="button"
										onclick={() => (connect = 'agent')}
										class="inline-flex items-center gap-1.5 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
									>
										<IconifyIcon icon="material-symbols:add-link" class="h-4 w-4" />
										{m.home_step_agent_action()}
									</button>
								</div>
							{:else}
								<ul class="mt-3 space-y-3">
									{#each agents as a (a.key)}
										<li>
											<div class="flex items-center gap-2 text-sm">
												<IconifyIcon
													icon="material-symbols:smart-toy-outline"
													class="h-4 w-4 flex-shrink-0 text-earthy-terracotta-600"
												/>
												<span class="truncate font-medium text-gray-900 dark:text-gray-100"
													>{subjectParts(a.label).name}</span
												>
												<span
													class="ml-auto whitespace-nowrap text-xs tabular-nums text-gray-500 dark:text-gray-400"
												>
													{m.home_agents_counts({
														total: formatNumber(a.total),
														denied: formatNumber(a.denied)
													})}
												</span>
											</div>
											<div
												class="mt-1.5 flex h-1.5 overflow-hidden rounded-full bg-gray-100 dark:bg-gray-700/60"
												style="width: {(a.total / agentMax) * 100}%"
											>
												<div
													class="bg-earthy-terracotta-400 dark:bg-earthy-terracotta-500"
													style="width: {(a.allowed / Math.max(1, a.total)) * 100}%"
												></div>
												<div
													class="bg-red-300 dark:bg-red-400/70"
													style="width: {(a.denied / Math.max(1, a.total)) * 100}%"
												></div>
											</div>
										</li>
									{/each}
								</ul>
								<p class="mt-3 text-[11px] text-gray-400">{m.home_agents_window()}</p>
							{/if}
						</section>
					{/if}

					<section class="{PANEL} p-5">
						{@render moduleHeader(
							m.home_attention_heading(),
							'material-symbols:notifications-outline'
						)}
						<div class="mt-3 space-y-0.5">
							{#if canQuery}
								{@render attentionRow(
									'material-symbols:block',
									m.home_attention_denied(),
									deniedToday,
									canViewAccess ? '/query-audit' : '/query',
									'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
								)}
							{/if}
							{@render attentionRow(
								'material-symbols:sync-problem',
								m.home_attention_failed_runs(),
								failedRuns,
								'/runs',
								'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-300'
							)}
						</div>
						{#if attentionTotal === 0}
							<p
								class="mt-3 flex items-center gap-1.5 border-t border-gray-100 pt-3 text-xs text-earthy-green-700 dark:border-gray-700 dark:text-earthy-green-400"
							>
								<IconifyIcon icon="material-symbols:check-circle-outline" class="h-4 w-4" />
								{m.home_attention_clear()}
							</p>
						{/if}
					</section>
				</div>
			</div>

			<!-- Your data, popular data and what the catalog holds -->
			<div class="grid gap-5 lg:grid-cols-3">
				<section class="{PANEL} p-5">
					{@render moduleHeader(
						m.home_your_assets_heading(),
						'material-symbols:person-pin-outline',
						m.home_view_all_link(),
						'/discover'
					)}
					{#if myAssets.length === 0}
						<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
							{m.home_your_assets_empty()}
						</p>
					{:else}
						<ul class="mt-3 space-y-1">
							{#each myAssets as a (a.id)}
								<li>
									<a
										href={resolve('/discover/[type]/[service]/[name]', myAssetParams(a))}
										class="flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-sm transition hover:bg-gray-50 dark:hover:bg-gray-700/40"
									>
										<Icon name={a.providers?.[0] ?? a.type} size="xs" showLabel={false} />
										<span class="truncate text-gray-800 dark:text-gray-200">{a.name}</span>
										<span class="ml-auto text-xs text-gray-400">{a.type}</span>
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="{PANEL} p-5">
					{@render moduleHeader(
						m.home_most_viewed_heading(),
						'material-symbols:visibility-outline',
						m.home_see_all_link(),
						'/insights'
					)}
					{#if viewed.length === 0}
						<p class="mt-6 text-sm text-gray-500 dark:text-gray-400">
							{m.home_most_viewed_empty()}
						</p>
					{:else}
						<ul class="mt-3 space-y-1">
							{#each viewed as a (a.asset_id)}
								<li>
									<a
										href={resolve('/discover/[type]/[service]/[name]', {
											type: a.asset_type,
											service: a.asset_provider,
											name: a.asset_name
										})}
										class="flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-sm transition hover:bg-gray-50 dark:hover:bg-gray-700/40"
									>
										<Icon name={a.asset_provider} size="xs" showLabel={false} />
										<span class="truncate text-gray-800 dark:text-gray-200">{a.asset_name}</span>
										<span class="ml-auto text-xs tabular-nums text-gray-400"
											>{formatNumber(a.count)}</span
										>
									</a>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<section class="{PANEL} p-5">
					{@render moduleHeader(
						m.home_pipelines_heading(),
						'material-symbols:account-tree-outline',
						m.home_view_all_link(),
						'/runs'
					)}
					{#if runs.length === 0}
						<div class="mt-4 flex flex-col items-start gap-2">
							<p class="text-sm text-gray-500 dark:text-gray-400">{m.home_pipelines_empty()}</p>
							{#if canManageIngestion}
								<button
									type="button"
									onclick={() => (connect = 'source')}
									class="inline-flex items-center gap-1.5 text-sm font-medium text-earthy-terracotta-700 hover:underline dark:text-earthy-terracotta-400"
								>
									<IconifyIcon icon="material-symbols:add" class="h-4 w-4" />
									{m.home_step_source_action()}
								</button>
							{/if}
						</div>
					{:else}
						<ul class="mt-3 space-y-1">
							{#each runs as run (run.id)}
								<li
									class="flex items-center gap-2.5 rounded-lg px-2 py-1.5 text-sm"
									title={run.status}
								>
									<span
										class="h-2 w-2 flex-shrink-0 rounded-full {run.status === 'succeeded'
											? 'bg-earthy-green-500'
											: run.status === 'failed'
												? 'bg-red-500'
												: run.status === 'running' || run.status === 'claimed'
													? 'bg-earthy-blue-500'
													: 'bg-gray-400'}"
									></span>
									<span class="truncate text-gray-800 dark:text-gray-200">{run.pipeline_name}</span>
									<span class="ml-auto whitespace-nowrap text-xs text-gray-400">
										{#if run.status === 'succeeded'}
											{m.home_pipelines_changed({
												count: formatNumber(run.assets_created + run.assets_updated)
											})} ·
										{/if}
										{formatRelativeTime(run.finished_at ?? run.started_at ?? run.created_at)}
									</span>
								</li>
							{/each}
						</ul>
					{/if}
				</section>
			</div>
		{/if}
	{/if}
</div>

{#if confirmClose}
	<Modal
		title={m.home_setup_remove_title()}
		subtitle={m.home_setup_remove_body()}
		onClose={() => (confirmClose = false)}
		width="md"
	>
		{#snippet footer(close)}
			<button
				type="button"
				onclick={close}
				class="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
				>{m.common_cancel()}</button
			>
			<button
				type="button"
				onclick={() => {
					setupRemoved.set(true);
					confirmClose = false;
				}}
				class={BTN_DANGER}>{m.home_setup_remove_confirm()}</button
			>
		{/snippet}
	</Modal>
{/if}

{#if connect}
	<ConnectBlade tab={connect} onClose={() => (connect = null)} />
{/if}

{#if detailId}
	<QueryDetail queryId={detailId} onClose={() => (detailId = null)} />
{/if}

<style>
	/* The blank-state preview fades out downwards so it reads as a glimpse of what is coming, not as broken panels */
	.ghost {
		mask-image: linear-gradient(to bottom, black 55%, transparent);
	}

	/* A faint dot grid behind the hero, fading out towards the copy so the text stays clean */
	.hero::before {
		content: '';
		position: absolute;
		inset: 0;
		background-image: radial-gradient(rgb(148 163 184 / 0.35) 1px, transparent 1px);
		background-size: 18px 18px;
		mask-image: linear-gradient(to right, transparent 25%, black 70%);
	}
</style>

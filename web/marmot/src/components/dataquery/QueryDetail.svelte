<script lang="ts">
	import IconifyIcon from '@iconify/svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import CodeBlock from '$components/editor/CodeBlock.svelte';
	import Blade from '$components/ui/Blade.svelte';
	import Icon from '$components/ui/Icon.svelte';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatDateTime, formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { getQueryRecord } from '$lib/dataquery/api';
	import {
		assetRouteParams,
		clientLabel,
		isRealTime,
		outcome,
		subjectParts
	} from '$lib/dataquery/format';
	import type { QueryAssetContext, QueryRecord } from '$lib/dataquery/types';

	interface Props {
		queryId: string;
		onClose: () => void;
		// Inside the query workspace the statement loads in place; elsewhere the blade navigates to the workspace
		onOpen?: (source: string, statement: string, run: boolean) => void;
	}

	let { queryId, onClose, onOpen }: Props = $props();

	const canQuery = auth.hasPermission('assets', 'query');

	let query = $state<QueryRecord | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function load(id: string) {
		loading = true;
		error = '';
		query = null;
		try {
			query = await getQueryRecord(id);
		} catch (e) {
			error = getErrorMessage(e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load(queryId);
	});

	interface TableRow {
		mrn: string;
		name: string;
		wholeSource: boolean;
		context?: QueryAssetContext;
		route: { type: string; service: string; name: string } | null;
	}

	let who = $derived(query ? subjectParts(query.audit_subject) : null);
	let badge = $derived(query ? outcome(query) : null);
	let durationMs = $derived(
		query?.completed_at && isRealTime(query.completed_at)
			? new Date(query.completed_at).getTime() - new Date(query.started_at).getTime()
			: null
	);
	let tables = $derived<TableRow[]>(
		(query?.referenced_mrns ?? []).map((mrn) => {
			const context = query?.context?.find((c) => c.mrn === mrn);
			const wholeSource = mrn.startsWith('mrn://source/');
			return {
				mrn,
				name: context?.name || mrn.split('/').pop() || mrn,
				wholeSource,
				context,
				route: context?.asset_id ? assetRouteParams(mrn) : null
			};
		})
	);
	// The denial reason names the table by MRN, so it is rewritten with the table's name
	let reason = $derived.by(() => {
		const text = query?.decision_detail?.reason ?? '';
		return tables.reduce((out, t) => out.replaceAll(t.mrn, t.name), text);
	});

	function duration(ms: number): string {
		return ms < 1000 ? `${formatNumber(ms)} ms` : `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`;
	}

	function open(run: boolean) {
		if (!query) return;
		if (onOpen) {
			onOpen(query.source_name, query.statement, run);
			onClose();
			return;
		}
		const params = `source=${encodeURIComponent(query.source_name)}&q=${encodeURIComponent(query.statement)}${run ? '&run=1' : ''}`;
		goto(resolve(`/query?${params}`));
	}
</script>

{#snippet fact(label: string, icon: string)}
	<dt
		class="flex items-center gap-1.5 text-[11px] font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400"
	>
		<IconifyIcon {icon} class="h-3.5 w-3.5" />
		{label}
	</dt>
{/snippet}

{#snippet sectionTitle(text: string)}
	<h3 class="mb-2 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400">
		{text}
	</h3>
{/snippet}

<Blade
	title={m.dataquery_query_detail_heading()}
	subtitle={query && who
		? m.dataquery_detail_subtitle({ who: who.name, time: formatRelativeTime(query.started_at) })
		: undefined}
	{onClose}
	width="2xl"
>
	{#snippet actions()}
		{#if canQuery && query}
			<button
				type="button"
				onclick={() => open(false)}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				<IconifyIcon icon="material-symbols:terminal" class="h-4 w-4" />
				{m.dataquery_detail_open_editor()}
			</button>
			<button
				type="button"
				onclick={() => open(true)}
				class="inline-flex items-center gap-1.5 rounded-lg bg-earthy-terracotta-600 px-3 py-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500"
			>
				<IconifyIcon icon="material-symbols:play-arrow-rounded" class="h-5 w-5" />
				{m.dataquery_detail_run()}
			</button>
		{/if}
	{/snippet}

	{#if loading}
		<div class="space-y-4">
			<div class="h-24 animate-pulse rounded-xl bg-white dark:bg-gray-800"></div>
			<div class="h-20 animate-pulse rounded-xl bg-white dark:bg-gray-800"></div>
			<div class="h-32 animate-pulse rounded-xl bg-white dark:bg-gray-800"></div>
		</div>
	{:else if error}
		<p class="text-sm text-red-700 dark:text-red-300">{error}</p>
	{:else if query && who && badge}
		<div class="space-y-6">
			<!-- Summary -->
			<div
				class="rounded-xl border border-gray-200 bg-white p-5 dark:border-gray-700 dark:bg-gray-800"
			>
				<div class="flex flex-wrap items-center gap-2">
					<span
						class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-sm font-semibold {badge.classes}"
					>
						<IconifyIcon
							icon={query.decision === 'denied'
								? 'material-symbols:block'
								: query.status === 'failed'
									? 'material-symbols:error-outline'
									: 'material-symbols:check-circle-outline'}
							class="h-4 w-4"
						/>
						{badge.label}
					</span>
					{#if query.truncated}
						<span
							class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
							>{m.dataquery_query_truncated()}</span
						>
					{/if}
					<span
						class="ml-auto text-xs tabular-nums text-gray-500 dark:text-gray-400"
						title={formatDateTime(query.started_at)}>{formatDateTime(query.started_at)}</span
					>
				</div>
				<dl class="mt-4 grid grid-cols-2 gap-x-6 gap-y-4 sm:grid-cols-3">
					<div>
						{@render fact(
							m.dataquery_query_col_who(),
							query.principal_type === 'user'
								? 'material-symbols:person-outline'
								: 'material-symbols:smart-toy-outline'
						)}
						<dd class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
							{who.name}
							{#if who.kind}
								<span class="ml-1 text-xs font-normal text-gray-500 dark:text-gray-400"
									>{who.kind}</span
								>
							{/if}
						</dd>
					</div>
					<div>
						{@render fact(m.dataquery_query_col_source(), 'material-symbols:database-outline')}
						<dd class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-gray-100">
							{query.source_name}
						</dd>
					</div>
					<div>
						{@render fact(m.dataquery_query_col_client(), 'material-symbols:devices-outline')}
						<dd class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">
							{clientLabel(query.client)}
						</dd>
					</div>
					<div>
						{@render fact(m.dataquery_detail_duration(), 'material-symbols:timer-outline')}
						<dd class="mt-1 text-sm font-medium tabular-nums text-gray-900 dark:text-gray-100">
							{durationMs !== null ? duration(durationMs) : '—'}
						</dd>
					</div>
					<div>
						{@render fact(m.dataquery_query_col_rows(), 'material-symbols:table-rows-outline')}
						<dd class="mt-1 text-sm font-medium tabular-nums text-gray-900 dark:text-gray-100">
							{query.rows_returned != null ? formatNumber(query.rows_returned) : '—'}
						</dd>
					</div>
					<div>
						{@render fact(m.dataquery_detail_tables_count(), 'material-symbols:table-outline')}
						<dd class="mt-1 text-sm font-medium tabular-nums text-gray-900 dark:text-gray-100">
							{formatNumber(tables.length)}
						</dd>
					</div>
				</dl>
			</div>

			<!-- Statement -->
			<section>
				{@render sectionTitle(m.dataquery_query_statement_heading())}
				<div
					class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800"
				>
					<CodeBlock code={query.statement} language="sql" />
				</div>
			</section>

			{#if query.decision === 'denied' && reason}
				<div
					class="flex gap-3 rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-900/60 dark:bg-red-950/30"
				>
					<IconifyIcon
						icon="material-symbols:lock-outline"
						class="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600 dark:text-red-300"
					/>
					<div>
						<p class="text-sm font-semibold text-red-900 dark:text-red-200">
							{m.dataquery_detail_refused_heading()}
						</p>
						<p class="mt-0.5 text-sm text-red-800 dark:text-red-300">{reason}</p>
					</div>
				</div>
			{/if}

			{#if query.error}
				<div
					class="flex gap-3 rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/60 dark:bg-amber-950/30"
				>
					<IconifyIcon
						icon="material-symbols:error-outline"
						class="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600 dark:text-amber-300"
					/>
					<p class="break-words font-mono text-sm text-amber-800 dark:text-amber-300">
						{query.error}
					</p>
				</div>
			{/if}

			<!-- Tables read, with what the catalog knew about each -->
			<section>
				{@render sectionTitle(m.dataquery_detail_tables_heading())}
				<p class="-mt-1 mb-3 text-xs text-gray-500 dark:text-gray-400">
					{m.dataquery_query_context_hint()}
				</p>
				{#if tables.length === 0}
					<p class="text-sm text-gray-500 dark:text-gray-400">
						{m.dataquery_query_none_recorded()}
					</p>
				{:else}
					<ul class="space-y-3">
						{#each tables as table (table.mrn)}
							<li
								class="rounded-xl border border-gray-200 bg-white p-4 dark:border-gray-700 dark:bg-gray-800"
							>
								<div class="flex items-start gap-3">
									<span
										class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-50 dark:bg-gray-900"
									>
										{#if table.context?.providers?.[0]}
											<Icon name={table.context.providers[0]} size="sm" showLabel={false} />
										{:else}
											<IconifyIcon
												icon={table.wholeSource
													? 'material-symbols:database-outline'
													: 'material-symbols:table-outline'}
												class="h-5 w-5 text-gray-400"
											/>
										{/if}
									</span>
									<div class="min-w-0 flex-1">
										<div class="flex items-center gap-2">
											{#if table.wholeSource}
												<span class="truncate font-semibold text-gray-900 dark:text-gray-100"
													>{m.dataquery_detail_whole_source({ source: query.source_name })}</span
												>
											{:else if table.route}
												<a
													href={resolve('/discover/[type]/[service]/[name]', table.route)}
													class="group inline-flex min-w-0 items-center gap-1 font-semibold text-gray-900 hover:text-earthy-terracotta-700 dark:text-gray-100 dark:hover:text-earthy-terracotta-400"
												>
													<span class="truncate">{table.name}</span>
													<IconifyIcon
														icon="material-symbols:arrow-outward"
														class="h-3.5 w-3.5 flex-shrink-0 text-gray-400 group-hover:text-earthy-terracotta-600"
													/>
												</a>
											{:else}
												<span class="truncate font-semibold text-gray-900 dark:text-gray-100"
													>{table.name}</span
												>
											{/if}
											{#if table.context?.type}
												<span
													class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium uppercase text-gray-600 dark:bg-gray-700 dark:text-gray-300"
													>{table.context.type}</span
												>
											{/if}
										</div>
										{#if table.context?.description}
											<p class="mt-1 text-sm text-gray-600 dark:text-gray-300">
												{table.context.description}
											</p>
										{/if}
										{#if table.context?.tags?.length}
											<div class="mt-2 flex flex-wrap gap-1">
												{#each table.context.tags as tag (tag)}
													<span
														class="rounded-full bg-earthy-brown-100 px-2 py-0.5 text-[11px] text-earthy-brown-900 dark:bg-gray-700 dark:text-gray-300"
														>{tag}</span
													>
												{/each}
											</div>
										{/if}

										<div
											class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1.5 border-t border-gray-100 pt-3 text-xs dark:border-gray-700"
										>
											{#if query.decision === 'denied'}
												<span
													class="inline-flex items-center gap-1.5 rounded-full bg-red-100 px-2 py-0.5 font-medium text-red-800 dark:bg-red-900/40 dark:text-red-300"
												>
													<IconifyIcon icon="material-symbols:lock-outline" class="h-3.5 w-3.5" />
													{m.dataquery_decision_denied()}
												</span>
											{/if}
											{#if table.context}
												<span class="ml-auto text-gray-400">
													{isRealTime(table.context.last_sync_at)
														? m.dataquery_query_last_synced({
																time: formatRelativeTime(table.context.last_sync_at)
															})
														: m.dataquery_query_never_synced()}
												</span>
											{/if}
										</div>
									</div>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>
	{/if}
</Blade>

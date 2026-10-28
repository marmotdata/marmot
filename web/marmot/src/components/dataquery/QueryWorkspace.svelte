<script lang="ts">
	import { onMount } from 'svelte';
	import IconifyIcon from '@iconify/svelte';
	import { m } from '$lib/paraglide/messages';
	import { ApiError, getErrorMessage } from '$lib/stores/toast';
	import { formatNumber, formatRelativeTime } from '$lib/utils/format';
	import { listQueryHistory, listQuerySourceTables, runQuery } from '$lib/dataquery/api';
	import type { QueryRecord, QueryResult, QuerySourceTable } from '$lib/dataquery/types';
	import Blade from '$components/ui/Blade.svelte';
	import QueryDetail from './QueryDetail.svelte';
	import QueryLog from './QueryLog.svelte';
	import QueryEmptyState from './QueryEmptyState.svelte';
	import SourceExplorer from './SourceExplorer.svelte';
	import SourcePicker from './SourcePicker.svelte';
	import SqlEditor from './SqlEditor.svelte';
	import QueryResultsTable from './QueryResultsTable.svelte';
	import QueryResultContext from './QueryResultContext.svelte';
	import QueryResultsHistory from './QueryResultsHistory.svelte';

	let {
		hasSources,
		initialSource = '',
		initialStatement = '',
		autoRun = false
	}: {
		hasSources: boolean;
		initialSource?: string;
		initialStatement?: string;
		autoRun?: boolean;
	} = $props();

	type ResultsTab = 'results' | 'context' | 'history';
	type Drawer = 'history' | null;

	const ROW_LIMITS = [100, 1000, 10000];
	const EDITOR_KEY = 'marmot.query.editorHeight';
	const EXPLORER_KEY = 'marmot.query.explorer';

	let source = $state('');
	let statement = $state('');
	let maxRows = $state(100);
	let running = $state(false);
	let result = $state<QueryResult | null>(null);
	let denied = $state('');
	let failure = $state('');
	let elapsedMs = $state(0);
	let liveMs = $state(0);
	let detailId = $state<string | null>(null);
	let editor = $state<HTMLTextAreaElement | null>(null);
	let workspace = $state<HTMLElement | null>(null);

	let explorerOpen = $state(true);
	let editorHeight = $state(260);
	let resultsTab = $state<ResultsTab>('results');
	let drawer = $state<Drawer>(null);

	let recent = $state<QueryRecord[]>([]);
	let starters = $state<QuerySourceTable[]>([]);
	let startersRequest = 0;
	// A statement the workspace filled in itself, which it may replace when the source changes; anything typed is left alone
	let autoFilled = $state('');

	let canRun = $derived(!!source && statement.trim().length > 0 && !running);
	let isMac = typeof navigator !== 'undefined' && /Mac/.test(navigator.platform);
	let runShortcut = $derived(isMac ? '⌘↵' : 'Ctrl ↵');
	let explorerShortcut = $derived(isMac ? '⌘B' : 'Ctrl B');
	let contextCount = $derived(result?.context?.length ?? 0);

	function readStored(key: string): string | null {
		try {
			return localStorage.getItem(key);
		} catch {
			return null;
		}
	}

	function store(key: string, value: string) {
		try {
			localStorage.setItem(key, value);
		} catch {
			// A private window or blocked storage only loses the remembered layout
		}
	}

	async function loadRecent() {
		try {
			recent = (await listQueryHistory({ mine: true, limit: 25 })).queries;
		} catch {
			recent = [];
		}
	}

	async function loadStarters(name: string) {
		const id = ++startersRequest;
		starters = [];
		if (!name) return;
		try {
			const tables = await listQuerySourceTables(name, { limit: 6 });
			if (id !== startersRequest) return;
			starters = tables.filter((t) => t.sample_statement);
			const preview = starters[0]?.sample_statement ?? '';
			if (preview && (statement.trim() === '' || statement === autoFilled)) setStatement(preview);
		} catch {
			// Starters are a convenience; the workspace works without them
		}
	}

	function selectSource(name: string) {
		if (name === source) return;
		source = name;
		loadStarters(name);
	}

	function setStatement(text: string) {
		statement = text;
		autoFilled = text;
	}

	function clearOutcome() {
		result = null;
		denied = '';
		failure = '';
	}

	// Explorer clicks drop the name at the caret, the way schema trees in desktop SQL tools do
	function insertAtCaret(text: string) {
		if (!editor) {
			statement += text;
			return;
		}
		const { selectionStart, selectionEnd } = editor;
		statement = statement.slice(0, selectionStart) + text + statement.slice(selectionEnd);
		const caret = selectionStart + text.length;
		queueMicrotask(() => {
			editor?.focus();
			editor?.setSelectionRange(caret, caret);
		});
	}

	// Clicking a table only writes its sample query into the editor; nothing runs until the Run button or the shortcut is used
	function preview(name: string, text: string) {
		openStatement(name, text, false);
	}

	function openRecord(query: QueryRecord, andRun = false) {
		openStatement(query.source_name, query.statement, andRun);
	}

	function openStatement(name: string, text: string, andRun: boolean) {
		selectSource(name);
		setStatement(text);
		drawer = null;
		editor?.focus();
		if (andRun) run();
		else resultsTab = 'results';
	}

	async function run() {
		if (!source || statement.trim().length === 0 || running) return;
		running = true;
		clearOutcome();
		resultsTab = 'results';
		const started = performance.now();
		const ticker = setInterval(() => (liveMs = Math.round(performance.now() - started)), 100);
		try {
			result = await runQuery({ source, statement, max_rows: maxRows });
		} catch (e) {
			if (e instanceof ApiError && e.status === 403) denied = e.message;
			else failure = getErrorMessage(e);
		} finally {
			clearInterval(ticker);
			elapsedMs = Math.round(performance.now() - started);
			running = false;
			loadRecent();
		}
	}

	function onWindowKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'b') {
			e.preventDefault();
			toggleExplorer();
		}
	}

	function toggleExplorer() {
		explorerOpen = !explorerOpen;
		store(EXPLORER_KEY, explorerOpen ? '1' : '0');
	}

	// Dragging the bar between editor and results trades height between them, and the split is remembered
	function startResize(e: PointerEvent) {
		if (!workspace) return;
		e.preventDefault();
		const startY = e.clientY;
		const startHeight = editorHeight;
		const maxHeight = workspace.clientHeight - 180;
		const move = (ev: PointerEvent) => {
			editorHeight = Math.min(maxHeight, Math.max(120, startHeight + ev.clientY - startY));
		};
		const up = () => {
			window.removeEventListener('pointermove', move);
			window.removeEventListener('pointerup', up);
			store(EDITOR_KEY, String(editorHeight));
		};
		window.addEventListener('pointermove', move);
		window.addEventListener('pointerup', up);
	}

	function nudgeResize(e: KeyboardEvent) {
		if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return;
		e.preventDefault();
		editorHeight = Math.max(120, editorHeight + (e.key === 'ArrowDown' ? 24 : -24));
		store(EDITOR_KEY, String(editorHeight));
	}

	function deniedTable(message: string): string {
		const match = message.match(/mrn:\/\/\S+/);
		return match ? match[0] : '';
	}

	// The Results tab can show the rows as a table or as CSV or JSON text, so a result can be copied straight into a chat or a doc and only saved as a file when that is wanted
	let resultView = $state<'table' | 'csv' | 'json'>('table');

	function oneLine(text: string): string {
		return text.replace(/\s+/g, ' ').trim();
	}

	onMount(() => {
		const height = Number(readStored(EDITOR_KEY));
		if (height >= 120) editorHeight = height;
		explorerOpen = readStored(EXPLORER_KEY) !== '0';
		if (initialStatement) setStatement(initialStatement);
		loadRecent().then(() => {
			const name = initialSource || recent.find((q) => q.decision === 'allowed')?.source_name || '';
			if (name) selectSource(name);
			if (autoRun && initialStatement) run();
		});
	});
</script>

<svelte:window onkeydown={onWindowKeydown} />

{#snippet toolbarButton(label: string, icon: string, onclick: () => void, pressed = false)}
	<button
		type="button"
		{onclick}
		aria-pressed={pressed}
		class="inline-flex h-8 items-center gap-1.5 rounded-md px-2.5 text-sm font-medium transition {pressed
			? 'bg-gray-100 text-gray-900 dark:bg-gray-700 dark:text-gray-100'
			: 'text-gray-600 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-100'}"
	>
		<IconifyIcon {icon} class="h-4 w-4" />
		<span class="hidden lg:inline">{label}</span>
	</button>
{/snippet}

{#snippet emptyState()}
	<div class="mx-auto max-w-2xl px-6 py-10">
		<p class="text-center text-sm text-gray-500 dark:text-gray-400">
			{m.dataquery_workspace_empty({ shortcut: runShortcut })}
		</p>
		{#if starters.length > 0}
			<h4
				class="mt-8 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
			>
				{m.dataquery_workspace_starters({ source })}
			</h4>
			<div class="mt-2 grid gap-2 sm:grid-cols-2">
				{#each starters as table (table.mrn)}
					<button
						type="button"
						onclick={() => preview(source, table.sample_statement ?? '')}
						class="group flex items-start gap-3 rounded-lg border border-gray-200 bg-white p-3 text-left transition hover:border-earthy-terracotta-300 hover:shadow-sm dark:border-gray-700 dark:bg-gray-800 dark:hover:border-earthy-terracotta-700"
					>
						<IconifyIcon
							icon={table.type === 'View'
								? 'material-symbols:visibility-outline'
								: 'material-symbols:table-outline'}
							class="mt-0.5 h-4 w-4 flex-shrink-0 text-gray-400"
						/>
						<span class="min-w-0">
							<span class="block truncate font-mono text-sm text-gray-900 dark:text-gray-100"
								>{table.name}</span
							>
							<span class="line-clamp-1 text-xs text-gray-500 dark:text-gray-400"
								>{table.description || m.dataquery_workspace_preview_rows()}</span
							>
						</span>
						<IconifyIcon
							icon="material-symbols:play-arrow-rounded"
							class="ml-auto h-5 w-5 flex-shrink-0 text-gray-300 group-hover:text-earthy-terracotta-600"
						/>
					</button>
				{/each}
			</div>
		{/if}
		{#if recent.length > 0}
			<h4
				class="mt-8 text-xs font-semibold uppercase tracking-wider text-gray-500 dark:text-gray-400"
			>
				{m.dataquery_workspace_recent()}
			</h4>
			<ul class="mt-2 divide-y divide-gray-100 dark:divide-gray-800">
				{#each recent.slice(0, 4) as query (query.id)}
					<li>
						<button
							type="button"
							onclick={() => openRecord(query)}
							class="flex w-full items-center gap-3 py-2 text-left text-sm hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400"
						>
							<IconifyIcon
								icon="material-symbols:history"
								class="h-4 w-4 flex-shrink-0 text-gray-400"
							/>
							<span class="truncate font-mono text-xs text-gray-700 dark:text-gray-300"
								>{oneLine(query.statement)}</span
							>
							<span class="ml-auto whitespace-nowrap text-xs text-gray-400"
								>{formatRelativeTime(query.started_at)}</span
							>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/snippet}

{#if !hasSources}
	<QueryEmptyState />
{:else}
	<div class="flex min-h-0 flex-1">
		{#if explorerOpen}
			<aside
				class="hidden w-72 flex-shrink-0 border-r border-gray-200 bg-gray-50/60 md:block dark:border-gray-800 dark:bg-gray-900/40"
				aria-label={m.dataquery_explorer_heading()}
			>
				<SourceExplorer
					active={source}
					onSelect={selectSource}
					onInsert={insertAtCaret}
					onPreview={preview}
				/>
			</aside>
		{/if}

		<section bind:this={workspace} class="flex min-w-0 flex-1 flex-col">
			<!-- Toolbar -->
			<div
				class="flex h-12 flex-shrink-0 items-center gap-2 border-b border-gray-200 px-3 dark:border-gray-800"
			>
				<button
					type="button"
					onclick={toggleExplorer}
					class="hidden h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-900 md:inline-flex dark:hover:bg-gray-800 dark:hover:text-gray-100"
					title="{m.dataquery_workspace_toggle_explorer()} ({explorerShortcut})"
					aria-label={m.dataquery_workspace_toggle_explorer()}
					aria-pressed={explorerOpen}
				>
					<IconifyIcon icon="material-symbols:side-navigation" class="h-[18px] w-[18px]" />
				</button>
				<SourcePicker bind:value={source} onchange={loadStarters} />
				<span
					class="hidden items-center gap-1 rounded-full border border-gray-200 px-2 py-0.5 text-[11px] font-medium text-gray-500 xl:inline-flex dark:border-gray-700 dark:text-gray-400"
					title={m.dataquery_workspace_read_only_hint()}
				>
					<IconifyIcon icon="material-symbols:lock-outline" class="h-3 w-3" />
					{m.dataquery_console_read_only()}
				</span>

				<div class="ml-auto flex items-center gap-1">
					{@render toolbarButton(
						m.dataquery_tab_history(),
						'material-symbols:history',
						() => (drawer = drawer === 'history' ? null : 'history'),
						drawer === 'history'
					)}
					<label
						class="ml-1 hidden items-center gap-1.5 text-xs text-gray-500 sm:flex dark:text-gray-400"
					>
						{m.dataquery_console_row_limit()}
						<select
							bind:value={maxRows}
							class="h-8 rounded-md border-gray-200 py-0 pl-2 pr-7 text-xs text-gray-700 focus:border-earthy-terracotta-500 focus:ring-earthy-terracotta-500 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-200"
						>
							{#each ROW_LIMITS as limit (limit)}
								<option value={limit}>{formatNumber(limit)}</option>
							{/each}
						</select>
					</label>
					<button
						type="button"
						onclick={run}
						disabled={!canRun}
						title="{m.dataquery_console_run()} ({runShortcut})"
						class="ml-1 inline-flex h-8 items-center gap-1.5 rounded-md bg-earthy-terracotta-600 pl-2.5 pr-2 text-sm font-semibold text-white transition hover:bg-earthy-terracotta-500 disabled:cursor-not-allowed disabled:opacity-40"
					>
						{#if running}
							<IconifyIcon icon="material-symbols:progress-activity" class="h-4 w-4 animate-spin" />
							{m.dataquery_console_running()}
						{:else}
							<IconifyIcon icon="material-symbols:play-arrow-rounded" class="h-5 w-5" />
							{m.dataquery_console_run()}
							<kbd class="ml-1 rounded bg-white/20 px-1 font-sans text-[10px] font-medium leading-4"
								>{runShortcut}</kbd
							>
						{/if}
					</button>
				</div>
			</div>

			<!-- Editor -->
			<SqlEditor bind:value={statement} height={editorHeight} bind:editorEl={editor} onRun={run} />

			<!-- Splitter -->
			<button
				type="button"
				aria-label={m.dataquery_workspace_resize()}
				onpointerdown={startResize}
				onkeydown={nudgeResize}
				class="group relative h-2 w-full flex-shrink-0 cursor-row-resize border-y border-gray-200 bg-gray-50 focus:outline-none focus-visible:bg-earthy-terracotta-100 dark:border-gray-800 dark:bg-gray-900"
			>
				<span
					class="absolute left-1/2 top-1/2 h-0.5 w-8 -translate-x-1/2 -translate-y-1/2 rounded-full bg-gray-300 group-hover:bg-earthy-terracotta-400 dark:bg-gray-700"
				></span>
			</button>

			<!-- Results -->
			<div class="flex min-h-0 flex-1 flex-col">
				<div
					class="flex h-10 flex-shrink-0 items-center gap-1 border-b border-gray-200 px-3 dark:border-gray-800"
					role="tablist"
				>
					{#each [{ id: 'results', label: m.dataquery_workspace_tab_results(), count: result ? result.row_count : null }, { id: 'context', label: m.dataquery_workspace_tab_context(), count: result ? contextCount : null }, { id: 'history', label: m.dataquery_workspace_tab_history(), count: null }] as tab (tab.id)}
						<button
							type="button"
							role="tab"
							aria-selected={resultsTab === tab.id}
							onclick={() => (resultsTab = tab.id as ResultsTab)}
							class="relative inline-flex h-10 items-center gap-1.5 px-2.5 text-sm font-medium transition {resultsTab ===
							tab.id
								? 'text-gray-900 dark:text-gray-100'
								: 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'}"
						>
							{tab.label}
							{#if tab.count !== null}
								<span
									class="rounded-full bg-gray-100 px-1.5 text-[11px] tabular-nums text-gray-600 dark:bg-gray-800 dark:text-gray-300"
									>{formatNumber(tab.count)}</span
								>
							{/if}
							{#if resultsTab === tab.id}
								<span
									class="absolute inset-x-2 bottom-0 h-0.5 rounded-full bg-earthy-terracotta-500"
								></span>
							{/if}
						</button>
					{/each}

					<div class="ml-auto flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
						{#if running}
							<span class="inline-flex items-center gap-1.5">
								<IconifyIcon
									icon="material-symbols:progress-activity"
									class="h-3.5 w-3.5 animate-spin"
								/>
								{m.dataquery_console_elapsed({ ms: formatNumber(liveMs) })}
							</span>
						{:else if result}
							{#if result.truncated}
								<span
									class="rounded-full bg-amber-100 px-2 py-0.5 font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-300"
									>{m.dataquery_console_truncated()}</span
								>
							{/if}
							<span class="hidden sm:inline"
								>{m.dataquery_console_elapsed({ ms: formatNumber(elapsedMs) })}</span
							>
							<button
								type="button"
								onclick={() => (detailId = result?.query_id ?? null)}
								class="inline-flex items-center gap-1 hover:text-earthy-terracotta-700 dark:hover:text-earthy-terracotta-400"
								title={m.dataquery_workspace_audit_hint()}
							>
								<IconifyIcon icon="material-symbols:receipt-long-outline" class="h-4 w-4" />
								{m.dataquery_console_recorded({ id: result.query_id.slice(0, 8) })}
							</button>
							{#if result.columns.length > 0}
								<span class="h-4 w-px bg-gray-200 dark:bg-gray-700"></span>
								<div
									class="inline-flex rounded-md border border-gray-200 bg-white p-0.5 dark:border-gray-700 dark:bg-gray-800"
									role="group"
									aria-label={m.dataquery_workspace_view()}
								>
									{#each [{ id: 'table', label: m.dataquery_workspace_view_table() }, { id: 'csv', label: m.dataquery_workspace_export_csv() }, { id: 'json', label: m.dataquery_workspace_export_json() }] as v (v.id)}
										<button
											type="button"
											onclick={() => {
												resultView = v.id as 'table' | 'csv' | 'json';
												resultsTab = 'results';
											}}
											aria-pressed={resultView === v.id}
											class="rounded px-2 py-0.5 text-xs font-medium transition {resultView === v.id
												? 'bg-gray-900 text-white dark:bg-gray-100 dark:text-gray-900'
												: 'text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'}"
											>{v.label}</button
										>
									{/each}
								</div>
							{/if}
						{/if}
					</div>
				</div>

				<div class="min-h-0 flex-1 overflow-auto">
					{#if resultsTab === 'history'}
						<QueryResultsHistory
							{recent}
							onOpen={openRecord}
							onViewAll={() => (drawer = 'history')}
						/>
					{:else if running && !result}
						<div class="space-y-2 p-4">
							{#each Array(6) as _, i (i)}
								<div class="h-7 animate-pulse rounded bg-gray-100 dark:bg-gray-800"></div>
							{/each}
						</div>
					{:else if denied}
						{@const table = deniedTable(denied)}
						<div
							class="result-in m-4 flex gap-4 rounded-xl border border-red-200 bg-red-50 p-5 dark:border-red-900/60 dark:bg-red-950/30"
						>
							<div
								class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-red-100 dark:bg-red-900/50"
							>
								<IconifyIcon
									icon="material-symbols:lock-outline"
									class="h-5 w-5 text-red-600 dark:text-red-300"
								/>
							</div>
							<div class="min-w-0">
								<h3 class="font-semibold text-red-900 dark:text-red-200">
									{m.dataquery_console_denied_title()}
								</h3>
								<p class="mt-1 text-sm text-red-800 dark:text-red-300">
									{#if table}
										{m.dataquery_console_denied_table()}
										<code
											class="rounded bg-red-100 px-1.5 py-0.5 font-mono text-xs dark:bg-red-900/50"
											>{table}</code
										>
									{:else}
										{denied}
									{/if}
								</p>
								<p class="mt-2 text-sm text-red-700/80 dark:text-red-300/80">
									{m.dataquery_console_denied_help()}
								</p>
							</div>
						</div>
					{:else if failure}
						<div
							class="result-in m-4 flex gap-4 rounded-xl border border-amber-200 bg-amber-50 p-5 dark:border-amber-900/60 dark:bg-amber-950/30"
						>
							<IconifyIcon
								icon="material-symbols:error-outline"
								class="h-6 w-6 flex-shrink-0 text-amber-600 dark:text-amber-300"
							/>
							<div class="min-w-0">
								<h3 class="font-semibold text-amber-900 dark:text-amber-200">
									{m.dataquery_console_failed_title()}
								</h3>
								<p class="mt-1 break-words font-mono text-sm text-amber-800 dark:text-amber-300">
									{failure}
								</p>
							</div>
						</div>
					{:else if !result}
						{@render emptyState()}
					{:else if resultsTab === 'context'}
						<QueryResultContext context={result.context ?? []} />
					{:else}
						<QueryResultsTable {result} {resultView} {source} />
					{/if}
				</div>
			</div>
		</section>
	</div>
{/if}

{#if drawer}
	<Blade title={m.dataquery_tab_history()} onClose={() => (drawer = null)}>
		<QueryLog mine heading={false} />
	</Blade>
{/if}

{#if detailId}
	<QueryDetail queryId={detailId} onClose={() => (detailId = null)} onOpen={openStatement} />
{/if}

<style>
	.result-in {
		animation: result-in 0.3s cubic-bezier(0.2, 0.8, 0.2, 1) both;
	}

	@keyframes result-in {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.result-in {
			animation: none;
		}
	}
</style>

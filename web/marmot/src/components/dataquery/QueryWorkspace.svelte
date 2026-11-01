<script lang="ts">
	import { onMount } from 'svelte';
	import '@marmotdata/elements/split-pane.js';
	import '@marmotdata/elements/toolbar.js';
	import '@marmotdata/elements/sql-editor.js';
	import '@marmotdata/elements/results-grid.js';
	import '@marmotdata/elements/query-history.js';
	import '@marmotdata/elements/progress.js';
	import '@marmotdata/elements/visually-hidden.js';
	import type { MmSqlEditor, SqlError, SqlSchema } from '@marmotdata/elements/classes/sql-editor';
	import type { DataTableColumn } from '@marmotdata/elements/classes/data-table';
	import type { QueryHistoryItem } from '@marmotdata/elements/classes/query-history';
	import { m } from '$lib/paraglide/messages';
	import { ApiError, getErrorMessage, toasts } from '$lib/stores/toast';
	import { formatDateTime, formatNumber } from '$lib/utils/format';
	import { html } from 'lit';
	import '@marmotdata/elements/tag.js';
	import { holdsAtRoot, loadMyAccess, myAccess } from '$lib/iam/myAccess';
	import {
		cancelQuery,
		listQueryHistory,
		listQuerySources,
		listQuerySourceTables,
		QueryRefusedError,
		requestQueryAccess,
		runQuery
	} from '$lib/dataquery/api';
	import {
		assetRouteParams,
		durationMs,
		isRealTime,
		outcomeReason,
		qualifiedName
	} from '$lib/dataquery/format';
	import type {
		QueryAssetContext,
		QueryRecord,
		QueryRefusal,
		QueryResult,
		QuerySource,
		QuerySourceTable
	} from '$lib/dataquery/types';
	import { resolve } from '$app/paths';
	import QueryDetail from './QueryDetail.svelte';
	import QueryEmptyState from './QueryEmptyState.svelte';
	import SavedQueries from './SavedQueries.svelte';
	import SourceExplorer from './SourceExplorer.svelte';

	let {
		initialSource = '',
		initialStatement = '',
		autoRun = false
	}: {
		initialSource?: string;
		initialStatement?: string;
		autoRun?: boolean;
	} = $props();

	type Tab = 'results' | 'tables' | 'history';

	// The server defaults to 1,000 rows and caps every query at 10,000, so there is no "No limit"
	const ROW_LIMITS = [10, 100, 1000, 10000];
	const SOURCE_PAGE = 200;
	const HISTORY_PAGE = 25;
	const EXPLORER_KEY = 'marmot.query.explorer';
	const PROGRESS_POLL_MS = 1000;

	let sources = $state<QuerySource[]>([]);
	let sourcesTotal = $state(0);
	let sourcesLoading = $state(true);
	let sourcesLoaded = $state(false);
	let sourcesError = $state('');

	let source = $state('');
	let statement = $state('');
	let maxRows = $state(1000);
	let running = $state(false);
	let result = $state<QueryResult | null>(null);
	let denied = $state('');
	let refusal = $state<QueryRefusal | null>(null);
	let accessRequested = $state(false);
	let requestingAccess = $state(false);
	let cancelled = $state(false);
	// The running query's audit record, found by polling the caller's running queries, so Cancel can name it and progress can count its rows
	let runningId = $state('');
	let rowsReceived = $state<number | null>(null);
	let runController: AbortController | null = null;
	let progressTimer: ReturnType<typeof setInterval> | undefined;
	let failure = $state('');
	let unreachable = $state('');
	let sqlError = $state<SqlError | null>(null);
	let elapsedMs = $state(0);
	// The audit record of the last run, found from history when the run itself returned an error
	let recordId = $state('');
	let lastRun = '';
	let detailId = $state<string | null>(null);
	let editor = $state<MmSqlEditor>();

	let explorerOpen = $state(true);
	let wide = $state(true);
	let tab = $state<Tab>('results');
	let outputHeight = $state(0);

	let recent = $state<QueryRecord[]>([]);
	let recentTotal = $state(0);
	let starters = $state<QuerySourceTable[]>([]);
	let startersRequest = 0;
	// A statement the workspace filled in itself, which it may replace when the source changes; anything typed is left alone
	let autoFilled = '';

	// Completion metadata per source, filled as the explorer loads tables and columns
	let schemas = $state<Record<string, SqlSchema>>({});

	let current = $derived(sources.find((s) => s.name === source));
	let editorSchema = $derived(schemas[source] ?? {});
	let defaultSchema = $derived(mostCommonPrefix(Object.keys(editorSchema)));
	let canViewAudit = $derived(holdsAtRoot($myAccess, 'access:view'));
	let canRun = $derived(!!source && statement.trim().length > 0 && !running);
	let context = $derived(result?.context ?? []);
	let gridHeight = $derived(Math.max(200, outputHeight - 128));
	let providers = $derived(new Map(sources.map((s) => [s.name, s.plugin_id])));
	let history = $derived<QueryHistoryItem[]>(
		recent.map((q) => ({
			id: q.id,
			sql: q.statement,
			status:
				q.decision === 'denied'
					? 'denied'
					: q.status === 'completed'
						? 'succeeded'
						: q.status === 'failed'
							? 'failed'
							: q.status === 'cancelled'
								? 'cancelled'
								: 'running',
			source: { name: q.source_name, provider: providers.get(q.source_name) ?? '' },
			rows: q.rows_returned ?? null,
			durationMs: durationMs(q) ?? null,
			startedAt: q.started_at,
			reason: outcomeReason(q) || undefined
		}))
	);

	const tableColumns: DataTableColumn<QueryAssetContext>[] = [
		{
			key: 'name',
			header: m.dataquery_tables_col_table(),
			width: 260,
			value: (r) => r.name || r.mrn,
			href: (r) => {
				const route = r.asset_id ? assetRouteParams(r.mrn) : null;
				return route ? resolve('/discover/[type]/[service]/[name]', route) : undefined;
			}
		},
		{
			key: 'owners',
			header: m.dataquery_tables_col_owner(),
			width: 190,
			value: (r) => (r.owners ?? []).join(', '),
			empty: m.dataquery_tables_no_owner()
		},
		{ key: 'row_count', header: m.dataquery_tables_col_rows(), type: 'number', width: 110 },
		{
			key: 'columns_read',
			header: m.dataquery_tables_col_columns_read(),
			type: 'number',
			width: 120
		},
		{
			key: 'personal_data',
			header: m.dataquery_tables_col_personal_data(),
			minWidth: 200,
			value: (r) => (r.personal_data?.length ? r.personal_data : null),
			empty: m.dataquery_tables_none(),
			// Columns the query returned come first; the rest are named as not read, so the cell says what left the source
			render: (r) => {
				const read = (r.personal_data ?? []).filter((c) => c.read).map((c) => c.name);
				const unread = (r.personal_data ?? []).filter((c) => !c.read).map((c) => c.name);
				const parts = [
					read.join(', '),
					unread.length ? m.dataquery_tables_not_read({ columns: unread.join(', ') }) : ''
				]
					.filter(Boolean)
					.join(', ');
				return html`<span class="mm-cluster" style="flex-wrap:nowrap"
					><mm-tag variant="pii"></mm-tag
					><mm-text variant="dense" tone="secondary">${parts}</mm-text></span
				>`;
			}
		},
		{
			key: 'certified',
			header: m.dataquery_tables_col_certified(),
			width: 120,
			render: (r) =>
				r.certified
					? html`<mm-tag variant="certified"></mm-tag>`
					: html`<mm-text variant="dense" tone="secondary"
							>${m.dataquery_tables_not_certified()}</mm-text
						>`
		},
		{
			key: 'last_sync_at',
			header: m.dataquery_tables_col_refreshed(),
			type: 'date',
			width: 170,
			value: (r) => (isRealTime(r.last_sync_at) ? r.last_sync_at : null),
			empty: m.dataquery_query_never_synced()
		}
	];

	function mostCommonPrefix(names: string[]): string {
		const counts: Record<string, number> = {};
		for (const name of names) {
			const prefix = name.split('.').slice(0, -1).join('.');
			if (prefix) counts[prefix] = (counts[prefix] ?? 0) + 1;
		}
		return Object.entries(counts).sort((a, b) => b[1] - a[1])[0]?.[0] ?? '';
	}

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

	async function loadSources(append = false) {
		sourcesLoading = true;
		sourcesError = '';
		try {
			const data = await listQuerySources({
				limit: SOURCE_PAGE,
				offset: append ? sources.length : 0
			});
			sources = append ? [...sources, ...data.sources] : data.sources;
			sourcesTotal = data.total;
		} catch (e) {
			sourcesError = getErrorMessage(e);
		} finally {
			sourcesLoading = false;
			sourcesLoaded = true;
		}
	}

	async function loadRecent(limit = Math.max(recent.length, HISTORY_PAGE)) {
		try {
			const data = await listQueryHistory({ mine: true, limit });
			recent = data.queries;
			recentTotal = data.total;
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
			addTables(name, tables);
			starters = tables.filter((t) => t.sample_statement);
			const preview = starters[0]?.sample_statement ?? '';
			if (preview && (statement.trim() === '' || statement === autoFilled)) setStatement(preview);
		} catch {
			// Starters are a convenience; the workspace works without them
		}
	}

	function addTables(name: string, tables: QuerySourceTable[]) {
		const next = { ...schemas[name] };
		for (const t of tables) next[qualifiedName(t)] ??= [];
		schemas = { ...schemas, [name]: next };
	}

	function addColumns(name: string, table: QuerySourceTable, columns: { name: string }[]) {
		schemas = {
			...schemas,
			[name]: { ...schemas[name], [qualifiedName(table)]: columns.map((c) => c.name) }
		};
	}

	function selectSource(name: string) {
		if (name === source) return;
		source = name;
		loadStarters(name);
	}

	function setStatement(text: string) {
		statement = text;
		autoFilled = text;
		sqlError = null;
	}

	function clearOutcome() {
		result = null;
		denied = '';
		refusal = null;
		accessRequested = false;
		cancelled = false;
		failure = '';
		unreachable = '';
		sqlError = null;
		recordId = '';
	}

	// Explorer picks drop the name at the caret, the way schema trees in desktop SQL tools do
	function insertAtCaret(text: string) {
		const view = editor?.view;
		if (!view) {
			statement += text;
			return;
		}
		view.dispatch(view.state.replaceSelection(text));
		view.focus();
	}

	// A sample query only fills the editor; nothing runs until Run query or the shortcut
	function openStatement(name: string, text: string, andRun = false) {
		selectSource(name);
		setStatement(text);
		tab = 'results';
		editor?.focus();
		if (andRun) run(text);
	}

	// The error names the failing position when the source reports one, so the editor can underline it
	function errorPosition(message: string, text: string): SqlError | null {
		const lineCol = message.match(/line (\d+),? (?:column|col) (\d+)/i);
		if (lineCol) return { line: +lineCol[1], column: +lineCol[2], message };
		const offset = message.match(/(?:at character|position) (\d+)/i);
		if (!offset) return null;
		const before = text.slice(0, Math.max(0, +offset[1] - 1)).split('\n');
		return { line: before.length, column: before[before.length - 1].length + 1, message };
	}

	// The API prefixes refusals and failures with the error kind, which the callout heading already says
	function stripKind(message: string): string {
		return message.replace(/^(query denied by policy|query failed|invalid input):\s*/i, '');
	}

	// Denial reasons name tables by MRN; the last segment is the name people know
	function readableReason(message: string): string {
		return stripKind(message).replace(/mrn:\/\/\S+/g, (mrn) => mrn.split('/').pop() ?? mrn);
	}

	async function run(text = statement) {
		if (!source || text.trim().length === 0 || running) return;
		running = true;
		clearOutcome();
		tab = 'results';
		lastRun = text;
		const started = performance.now();
		const controller = new AbortController();
		runController = controller;
		watchProgress(source, text);
		try {
			result = await runQuery({ source, statement: text, max_rows: maxRows }, controller.signal);
			recordId = result.query_id;
		} catch (e) {
			const message = getErrorMessage(e);
			if (e instanceof QueryRefusedError) {
				refusal = e.refusal;
				denied = readableReason(e.refusal.reason || message);
				recordId = e.refusal.query_id ?? '';
			} else if (controller.signal.aborted || (e instanceof ApiError && e.status === 409))
				cancelled = true;
			else if (e instanceof ApiError && e.status === 403) denied = readableReason(message);
			else if (e instanceof ApiError && e.status === 400) {
				failure = stripKind(message);
				sqlError = errorPosition(failure, text);
			} else unreachable = message;
		} finally {
			stopProgress();
			runController = null;
			elapsedMs = Math.round(performance.now() - started);
			running = false;
		}
		if (cancelled && runningId) recordId = runningId;
		runningId = '';
		// The cancelled record is only marked once the server notices the hang-up, so history waits a moment for it
		if (cancelled) await new Promise((r) => setTimeout(r, 300));
		await loadRecent();
		if (!result && !recordId && recent[0]?.statement === lastRun) recordId = recent[0].id;
	}

	// The run request only answers when the query ends, so its record is found among the caller's running queries to count rows and to cancel by id
	function watchProgress(name: string, text: string) {
		stopProgress();
		runningId = '';
		rowsReceived = null;
		progressTimer = setInterval(async () => {
			try {
				const data = await listQueryHistory({
					mine: true,
					status: 'running',
					source: name,
					limit: 5
				});
				if (!running) return;
				const q = data.queries.find((r) => r.statement === text);
				if (!q) return;
				runningId = q.id;
				if (q.rows_returned != null) rowsReceived = q.rows_returned;
			} catch {
				// Progress is a nicety; the run itself reports how it ended
			}
		}, PROGRESS_POLL_MS);
	}

	function stopProgress() {
		clearInterval(progressTimer);
		progressTimer = undefined;
	}

	// Cancelling asks the replica running the query to stop it and hangs up, which stops it wherever it runs
	async function cancel() {
		const id = runningId;
		if (id) await cancelQuery(id).catch(() => {});
		runController?.abort();
	}

	async function requestAccess() {
		if (!refusal) return;
		requestingAccess = true;
		try {
			const req = await requestQueryAccess(
				refusal.query_id ? { query_id: refusal.query_id } : { source: refusal.source }
			);
			accessRequested = true;
			if (req.repeat) toasts.info(m.dataquery_access_request_repeat());
			else if (req.notified > 0) toasts.success(m.dataquery_access_request_sent());
			else toasts.info(m.dataquery_access_request_recorded());
		} catch (e) {
			toasts.error(getErrorMessage(e));
		} finally {
			requestingAccess = false;
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

	onMount(() => {
		explorerOpen = readStored(EXPLORER_KEY) !== '0';
		const mq = window.matchMedia('(min-width: 768px)');
		wide = mq.matches;
		const onChange = () => (wide = mq.matches);
		mq.addEventListener('change', onChange);
		if (initialStatement) setStatement(initialStatement);
		loadMyAccess();
		loadSources();
		loadRecent().then(() => {
			const name = initialSource || recent.find((q) => q.decision === 'allowed')?.source_name || '';
			if (name) selectSource(name);
			if (autoRun && initialStatement) run(initialStatement);
		});
		return () => {
			mq.removeEventListener('change', onChange);
			stopProgress();
		};
	});
</script>

<svelte:window onkeydown={onWindowKeydown} />

{#snippet callout(heading: string, body: string, next: string, retry: boolean, urgent: boolean)}
	<div class="pad mm-stack">
		<mm-callout tone="danger" {heading} {urgent}>
			<span class="reason">{body}</span>
			{next}
			{#if retry}
				<mm-button
					slot="actions"
					size="sm"
					variant="secondary"
					icon="refresh"
					onclick={() => run(lastRun)}>{m.dataquery_workspace_run_again()}</mm-button
				>
			{/if}
			{#if recordId}
				<mm-button
					slot="actions"
					size="sm"
					variant="ghost"
					icon="receipt-long"
					onclick={() => (detailId = recordId)}>{m.dataquery_workspace_open_details()}</mm-button
				>
			{/if}
		</mm-callout>
	</div>
{/snippet}

{#snippet resultsPanel()}
	{#if running}
		<div class="pad">
			{#if rowsReceived !== null && rowsReceived > 0}
				<mm-progress
					label={m.dataquery_workspace_rows_received()}
					value={rowsReceived}
					max={maxRows}
					unit={m.dataquery_workspace_rows_unit()}
					class="progress"
				></mm-progress>
			{:else}
				<mm-progress label={m.dataquery_workspace_running_label()} indeterminate class="progress"
				></mm-progress>
			{/if}
		</div>
	{:else if refusal}
		<div class="pad mm-stack">
			<mm-callout
				tone="danger"
				urgent
				heading={refusal.rule === 'permission' && refusal.permission
					? m.dataquery_workspace_denied_permission_heading({ permission: refusal.permission })
					: m.dataquery_workspace_denied_heading()}
			>
				<span class="reason">{denied}</span>
				{refusal.needed
					? m.dataquery_workspace_denied_ask({ needed: refusal.needed })
					: m.dataquery_workspace_denied_next()}
				<mm-button
					slot="actions"
					size="sm"
					variant="secondary"
					icon="vpn-key"
					loading={requestingAccess}
					disabled={accessRequested}
					onclick={requestAccess}
					>{accessRequested
						? m.dataquery_workspace_access_requested()
						: m.dataquery_workspace_request_access()}</mm-button
				>
				{#if refusal.query_id && canViewAudit}
					<mm-button
						slot="actions"
						size="sm"
						variant="ghost"
						icon="policy"
						href={`${resolve('/query-audit')}?id=${encodeURIComponent(refusal.query_id)}`}
						>{m.dataquery_workspace_view_in_audit()}</mm-button
					>
				{:else if recordId}
					<mm-button
						slot="actions"
						size="sm"
						variant="ghost"
						icon="receipt-long"
						onclick={() => (detailId = recordId)}>{m.dataquery_workspace_open_details()}</mm-button
					>
				{/if}
			</mm-callout>
			<!-- The callout already says why; the details add only what it does not -->
			<mm-description-list layout="inline-grid" columns={3}>
				{#if refusal.policy}
					<mm-description term={m.dataquery_refusal_policy()}>{refusal.policy}</mm-description>
				{/if}
				{#if refusal.permission}
					<mm-description term={m.dataquery_refusal_permission()}
						>{refusal.permission}</mm-description
					>
				{/if}
				<mm-description term={m.dataquery_refusal_roles()}
					>{refusal.roles.length > 0
						? refusal.roles.join(', ')
						: m.dataquery_refusal_no_roles()}</mm-description
				>
				{#if refusal.query_id}
					<mm-description term={m.dataquery_detail_query_id()} mono
						>{refusal.query_id}</mm-description
					>
				{/if}
				<mm-description term={m.dataquery_refusal_decided()}
					>{formatDateTime(refusal.decided_at)}</mm-description
				>
			</mm-description-list>
		</div>
	{:else if cancelled}
		<div class="pad">
			<mm-callout tone="info" heading={m.dataquery_workspace_cancelled_heading()}>
				{m.dataquery_workspace_cancelled_body()}
				<mm-button
					slot="actions"
					size="sm"
					variant="secondary"
					icon="refresh"
					onclick={() => run(lastRun)}>{m.dataquery_workspace_run_again()}</mm-button
				>
			</mm-callout>
		</div>
	{:else if denied}
		{@render callout(
			m.dataquery_workspace_denied_heading(),
			denied,
			m.dataquery_workspace_denied_next(),
			false,
			true
		)}
	{:else if failure}
		{@render callout(
			m.dataquery_workspace_failed_heading({ source }),
			failure,
			m.dataquery_workspace_failed_next(),
			false,
			false
		)}
	{:else if unreachable}
		{@render callout(
			m.dataquery_workspace_error_heading({ source }),
			unreachable,
			m.dataquery_workspace_error_next(),
			true,
			false
		)}
	{:else if result && result.columns.length === 0}
		<div class="pad">
			<mm-empty-state compact heading={m.dataquery_console_no_columns()}></mm-empty-state>
		</div>
	{:else if result}
		<mm-results-grid
			label={m.dataquery_workspace_tab_results()}
			filename="{source}-{result.query_id.slice(0, 8)}"
			columns={result.columns}
			rows={result.rows}
			duration={elapsedMs}
			truncated={result.truncated}
			style="--mm-results-grid-height: {gridHeight}px"
		>
			<mm-button
				slot="actions"
				size="sm"
				variant="ghost"
				icon="receipt-long"
				onclick={() => (detailId = result?.query_id ?? null)}
				>{m.dataquery_workspace_open_details()}</mm-button
			>
		</mm-results-grid>
	{:else}
		<div class="pad mm-stack">
			<mm-empty-state
				compact
				heading={m.dataquery_workspace_results_empty_heading()}
				description={m.dataquery_workspace_results_empty_body()}
			></mm-empty-state>
			{#if starters.length > 0}
				<section class="mm-stack starters">
					<mm-heading level={2} variant="panel-title"
						>{m.dataquery_workspace_starters({ source })}</mm-heading
					>
					<mm-list dense aria-label={m.dataquery_workspace_starters({ source })}>
						{#each starters as table (table.mrn)}
							<mm-list-item
								interactive
								secondary={table.description || m.dataquery_workspace_preview_rows()}
								onmm-activate={() => openStatement(source, table.sample_statement ?? '')}
							>
								<mm-icon slot="start" name={table.type === 'View' ? 'visibility' : 'table'}
								></mm-icon>
								{table.name}
							</mm-list-item>
						{/each}
					</mm-list>
				</section>
			{/if}
		</div>
	{/if}
{/snippet}

{#snippet workspace()}
	<div class="main">
		<mm-visually-hidden><mm-heading level={1}>{m.nav_query()}</mm-heading></mm-visually-hidden>
		<mm-toolbar label={m.dataquery_workspace_toolbar_label()} class="toolbar">
			{#if wide}
				<mm-button
					variant="ghost"
					icon={explorerOpen ? 'left-panel-close' : 'left-panel-open'}
					label={m.dataquery_workspace_toggle_explorer()}
					pressed={explorerOpen ? 'true' : 'false'}
					onclick={toggleExplorer}
				></mm-button>
			{/if}
			<mm-select
				label={m.dataquery_query_col_source()}
				hide-label
				value={source}
				onmm-change={(e: CustomEvent<{ value: string }>) => selectSource(e.detail.value)}
				class="source-select"
			>
				{#if source && !current}
					<mm-option value={source}>{source}</mm-option>
				{/if}
				{#each sources as s (s.id)}
					<mm-option value={s.name} provider={s.plugin_id}>{s.name}</mm-option>
				{/each}
			</mm-select>
			<mm-status-pill status="neutral" icon="lock" title={m.dataquery_workspace_read_only_hint()}
				>{m.dataquery_console_read_only()}</mm-status-pill
			>
			<mm-toolbar-separator></mm-toolbar-separator>
			<mm-select
				label={m.dataquery_console_row_limit()}
				hide-label
				value={String(maxRows)}
				onmm-change={(e: CustomEvent<{ value: string }>) => (maxRows = Number(e.detail.value))}
				class="limit-select"
			>
				{#each ROW_LIMITS as limit (limit)}
					<mm-option value={String(limit)}
						>{m.dataquery_console_rows({ count: formatNumber(limit) })}</mm-option
					>
				{/each}
			</mm-select>
			<span slot="end" class="mm-cluster">
				{#if running}
					<mm-button variant="secondary" icon="stop" onclick={cancel}
						>{m.dataquery_workspace_cancel()}</mm-button
					>
					<mm-button variant="primary" loading>{m.dataquery_workspace_running()}</mm-button>
				{:else}
					<mm-button variant="ghost" icon="data-object" onclick={() => editor?.format()}
						>{m.dataquery_workspace_format()}</mm-button
					>
					<mm-kbd keys="mod+enter" label={m.dataquery_workspace_run_shortcut()}></mm-kbd>
					<mm-button
						variant="primary"
						icon="play-arrow"
						disabled={!canRun}
						onclick={() => editor?.run()}>{m.dataquery_workspace_run()}</mm-button
					>
				{/if}
			</span>
		</mm-toolbar>
		<mm-sql-editor
			bind:this={editor}
			label={m.dataquery_console_editor_label()}
			placeholder={m.dataquery_console_placeholder()}
			dialect={current?.plugin_id || 'postgresql'}
			default-schema={defaultSchema}
			hide-shortcut
			schema={editorSchema}
			value={statement}
			error={sqlError}
			onmm-input={(e: CustomEvent<{ value: string }>) => {
				statement = e.detail.value;
				sqlError = null;
			}}
			onmm-run={(e: CustomEvent<{ sql: string }>) => run(e.detail.sql)}
			style="--mm-code-editor-height: 300px"
		></mm-sql-editor>
		<div class="output" bind:clientHeight={outputHeight}>
			<mm-tabs
				label={m.dataquery_workspace_output_label()}
				value={tab}
				onmm-tab-change={(e: CustomEvent<{ value: string }>) => (tab = e.detail.value as Tab)}
			>
				<mm-tab value="results" icon="table-rows">{m.dataquery_workspace_tab_results()}</mm-tab>
				<mm-tab value="tables" icon="table" count={result ? context.length : undefined}
					>{m.dataquery_workspace_tab_context()}</mm-tab
				>
				<mm-tab value="history" icon="history" count={recentTotal || undefined}
					>{m.dataquery_workspace_tab_history()}</mm-tab
				>
				<mm-tab-panel value="results">{@render resultsPanel()}</mm-tab-panel>
				<mm-tab-panel value="tables">
					<div class="pad">
						<mm-data-table
							label={m.dataquery_workspace_tab_context()}
							density="compact"
							columns={tableColumns}
							rows={context}
							empty-heading={m.dataquery_workspace_tables_empty_heading()}
							empty-text={result
								? m.dataquery_console_context_empty()
								: m.dataquery_workspace_tables_empty_body()}
						></mm-data-table>
					</div>
				</mm-tab-panel>
				<mm-tab-panel value="history">
					<div class="pad mm-stack">
						<SavedQueries {source} {statement} {providers} onOpen={openStatement} />
						<mm-query-history
							label={m.dataquery_workspace_recent()}
							queries={history}
							onmm-open-query={(e: CustomEvent<{ query: QueryHistoryItem }>) => {
								const q = recent.find((r) => r.id === e.detail.query.id);
								if (q) openStatement(q.source_name, q.statement);
							}}
							onmm-run={(e: CustomEvent<{ query: QueryHistoryItem }>) => {
								const q = recent.find((r) => r.id === e.detail.query.id);
								if (q) openStatement(q.source_name, q.statement, true);
							}}
						></mm-query-history>
						{#if recent.length < recentTotal}
							<div>
								<mm-button
									size="sm"
									variant="secondary"
									onclick={() => loadRecent(recent.length + HISTORY_PAGE)}
									>{m.dataquery_workspace_history_more()}</mm-button
								>
							</div>
						{/if}
					</div>
				</mm-tab-panel>
			</mm-tabs>
		</div>
	</div>
{/snippet}

{#if sourcesLoaded && !sourcesError && sourcesTotal === 0}
	<QueryEmptyState />
{:else if sourcesLoaded}
	{#if wide && explorerOpen}
		<mm-split-pane
			label={m.dataquery_workspace_resize_explorer()}
			default-position={336}
			min-start={240}
			storage-key="marmot.query.explorer-width"
			class="fill"
		>
			<div slot="start" class="fill">
				<SourceExplorer
					{sources}
					total={sourcesTotal}
					loading={sourcesLoading && sources.length === 0}
					error={sourcesError}
					active={source}
					onMore={() => loadSources(true)}
					onRetry={() => loadSources()}
					onSelect={selectSource}
					onInsert={insertAtCaret}
					onSample={(name, text) => openStatement(name, text)}
					onTables={addTables}
					onColumns={addColumns}
				/>
			</div>
			<div slot="end" class="fill">{@render workspace()}</div>
		</mm-split-pane>
	{:else}
		{@render workspace()}
	{/if}
{/if}

{#if detailId}
	<QueryDetail queryId={detailId} onClose={() => (detailId = null)} onOpen={openStatement} />
{/if}

<style>
	.fill {
		display: block;
		block-size: 100%;
		min-block-size: 0;
	}

	.main {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		grid-template-rows: auto auto auto minmax(0, 1fr);
		block-size: 100%;
		min-inline-size: 0;
	}

	.toolbar {
		padding: var(--space-3) var(--space-5);
	}

	.source-select {
		inline-size: 14rem;
	}

	.limit-select {
		inline-size: 8.5rem;
	}

	.output {
		min-block-size: 0;
		overflow: auto;
	}

	.pad {
		--mm-gap: var(--space-4);
		padding: var(--space-4) var(--space-5);
	}

	.progress {
		max-inline-size: 28rem;
	}

	.reason {
		display: block;
		margin-block-end: var(--space-1);
		overflow-wrap: anywhere;
	}

	.starters {
		--mm-gap: var(--space-2);
		max-inline-size: 40rem;
	}

	@media (max-width: 767px) {
		.toolbar {
			padding: var(--space-3) var(--space-4);
		}

		.source-select {
			inline-size: 10rem;
		}
	}
</style>

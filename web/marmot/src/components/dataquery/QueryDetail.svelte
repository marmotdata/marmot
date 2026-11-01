<script lang="ts">
	import '@marmotdata/elements/code-sample.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { BladeSlot } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { getErrorMessage } from '$lib/stores/toast';
	import { formatDateTime, formatNumber } from '$lib/utils/format';
	import { getQueryRecord } from '$lib/dataquery/api';
	import {
		actorKind,
		actorName,
		actorServiceAccount,
		assetRouteParams,
		clientLabel,
		decisionLabel,
		durationMs,
		formatQueryDuration,
		outcomeReason,
		outcomeReasonLabel,
		outcomeState,
		subjectParts,
		workspaceHref
	} from '$lib/dataquery/format';
	import type { QueryRecord } from '$lib/dataquery/types';

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
		href: string;
	}

	let who = $derived(query ? subjectParts(query.audit_subject) : null);
	let outcome = $derived(query ? outcomeState(query) : 'allowed');
	let duration = $derived(query ? durationMs(query) : undefined);
	let tables = $derived<TableRow[]>(
		(query?.referenced_mrns ?? []).map((mrn) => {
			const context = query?.context?.find((c) => c.mrn === mrn);
			const route = context?.asset_id ? assetRouteParams(mrn) : null;
			return {
				mrn,
				name: context?.name || mrn.split('/').pop() || mrn,
				wholeSource: mrn.startsWith('mrn://source/'),
				href: route ? resolve('/discover/[type]/[service]/[name]', route) : ''
			};
		})
	);
	// The denial reason names tables by MRN, so it is rewritten with the names people know
	let reason = $derived.by(() => {
		const text = query ? outcomeReason(query) : '';
		return tables.reduce((out, t) => out.replaceAll(t.mrn, t.name), text);
	});

	function open(run: boolean) {
		if (!query) return;
		if (onOpen) {
			onOpen(query.source_name, query.statement, run);
			onClose();
			return;
		}
		goto(resolve(workspaceHref(query.source_name, query.statement, run) as '/query'));
	}

	// The shell's close button and Escape both ask to close; the page owns whether the blade is shown
	function close(e: Event) {
		e.preventDefault();
		onClose();
	}
</script>

<BladeSlot label={m.dataquery_query_detail_heading()}>
	<mm-details-blade heading={m.dataquery_query_detail_heading()} closable onmm-close={close}>
		{#if loading}
			<mm-blade-section heading={m.dataquery_detail_at_a_glance()}>
				<mm-skeleton count={3}></mm-skeleton>
			</mm-blade-section>
		{:else if error}
			<mm-blade-section heading={m.dataquery_query_detail_heading()}>
				<mm-callout tone="danger" heading={m.dataquery_detail_load_failed()}>{error}</mm-callout>
			</mm-blade-section>
		{:else if query && who}
			<mm-blade-section heading={m.dataquery_detail_at_a_glance()}>
				<mm-description-list>
					<mm-description term={m.dataquery_query_col_who()}>
						<mm-owner-chip
							name={actorName(query)}
							kind={actorKind(query)}
							service-account={actorServiceAccount(query)}
						></mm-owner-chip>
					</mm-description>
					<mm-description term={m.dataquery_query_col_outcome()}>
						{#if outcome === 'denied'}
							<mm-status-pill state="denied">{m.dataquery_status_denied()}</mm-status-pill>
						{:else if outcome === 'failed'}
							<mm-status-pill state="failed">{m.dataquery_status_failed()}</mm-status-pill>
						{:else if outcome === 'cancelled'}
							<mm-status-pill state="cancelled">{m.dataquery_status_cancelled()}</mm-status-pill>
						{:else if outcome === 'running'}
							<mm-status-pill state="running">{m.dataquery_status_running()}</mm-status-pill>
						{:else}
							{m.dataquery_decision_allowed()}
						{/if}
					</mm-description>
					<mm-description term={m.dataquery_detail_rows_returned()}>
						{#if query.rows_returned != null}
							<span class="mm-num">{formatNumber(query.rows_returned)}</span>
							{#if query.truncated}
								<mm-status-pill status="warning" icon="warning"
									>{m.dataquery_query_truncated()}</mm-status-pill
								>
							{/if}
						{:else}
							{m.dataquery_detail_rows_none()}
						{/if}
					</mm-description>
				</mm-description-list>
			</mm-blade-section>

			{#if reason}
				<mm-blade-section
					heading={outcome === 'denied'
						? m.dataquery_detail_refused_heading()
						: m.dataquery_detail_failed_heading()}
				>
					<mm-text block variant="dense">{reason}</mm-text>
				</mm-blade-section>
			{/if}

			<mm-blade-section heading={m.dataquery_detail_what_ran()}>
				<mm-code-sample
					language="sql"
					label={m.dataquery_query_statement_heading()}
					wrap
					code={query.statement}
				></mm-code-sample>
			</mm-blade-section>

			<mm-blade-section heading={m.dataquery_detail_what_it_read()}>
				{#if tables.length === 0}
					<mm-text block variant="dense" tone="secondary"
						>{m.dataquery_query_none_recorded()}</mm-text
					>
				{:else}
					<mm-list dense aria-label={m.dataquery_detail_what_it_read()}>
						{#each tables as table (table.mrn)}
							<mm-list-item href={table.href || undefined}>
								{table.wholeSource
									? m.dataquery_detail_whole_source({ source: query.source_name })
									: table.name}
							</mm-list-item>
						{/each}
					</mm-list>
				{/if}
			</mm-blade-section>

			<mm-blade-section heading={m.dataquery_detail_more()} collapsible>
				<mm-description-list>
					<mm-description term={m.dataquery_detail_decision()}
						>{query.outcome_reason
							? outcomeReasonLabel(query.outcome_reason)
							: decisionLabel(query.decision)}</mm-description
					>
					{#if query.decision === 'denied' && query.decision_detail?.needed}
						<mm-description term={m.dataquery_refusal_needed()}
							>{query.decision_detail.needed}</mm-description
						>
					{/if}
					{#if query.decision === 'denied' && query.decision_detail?.roles}
						<mm-description term={m.dataquery_refusal_their_roles()}
							>{query.decision_detail.roles.length > 0
								? query.decision_detail.roles.join(', ')
								: m.dataquery_refusal_no_roles()}</mm-description
						>
					{/if}
					<mm-description term={m.dataquery_query_col_client()}
						>{clientLabel(query.client)}</mm-description
					>
					<mm-description term={m.dataquery_query_col_source()}>{query.source_name}</mm-description>
					<mm-description term={m.dataquery_detail_started()}
						>{formatDateTime(query.started_at)}</mm-description
					>
					<mm-description term={m.dataquery_detail_duration()}
						>{duration !== undefined ? formatQueryDuration(duration) : ''}</mm-description
					>
					<mm-description term={m.dataquery_detail_query_id()} mono>{query.id}</mm-description>
				</mm-description-list>
			</mm-blade-section>

			{#if canQuery}
				<div slot="footer" class="mm-cluster">
					<mm-button variant="primary" icon="terminal" onclick={() => open(false)}
						>{m.dataquery_detail_open_editor()}</mm-button
					>
					<mm-button icon="refresh" onclick={() => open(true)}>{m.dataquery_detail_run()}</mm-button
					>
				</div>
			{/if}
		{/if}
	</mm-details-blade>
</BladeSlot>

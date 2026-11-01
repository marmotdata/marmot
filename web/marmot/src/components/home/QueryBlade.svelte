<script lang="ts">
	import '@marmotdata/elements/code-sample.js';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { BladeSlot } from '$lib/mm';
	import { auth } from '$lib/stores/auth';
	import { m } from '$lib/paraglide/messages';
	import { formatDateTime, formatNumber } from '$lib/utils/format';
	import { getQueryRecord } from '$lib/dataquery/api';
	import { assetRouteParams, clientLabel, isRealTime, subjectParts } from '$lib/dataquery/format';
	import type { QueryRecord } from '$lib/dataquery/types';

	// A query opened from Home's activity: who and what happened first, then why, what ran and what it read
	let { queryId, onClose }: { queryId: string; onClose: () => void } = $props();

	const canQuery = auth.hasPermission('assets', 'query');
	const canViewAccess = auth.hasPermission('access', 'view');

	let query = $state<QueryRecord | null>(null);
	let loading = $state(true);
	let failed = $state(false);

	async function load(id: string) {
		loading = true;
		failed = false;
		query = null;
		try {
			query = await getQueryRecord(id);
		} catch {
			failed = true;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load(queryId);
	});

	let who = $derived(query ? subjectParts(query.audit_subject) : null);
	let tables = $derived(
		(query?.referenced_mrns ?? []).map((mrn) => {
			const context = query?.context?.find((c) => c.mrn === mrn);
			return {
				mrn,
				name: context?.name || mrn.split('/').pop() || mrn,
				provider: context?.providers?.[0] ?? '',
				route: context?.asset_id ? assetRouteParams(mrn) : null
			};
		})
	);
	// The denial reason names tables by MRN, so it is rewritten with their names
	let reason = $derived.by(() => {
		if (!query) return '';
		const text =
			query.decision === 'denied' ? (query.decision_detail?.reason ?? '') : (query.error ?? '');
		return tables.reduce((out, t) => out.replaceAll(t.mrn, t.name), text);
	});
	let durationMs = $derived(
		query?.completed_at && isRealTime(query.completed_at)
			? new Date(query.completed_at).getTime() - new Date(query.started_at).getTime()
			: null
	);

	function duration(ms: number): string {
		return ms < 1000 ? `${formatNumber(ms)} ms` : `${(ms / 1000).toFixed(ms < 10_000 ? 1 : 0)} s`;
	}

	function openInQuery() {
		if (!query) return;
		const params = `source=${encodeURIComponent(query.source_name)}&q=${encodeURIComponent(query.statement)}`;
		goto(resolve(`/query?${params}`));
	}
</script>

<BladeSlot label={m.home_query_heading()}>
	<mm-details-blade heading={m.home_query_heading()} closable onmm-close={onClose}>
		{#if loading}
			<mm-blade-section heading={m.home_query_glance()}>
				<mm-skeleton loading count={4}></mm-skeleton>
			</mm-blade-section>
		{:else if failed || !query}
			<mm-blade-section heading={m.home_query_glance()}>
				<mm-callout tone="danger" heading={m.home_query_error()}>
					<mm-button slot="actions" size="sm" icon="refresh" onclick={() => load(queryId)}
						>{m.common_retry()}</mm-button
					>
				</mm-callout>
			</mm-blade-section>
		{:else}
			<mm-blade-section heading={m.home_query_glance()}>
				<mm-description-list>
					<mm-description term={m.home_query_who()}>
						<mm-owner-chip
							name={who?.name ?? ''}
							kind={query.principal_type === 'user' ? 'person' : 'agent'}
						></mm-owner-chip>
					</mm-description>
					<mm-description term={m.home_query_outcome()}>
						{#if query.decision === 'denied'}
							<mm-status-pill state="denied"></mm-status-pill>
						{:else if query.status === 'failed'}
							<mm-status-pill state="failed"></mm-status-pill>
						{:else if query.status === 'running'}
							<mm-status-pill state="running" live></mm-status-pill>
						{:else}
							{m.home_query_allowed()}
						{/if}
					</mm-description>
					<mm-description term={m.home_query_rows()}>
						{query.rows_returned === undefined || query.rows_returned === null
							? m.home_query_no_rows()
							: formatNumber(query.rows_returned)}
					</mm-description>
				</mm-description-list>
			</mm-blade-section>

			{#if reason}
				<mm-blade-section
					heading={query.decision === 'denied'
						? m.home_query_why_refused()
						: m.home_query_why_failed()}
				>
					<mm-text variant="dense" block>{reason}</mm-text>
				</mm-blade-section>
			{/if}

			<mm-blade-section heading={m.home_query_what_ran()}>
				<mm-code-sample language="sql" wrap label={m.home_query_statement()} code={query.statement}
				></mm-code-sample>
			</mm-blade-section>

			{#if tables.length > 0}
				<mm-blade-section heading={m.home_query_what_read()} flush>
					<mm-list dense>
						{#each tables as t (t.mrn)}
							<mm-list-item
								href={t.route ? resolve('/discover/[type]/[service]/[name]', t.route) : undefined}
							>
								{#if t.provider}
									<mm-provider-logo slot="start" provider={t.provider} size="16" decorative
									></mm-provider-logo>
								{:else}
									<mm-icon slot="start" name="table" aria-hidden="true"></mm-icon>
								{/if}
								{t.name}
							</mm-list-item>
						{/each}
					</mm-list>
				</mm-blade-section>
			{/if}

			<mm-blade-section heading={m.home_query_more()} collapsible>
				<mm-description-list>
					<mm-description term={m.home_query_client()}>{clientLabel(query.client)}</mm-description>
					<mm-description term={m.home_query_source()}>{query.source_name}</mm-description>
					<mm-description term={m.home_query_started()}
						>{formatDateTime(query.started_at)}</mm-description
					>
					{#if durationMs !== null}
						<mm-description term={m.home_query_duration()}>{duration(durationMs)}</mm-description>
					{/if}
					<mm-description term={m.home_query_id()} mono>{query.id}</mm-description>
				</mm-description-list>
			</mm-blade-section>

			{#if canQuery || canViewAccess}
				<div slot="footer" class="mm-cluster" style="--mm-gap: var(--space-2)">
					{#if canViewAccess}
						<mm-button href={resolve('/query-audit')} icon="policy"
							>{m.home_query_open_audit()}</mm-button
						>
					{/if}
					{#if canQuery}
						<mm-button variant="primary" icon="terminal" onclick={openInQuery}
							>{m.home_query_open_editor()}</mm-button
						>
					{/if}
				</div>
			{/if}
		{/if}
	</mm-details-blade>
</BladeSlot>

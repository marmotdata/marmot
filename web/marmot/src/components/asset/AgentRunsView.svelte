<script lang="ts">
	import { fetchApi } from '$lib/api';
	import type { Asset } from '$lib/assets/types';
	import { m } from '$lib/paraglide/messages';
	import { getLocale } from '$lib/paraglide/runtime';
	import { formatRelativeTime } from '$lib/utils';
	import { formatMs } from './assetFormat';
	import '@marmotdata/elements/run-strip.js';
	import '@marmotdata/elements/code-sample.js';
	import type { Run as StripRun } from '@marmotdata/elements/classes/run-strip.js';

	// An agent's runs as a list on the left and the selected run read as a story on the right, as the guidelines ask for runs and traces.
	let { asset }: { asset: Asset } = $props();

	interface ToolCall {
		ordinal: number;
		tool_name: string;
		target_mrn?: string;
		started_at: string;
		duration_ms?: number;
		status: string;
		/** The call's argument in readable form, such as the SQL of a query. */
		detail?: string;
		/** What came back, such as "5 rows". */
		result?: string;
		/** Why a refused or failed call did not go through, and the rule that refused it. */
		reason?: string;
		rule?: string;
	}

	interface Run {
		id: string;
		run_id: string;
		/** What the run was asked. */
		prompt?: string;
		started_at: string;
		ended_at?: string;
		duration_ms?: number;
		status: string;
		model?: string;
		tokens_in: number;
		tokens_out: number;
		error?: string;
		tool_calls?: ToolCall[];
	}

	interface Stats {
		run_count: number;
		success_rate: number;
		median_latency_ms: number;
		p95_latency_ms: number;
		tokens_in: number;
		tokens_out: number;
	}

	let stats: Stats | null = $state(null);
	let runs: Run[] = $state([]);
	let loading = $state(true);
	let error: string | null = $state(null);
	let selectedId = $state<string | null>(null);
	let filter = $state('');
	let outcome = $state<'all' | 'succeeded' | 'failed'>('all');

	async function load() {
		loading = true;
		error = null;
		try {
			const [statsRes, runsRes] = await Promise.all([
				fetchApi(`/agents/${asset.id}/stats?period=24h`),
				fetchApi(`/agents/${asset.id}/runs?period=24h&limit=25`)
			]);
			if (!statsRes.ok || !runsRes.ok) {
				throw new Error(m.asset_agent_runs_load_error());
			}
			stats = await statsRes.json();
			const body = await runsRes.json();
			runs = body.runs ?? [];
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (asset?.id) {
			load();
		}
	});

	const failed = (r: Run) => r.status !== 'success';

	let visible = $derived(
		runs.filter((r) => {
			if (outcome === 'succeeded' && failed(r)) return false;
			if (outcome === 'failed' && !failed(r)) return false;
			const q = filter.trim().toLowerCase();
			if (!q) return true;
			return (
				r.run_id.toLowerCase().includes(q) ||
				(r.prompt ?? '').toLowerCase().includes(q) ||
				(r.tool_calls ?? []).some(
					(c) =>
						c.tool_name.toLowerCase().includes(q) || (c.target_mrn ?? '').toLowerCase().includes(q)
				)
			);
		})
	);

	let selected = $derived(visible.find((r) => r.run_id === selectedId) ?? visible[0] ?? null);

	let strip = $derived<StripRun[]>(
		[...runs].reverse().map((r) => ({
			id: r.run_id.slice(0, 8),
			status: failed(r) ? 'failed' : 'succeeded',
			startedAt: r.started_at,
			durationMs: r.duration_ms
		}))
	);

	function when(iso: string): string {
		return new Date(iso).toLocaleString(getLocale(), {
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	function words(tool: string): string {
		return tool.replace(/_/g, ' ').replace(/^\w/, (c) => c.toUpperCase());
	}

	function shortTarget(mrn?: string): string {
		if (!mrn) return '';
		return mrn.split('/').pop() ?? mrn;
	}

	// Each tool call as a plain step: what the agent did and what it touched, never the raw function-call syntax.
	function stepLabel(c: ToolCall): string {
		const target = shortTarget(c.target_mrn);
		const tool = c.tool_name.toLowerCase();
		if (/search|discover|find/.test(tool)) return m.asset_page_step_searched();
		if (/query|sql/.test(tool))
			return target ? m.asset_page_step_queried({ target }) : m.asset_page_step_ran_query();
		if (/get|read|lookup|fetch/.test(tool) && target) return m.asset_page_step_read({ target });
		return target
			? m.asset_page_step_called_on({ tool: words(c.tool_name), target })
			: words(c.tool_name);
	}

	// A step the policy turned away reads as refused, apart from one that broke.
	const refused = (c: ToolCall) => ['denied', 'refused'].includes(c.status) || !!c.rule;
	const broke = (c: ToolCall) => !refused(c) && ['error', 'failed'].includes(c.status);
	const isQuery = (c: ToolCall) => /query|sql/.test(c.tool_name.toLowerCase());

	function stepIcon(c: ToolCall): string {
		if (refused(c) || broke(c)) return 'block';
		const tool = c.tool_name.toLowerCase();
		if (/search|discover|find/.test(tool)) return 'search';
		if (/query|sql/.test(tool)) return 'terminal';
		if (/get|read|lookup|fetch/.test(tool)) return 'table';
		return 'build';
	}

	function steps(run: Run) {
		const calls = [...(run.tool_calls ?? [])].sort((a, b) => a.ordinal - b.ordinal);
		const t0 = new Date(run.started_at).getTime();
		const ends = calls.map(
			(c) => new Date(c.started_at).getTime() - t0 + Math.max(c.duration_ms ?? 0, 0)
		);
		const total = Math.max(run.duration_ms ?? 0, ...ends, 1);
		return calls.map((c) => {
			const start = Math.max(new Date(c.started_at).getTime() - t0, 0);
			return {
				c,
				left: (start / total) * 100,
				width: Math.max(((c.duration_ms ?? 0) / total) * 100, 0.5)
			};
		});
	}

	function caption(r: Run): string {
		const parts = [formatRelativeTime(r.started_at)];
		if (r.duration_ms != null) parts.push(formatMs(r.duration_ms));
		parts.push(m.asset_page_run_steps({ count: r.tool_calls?.length ?? 0 }));
		return parts.join(' · ');
	}
</script>

{#if loading}
	<mm-loading active caption={m.asset_page_runs_loading()}></mm-loading>
{:else if error}
	<mm-callout tone="danger" heading={m.asset_agent_runs_load_error()}>
		{error}
		<mm-button slot="actions" size="sm" variant="secondary" icon="refresh" onclick={load}
			>{m.common_retry()}</mm-button
		>
	</mm-callout>
{:else if runs.length === 0}
	<mm-empty-state
		heading={m.asset_page_agent_runs_empty_heading()}
		description={m.asset_agent_runs_empty_hint()}
		seed={3}
	></mm-empty-state>
{:else}
	<div class="mm-stack" style="--mm-gap: var(--space-5)">
		{#if stats}
			<mm-description-list layout="inline-grid" columns={4}>
				<mm-description term={m.asset_agent_runs_stat_runs_24h()}
					><mm-text numeric>{stats.run_count}</mm-text></mm-description
				>
				<mm-description term={m.asset_agent_runs_stat_success_rate()}
					><mm-text numeric
						>{stats.run_count > 0 ? `${Math.round(stats.success_rate * 100)}%` : ''}</mm-text
					></mm-description
				>
				<mm-description term={m.asset_agent_runs_stat_median_latency()}
					><mm-text numeric>{stats.run_count > 0 ? formatMs(stats.median_latency_ms) : ''}</mm-text
					></mm-description
				>
				<mm-description term={m.asset_agent_runs_stat_tokens_24h()}
					><mm-text numeric
						>{(stats.tokens_in + stats.tokens_out).toLocaleString(getLocale())}</mm-text
					></mm-description
				>
			</mm-description-list>
		{/if}

		<mm-toolbar label={m.asset_page_filter_runs()}>
			<mm-search-field
				label={m.asset_page_find_run()}
				hide-label
				placeholder={m.asset_page_find_run()}
				value={filter}
				style="inline-size: 18rem"
				onmm-input={(e: CustomEvent<{ value: string }>) => (filter = e.detail.value)}
			></mm-search-field>
			<mm-select
				label={m.asset_page_outcome()}
				hide-label
				value={outcome}
				style="inline-size: 11rem"
				onmm-change={(e: CustomEvent<{ value: string }>) =>
					(outcome = e.detail.value as typeof outcome)}
			>
				<mm-option value="all">{m.asset_page_outcome_all()}</mm-option>
				<mm-option value="succeeded">{m.asset_page_run_succeeded()}</mm-option>
				<mm-option value="failed">{m.asset_page_run_failed()}</mm-option>
			</mm-select>
			<mm-run-strip
				slot="end"
				label={m.asset_page_runs_strip_label({ name: asset.name })}
				runs={strip}
				selectable
				onmm-select={(e: CustomEvent<{ index: number }>) => {
					const r = [...runs].reverse()[e.detail.index];
					if (r) selectedId = r.run_id;
				}}
			></mm-run-strip>
		</mm-toolbar>

		<div class="split">
			<mm-list aria-label={m.asset_page_runs_list_label({ name: asset.name })} class="runs">
				{#each visible as r (r.run_id)}
					<mm-list-item
						interactive
						selected={selected?.run_id === r.run_id}
						secondary={caption(r)}
						onmm-activate={() => (selectedId = r.run_id)}
					>
						{r.prompt || when(r.started_at)}
						{#if failed(r)}
							<mm-status-pill slot="end" state="failed"></mm-status-pill>
						{/if}
					</mm-list-item>
				{:else}
					<mm-list-item>{m.asset_page_no_matching_runs()}</mm-list-item>
				{/each}
			</mm-list>

			{#if selected}
				{@const story = steps(selected)}
				{@const refusedSteps = (selected.tool_calls ?? []).filter(refused).length}
				<section
					class="mm-stack"
					style="--mm-gap: var(--space-5); min-width: 0"
					aria-label={m.asset_page_run_aria({ id: selected.run_id })}
				>
					<div class="mm-stack" style="--mm-gap: var(--space-1)">
						<mm-heading level={2} variant="heading"
							>{selected.prompt || when(selected.started_at)}</mm-heading
						>
						<mm-text variant="caption" tone="secondary" block>{caption(selected)}</mm-text>
						{#if failed(selected) || refusedSteps}
							<span class="mm-cluster" style="--mm-gap: var(--space-2)">
								{#if failed(selected)}
									<mm-status-pill state="failed"></mm-status-pill>
								{/if}
								{#if refusedSteps}
									<mm-status-pill state="denied"
										>{m.asset_page_run_refused_steps({
											count: refusedSteps,
											formatted: refusedSteps.toLocaleString(getLocale())
										})}</mm-status-pill
									>
								{/if}
							</span>
						{/if}
					</div>
					{#if selected.error}
						<mm-callout tone="danger" heading={m.asset_page_run_failed_heading()} compact>
							{selected.error}
						</mm-callout>
					{/if}
					{#if story.length > 0}
						<ol class="steps" aria-label={m.asset_page_run_steps_label()}>
							{#each story as { c, left, width } (c.ordinal)}
								{@const bad = refused(c) || broke(c)}
								<li>
									<mm-icon
										name={stepIcon(c)}
										aria-hidden="true"
										style="color: {bad ? 'var(--mm-danger)' : 'var(--mm-text-2)'}"
									></mm-icon>
									<div class="mm-stack" style="--mm-gap: var(--space-2); min-width: 0">
										<mm-text variant="dense" block
											>{stepLabel(c)}{#if c.result}
												<mm-text variant="dense" tone="secondary"> → {c.result}</mm-text>
											{/if}</mm-text
										>
										{#if c.target_mrn}
											<mm-text variant="mono-dense" tone="secondary" block class="wrap"
												>{c.target_mrn}</mm-text
											>
										{/if}
										{#if c.reason || c.rule}
											<mm-text variant="dense" tone="secondary" block
												>{[c.reason, c.rule ? m.asset_page_step_rule({ rule: c.rule }) : '']
													.filter(Boolean)
													.join(' ')}</mm-text
											>
										{/if}
										<div class="bar" aria-hidden="true">
											<div
												style="inset-inline-start: {left}%; width: {width}%; background: {bad
													? 'var(--mm-danger)'
													: 'var(--mm-text-3)'}"
											></div>
										</div>
										{#if c.detail && isQuery(c)}
											<mm-disclosure summary={m.asset_page_step_show_query()}>
												<mm-code-sample
													language="sql"
													wrap
													label={m.asset_page_step_query_label({ n: c.ordinal + 1 })}
													code={c.detail}
												></mm-code-sample>
											</mm-disclosure>
										{:else if c.detail}
											<mm-text variant="mono-dense" tone="secondary" block class="wrap"
												>{c.detail}</mm-text
											>
										{/if}
									</div>
									{#if refused(c)}
										<mm-status-pill state="denied"></mm-status-pill>
									{:else if bad}
										<mm-status-pill state="failed"></mm-status-pill>
									{:else}
										<mm-text variant="caption" tone="secondary" numeric
											>{formatMs(c.duration_ms)}</mm-text
										>
									{/if}
								</li>
							{/each}
						</ol>
					{:else}
						<mm-text variant="dense" tone="secondary" block>{m.asset_page_run_no_steps()}</mm-text>
					{/if}
					<mm-blade-section heading={m.asset_page_more_details()} collapsible style="padding: 0">
						<mm-description-list>
							<mm-description term={m.asset_page_tokens_in()}
								>{selected.tokens_in.toLocaleString(getLocale())}</mm-description
							>
							<mm-description term={m.asset_page_tokens_out()}
								>{selected.tokens_out.toLocaleString(getLocale())}</mm-description
							>
							{#if selected.model}
								<mm-description term={m.asset_agent_spec_model()} mono
									>{selected.model}</mm-description
								>
							{/if}
							<mm-description term={m.asset_page_run_id()} mono>{selected.run_id}</mm-description>
						</mm-description-list>
					</mm-blade-section>
				</section>
			{/if}
		</div>
	</div>
{/if}

<style>
	.split {
		display: grid;
		grid-template-columns: minmax(16rem, 22rem) minmax(0, 1fr);
		gap: var(--space-6);
		align-items: start;
	}
	@media (max-width: 900px) {
		.split {
			grid-template-columns: minmax(0, 1fr);
		}
	}
	.runs {
		--mm-list-inset: var(--space-2);
		border-block: 1px solid var(--mm-rule);
	}
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
		border-block: 1px solid var(--mm-rule);
	}
	.steps > li {
		display: grid;
		grid-template-columns: 24px minmax(0, 1fr) auto;
		gap: var(--space-1) var(--space-3);
		padding: var(--space-3) var(--space-2);
	}
	.steps > li + li {
		border-block-start: 1px solid var(--mm-rule);
	}
	.bar {
		position: relative;
		height: 4px;
		background: var(--mm-sunken);
		border-radius: 2px;
	}
	.bar > div {
		position: absolute;
		inset-block: 0;
		min-width: 2px;
		border-radius: 2px;
	}
	.wrap {
		overflow-wrap: anywhere;
	}
</style>

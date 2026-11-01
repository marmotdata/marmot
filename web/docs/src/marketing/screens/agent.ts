/**
 * An agent's asset page, Runs tab: runs on the left, the selected run read as a story on the
 * right. The mockup shows support-triage-agent's finished run; here it is revenue-agent answering
 * a finance question, and the story grows as the demo plays: steps appear, run, finish, then the
 * answer.
 */
import { html, nothing, type TemplateResult } from 'lit';
import type { Run } from '@marmotdata/elements/classes/run-strip.js';
import { shell } from './shell';
import { NOW, asset } from './data/catalog';
import { n, ms, when, revenueRuns, type AgentRun } from './data/asset';
import { QUERY_SQL } from './data/ops';
import { assetPage, crumbsFor, lineageList, moreMenu, subscribe, tabs, type Tab } from './asset';

export const AGENT_QUESTION = 'What was revenue last week, and can I trust the number?';

const ANSWER =
	'Revenue last week was £413,180 across 8,615 orders. It comes from orders_daily, which Analytics Engineering certifies, so it is the figure finance reports use.';

const agent = asset('revenue-agent');
const agentTabs: Tab[] = [
	{ id: 'documentation', label: 'Documentation', icon: 'description' },
	{ id: 'runs', label: 'Runs', icon: 'play-arrow', count: 286 },
	{ id: 'lineage', label: 'Lineage', icon: 'account-tree', count: 6 },
	{ id: 'queries', label: 'Queries', icon: 'query-stats', count: 941 },
	{ id: 'metadata', label: 'Metadata', icon: 'data-object' }
];

/** The tool calls of the run the demo plays, as plain steps: what the agent did, what came back. */
const steps = [
	{
		tool: 'search_assets',
		icon: 'search',
		label: 'Searched the catalog',
		result: 'found orders_daily, certified, owned by Analytics Engineering',
		ms: 160
	},
	{
		tool: 'get_lineage',
		icon: 'account-tree',
		label: 'Read its lineage',
		result: 'built by dbt from shop.public.orders, refreshed 5 hours ago',
		ms: 120
	},
	{
		tool: 'check_access',
		icon: 'policy',
		label: 'Checked its access',
		result: 'svc-finance-agent may read orders_daily',
		ms: 40
	},
	{
		tool: 'run_query',
		icon: 'terminal',
		label: 'Ran a query on orders_daily',
		result: '7 rows in 412 ms',
		ms: 412,
		sql: QUERY_SQL
	}
];
export const AGENT_STEP_COUNT = steps.length;

/** Steps start on the run's own clock, with the model's thinking between them. */
const timed = (() => {
	let t = 0;
	return steps.map((c) => {
		const start = t;
		t += c.ms + 1_400;
		return { c, start };
	});
})();
const TOTAL = timed.reduce((end, { c, start }) => Math.max(end, start + c.ms), 0) + 1_900;

export interface AgentState {
	question: string;
	/** How many steps are on screen. */
	stepsShown: number;
	/** How many steps have finished; a shown step at or past this index is still running. */
	stepsDone: number;
	/** Whether the answer is in. */
	answered: boolean;
}

/** "Slack #finance-questions: Refunds by region" reads as the question, with the channel as its caption. */
const askedOf = (r: AgentRun) => {
	const m = /^(.*?):\s*(.+)$/.exec(r.trigger);
	return m ? { question: m[2]!, source: m[1]! } : { question: r.trigger, source: '' };
};

const currentRun = (s: AgentState): AgentRun => ({
	id: 'arun_7d2b19',
	started: new Date(NOW.getTime() - 20_000).toISOString(),
	trigger: `Slack #finance-questions: ${s.question}`,
	status: s.answered ? 'succeeded' : 'running',
	durationMs: TOTAL,
	toolCalls: Math.min(s.stepsShown, steps.length),
	tokensIn: 6_388,
	tokensOut: 214
});

const runStrip = (running: boolean): Run[] =>
	Array.from({ length: 30 }, (_, i) => ({
		id: `arun_${(0x7d2b19 - (29 - i) * 7).toString(16)}`,
		status: i === 29 && running ? 'running' : i === 27 || i === 14 ? 'failed' : 'succeeded',
		startedAt: new Date(NOW.getTime() - (29 - i) * 97 * 60_000),
		durationMs: 6_000 + ((i * 1_337) % 6_000)
	}));

const runRow = (r: AgentRun, selected: boolean) => {
	const a = askedOf(r);
	const secondary =
		r.status === 'running'
			? `${when(r.started)} · Running`
			: `${when(r.started)} · ${ms(r.durationMs)} · ${r.toolCalls} steps`;
	return html`<mm-list-item
		href="#asset/revenue-agent?tab=runs&run=${r.id}"
		?selected=${selected}
		secondary=${secondary}
	>
		${a.question}
		${r.status === 'failed' || r.status === 'running' ? html`<mm-status-pill slot="end" state=${r.status}></mm-status-pill>` : nothing}
	</mm-list-item>`;
};

/** The run as a story: each step with a timing bar on the run's own clock, then the answer. */
const runStory = (run: AgentRun, s: AgentState) => {
	const a = askedOf(run);
	const shown = timed.slice(0, Math.max(0, Math.min(s.stepsShown, steps.length)));
	return html`<section
		data-demo="run"
		class="mm-stack"
		style="--mm-gap: var(--space-5); min-width: 0"
		aria-label="Run ${run.id}"
	>
		<div class="mm-stack" style="--mm-gap: var(--space-1)">
			<mm-heading level="2" variant="heading">${a.question}</mm-heading>
			<mm-text variant="caption" tone="secondary" block
				>${[a.source, s.answered ? 'Succeeded' : 'Running', s.answered ? ms(run.durationMs) : '', s.stepsDone >= steps.length ? 'Read orders_daily' : ''].filter(Boolean).join(' · ')}</mm-text
			>
		</div>
		${
			shown.length
				? html`<ol
						data-demo="steps"
						aria-label="Steps of this run"
						style="list-style: none; margin: 0; padding: 0; border-block: 1px solid var(--mm-rule)"
					>
						${shown.map(({ c, start }, i) => {
							const running = i >= s.stepsDone;
							return html`<li
								style="display: grid; grid-template-columns: 24px minmax(0, 1fr) auto; gap: var(--space-1) var(--space-3); padding: var(--space-3) var(--space-2); ${i ? 'border-block-start: 1px solid var(--mm-rule);' : ''}"
							>
								<mm-icon
									name=${c.icon}
									aria-hidden="true"
									style="color: var(--mm-text-2); margin-block-start: 2px"
								></mm-icon>
								<div class="mm-stack" style="--mm-gap: var(--space-2); min-width: 0">
									<mm-text variant="dense" block
										>${c.label}${running ? nothing : html` <mm-text variant="dense" tone="secondary">→ ${c.result}</mm-text>`}</mm-text
									>
									<div
										aria-hidden="true"
										style="position: relative; height: 4px; background: var(--mm-sunken); border-radius: 2px"
									>
										<div
											style="position: absolute; inset-block: 0; inset-inline-start: ${(start / TOTAL) * 100}%; width: max(2px, ${(c.ms / TOTAL) * 100}%); background: var(--mm-text-3); border-radius: 2px; ${running ? 'opacity: 0.5;' : ''}"
										></div>
									</div>
									${c.sql && !running ? html`<mm-disclosure summary="Show query"><mm-code-sample language="sql" wrap label="Query of step ${i + 1}" .code=${c.sql}></mm-code-sample></mm-disclosure>` : nothing}
								</div>
								<mm-text variant="caption" tone="secondary" numeric
									>${running ? html`<mm-status-pill state="running"></mm-status-pill>` : ms(c.ms)}</mm-text
								>
							</li>`;
						})}
					</ol>`
				: nothing
		}
		${
			s.answered
				? html`<div class="mm-stack" style="--mm-gap: var(--space-2)" data-demo="answer">
						<p class="mm-label" style="margin: 0">Answer</p>
						<mm-text block>${ANSWER}</mm-text>
					</div>`
				: nothing
		}
		<mm-blade-section heading="More details" collapsible style="padding: 0">
			<mm-description-list>
				<mm-description term="Tokens in">${n(run.tokensIn)}</mm-description>
				<mm-description term="Tokens out">${n(run.tokensOut)}</mm-description>
				<mm-description term="Model" mono>hosted model</mm-description>
				<mm-description term="Run ID" mono>${run.id}</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<div><mm-button icon="policy" href="#audit?run=${run.id}">Open in Query audit</mm-button></div>
	</section>`;
};

const agentRunsTab = (s: AgentState) => {
	const run = currentRun(s);
	return html`<div class="mm-stack" style="--mm-gap: var(--space-4)">
		<mm-toolbar label="Filter runs">
			<mm-search-field
				label="Find a run"
				hide-label
				placeholder="Find a run"
				style="inline-size: 18rem"
			></mm-search-field>
			<mm-select label="Outcome" hide-label value="all" style="inline-size: 11rem">
				<mm-option value="all">All outcomes</mm-option>
				<mm-option value="succeeded">Succeeded</mm-option>
				<mm-option value="refused">With a refused step</mm-option>
				<mm-option value="failed">Failed</mm-option>
			</mm-select>
			<mm-run-strip
				slot="end"
				label="Last 30 runs of revenue-agent"
				.runs=${runStrip(!s.answered)}
			></mm-run-strip>
		</mm-toolbar>
		<div
			style="display: grid; grid-template-columns: minmax(16rem, 22rem) minmax(0, 1fr); gap: var(--space-6); align-items: start"
		>
			<mm-list
				aria-label="Runs of revenue-agent"
				style="--mm-list-inset: var(--space-2); border-block: 1px solid var(--mm-rule)"
			>
				${[run, ...revenueRuns].map((r) => runRow(r, r.id === run.id))}
			</mm-list>
			${runStory(run, s)}
		</div>
	</div>`;
};

const agentBlade = () =>
	html`<mm-details-blade heading="About this agent" collapsible collapsed>
		<mm-blade-section heading="At a glance">
			<mm-description-list>
				<mm-description term="Can read"
					>Certified finance tables, without personal data</mm-description
				>
				<mm-description term="Connects over">MCP</mm-description>
			</mm-description-list>
		</mm-blade-section>
		<mm-blade-section heading="About">
			<mm-button
				slot="actions"
				variant="ghost"
				size="sm"
				icon="edit"
				label="Edit description"
			></mm-button>
			<mm-text variant="dense" block>${agent.description}</mm-text>
		</mm-blade-section>
		<mm-blade-section heading="What it reads" flush>
			${lineageList('Assets this agent reads', [
				{ name: 'orders_daily', provider: 'dbt', what: '88 lookups this week' },
				{ name: 'charges', provider: 'snowflake', what: '41 lookups this week' },
				{ name: 'Revenue overview', provider: 'looker', what: '12 lookups this week' }
			])}
		</mm-blade-section>
		<mm-blade-section heading="More details" collapsible>
			<mm-description-list>
				<mm-description term="Created by">Sam Jones</mm-description>
				<mm-description term="Created">26 August 2026, 11:40 UTC</mm-description>
				<mm-description term="Last run">27 October 2026, 09:30 UTC</mm-description>
			</mm-description-list>
		</mm-blade-section>
	</mm-details-blade>`;

export const agentScreen = (s: AgentState): TemplateResult =>
	shell({
		current: 'discover',
		crumbs: crumbsFor('revenue-agent', 'Agents'),
		content: assetPage(
			html`<mm-page-header
				heading="revenue-agent"
				provider="agent"
				caption=${s.answered ? 'Runs as svc-finance-agent · Last run just now' : 'Runs as svc-finance-agent · Running now'}
				mrn=${agent.mrn}
			>
				<mm-tag slot="tags">finance</mm-tag>
				<mm-owner-chip slot="owners" name="Finance Analytics" kind="team"></mm-owner-chip>
				<mm-owner-chip slot="owners" name="Sam Jones"></mm-owner-chip>
				${subscribe()} ${moreMenu('revenue-agent')}
			</mm-page-header>`,
			tabs('runs', agentTabs, 'revenue-agent'),
			agentRunsTab(s)
		),
		blade: agentBlade()
	});

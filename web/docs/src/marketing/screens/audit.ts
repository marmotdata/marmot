/**
 * Query audit (/query-audit): every query run through Marmot, by people and agents, with a range
 * control, filters and the expandable audit log. The demo adds revenue-agent's queries to the top
 * of the log as they happen.
 */
import { html, type TemplateResult } from 'lit';
import { repeat } from 'lit/directives/repeat.js';
import { shell } from './shell';
import { agentAudit, audit, type AuditEvent } from './data/ops';

/** The row the mockup opens when nothing new has arrived. */
const OPEN = 'q_01JBK7J7RZ2KQ5';

const row = (e: AuditEvent, expanded = false, selected = false) =>
	html`<mm-audit-row
		time=${e.at}
		actor=${e.actor}
		actor-kind=${e.kind}
		service-account=${e.sa ?? ''}
		client=${e.client}
		source=${e.source}
		provider=${e.provider}
		.query=${e.sql}
		.rows=${e.rows}
		?truncated=${e.truncated}
		outcome=${e.outcome}
		reason=${e.reason ?? ''}
		.duration=${e.ms}
		.tables=${e.tables}
		query-id=${e.id}
		?expanded=${expanded}
		?selected=${selected}
	></mm-audit-row>`;

export interface AuditState {
	/** How many of revenue-agent's two new queries have arrived: 0, 1 or 2. */
	newest: number;
}

export const auditScreen = (s: AuditState): TemplateResult => {
	const fresh = agentAudit.slice(0, Math.max(0, Math.min(s.newest, agentAudit.length))).reverse();
	const log = [...fresh, ...audit];
	// The newest arrival is the one to read, so it opens; before any arrive, the mockup's row does.
	const open = fresh[0]?.id ?? OPEN;
	return shell({
		current: 'audit',
		crumbs: [{ label: 'Query audit' }],
		content: html`<div class="mm-stack" style="--mm-gap:var(--space-5)">
			<mm-page-header
				heading="Query audit"
				description="Every query run through Marmot, and the decision on it."
			>
				<mm-segmented-control slot="actions" label="Time range" hide-label value="24h">
					<mm-segment value="24h">24h</mm-segment>
					<mm-segment value="7d">7d</mm-segment>
					<mm-segment value="30d">30d</mm-segment>
					<mm-segment value="90d">90d</mm-segment>
				</mm-segmented-control>
				<mm-button slot="actions" variant="secondary" icon="download">Export CSV</mm-button>
			</mm-page-header>
			<mm-toolbar label="Filter the audit log">
				<mm-search-field
					label="Search callers, tables and SQL"
					hide-label
					placeholder="Search callers, tables and SQL"
					shortcut="/"
					style="inline-size:22rem"
				></mm-search-field>
				<mm-select label="Outcome" hide-label value="all" style="inline-size:14rem">
					<mm-option value="all">All outcomes</mm-option>
					<mm-option-group label="Allowed">
						<mm-option value="allowed">Any reason</mm-option>
						<mm-option value="allowed-role">By role</mm-option>
						<mm-option value="allowed-grant">By grant</mm-option>
					</mm-option-group>
					<mm-option-group label="Denied">
						<mm-option value="denied">Any reason</mm-option>
						<mm-option value="denied-pii">By the personal data policy</mm-option>
						<mm-option value="denied-aggregates">Totals only</mm-option>
					</mm-option-group>
					<mm-option value="failed">Failed</mm-option>
				</mm-select>
				<mm-select label="Source" hide-label value="all" style="inline-size:12rem">
					<mm-option value="all">All sources</mm-option>
					<mm-option value="postgres-shop" provider="postgresql">postgres-shop</mm-option>
					<mm-option value="snowflake-finance" provider="snowflake">snowflake-finance</mm-option>
					<mm-option value="bigquery-web" provider="bigquery">bigquery-web</mm-option>
				</mm-select>
			</mm-toolbar>
			<mm-audit-log
				label="Query audit log"
				empty-heading="No queries in the last 24 hours"
				empty-description="Nobody has queried a source through Marmot in this range. Try a longer range."
			>
				${repeat(
					log,
					// Keyed, so a new query is inserted at the top instead of every row shifting its contents down.
					(a) => a.id,
					(a) => row(a, a.id === open, fresh.length > 0 && a.id === open)
				)}
			</mm-audit-log>
			<mm-pagination
				page="1"
				page-size="50"
				total=${1_284 + fresh.length}
				label="Audit log pages"
			></mm-pagination>
		</div>`
	});
};

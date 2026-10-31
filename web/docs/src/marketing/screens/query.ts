/**
 * Query (/query): the full-viewport workspace. Source explorer on the left, then the toolbar,
 * the CodeMirror SQL editor and the result tabs: Results, Tables read, History. The demo types
 * into the editor and runs it, so the editor shows whatever SQL it is given.
 */
import { html, type TemplateResult } from 'lit';
import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
import { shell } from './shell';
import { NOW } from './data/catalog';
import {
	deniedTablesRead,
	editorSchema,
	history,
	rel,
	resultColumns,
	resultRows,
	sourceTree,
	tablesRead,
	treeExpanded,
	treeSelected
} from './data/ops';

export { QUERY_SQL, DENIED_SQL } from './data/ops';

export type QueryRunState = 'idle' | 'running' | 'results' | 'denied';

type TableRead = (typeof tablesRead)[number];
const tablesReadColumns: DataTableColumn<TableRead>[] = [
	{ key: 'name', header: 'Table', width: 260, href: (r) => `#assets/${encodeURIComponent(r.mrn)}` },
	{ key: 'owner', header: 'Owner', width: 190 },
	{ key: 'rows', header: 'Rows', type: 'number', width: 110 },
	{ key: 'columnsRead', header: 'Columns read', type: 'number', width: 120 },
	{
		key: 'pii',
		header: 'Personal data',
		minWidth: 200,
		render: (r) =>
			r.pii
				? html`<span class="mm-cluster" style="flex-wrap:nowrap"
						><mm-tag variant="pii"></mm-tag
						><mm-text variant="dense" tone="secondary">${r.pii}</mm-text></span
					>`
				: html`<mm-text variant="dense" tone="secondary">None</mm-text>`
	},
	{
		key: 'certified',
		header: 'Certified',
		width: 120,
		render: (r) =>
			r.certified
				? html`<mm-tag variant="certified"></mm-tag>`
				: html`<mm-text variant="dense" tone="secondary">Not certified</mm-text>`
	},
	{
		key: 'updated',
		header: 'Metadata refreshed',
		width: 160,
		render: (r) => html`<mm-text variant="dense" tone="secondary">${rel(r.updated)}</mm-text>`
	}
];

// The tree keeps its own expanded state once drawn, so it shows the same branch in every state.
const explorer = () =>
	html`<div
		slot="start"
		class="mm-stack"
		style="--mm-gap:var(--space-3);padding:var(--space-4) var(--space-3) var(--space-4) var(--space-4);block-size:100%;box-sizing:border-box"
	>
		<div class="mm-cluster mm-cluster--between">
			<mm-heading level="2" variant="panel-title">Sources</mm-heading>
			<mm-text variant="caption" tone="secondary">3 queryable</mm-text>
		</div>
		<mm-tree
			label="Queryable sources"
			filterable
			filter-label="Filter tables and columns"
			filter-placeholder="Filter tables and columns"
			.nodes=${sourceTree}
			.expandedIds=${treeExpanded}
			.selectedIds=${[treeSelected]}
			.actions=${[
				{ id: 'insert', label: 'Insert into editor', icon: 'input' },
				{ id: 'sample', label: 'Write a sample query', icon: 'code' },
				{ id: 'catalog', label: 'Open in catalog', icon: 'open-in-new' }
			]}
			style="--mm-tree-height:100%;flex:1;min-block-size:0"
		></mm-tree>
	</div>`;

// Marmot's query console presets. The server defaults to 1,000 and caps every query at 10,000, so there is no "No limit".
const rowLimits = [10, 100, 1000, 10000];

const toolbar = (state: QueryRunState) =>
	html`<mm-toolbar label="Query" style="padding:var(--space-3) var(--space-5)">
		<mm-select label="Source" hide-label value="snowflake-finance" style="inline-size:14rem">
			<mm-option value="postgres-shop" provider="postgresql">postgres-shop</mm-option>
			<mm-option value="snowflake-finance" provider="snowflake">snowflake-finance</mm-option>
			<mm-option value="bigquery-web" provider="bigquery">bigquery-web</mm-option>
		</mm-select>
		<mm-status-pill status="neutral" icon="lock">Read only</mm-status-pill>
		<mm-toolbar-separator></mm-toolbar-separator>
		<mm-select label="Row limit" hide-label value="1000" style="inline-size:8.5rem">
			${rowLimits.map((n) => html`<mm-option value=${String(n)}>${n.toLocaleString('en-GB')} rows</mm-option>`)}
		</mm-select>
		<span slot="end" class="mm-cluster">
			${
				state === 'running'
					? html`<mm-button variant="secondary" icon="stop">Cancel query</mm-button
							><mm-button variant="primary" loading>Running query</mm-button>`
					: html`<mm-button variant="ghost" icon="data-object">Format SQL</mm-button>
							<mm-kbd keys="mod+enter" label="Runs the query"></mm-kbd>
							<mm-button variant="primary" icon="play-arrow">Run query</mm-button>`
			}
		</span>
	</mm-toolbar>`;

/* The callout already names the refused table and column, so the details add only what it does not. */
const decision = html`<mm-description-list layout="inline-grid" columns="3">
	<mm-description term="Policy">pii-columns-require-grant</mm-description>
	<mm-description term="Grant needed">pii-read</mm-description>
	<mm-description term="Your roles">analyst</mm-description>
	<mm-description term="Query ID" mono>q_01JBK8C2W7T9RM</mm-description>
	<mm-description term="Decided">09:30:04 UTC</mm-description>
</mm-description-list>`;

const resultsPanel = (state: QueryRunState): TemplateResult => {
	const pad = (t: TemplateResult) =>
		html`<div
			class="mm-stack"
			style="--mm-gap:var(--space-4);padding:var(--space-4) var(--space-5)"
		>
			${t}
		</div>`;
	switch (state) {
		case 'denied':
			return pad(
				html`<mm-callout
						tone="danger"
						urgent
						heading="Marmot refused this query because it reads customers_dim.email, which is tagged PII."
					>
						Remove c.email from the select list, or ask Governance for the pii-read grant.
						<mm-button slot="actions" size="sm" variant="secondary" icon="vpn-key"
							>Request pii-read grant</mm-button
						>
						<mm-button slot="actions" size="sm" variant="ghost" icon="policy"
							>View in Query audit</mm-button
						>
					</mm-callout>
					${decision}`
			);
		case 'running':
			// A 7-row query has no row count worth showing while it runs, so the bar is indeterminate.
			return pad(
				html`<mm-progress
					label="Waiting for snowflake-finance"
					indeterminate
					style="max-inline-size:28rem"
				></mm-progress>`
			);
		case 'idle':
			return pad(
				html`<mm-text variant="dense" tone="secondary" block
					>Run the query to see its results.</mm-text
				>`
			);
		default:
			// The mockup subtracts a 61px toolbar; at the landing page's 1280px the toolbar wraps to 104px.
			return html`<mm-results-grid
				label="Results"
				filename="revenue-last-week"
				.columns=${resultColumns}
				.rows=${resultRows}
				duration="412"
				style="--mm-results-grid-height:calc(var(--screen-h, 100vh) - 52px - 104px - 280px - 44px - 92px)"
			></mm-results-grid>`;
	}
};

export interface QueryState {
	sql: string;
	state: QueryRunState;
}

export const queryScreen = (s: QueryState): TemplateResult => {
	const denied = s.state === 'denied';
	const read = s.state === 'results' ? tablesRead : denied ? deniedTablesRead : [];
	return shell({
		current: 'query',
		bleed: true,
		crumbs: [{ label: 'Query' }],
		content: html`<div style="block-size:100%;display:grid;grid-template-columns:minmax(0,1fr)">
			<mm-split-pane
				label="Resize source explorer"
				default-position="336"
				min-start="240"
				style="block-size:100%;min-block-size:0"
			>
				${explorer()}
				<div
					slot="end"
					style="display:grid;grid-template-columns:minmax(0,1fr);grid-template-rows:auto auto auto minmax(0,1fr);block-size:100%;min-inline-size:0"
				>
					<mm-visually-hidden><mm-heading level="1">Query</mm-heading></mm-visually-hidden>
					${toolbar(s.state)}
					<mm-sql-editor
						label="SQL"
						dialect="snowflake"
						default-schema="analytics.marts"
						hide-shortcut
						.schema=${editorSchema}
						.value=${s.sql}
						style="--mm-code-editor-height:280px"
					></mm-sql-editor>
					<mm-tabs label="Query output" value="results" style="min-block-size:0">
						<mm-tab value="results" icon="table-rows">Results</mm-tab>
						<mm-tab value="tables" icon="table" .count=${read.length || undefined}
							>Tables read</mm-tab
						>
						<mm-tab value="history" icon="history" .count=${history.length}>History</mm-tab>
						<mm-tab-panel value="results">${resultsPanel(s.state)}</mm-tab-panel>
						<mm-tab-panel value="tables">
							<div
								class="mm-stack"
								style="--mm-gap:var(--space-3);padding:var(--space-4) var(--space-5)"
							>
								<mm-data-table
									label="Tables read"
									density="compact"
									.columns=${tablesReadColumns}
									.rows=${read}
								></mm-data-table>
							</div>
						</mm-tab-panel>
						<mm-tab-panel value="history">
							<div style="padding:var(--space-4) var(--space-5)">
								<mm-query-history
									label="Your recent queries"
									.queries=${history}
									.now=${NOW.getTime()}
								></mm-query-history>
							</div>
						</mm-tab-panel>
					</mm-tabs>
				</div>
			</mm-split-pane>
		</div>`
	});
};

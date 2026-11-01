/**
 * Query (/query): the full-viewport workspace, drawn as QueryWorkspace.svelte draws it. The source
 * explorer is on the left, then the toolbar, the CodeMirror SQL editor and the result tabs: Results,
 * Tables read and History. The demo types into the editor and runs it, so the editor shows whatever
 * SQL it is given.
 */
import { html, type TemplateResult } from 'lit';
import type { DataTableColumn } from '@marmotdata/elements/classes/data-table.js';
import type { TreeAction, TreeNode } from '@marmotdata/elements/classes/tree.js';
import { shell } from './shell';
import { NOW } from './data/catalog';
import {
	editorSchema,
	history,
	rel,
	resultColumns,
	resultRows,
	sourceTree,
	starters,
	tablesRead,
	treeExpanded,
	treeSelected
} from './data/ops';

export { QUERY_SQL, DENIED_SQL } from './data/ops';

export type QueryRunState = 'idle' | 'running' | 'results' | 'denied';

const SOURCE = 'snowflake-finance';

/** The refused query's audit record, the same one the Query audit shows for revenue-agent. */
const DENIED_ID = '62bacc3c-3b2d-4f55-ae98-4603ca18016a';

type TableRead = (typeof tablesRead)[number];
const tablesReadColumns: DataTableColumn<TableRead>[] = [
	{ key: 'name', header: 'Table', width: 260, href: (r) => `#assets/${encodeURIComponent(r.mrn)}` },
	{ key: 'owner', header: 'Owner', width: 190, empty: 'No owner' },
	{ key: 'rows', header: 'Rows', type: 'number', width: 110 },
	{ key: 'columnsRead', header: 'Columns read', type: 'number', width: 120 },
	{
		key: 'pii',
		header: 'Personal data',
		minWidth: 200,
		value: (r) => r.pii || null,
		empty: 'None',
		render: (r) =>
			html`<span class="mm-cluster" style="flex-wrap:nowrap"
				><mm-tag variant="pii"></mm-tag
				><mm-text variant="dense" tone="secondary">${r.pii}</mm-text></span
			>`
	},
	{
		key: 'updated',
		header: 'Metadata refreshed',
		width: 170,
		// The app gives this column the date type, which reads relative to the real clock; the mock-up's clock is fixed, so it formats against that instead.
		render: (r) => html`<mm-text variant="dense" tone="secondary">${rel(r.updated)}</mm-text>`
	}
];

// Columns can only go into the editor; tables can also open their sample query or their catalog page.
const insert: TreeAction = { id: 'insert', label: 'Insert into editor', icon: 'input' };
const treeActions = (node: TreeNode): TreeAction[] => {
	if (node.id.startsWith('c:')) return [insert];
	if (!node.id.startsWith('t:')) return [];
	return [
		insert,
		{ id: 'sample', label: 'Open a sample query', icon: 'code' },
		{ id: 'catalog', label: 'Open in the catalog', icon: 'open-in-new' }
	];
};

// The tree keeps its own expanded state once drawn, so it shows the same branch in every state.
const explorer = () =>
	html`<div
		slot="start"
		class="mm-stack"
		style="--mm-gap:var(--space-3);padding:var(--space-4) var(--space-3) var(--space-4) var(--space-4);block-size:100%;box-sizing:border-box"
	>
		<div class="mm-cluster mm-cluster--between">
			<mm-heading level="2" variant="panel-title">Sources</mm-heading>
			<mm-text variant="caption" tone="secondary" numeric>${sourceTree.length}</mm-text>
		</div>
		<mm-tree
			label="Queryable sources"
			filterable
			filter-label="Filter tables and columns"
			filter-placeholder="Filter tables and columns"
			empty-text="Connect a source to start querying"
			.nodes=${sourceTree}
			.expandedIds=${treeExpanded}
			.selectedIds=${[treeSelected]}
			.actions=${treeActions}
			style="--mm-tree-height:100%;flex:1;min-block-size:0"
		></mm-tree>
	</div>`;

// The server defaults to 1,000 rows and caps every query at 10,000, so there is no "No limit".
const rowLimits = [10, 100, 1000, 10000];

const toolbar = (s: QueryState) =>
	html`<mm-toolbar label="Query editor" style="padding:var(--space-3) var(--space-5)">
		<mm-button
			variant="ghost"
			icon="left-panel-close"
			label="Show or hide sources"
			pressed="true"
		></mm-button>
		<mm-select label="Source" hide-label value=${SOURCE} style="inline-size:14rem">
			<mm-option value="postgres-shop" provider="postgresql">postgres-shop</mm-option>
			<mm-option value="snowflake-finance" provider="snowflake">snowflake-finance</mm-option>
			<mm-option value="bigquery-web" provider="bigquery">bigquery-web</mm-option>
		</mm-select>
		<mm-status-pill
			status="neutral"
			icon="lock"
			title="Statements run read-only, as you, and are checked against your grants and recorded."
			>Read only</mm-status-pill
		>
		<mm-toolbar-separator></mm-toolbar-separator>
		<mm-select label="Row limit" hide-label value="1000" style="inline-size:8.5rem">
			${rowLimits.map((n) => html`<mm-option value=${String(n)}>${n.toLocaleString('en-GB')} rows</mm-option>`)}
		</mm-select>
		<span slot="end" class="mm-cluster">
			${
				s.state === 'running'
					? html`<mm-button variant="secondary" icon="stop">Cancel query</mm-button
							><mm-button variant="primary" loading>Running query</mm-button>`
					: html`<mm-button variant="ghost" icon="data-object">Format SQL</mm-button>
							<mm-kbd keys="mod+enter" label="Runs the query"></mm-kbd>
							<mm-button variant="primary" icon="play-arrow" ?disabled=${!s.sql.trim()}
								>Run query</mm-button
							>`
			}
		</span>
	</mm-toolbar>`;

const pad = (t: TemplateResult) =>
	html`<div class="mm-stack" style="--mm-gap:var(--space-4);padding:var(--space-4) var(--space-5)">
		${t}
	</div>`;

/*
 * A table refused by the authorizer: the refusal names no permission and asks for nothing in
 * particular, so the app shows its general heading and next step. The reason has the MRN shortened
 * to the name people know, as the app does. The details add only what the callout does not say.
 */
const refusal = html`<mm-callout
		tone="danger"
		urgent
		heading="Marmot refused this query because of an access policy."
	>
		<span style="display:block;margin-block-end:var(--space-1);overflow-wrap:anywhere"
			>you do not have asset:query on analytics.marts.customers_dim</span
		>
		Change the query to leave out what you cannot read, or ask an administrator for access.
		<mm-button slot="actions" size="sm" variant="secondary" icon="vpn-key">Request access</mm-button>
		<mm-button slot="actions" size="sm" variant="ghost" icon="policy">View in Query audit</mm-button>
	</mm-callout>
	<mm-description-list layout="inline-grid" columns="3">
		<mm-description term="Your roles">user</mm-description>
		<mm-description term="Query ID" mono>${DENIED_ID}</mm-description>
		<mm-description term="Decided">Oct 27, 2026, 09:29 AM</mm-description>
	</mm-description-list>`;

// Before a run, the app offers the source's first few tables, each opening its sample query.
const empty = pad(html`<mm-empty-state
		compact
		heading="Run a query to see results"
		description="Write a statement above and run it. Queries are read only and checked against your access."
	></mm-empty-state>
	<section class="mm-stack" style="--mm-gap:var(--space-2);max-inline-size:40rem">
		<mm-heading level="2" variant="panel-title">Start with a table in ${SOURCE}</mm-heading>
		<mm-list dense aria-label="Start with a table in ${SOURCE}">
			${starters.map(
				(t) =>
					html`<mm-list-item interactive secondary=${t.description || 'Preview the first rows'}
						><mm-icon slot="start" name="table"></mm-icon>${t.name}</mm-list-item
					>`
			)}
		</mm-list>
	</section>`);

const resultsPanel = (state: QueryRunState): TemplateResult => {
	switch (state) {
		case 'denied':
			return pad(refusal);
		case 'running':
			// Until the source reports rows, the app has no count to show, so the bar is indeterminate.
			return pad(
				html`<mm-progress
					label="Waiting for the source"
					indeterminate
					style="max-inline-size:28rem"
				></mm-progress>`
			);
		case 'idle':
			return empty;
		default:
			// The app sizes the grid to the output area less 128px for the tabs and the grid's footer, and never below 200px.
			return html`<mm-results-grid
				label="Results"
				filename="${SOURCE}-9ec02ef9"
				.columns=${resultColumns}
				.rows=${resultRows}
				duration="412"
				style="--mm-results-grid-height:max(200px, calc(var(--screen-h, 100vh) - 52px - 104px - 300px - 136px))"
			>
				<mm-button slot="actions" size="sm" variant="ghost" icon="receipt-long"
					>Open query details</mm-button
				>
			</mm-results-grid>`;
	}
};

export interface QueryState {
	sql: string;
	state: QueryRunState;
}

export const queryScreen = (s: QueryState): TemplateResult => {
	// Tables read comes with a result, so a refused query has none.
	const read = s.state === 'results' ? tablesRead : [];
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
				${toolbar(s)}
				<mm-sql-editor
					label="SQL statement"
					placeholder="SELECT * FROM orders LIMIT 10"
					dialect="snowflake"
					default-schema="analytics.marts"
					hide-shortcut
					.schema=${editorSchema}
					.value=${s.sql}
					style="--mm-code-editor-height:300px"
				></mm-sql-editor>
				<div style="min-block-size:0;overflow:auto">
					<mm-tabs label="Query output" value="results">
						<mm-tab value="results" icon="table-rows">Results</mm-tab>
						<mm-tab
							value="tables"
							icon="table"
							.count=${s.state === 'results' ? read.length : undefined}
							>Tables read</mm-tab
						>
						<mm-tab value="history" icon="history" .count=${history.length}>History</mm-tab>
						<mm-tab-panel value="results">${resultsPanel(s.state)}</mm-tab-panel>
						<mm-tab-panel value="tables">
							<div style="padding:var(--space-4) var(--space-5)">
								<mm-data-table
									label="Tables read"
									density="compact"
									.columns=${tablesReadColumns}
									.rows=${read}
									empty-heading="No tables read yet"
									empty-text="Run a query to see what the catalog knows about each table it reads."
								></mm-data-table>
							</div>
						</mm-tab-panel>
						<mm-tab-panel value="history">
							<div
								class="mm-stack"
								style="--mm-gap:var(--space-4);padding:var(--space-4) var(--space-5)"
							>
								<section class="mm-stack" style="--mm-gap:var(--space-3)">
									<div class="mm-cluster" style="justify-content:space-between">
										<mm-heading level="2" variant="panel-title">Saved queries</mm-heading>
										<mm-button
											size="sm"
											variant="secondary"
											icon="bookmark"
											?disabled=${!s.sql.trim()}
											>Save query</mm-button
										>
									</div>
									<mm-text variant="dense" tone="secondary" block
										>Save a query to run it again from here or from Home.</mm-text
									>
								</section>
								<mm-query-history
									label="Your recent queries"
									.queries=${history}
									.now=${NOW.getTime()}
								></mm-query-history>
							</div>
						</mm-tab-panel>
					</mm-tabs>
				</div>
			</div>
		</mm-split-pane>
		</div>`
	});
};

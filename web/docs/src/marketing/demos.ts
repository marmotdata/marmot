// The tour's scripts. One question threads them: finance wants last week's revenue, an agent
// answers it from orders_daily, and every step happens in the real product screens.
import type { TemplateResult } from 'lit';
import { type, type Demo } from './demo';
import {
	AGENT_QUESTION,
	AGENT_STEP_COUNT,
	DENIED_SQL,
	LINEAGE_DASHBOARDS,
	LINEAGE_OPENLINEAGE_JOB,
	LINEAGE_PATH,
	LINEAGE_PLUGIN_EDGE,
	QUERY_SQL,
	accessScreen,
	agentScreen,
	assetScreen,
	auditScreen,
	discoverScreen,
	lineageScreen,
	queryScreen,
	type AccessState,
	type AgentState,
	type AssetState,
	type DiscoverState,
	type LineageState,
	type QueryState
} from './screens';

// `beat` marks which of the chapter's points the script is showing, from 0. The chapter reads the
// timing of each beat from the script itself, so it never drifts from what plays.
export interface DemoConfig<S extends { focus?: string; beat?: number }> {
	label: string;
	screen: (s: S) => TemplateResult;
	initial: S;
	steps: (demo: Demo, set: (patch: Partial<S>) => void) => Promise<void>;
	rest?: number;
}

type Focus = { focus?: string; beat?: number };

// The one demo that shows the whole app first, before the camera moves in on the run.
export const agentDemo: DemoConfig<AgentState & Focus> = {
	label:
		'revenue-agent is asked what revenue was last week. It searches the catalog, reads the lineage of orders_daily, checks its access, runs a query and answers.',
	screen: agentScreen,
	initial: { question: '', stepsShown: 0, stepsDone: 0, answered: false },
	async steps(demo, set) {
		await demo.wait(1800);
		set({ focus: '[data-demo="run"]' });
		await demo.wait(700);
		await type(demo, AGENT_QUESTION, (question) => set({ question }), 30);
		for (let i = 0; i < AGENT_STEP_COUNT; i++) {
			await demo.wait(450);
			set({ stepsShown: i + 1, focus: '[data-demo="steps"]' });
			await demo.wait(1000);
			set({ stepsDone: i + 1 });
		}
		await demo.wait(500);
		set({ answered: true, focus: '[data-demo="answer"]' });
		await demo.wait(4500);
	},
	rest: 3000
};

export const findDemo: DemoConfig<DiscoverState & Focus> = {
	label:
		'Discover lists tables, topics, dashboards and models together, is narrowed to finance tables with the Type and Tags filters, and previews ORDERS_DAILY from Snowflake, showing when it last synced.',
	screen: discoverScreen,
	initial: { query: '', beat: 0 },
	async steps(demo, set) {
		await demo.wait(1400);
		set({ focus: 'ul[aria-label="Search results"]' });
		await demo.wait(2800);
		set({ beat: 1, focus: '.mm-split' });
		await demo.wait(900);
		set({ query: '@kind = asset AND @type = table' });
		await demo.wait(1300);
		set({ query: '@kind = asset AND @type = table AND @tag = finance' });
		await demo.wait(1500);
		set({ focus: 'mm-search-bar' });
		await demo.wait(1600);
		set({
			beat: 2,
			selected: 'ORDERS_DAILY',
			focus: 'mm-blade-section[heading="At a glance"]'
		});
		await demo.wait(4500);
	}
};

export const understandDemo: DemoConfig<AssetState & Focus> = {
	label:
		'The page for orders_daily shows Analytics Engineering and Jane Doe as its owners. In the schema, net_revenue_gbp opens with the glossary term Net revenue, and is then marked as certified.',
	screen: assetScreen,
	initial: { tab: 'documentation', certified: false, focus: 'mm-page-header', beat: 0 },
	async steps(demo, set) {
		await demo.wait(1200);
		set({ focus: 'mm-owner-chip[kind="team"]' });
		await demo.wait(3000);
		set({ tab: 'schema', field: '', focus: 'mm-schema-table', beat: 1 });
		await demo.wait(1500);
		set({ field: 'net_revenue_gbp', focus: '[data-demo="field-terms"]' });
		await demo.wait(3200);
		set({ focus: '[data-demo="field-actions"]', beat: 2 });
		await demo.wait(1100);
		set({ certified: true });
		await demo.wait(1500);
		set({ focus: 'mm-schema-table' });
		await demo.wait(2600);
	}
};

// The blade fills the screen's height, so framing any part of it would hide the graph. The camera
// stays on the whole screen, graph and blade side by side, and the beats carry what to look at.
export const traceDemo: DemoConfig<LineageState & Focus> = {
	label:
		'The full-screen lineage of orders_daily follows orders from the PostgreSQL table through the Airflow load and the dbt models to the Revenue overview dashboard. An edge declared by the dbt-analytics pipeline and a Spark job reported through OpenLineage open in the blade, then the two dashboards downstream of orders_daily and who to ask about them.',
	screen: lineageScreen,
	initial: { beat: 0 },
	async steps(demo, set) {
		await demo.wait(1000);
		for (const id of LINEAGE_PATH) {
			set({ selected: id });
			await demo.wait(1150);
		}
		set({ selected: LINEAGE_PLUGIN_EDGE, beat: 1 });
		await demo.wait(2400);
		set({ selected: LINEAGE_OPENLINEAGE_JOB });
		await demo.wait(1700);
		set({ details: true });
		await demo.wait(2500);
		set({ selected: LINEAGE_DASHBOARDS[0], details: false, beat: 2 });
		await demo.wait(2800);
		set({ selected: LINEAGE_DASHBOARDS[1] });
		await demo.wait(3400);
	},
	rest: 3000
};

export const accessDemo: DemoConfig<AccessState & Focus> = {
	label:
		'An administrator grants the service account svc-finance-agent the asset.dataQuerier role on orders_daily, and nothing else.',
	screen: accessScreen,
	initial: { granted: false, focus: 'mm-page-header' },
	async steps(demo, set) {
		await demo.wait(1600);
		set({ focus: '[label^="Access of"]' });
		await demo.wait(1600);
		set({ granted: true });
		await demo.wait(4000);
	}
};

type QueryDemoState = QueryState & Focus & { view: 'query' | 'audit'; newest: number };

export const queryDemo: DemoConfig<QueryDemoState> = {
	label:
		'A query on orders_daily runs and returns 7 rows. The next query reads customers_dim, a table the caller has no access to, and is refused. Both land in the query audit.',
	screen: (s) => (s.view === 'audit' ? auditScreen({ newest: s.newest }) : queryScreen(s)),
	initial: { view: 'query', sql: '', state: 'idle', newest: 0, focus: 'mm-sql-editor', beat: 0 },
	async steps(demo, set) {
		await demo.wait(800);
		await type(demo, QUERY_SQL, (sql) => set({ sql }), 16);
		await demo.wait(600);
		set({ state: 'running' });
		await demo.wait(1100);
		set({ state: 'results', focus: 'mm-results-grid' });
		await demo.wait(2800);
		set({ sql: '', focus: 'mm-sql-editor', beat: 1 });
		await type(demo, DENIED_SQL, (sql) => set({ sql }), 18);
		await demo.wait(600);
		set({ state: 'denied', focus: 'mm-callout' });
		await demo.wait(3200);
		set({ view: 'audit', newest: 1, focus: 'mm-audit-log', beat: 2 });
		await demo.wait(1200);
		set({ newest: 2 });
		await demo.wait(4000);
	}
};

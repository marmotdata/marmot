// The tour's scripts. One question threads them: finance wants last week's revenue, an agent
// answers it from orders_daily, and every step happens in the real product screens.
import type { TemplateResult } from 'lit';
import { type, type Demo } from './demo';
import {
	AGENT_QUESTION,
	AGENT_STEP_COUNT,
	DENIED_SQL,
	LINEAGE_PATH,
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

export interface DemoConfig<S extends { focus?: string }> {
	label: string;
	screen: (s: S) => TemplateResult;
	initial: S;
	steps: (demo: Demo, set: (patch: Partial<S>) => void) => Promise<void>;
	rest?: number;
}

type Focus = { focus?: string };

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
		'Someone types “revenue” into Discover, narrows it to tables, and opens orders_daily in the preview.',
	screen: discoverScreen,
	initial: { query: '', focus: 'mm-search-bar' },
	async steps(demo, set) {
		await demo.wait(900);
		await type(demo, 'revenue', (query) => set({ query }), 110);
		await demo.wait(700);
		set({ focus: '[aria-label="Search results"]' });
		await demo.wait(1800);
		set({ query: 'revenue AND @type = table', focus: 'mm-search-bar' });
		await demo.wait(1600);
		set({ selected: 'orders_daily', focus: 'mm-details-blade' });
		await demo.wait(3500);
	}
};

export const understandDemo: DemoConfig<AssetState & Focus> = {
	label:
		'The page for orders_daily shows its owners and description, then its schema, where the net_revenue_gbp field opens with its type and glossary term.',
	screen: assetScreen,
	initial: { tab: 'documentation', focus: 'mm-page-header' },
	async steps(demo, set) {
		await demo.wait(2600);
		set({ tab: 'schema', focus: 'mm-schema-table' });
		await demo.wait(1800);
		set({ field: 'net_revenue_gbp' });
		await demo.wait(4200);
	}
};

export const traceDemo: DemoConfig<LineageState & Focus> = {
	label:
		'Lineage follows orders from the PostgreSQL table through the hourly load and the dbt models to the Revenue overview dashboard.',
	screen: lineageScreen,
	initial: { focus: 'mm-lineage-canvas' },
	async steps(demo, set) {
		for (const id of LINEAGE_PATH) {
			await demo.wait(1300);
			set({ selected: id });
		}
		await demo.wait(900);
		set({ focus: 'mm-details-blade' });
		await demo.wait(3200);
	}
};

export const accessDemo: DemoConfig<AccessState & Focus> = {
	label:
		'An administrator grants the service account svc-finance-agent the asset.dataViewer role on orders_daily, and nothing else.',
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
		'revenue-agent’s query on orders_daily runs and returns 7 rows. Its next query reads a personal data column and is refused. Both land in the query audit.',
	screen: (s) => (s.view === 'audit' ? auditScreen({ newest: s.newest }) : queryScreen(s)),
	initial: { view: 'query', sql: '', state: 'idle', newest: 0, focus: 'mm-sql-editor' },
	async steps(demo, set) {
		await demo.wait(800);
		await type(demo, QUERY_SQL, (sql) => set({ sql }), 16);
		await demo.wait(600);
		set({ state: 'running' });
		await demo.wait(1100);
		set({ state: 'results', focus: 'mm-results-grid' });
		await demo.wait(2800);
		set({ sql: '', state: 'idle', focus: 'mm-sql-editor' });
		await type(demo, DENIED_SQL, (sql) => set({ sql }), 18);
		await demo.wait(600);
		set({ state: 'denied', focus: 'mm-callout' });
		await demo.wait(3200);
		set({ view: 'audit', newest: 1, focus: 'mm-audit-log' });
		await demo.wait(1200);
		set({ newest: 2 });
		await demo.wait(4000);
	}
};

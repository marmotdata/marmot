/**
 * The product's screens, drawn from the Marmot design system exactly as the app draws them, for
 * the landing page's demos. Each takes the state a demo steps through and returns a whole app
 * page. Call `defineScreenElements` from `./elements` in the browser before rendering one.
 */
export { agentScreen, AGENT_QUESTION, AGENT_STEP_COUNT, type AgentState } from './agent';
export { discoverScreen, type DiscoverState } from './discover';
export { assetScreen, type AssetState } from './asset';
export { lineageScreen, LINEAGE_PATH, type LineageState } from './lineage';
export { accessScreen, type AccessState } from './access';
export { queryScreen, QUERY_SQL, DENIED_SQL, type QueryState, type QueryRunState } from './query';
export { auditScreen, type AuditState } from './audit';

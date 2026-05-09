/**
 * LangChain integration for the Marmot TypeScript SDK.
 *
 * Exposes:
 * - {@link catalogTools} — turn a Marmot client into a list of LangChain
 *   tools (search, lookup, get, lineage) that an agent can call.
 * - {@link MarmotCallbackHandler} — auto-register the agent as a Marmot asset
 *   on first run and capture lineage edges from the data sources it touches.
 * - {@link marmotTool} — opt-in helper for declaring the upstream MRN of a
 *   custom (non-Marmot) tool so its usage shows up in lineage.
 *
 * Requires `@langchain/core` as a peer dependency.
 */

export { catalogTools } from "./tools.js";
export {
  MarmotCallbackHandler,
  type MarmotCallbackHandlerOptions,
  marmotTool,
  TOOL_METADATA_KEY,
} from "./callback.js";

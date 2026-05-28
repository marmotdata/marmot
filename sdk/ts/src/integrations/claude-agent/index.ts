/**
 * Claude Agent SDK integration for the Marmot TypeScript SDK.
 *
 * Exposes {@link MarmotAgentTracker}, which auto-registers a Claude Agent SDK
 * agent as a Marmot asset and writes lineage edges for every tool call.
 *
 * Pair with the Marmot MCP server to give the agent catalog-aware tools:
 *
 * ```ts
 * const tracker = new MarmotAgentTracker(client, { name: "catalog-explorer" });
 * for await (const msg of query({
 *   prompt: "Find orders data",
 *   options: {
 *     mcpServers: { marmot: { command: "marmot", args: ["mcp"] } },
 *     hooks: tracker.hooks(),
 *   },
 * })) { ... }
 * ```
 *
 * `@anthropic-ai/claude-agent-sdk` is an optional peer dependency.
 */

export {
  MarmotAgentTracker,
  type MarmotAgentTrackerOptions,
  type MarmotAgentHooks,
} from "./tracker.js";

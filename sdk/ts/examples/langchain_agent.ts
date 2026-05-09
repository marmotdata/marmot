/**
 * LangChain integration example for the Marmot TypeScript SDK.
 *
 * Spins up a small ReAct-style agent that can search the Marmot catalog and
 * auto-registers itself plus its data lineage.
 *
 *     pnpm add @marmotdata/sdk @langchain/core @langchain/openai langchain
 *     OPENAI_API_KEY=... tsx examples/langchain_agent.ts
 *
 * After the first run the agent appears in Marmot as
 * service=langchain, type=agent, name=catalog-explorer, with lineage edges
 * from any assets it touched.
 */

import { ChatOpenAI } from "@langchain/openai";
import { createReactAgent } from "langchain";
import { connect } from "@marmotdata/sdk";
import {
  MarmotCallbackHandler,
  catalogTools,
  marmotTool,
} from "@marmotdata/sdk/langchain";

const summarizeOrdersTable = marmotTool({
  name: "summarize_orders_table",
  description: "Return a short prose summary of the orders table.",
  assetMrn: "postgres://prod/sales/orders",
  schema: { type: "object", properties: {}, required: [] },
  func: async () => "orders has columns id, customer_id, total, created_at.",
});

async function main(): Promise<void> {
  const client = await connect();
  const tools = [...catalogTools(client), summarizeOrdersTable];

  const handler = new MarmotCallbackHandler(client, {
    name: "catalog-explorer",
    model: "gpt-4o-mini",
    owner: "data-eng",
    tools,
  });

  const agent = createReactAgent({
    llm: new ChatOpenAI({ model: "gpt-4o-mini", temperature: 0 }),
    tools,
  });

  const result = await agent.invoke(
    {
      messages: [
        { role: "user", content: "Find a postgres table about orders and summarize it." },
      ],
    },
    { callbacks: [handler] },
  );

  console.log(result);
  console.log("agent registered as:", handler.agentMrn);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

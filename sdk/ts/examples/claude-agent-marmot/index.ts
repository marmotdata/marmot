import { Client, resolve } from "@marmotdata/sdk";
import { MarmotAgentTracker } from "@marmotdata/sdk/claude-agent";
import { query } from "@anthropic-ai/claude-agent-sdk";

const PROMPT =
  process.argv.slice(2).join(" ") ||
  "Use the Marmot catalog tools to find a postgres table related to orders. " +
    "Reply with one sentence summarising the top hit and quoting its MRN.";

async function main() {
  // Run the SDK's own auth chain once: explicit kwargs → env vars →
  // ~/.config/marmot/credentials.json → workload identity. Whatever
  // `marmot login` populated will surface here, and the same credential
  // will authenticate both the SDK and the MCP transport.
  const { baseUrl, credential } = await resolve({});
  const client = new Client({ baseUrl, credential });

  const tracker = new MarmotAgentTracker(client, {
    name: "catalog-explorer-claude",
    model: "claude-sonnet-4-5",
    owner: "data-eng",
  });

  const mcpUrl = `${baseUrl.replace(/\/$/, "")}/api/v1/mcp`;
  const mcpHeaders: Record<string, string> =
    credential.scheme === "Bearer"
      ? { Authorization: `Bearer ${credential.token}` }
      : { "X-API-Key": credential.token };

  console.log(`Marmot host: ${baseUrl} (auth via ${credential.source})`);
  console.log(`Prompt: ${PROMPT}\n`);

  for await (const message of query({
    prompt: PROMPT,
    options: {
      mcpServers: {
        marmot: { type: "http", url: mcpUrl, headers: mcpHeaders },
      },
      hooks: tracker.hooks() as never,
      permissionMode: "bypassPermissions",
      allowedTools: [
        "mcp__marmot__discover_data",
        "mcp__marmot__find_ownership",
        "mcp__marmot__lookup_term",
      ],
    },
  })) {
    if (message.type === "assistant") {
      const blocks = (message as { message: { content: unknown[] } }).message.content;
      const text = blocks
        .filter((b): b is { type: "text"; text: string } =>
          typeof b === "object" && b !== null && (b as { type?: string }).type === "text",
        )
        .map((b) => b.text)
        .join("");
      if (text) console.log(text);
    }
  }

  console.log("\nagent registered as:", tracker.agentMrn ?? "(not yet registered)");
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});

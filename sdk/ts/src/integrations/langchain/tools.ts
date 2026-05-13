import { tool } from "@langchain/core/tools";
import type { Client } from "../../client.js";
import { NotFoundError } from "../../errors.js";

interface SearchInput {
  query: string;
  limit?: number;
}

interface IdInput {
  assetId: string;
}

interface LookupInput {
  assetType: string;
  service: string;
  name: string;
}

interface LineageInput {
  assetId: string;
  depth?: number;
}

/**
 * Return a list of LangChain tools that read from the given Marmot client.
 *
 * The tools are bound to `client`; they share its auth and HTTP session.
 *
 * ```ts
 * import { connect } from "@marmotdata/sdk";
 * import { catalogTools } from "@marmotdata/sdk/langchain";
 *
 * const client = await connect();
 * const tools = catalogTools(client);
 * ```
 */
export function catalogTools(client: Client) {
  const searchCatalog = tool(
    async (input: SearchInput) => {
      const raw = (await client.search(input.query, {
        limit: input.limit ?? 20,
      })) as {
        results?: Array<{
          id?: string;
          name?: string;
          description?: string;
          metadata?: { type?: string; primary_provider?: string; mrn?: string };
        }>;
        total?: number;
      };
      const hits = (raw.results ?? []).map((r) => ({
        id: r.id,
        name: r.name,
        type: r.metadata?.type,
        provider: r.metadata?.primary_provider,
        mrn: r.metadata?.mrn,
        description: r.description,
      }));
      return { results: hits, total: raw.total ?? hits.length };
    },
    {
      name: "search_catalog",
      description: [
        "Search the Marmot data catalog. Returns up to `limit` matches (max 100).",
        "",
        "`query` accepts plain free text OR Marmot's structured query language.",
        "Catalogs can hold millions of assets — prefer structured queries over",
        "broad free-text when you know any of: name, type, provider, or metadata.",
        "",
        "Field filters (combine with AND / OR / NOT, group with parentheses):",
        '  @type: "Table"             - asset type (Table, Topic, Bucket, Alias, Agent...)',
        '  @provider: "postgres"      - source platform (postgres, kafka, OpenSearch...)',
        '  @name: "users"             - exact name match',
        '  @name contains "customer"  - substring on name',
        '  @name: "customer*"         - wildcard',
        '  @metadata.team: "platform" - any metadata key (dot notation for nested)',
        "  @metadata.partitions > 10  - numeric: > < >= <=",
        "  @metadata.size range [100 TO 500]",
        "",
        "Examples — pick the most specific query you can:",
        '  By name:                @name: "metrics-current"',
        '  Name on a platform:     @name: "metrics-current" AND @provider: "OpenSearch"',
        '  All Kafka topics:       @type: "Topic" AND @provider: "kafka"',
        '  Postgres customers:     (@type: "Table" OR @type: "View") AND @provider: "postgres" AND @name contains "customer"',
        "  Free-text fallback:     user orders",
        "",
        "After this returns, use `lookup_asset` (when you know type+provider+name)",
        "or `get_asset` (when you have an id from these results) for full details.",
      ].join("\n"),
      schema: {
        type: "object",
        properties: {
          query: {
            type: "string",
            description:
              'Marmot query — free text or structured (e.g. `@name: "metrics-current"`).',
          },
          limit: { type: "integer", minimum: 1, maximum: 100 },
        },
        required: ["query"],
      },
    },
  );

  const getAsset = tool(async (input: IdInput) => await client.assets.get(input.assetId), {
    name: "get_asset",
    description:
      "Fetch the full details of a single asset by its Marmot ID. " +
      "Returns name, MRN, type, provider, description, owner, schema, " +
      "and provider-specific metadata. Use after search_catalog when you " +
      "need column/schema details.",
    schema: {
      type: "object",
      properties: {
        assetId: { type: "string", description: "The Marmot asset ID." },
      },
      required: ["assetId"],
    },
  });

  const lookupAsset = tool(
    async (input: LookupInput) => {
      try {
        return await client.assets.lookup({
          type: input.assetType,
          service: input.service,
          name: input.name,
        });
      } catch (e) {
        if (e instanceof NotFoundError) return null;
        throw e;
      }
    },
    {
      name: "lookup_asset",
      description:
        "Look up a single asset by its (type, service, name) triple. Use " +
        "this when you already know the natural identifiers — for example " +
        "type='table', service='postgres', name='prod.orders'. Returns " +
        "null if no asset matches.",
      schema: {
        type: "object",
        properties: {
          assetType: { type: "string" },
          service: { type: "string" },
          name: { type: "string" },
        },
        required: ["assetType", "service", "name"],
      },
    },
  );

  const getUpstreamLineage = tool(
    async (input: LineageInput) =>
      await client.lineage.upstream(input.assetId, { depth: input.depth ?? 2 }),
    {
      name: "get_upstream_lineage",
      description:
        "Trace the upstream lineage of an asset — what feeds into it. " +
        "Returns the graph of ancestors up to `depth` hops. Use this to " +
        "understand where data comes from or to find a root source you can " +
        "query directly.",
      schema: {
        type: "object",
        properties: {
          assetId: { type: "string" },
          depth: { type: "integer", minimum: 1, maximum: 10 },
        },
        required: ["assetId"],
      },
    },
  );

  return [searchCatalog, getAsset, lookupAsset, getUpstreamLineage];
}

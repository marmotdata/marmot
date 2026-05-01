/**
 * Quickstart for the Marmot TypeScript SDK.
 *
 * Run against a Marmot instance you've already authenticated with:
 *
 *   marmot login http://localhost:5173
 *   pnpm exec tsx examples/quickstart.ts
 */

import { connect } from "../src/index.js";

async function main() {
  const client = await connect();
  console.log(`Connected to ${client.baseUrl}`);

  const results = (await client.search("postgres", { limit: 3 })) as {
    results?: Array<{ id: string; name: string; type?: string; asset_type?: string }>;
  };
  const hits = results.results ?? [];
  console.log(`\nSearch found ${hits.length} matches:`);
  for (const hit of hits) {
    console.log(`  - ${hit.name} (${hit.type}/${hit.asset_type})`);
  }

  if (hits.length === 0) return;

  const first = hits[0];
  if (!first) return;
  const asset = (await client.assets.get(first.id)) as Record<string, unknown>;
  console.log(`\nFetched asset: ${asset.name}`);
  console.log(`  MRN: ${asset.mrn}`);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});

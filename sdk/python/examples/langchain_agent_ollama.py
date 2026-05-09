"""LangChain integration example using a local Ollama model.

Spins up a langgraph-based ReAct agent backed by a local Ollama model — no
API key needed. Targets LangChain 1.x. Run with::

    cd sdk/python
    uv pip install -e ".[langchain]" langchain langchain-ollama
    ollama pull qwen2.5:14b
    marmot login http://localhost:5173
    uv run python examples/langchain_agent_ollama.py

The agent appears in Marmot as ``service=LangChain, type=Agent,
name=catalog-explorer-ollama`` after the run, with one ``AGENT_LOOKUP``
lineage edge per asset it touched. This example issues batches of small
investigative queries against ~100 different assets in the catalog so the
agent ends up with a wide spread of observed lineage.
"""

from __future__ import annotations

import random
import time

import marmot
from langchain.agents import create_agent
from langchain_ollama import ChatOllama

from marmot.integrations.langchain import MarmotCallbackHandler, catalog_tools

MODEL = "qwen2.5:14b"
TARGET_LOOKUPS = 100
BATCH_SIZE = 5  # assets per agent invocation — small batches keep the
                # context manageable for a 14B local model.

SYSTEM_PROMPT = (
    "You are a data analyst inspecting the Marmot data catalog. "
    "The user will give you a short list of assets identified by "
    "(type, service, name). For each one:\n"
    "  1. Call `lookup_asset` with EXACTLY that (asset_type, service, name) "
    "triple — do not search, do not guess.\n"
    "  2. Read the returned description and any metadata.\n"
    "  3. Reply in one short sentence describing what the asset is.\n\n"
    "Always lookup every asset in the list before replying. Move on to "
    "the next asset even if a lookup returns null."
)


# ---- catalog discovery ---------------------------------------------------

# A spread of structured queries so we hit a mix of providers/types rather
# than 100 dbt models. Each query contributes up to ``per_query`` assets.
DISCOVERY_QUERIES: list[tuple[str, int]] = [
    ('@type:"table" AND @provider:"postgresql"', 18),
    ('@type:"table" AND @provider:"bigquery"', 12),
    ('@type:"table" AND @provider:"snowflake"', 6),
    ('@type:"table" AND @provider:"mysql"', 6),
    ('@type:"model" AND @provider:"dbt"', 14),
    ('@type:"topic" AND @provider:"kafka"', 12),
    ('@type:"topic" AND @provider:"redpanda"', 5),
    ('@type:"topic" AND @provider:"sns"', 4),
    ('@type:"queue" AND @provider:"sqs"', 5),
    ('@type:"function" AND @provider:"lambda"', 5),
    ('@type:"dag" AND @provider:"airflow"', 5),
    ('@type:"bucket" AND @provider:"s3"', 4),
    ('@type:"index" AND @provider:"elasticsearch"', 3),
    ('@type:"dashboard" AND @provider:"tableau"', 3),
    ('@type:"dashboard" AND @provider:"looker"', 2),
]


def pick_assets(client: marmot.Client, target: int) -> list[dict[str, str]]:
    """Discover ``target`` distinct assets across the catalog, biased toward
    a wide provider/type mix rather than the most-relevant matches.
    """
    seen: set[tuple[str, str, str]] = set()
    triples: list[dict[str, str]] = []

    for query, per_query in DISCOVERY_QUERIES:
        try:
            raw = client.search(query, limit=per_query)
        except Exception as e:
            print(f"  search failed for {query!r}: {e}")
            continue
        for r in raw.get("results") or []:
            md = r.get("metadata") or {}
            t = md.get("type")
            p = md.get("primary_provider")
            n = r.get("name")
            if not (t and p and n):
                continue
            key = (t, p, n)
            if key in seen:
                continue
            seen.add(key)
            triples.append({"type": t, "service": p, "name": n})
            if len(triples) >= target:
                return triples

    # Top up with a free-text catch-all if some queries returned nothing.
    if len(triples) < target:
        try:
            raw = client.search("", limit=target * 2)
            for r in raw.get("results") or []:
                md = r.get("metadata") or {}
                t = md.get("type")
                p = md.get("primary_provider")
                n = r.get("name")
                if not (t and p and n):
                    continue
                key = (t, p, n)
                if key in seen:
                    continue
                seen.add(key)
                triples.append({"type": t, "service": p, "name": n})
                if len(triples) >= target:
                    break
        except Exception as e:
            print(f"  catch-all search failed: {e}")

    return triples


# ---- agent loop ----------------------------------------------------------

def render_asset_list(batch: list[dict[str, str]]) -> str:
    return "\n".join(
        f"  {i+1}. type={a['type']!r}, service={a['service']!r}, name={a['name']!r}"
        for i, a in enumerate(batch)
    )


def main() -> None:
    with marmot.connect() as client:
        print(f"discovering up to {TARGET_LOOKUPS} assets to investigate...")
        targets = pick_assets(client, TARGET_LOOKUPS)
        if not targets:
            raise SystemExit(
                "no assets found in catalog — seed some assets first"
            )

        random.seed(7)
        random.shuffle(targets)  # mix providers across batches

        provider_counts: dict[str, int] = {}
        for t in targets:
            provider_counts[t["service"]] = provider_counts.get(t["service"], 0) + 1
        print(
            f"  selected {len(targets)} assets across "
            f"{len(provider_counts)} providers: "
            + ", ".join(f"{p}={c}" for p, c in sorted(provider_counts.items()))
        )

        tools = catalog_tools(client)

        agent = create_agent(
            model=ChatOllama(model=MODEL, temperature=0),
            tools=tools,
            system_prompt=SYSTEM_PROMPT,
        )

        handler = MarmotCallbackHandler(
            client,
            name="catalog-explorer-ollama",
            model=MODEL,
            owner="data-eng",
            tools=tools,
        )

        total_batches = (len(targets) + BATCH_SIZE - 1) // BATCH_SIZE
        ok_batches = 0
        t0 = time.time()

        for i in range(0, len(targets), BATCH_SIZE):
            batch = targets[i:i + BATCH_SIZE]
            batch_idx = i // BATCH_SIZE + 1
            print(
                f"\n=== batch {batch_idx}/{total_batches} "
                f"({len(batch)} assets) ==="
            )
            print(render_asset_list(batch))

            prompt = (
                "Look up each of these assets using `lookup_asset` and "
                "describe each in one short sentence:\n\n"
                + render_asset_list(batch)
            )

            try:
                result = agent.invoke(
                    {"messages": [{"role": "user", "content": prompt}]},
                    config={"callbacks": [handler]},
                )
                # Print the final assistant message only — full traces are
                # noisy when we're running 20 batches.
                final = result["messages"][-1]
                content = getattr(final, "content", "")
                if isinstance(content, list):  # multi-modal blocks
                    content = " ".join(
                        c.get("text", "") if isinstance(c, dict) else str(c)
                        for c in content
                    )
                snippet = (content or "").strip().splitlines()
                if snippet:
                    print(f"  -> {snippet[0][:200]}")
                ok_batches += 1
            except KeyboardInterrupt:
                print("interrupted — stopping early")
                break
            except Exception as e:
                print(f"  batch {batch_idx} failed: {type(e).__name__}: {e}")

        elapsed = time.time() - t0
        print(
            f"\n{ok_batches}/{total_batches} batches succeeded in "
            f"{elapsed:.0f}s — agent registered as: {handler.agent_mrn}"
        )

        # Read back the agent's recorded run history so the user can see
        # how many distinct assets ended up on the agent's lineage.
        try:
            agent_asset = client.assets.find(
                type="Agent", service="LangChain", name="catalog-explorer-ollama"
            )
            if agent_asset and agent_asset.get("id"):
                runs = client.agent_runs.list(agent_asset["id"], period="24h", limit=200)
                tool_targets: set[str] = set()
                for r in runs.get("runs") or []:
                    for tc in r.get("tool_calls") or []:
                        if tc.get("target_mrn"):
                            tool_targets.add(tc["target_mrn"])
                print(
                    f"agent has {len(runs.get('runs') or [])} runs in 24h and "
                    f"observed {len(tool_targets)} distinct asset MRNs"
                )
        except Exception as e:
            print(f"  failed to read back run summary: {e}")


if __name__ == "__main__":
    main()

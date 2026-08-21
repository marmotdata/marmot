"""Runnable example: the LangChain integration against a local Marmot.

Drives a real LangChain chain with a *scripted* chat model, so the whole
callback path runs — agent registration, tool-call capture, lineage edges and
the run record — without needing an LLM provider or API key. Swap
``_scripted_model`` for ChatAnthropic/ChatOllama/etc. to let a real model choose
the tools instead.

Usage:
    pip install "marmot-sdk[langchain]"
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_API_KEY=...            # or run `marmot login` first
    python main.py
"""

from __future__ import annotations

from langchain_core.language_models import FakeMessagesListChatModel
from langchain_core.messages import AIMessage, HumanMessage
from langchain_core.messages.ai import UsageMetadata
from langchain_core.runnables import RunnableConfig, RunnableLambda

from marmot import AuthenticatedApiClient
from marmot.auth import resolve_credential, resolve_host
from marmot.integrations import MarmotCatalog
from marmot.integrations.langchain import MarmotCallbackHandler, catalog_tools, marmot_tool

AGENT_NAME = "orders-analyst-langchain-py"
MODEL = "scripted-test-model"
OWNER = "mock-owner"
DEFAULT_PROMPT = (
    "What is the primary table for orders?"
)


@marmot_tool(asset_mrn="postgres://prod/sales/orders")
def query_orders(sql: str) -> str:
    """Run a read-only SQL query against the orders table."""
    return "42 rows"


def _scripted_model() -> FakeMessagesListChatModel:
    """Stands in for a provider: replies once, with token usage attached."""
    reply = AIMessage(
        content="The orders table is the primary sales fact table.",
        usage_metadata=UsageMetadata(input_tokens=120, output_tokens=18, total_tokens=138),
    )
    return FakeMessagesListChatModel(responses=[reply])


def main() -> None:
    host = resolve_host()
    credential = resolve_credential(host)
    catalog = MarmotCatalog(AuthenticatedApiClient(host, credential))
    print(f"Marmot host: {host} (auth via {credential.source})")

    tools = [*catalog_tools(catalog), query_orders]
    handler = MarmotCallbackHandler(
        catalog,
        name=AGENT_NAME,
        model=MODEL,
        owner=OWNER,
        tools=tools,
    )

    search = next(tool for tool in tools if tool.name == "search_catalog")
    model = _scripted_model()

    # The handler tracks a *chain* run: it registers on the root chain start and
    # flushes on its end, attributing every nested tool and model call to it.
    # A real agent supplies that chain; here one stands in and calls the tools a
    # model would have chosen. `config` must be forwarded so the nested runs
    # inherit the root.
    def pipeline(question: str, config: RunnableConfig) -> str:
        print("search_catalog ->", search.invoke({"query": "orders", "limit": 3}, config=config))
        print("query_orders ->", query_orders.invoke({"sql": "select 1"}, config=config))
        answer = model.invoke([HumanMessage(question)], config=config)
        return str(answer.content)

    reply = RunnableLambda(pipeline).invoke(
        "Summarise the orders table", config=RunnableConfig(callbacks=[handler])
    )
    print("model ->", reply)

    print("\nagent registered as:", handler.agent_mrn or "(not registered)")
    print(f"check the UI: /discover/Agent/LangChain/{AGENT_NAME}?tab=runs")


if __name__ == "__main__":
    main()

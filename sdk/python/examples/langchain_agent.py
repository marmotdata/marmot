"""LangChain integration example for the Marmot SDK.

Spins up a small ReAct-style agent that can search the Marmot catalog and
auto-registers itself plus its data lineage. Run with::

    pip install marmot-sdk[langchain] langchain langchain-openai
    marmot login http://localhost:5173
    OPENAI_API_KEY=... uv run python examples/langchain_agent.py

The agent will appear in Marmot as ``service=LangChain, type=Agent,
name=catalog-explorer`` after the first run, with lineage edges from any
assets it touched during the conversation.
"""

from __future__ import annotations

import marmot
from marmot.integrations.langchain import (
    MarmotCallbackHandler,
    catalog_tools,
    marmot_tool,
)


@marmot_tool(asset_mrn="postgres://prod/sales/orders")
def summarize_orders_table() -> str:
    """Return a short prose summary of the orders table.

    In a real app this would query the table; here it's a stub used to
    demonstrate that custom tools tagged with ``@marmot_tool`` show up in
    lineage automatically.
    """
    return "orders has columns id, customer_id, total, created_at."


def main() -> None:
    try:
        from langchain.agents import AgentExecutor, create_tool_calling_agent
        from langchain_core.prompts import ChatPromptTemplate
        from langchain_openai import ChatOpenAI
    except ImportError as e:
        raise SystemExit(
            "this example needs `langchain`, `langchain-openai`. "
            "install: pip install langchain langchain-openai"
        ) from e

    with marmot.connect() as client:
        tools = catalog_tools(client) + [summarize_orders_table]

        prompt = ChatPromptTemplate.from_messages(
            [
                ("system", "You are a data analyst with access to the Marmot catalog."),
                ("human", "{input}"),
                ("placeholder", "{agent_scratchpad}"),
            ]
        )
        llm = ChatOpenAI(model="gpt-4o-mini", temperature=0)
        agent = create_tool_calling_agent(llm, tools, prompt)
        executor = AgentExecutor(agent=agent, tools=tools, verbose=True)

        handler = MarmotCallbackHandler(
            client,
            name="catalog-explorer",
            model="gpt-4o-mini",
            owner="data-eng",
            tools=tools,
        )

        executor.invoke(
            {"input": "Find me a postgres table about orders and summarize it."},
            config={"callbacks": [handler]},
        )

        print(f"\nagent registered as: {handler.agent_mrn}")


if __name__ == "__main__":
    main()

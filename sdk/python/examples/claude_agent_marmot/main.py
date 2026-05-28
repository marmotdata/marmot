"""Runnable example: a Claude Agent SDK agent against the Marmot MCP server.

Auto-registers the agent as a Marmot Agent asset and writes lineage edges for
every catalog tool the agent calls. Mirrors `sdk/ts/examples/claude-agent-marmot`.

Usage:
    pip install marmot-sdk[claude-agent]
    python main.py [prompt...]

The Marmot host + credential is resolved via the SDK's standard chain:
explicit kwargs → MARMOT_HOST/MARMOT_API_KEY/MARMOT_TOKEN env vars →
~/.config/marmot/credentials.json (populated by `marmot login`) → workload
identity.
"""

from __future__ import annotations

import asyncio
import sys

from claude_agent_sdk import ClaudeAgentOptions, ClaudeSDKClient

from marmot import Client, resolve
from marmot.integrations.claude_agent import MarmotAgentTracker

DEFAULT_PROMPT = (
    "Use the Marmot catalog tools to find a postgres table related to orders. "
    "Reply with one sentence summarising the top hit and quoting its MRN."
)


async def main() -> None:
    prompt = " ".join(sys.argv[1:]) or DEFAULT_PROMPT

    base_url, credential = resolve(base_url=None)
    client = Client(base_url=base_url, credential=credential)
    tracker = MarmotAgentTracker(
        client,
        name="catalog-explorer-claude-py",
        model="claude-sonnet-4-5",
        owner="data-eng",
    )

    mcp_headers = (
        {"Authorization": f"Bearer {credential.token}"}
        if credential.scheme == "Bearer"
        else {"X-API-Key": credential.token}
    )
    options = ClaudeAgentOptions(
        mcp_servers={
            "marmot": {
                "type": "http",
                "url": f"{base_url}/api/v1/mcp",
                "headers": mcp_headers,
            }
        },
        hooks=tracker.hooks(),
        permission_mode="bypassPermissions",
        allowed_tools=[
            "mcp__marmot__discover_data",
            "mcp__marmot__find_ownership",
            "mcp__marmot__lookup_term",
        ],
    )

    print(f"Marmot host: {base_url} (auth via {credential.source})")
    print(f"Prompt: {prompt}\n")

    async with ClaudeSDKClient(options=options) as agent:
        await agent.query(prompt)
        async for msg in agent.receive_response():
            blocks = getattr(getattr(msg, "message", None), "content", None) or []
            for block in blocks:
                text = getattr(block, "text", None)
                if isinstance(text, str) and text:
                    print(text)

    print("\nagent registered as:", tracker.agent_mrn or "(not yet registered)")


if __name__ == "__main__":
    asyncio.run(main())

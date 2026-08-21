"""Runnable example: a Claude Agent SDK agent against the Marmot MCP server.

Auto-registers the agent as a Marmot Agent asset and writes lineage edges for
every catalog tool the agent calls. Mirrors `sdk/ts/examples/claude-agent-marmot`.

Usage:
    pip install "marmot-sdk[claude-agent]"
    export MARMOT_HOST=http://localhost:8080
    export MARMOT_API_KEY=...            # or run `marmot login` first
    python main.py [prompt...]

The Marmot host + credential is resolved via the SDK's standard chain:
explicit kwargs → MARMOT_API_KEY/MARMOT_TOKEN env vars → the token cached by
`marmot login` → workload identity. MARMOT_HOST is the bare server address; the
SDK appends the API base path itself, and the MCP endpoint sits under it.
"""

from __future__ import annotations

import asyncio
import sys

from claude_agent_sdk import (
    ClaudeAgentOptions,
    ClaudeSDKClient,
    ResultMessage,
    TextBlock,
    ToolUseBlock,
)

from marmot import AuthenticatedApiClient, mcp_url
from marmot.auth import SecurityScheme, resolve_credential, resolve_host
from marmot.integrations import MarmotCatalog
from marmot.integrations.claude_agent import MarmotAgentTracker

AGENT_NAME = "catalog-explorer-claude-py"
MODEL = "claude-sonnet-4-5"
OWNER = "mock-owner"
DEFAULT_PROMPT = "What is the primary table for order information? Use the Marmot catalog"
SYSTEM_PROMPT = (
    "You answer questions about an organisation's data using the Marmot catalog. "
    "Always consult the marmot tools; never guess from memory."
)


async def main() -> None:
    prompt = " ".join(sys.argv[1:]) or DEFAULT_PROMPT

    host = resolve_host()
    credential = resolve_credential(host)
    client = AuthenticatedApiClient(host, credential)
    catalog = MarmotCatalog(client)
    tracker = MarmotAgentTracker(
        catalog,
        name=AGENT_NAME,
        model=MODEL,
        owner=OWNER,
    )

    token = credential.get_token()
    mcp_headers = (
        {"Authorization": f"Bearer {token}"}
        if credential.scheme is SecurityScheme.bearer
        else {"X-API-Key": token}
    )
    options = ClaudeAgentOptions(
        mcp_servers={
            "marmot": {
                "type": "http",
                "url": mcp_url(client),
                "headers": mcp_headers,
            }
        },
        hooks=tracker.hooks(),
        permission_mode="bypassPermissions",
        # Without this the agent loads the developer's own settings and CLAUDE.md
        # files, and answers catalog questions from that context instead of asking
        # Marmot.
        setting_sources=[],
        system_prompt=SYSTEM_PROMPT,
        allowed_tools=[
            "mcp__marmot__discover_data",
            "mcp__marmot__find_ownership",
            "mcp__marmot__lookup_term",
        ],
    )

    print(f"Marmot host: {host} (auth via {credential.source})")
    print(f"Prompt: {prompt}\n")

    async with ClaudeSDKClient(options=options) as agent:
        await agent.query(prompt)
        async for message in agent.receive_response():
            for block in getattr(message, "content", None) or []:
                if isinstance(block, TextBlock):
                    print(block.text)
                elif isinstance(block, ToolUseBlock):
                    print(f"[calling {block.name}]")
            if isinstance(message, ResultMessage) and message.is_error:
                print(f"agent error: {message.errors or message.result}", file=sys.stderr)

    print("\nagent registered as:", tracker.agent_mrn or "(not yet registered)")
    print(f"check the UI: /discover/Agent/LangChain/{AGENT_NAME}?tab=runs")


if __name__ == "__main__":
    asyncio.run(main())

"""Claude Agent SDK integration for the Marmot SDK.

Exposes :class:`MarmotAgentTracker`, which auto-registers a Claude Agent SDK
agent as a Marmot ``Agent`` asset and writes lineage edges for every tool
call. Pair with the Marmot MCP server (built into your Marmot instance at
``/api/v1/mcp``) to give the agent catalog-aware tools.

Requires ``claude-agent-sdk``. Install via
``pip install marmot-sdk[claude-agent]``.
"""

from __future__ import annotations

from marmot.integrations.claude_agent._tracker import MarmotAgentTracker

__all__ = ["MarmotAgentTracker"]

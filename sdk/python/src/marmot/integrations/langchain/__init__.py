"""LangChain integration for the Marmot SDK.

Exposes:

- :func:`catalog_tools` — turn a Marmot client into a list of LangChain tools
  (search, lookup, get, lineage) that an agent can call.
- :class:`MarmotCallbackHandler` — auto-register the agent as a Marmot asset
  on first run and capture lineage edges from the data sources it touches.
- :func:`marmot_tool` / :class:`MarmotTool` — opt-in mechanisms for declaring
  the upstream MRN of a custom (non-Marmot) tool so its usage shows up in
  lineage.

Requires ``langchain-core``. Install via ``pip install marmot-sdk[langchain]``.
"""

from __future__ import annotations

from marmot.integrations.langchain._callback import (
    MarmotCallbackHandler,
    MarmotTool,
    marmot_tool,
)
from marmot.integrations.langchain._tools import catalog_tools

__all__ = [
    "MarmotCallbackHandler",
    "MarmotTool",
    "catalog_tools",
    "marmot_tool",
]

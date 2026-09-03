"""Optional integrations with third-party libraries.

Each subpackage requires its own extra to be installed, e.g.

    pip install marmot-sdk[langchain]
"""

from marmot.integrations.catalog import (
    AgentRegistry,
    AgentRunRecord,
    AgentSpec,
    CatalogReader,
    MarmotCatalog,
    ToolCall,
)

__all__ = [
    "AgentRegistry",
    "AgentRunRecord",
    "AgentSpec",
    "CatalogReader",
    "MarmotCatalog",
    "ToolCall",
]

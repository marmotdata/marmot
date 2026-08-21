"""Callback handler that registers a LangChain agent in Marmot and writes
lineage edges for the data sources it touches during a run."""

from __future__ import annotations

import functools
import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Any
from uuid import UUID

from marmot.generated import LineageEdge
from marmot.integrations import shared
from marmot.integrations.catalog import (
    AGENT_INVOKES,
    AgentRunRecord,
    AgentSpec,
    ToolCall,
)

if TYPE_CHECKING:
    from collections.abc import Callable

    from langchain_core.outputs import LLMResult
    from langchain_core.tools import BaseTool

    from marmot.integrations.catalog import AgentRegistry

try:
    from langchain_core.callbacks import BaseCallbackHandler as _BaseCallbackHandler
    from langchain_core.messages import BaseMessage
    from langchain_core.outputs import ChatGeneration

    _LANGCHAIN_AVAILABLE = True
except ImportError:
    _BaseCallbackHandler = object  # type: ignore[assignment,misc]
    # `isinstance(x, ())` is always False, so the checks below degrade cleanly
    # when langchain is absent — the handler cannot be constructed anyway.
    BaseMessage = ()  # type: ignore[assignment,misc]
    ChatGeneration = ()  # type: ignore[assignment,misc]
    _LANGCHAIN_AVAILABLE = False


_LOG = logging.getLogger("marmot.integrations.langchain")

_DEFAULT_SERVICE = "LangChain"
_FRAMEWORK = "LangChain"
_TOOL_METADATA_KEY = "marmot_asset_mrn"
# Tools may opt in to having their *output* mined for asset MRNs by setting
# this metadata flag. Used by lookup-style tools (e.g. get_asset, lookup_asset)
# whose return value identifies the specific asset the agent fetched.
# Search-style tools that list candidates should NOT set this — otherwise every
# search result becomes a spurious lineage edge.
_TOOL_RECORD_LOOKUPS_KEY = "marmot_record_lookups"


class MarmotCallbackHandler(_BaseCallbackHandler):  # type: ignore[misc,valid-type]
    """LangChain callback handler that auto-registers the agent and captures
    lineage to the data sources it reads.

    Usage::

        from langchain_core.runnables import RunnableConfig
        from marmot import AuthenticatedApiClient
        from marmot.integrations import MarmotCatalog
        from marmot.integrations.langchain import MarmotCallbackHandler

        catalog = MarmotCatalog(AuthenticatedApiClient.connect())
        handler = MarmotCallbackHandler(
            catalog,
            name="orders-analyst",
            model="claude-opus-4-7",
            owner="data-eng",
        )
        agent.invoke(
            {"input": "..."},
            config=RunnableConfig(callbacks=[handler]),
        )

    The first time the handler observes a chain start, it upserts an asset
    of type ``Agent`` keyed by ``(service="LangChain", name=name)``. As the
    agent runs, every tool call that resolves to an asset MRN is collected;
    on chain end (or error), a single batched lineage write attributes those
    edges to the agent.

    See :func:`marmot_tool` and :class:`MarmotTool` for declaring upstream
    MRNs on custom tools, or call :meth:`record_source` from inside a tool.
    """

    def __init__(
        self,
        catalog: AgentRegistry,
        *,
        name: str,
        service: str = _DEFAULT_SERVICE,
        model: str | None = None,
        version: str | None = None,
        owner: str | None = None,
        tools: list[BaseTool] | None = None,
        system_prompt: str | None = None,
        extra_metadata: dict[str, Any] | None = None,
    ) -> None:
        if not _LANGCHAIN_AVAILABLE:
            raise ImportError(
                "langchain-core is required for MarmotCallbackHandler. "
                "Install via `pip install marmot-sdk[langchain]`."
            )
        self._catalog = catalog
        self._tools = tools
        self._spec = AgentSpec(
            name=name,
            service=service,
            framework=_FRAMEWORK,
            model=model,
            version=version,
            owner=owner,
            tool_names=[t.name for t in tools] if tools else None,
            system_prompt_hash=shared.sha256_hex(system_prompt)[:16] if system_prompt else None,
            extra_metadata=extra_metadata or {},
        )

        self._agent_mrn: str | None = None
        self._agent_id: str | None = None

        # Per-run accumulators, keyed by the root chain run_id.
        self._root_of: dict[UUID, UUID] = {}
        self._upstreams: dict[UUID, set[str]] = {}
        self._run_started: dict[UUID, datetime] = {}
        self._tool_traces: dict[UUID, list[ToolCall]] = {}
        self._tokens: dict[UUID, list[int]] = {}  # [in, out]
        self._run_error: dict[UUID, str] = {}

        # In-flight tool calls, keyed by the tool's own run_id (not the root).
        self._tool_open: dict[UUID, dict[str, Any]] = {}

    @property
    def agent_mrn(self) -> str | None:
        """The MRN of the registered agent asset, once it has been upserted."""
        return self._agent_mrn

    def record_source(self, mrn: str, *, run_id: UUID | None = None) -> None:
        """Manually record an upstream MRN as having been read during the
        current (or specified) run. Call this from inside a custom tool
        implementation when neither :func:`marmot_tool` nor
        :class:`MarmotTool` is convenient.
        """
        root = self._root_of.get(run_id, run_id) if run_id else next(iter(self._upstreams), None)
        if root is None:
            _LOG.debug("record_source called outside of any active run; ignoring")
            return
        self._upstreams.setdefault(root, set()).add(mrn)

    def on_chain_start(
        self,
        serialized: dict[str, Any] | None,
        inputs: dict[str, Any],
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        if parent_run_id is None:
            self._root_of[run_id] = run_id
            self._upstreams[run_id] = set()
            self._run_started[run_id] = datetime.now(timezone.utc)
            self._tool_traces[run_id] = []
            self._tokens[run_id] = [0, 0]
            self._ensure_agent_registered()
        else:
            root = self._root_of.get(parent_run_id, parent_run_id)
            self._root_of[run_id] = root

    def on_tool_start(
        self,
        serialized: dict[str, Any] | None,
        input_str: str,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> None:
        root = self._resolve_root(run_id, parent_run_id)
        if root is None:
            return
        self._root_of[run_id] = root

        mrn = (metadata or {}).get(_TOOL_METADATA_KEY)
        if isinstance(mrn, str) and mrn:
            self._upstreams.setdefault(root, set()).add(mrn)

        self._tool_open[run_id] = {
            "tool_name": (serialized or {}).get("name") or kwargs.get("name") or "tool",
            "target_mrn": mrn if isinstance(mrn, str) and mrn else None,
            "started_at": datetime.now(timezone.utc),
            "record_lookups": bool((metadata or {}).get(_TOOL_RECORD_LOOKUPS_KEY, False)),
        }

    def on_tool_end(
        self,
        output: Any,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        root = self._resolve_root(run_id, parent_run_id)
        if root is None:
            self._tool_open.pop(run_id, None)
            return
        opened = self._tool_open.get(run_id, {})
        if opened.get("record_lookups"):
            for mrn in extract_mrns(output):
                self._upstreams.setdefault(root, set()).add(mrn)
        self._close_tool_call(run_id, root, status="success")

    def on_tool_error(
        self,
        error: BaseException,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        root = self._resolve_root(run_id, parent_run_id)
        if root is None:
            self._tool_open.pop(run_id, None)
            return
        self._close_tool_call(run_id, root, status="error")

    def _close_tool_call(self, tool_run_id: UUID, root_run_id: UUID, *, status: str) -> None:
        opened = self._tool_open.pop(tool_run_id, None)
        if opened is None:
            return
        ended = datetime.now(timezone.utc)
        duration_ms = max(0, int((ended - opened["started_at"]).total_seconds() * 1000))
        self._tool_traces.setdefault(root_run_id, []).append(
            ToolCall(
                tool_name=opened["tool_name"],
                target_mrn=opened["target_mrn"],
                started_at=opened["started_at"],
                duration_ms=duration_ms,
                status=status,
            )
        )

    def on_llm_end(
        self,
        response: LLMResult,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        root = self._resolve_root(run_id, parent_run_id)
        if root is None:
            return
        tin, tout = _extract_tokens(response)
        if tin == 0 and tout == 0:
            return
        bucket = self._tokens.setdefault(root, [0, 0])
        bucket[0] += tin
        bucket[1] += tout

    def on_retriever_start(
        self,
        serialized: dict[str, Any] | None,
        query: str,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        metadata: dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> None:
        root = self._resolve_root(run_id, parent_run_id)
        if root is None:
            return
        self._root_of[run_id] = root
        mrn = (metadata or {}).get(_TOOL_METADATA_KEY)
        if isinstance(mrn, str) and mrn:
            self._upstreams.setdefault(root, set()).add(mrn)

    def on_chain_end(
        self,
        outputs: dict[str, Any],
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        if parent_run_id is None:
            self._flush(run_id)

    def on_chain_error(
        self,
        error: BaseException,
        *,
        run_id: UUID,
        parent_run_id: UUID | None = None,
        **kwargs: Any,
    ) -> None:
        if parent_run_id is None:
            self._run_error[run_id] = f"{type(error).__name__}: {error}"
            self._flush(run_id)

    def _resolve_root(self, run_id: UUID, parent_run_id: UUID | None) -> UUID | None:
        if run_id in self._root_of:
            return self._root_of[run_id]
        if parent_run_id is not None and parent_run_id in self._root_of:
            return self._root_of[parent_run_id]
        return None

    def _flush(self, root_run_id: UUID) -> None:
        # Pull and clear all per-run state in one place so a partial failure
        # doesn't leak between runs.
        started_at = self._run_started.pop(root_run_id, None)
        tool_calls = self._tool_traces.pop(root_run_id, [])
        tokens = self._tokens.pop(root_run_id, [0, 0])
        error = self._run_error.pop(root_run_id, "")
        upstreams = self._upstreams.pop(root_run_id, set())
        # Clear any tool-open entries that belonged to this run (defensive — they
        # should already be gone through on_tool_end/on_tool_error).
        self._tool_open = {
            k: v for k, v in self._tool_open.items() if self._root_of.get(k) != root_run_id
        }
        # Garbage-collect run_id → root mappings for this run.
        self._root_of = {k: v for k, v in self._root_of.items() if v != root_run_id}

        if started_at is None or not self._agent_mrn:
            return

        ended_at = datetime.now(timezone.utc)
        status = "error" if error else "success"

        # Observed MRNs that aren't already represented as a tool_call.target_mrn
        # (e.g. catalog-traversal tools where the touched MRN comes out of the
        # tool's *output* rather than declared in metadata). Drop the agent's
        # own MRN — the agent encountering itself in a search result shouldn't
        # produce a self-loop.
        explicit = {call.target_mrn for call in tool_calls if call.target_mrn}
        observed_extras = sorted((upstreams - explicit) - {self._agent_mrn}) if upstreams else []

        try:
            self._catalog.record_run(
                AgentRunRecord(
                    agent_mrn=self._agent_mrn,
                    run_id=str(root_run_id),
                    started_at=started_at,
                    ended_at=ended_at,
                    status=status,
                    model=self._spec.model,
                    tokens_in=tokens[0],
                    tokens_out=tokens[1],
                    error=error or None,
                    tool_calls=tool_calls,
                    observed_assets=observed_extras,
                )
            )
        except Exception as e:
            _LOG.warning("failed to record Marmot agent run: %s", e)

    def _ensure_agent_registered(self) -> None:
        if self._agent_mrn is not None:
            return
        try:
            asset = self._catalog.register_agent(self._spec)
        except Exception as e:
            _LOG.warning("failed to register Marmot agent asset: %s", e)
            return

        self._agent_id = _str_or_none(asset.id)
        self._agent_mrn = _str_or_none(asset.mrn)
        self._emit_declared_invocations()

    def _emit_declared_invocations(self) -> None:
        """Emit one ``AGENT_INVOKES`` edge per tool that declares an upstream
        MRN at construction time. The server treats these as ``declared`` edges
        and they are stable across runs — repeated emission is a safe no-op via
        the existing ``(source, target, event_id)`` uniqueness.
        """
        if not self._agent_mrn or not self._tools:
            return
        edges = [
            LineageEdge(source=self._agent_mrn, target=mrn, type=AGENT_INVOKES)
            for tool in self._tools
            if (mrn := _tool_asset_mrn(tool))
        ]
        if not edges:
            return
        try:
            self._catalog.write_edges(edges)
        except Exception as e:
            _LOG.warning("failed to write %s edges: %s", AGENT_INVOKES, e)


def marmot_tool(
    *,
    asset_mrn: str,
    name: str | None = None,
    description: str | None = None,
) -> Callable[[Callable[..., Any]], BaseTool]:
    """Decorator that turns a function into a LangChain tool tagged with the
    upstream MRN it reads. The :class:`MarmotCallbackHandler` will pick up
    that tag and record an edge from ``asset_mrn`` to the agent on every call.

    Example::

        @marmot_tool(asset_mrn="postgres://prod/sales/orders")
        def query_orders(sql: str) -> list[dict]:
            \"\"\"Run a read-only SQL query against the orders table.\"\"\"
            return run_sql(sql)
    """
    try:
        from langchain_core.tools import StructuredTool
    except ImportError as e:
        raise ImportError(
            "langchain-core is required for marmot_tool. "
            "Install via `pip install marmot-sdk[langchain]`."
        ) from e

    def decorator(fn: Callable[..., Any]) -> BaseTool:
        tool = StructuredTool.from_function(
            fn,
            name=name or fn.__name__,
            description=description or (fn.__doc__ or fn.__name__),
            metadata={_TOOL_METADATA_KEY: asset_mrn},
        )
        functools.update_wrapper(tool, fn, updated=())  # type: ignore[arg-type]
        return tool

    return decorator


class MarmotTool:
    """Mixin that declares an upstream Marmot MRN on a custom
    :class:`langchain_core.tools.BaseTool` subclass.

    Tools that subclass :class:`BaseTool` directly can either set
    ``marmot_asset_mrn`` as a class attribute or inherit from this mixin and
    set it via the constructor::

        class OrdersTool(MarmotTool, BaseTool):
            name = "orders"
            marmot_asset_mrn = "postgres://prod/sales/orders"

    The :class:`MarmotCallbackHandler` reads ``metadata`` from each tool
    invocation; you must surface the MRN there. The default ``__init__``
    below populates ``self.metadata`` for you.
    """

    marmot_asset_mrn: str | None = None

    def __init__(self, *args: Any, marmot_asset_mrn: str | None = None, **kwargs: Any) -> None:
        super().__init__(*args, **kwargs)
        if marmot_asset_mrn is not None:
            self.marmot_asset_mrn = marmot_asset_mrn
        if self.marmot_asset_mrn:
            existing = getattr(self, "metadata", None) or {}
            existing[_TOOL_METADATA_KEY] = self.marmot_asset_mrn
            self.metadata = existing  # type: ignore[attr-defined]


def _str_or_none(v: Any) -> str | None:
    """Treat generated `Unset` and empty strings as missing."""
    return v if isinstance(v, str) and v else None


def extract_mrns(output: Any) -> set[str]:
    """Best-effort extraction of asset MRNs from a LangChain tool's output.

    LangChain 1.x wraps structured tool output in a ``ToolMessage`` whose
    payload lives on ``content`` (``str | list[str | dict]``); older agent types
    pass the raw return value. Both go to the shared walker, which covers
    dicts, JSON-encoded strings and free-text MRN URIs uniformly.
    """
    if isinstance(output, BaseMessage):
        output = output.content
    return shared.extract_mrns(output)


def _tool_asset_mrn(tool: Any) -> str | None:
    """Return the Marmot MRN declared by a LangChain tool, if any.

    The MRN can be supplied two ways:
    - via ``MarmotTool`` mixin / ``marmot_asset_mrn`` class attribute, or
    - via ``metadata={_TOOL_METADATA_KEY: "..."}`` on the tool (e.g. set by
      the :func:`marmot_tool` decorator).
    """
    direct = getattr(tool, "marmot_asset_mrn", None)
    if isinstance(direct, str) and direct:
        return direct
    metadata = getattr(tool, "metadata", None)
    if isinstance(metadata, dict):
        mrn = metadata.get(_TOOL_METADATA_KEY)
        if isinstance(mrn, str) and mrn:
            return mrn
    return None


def _extract_tokens(response: LLMResult) -> tuple[int, int]:
    """Extract (input_tokens, output_tokens) from an ``LLMResult``.

    ``usage_metadata`` on a chat message is LangChain's provider-agnostic
    accounting, so it is preferred. The ``llm_output`` and ``generation_info``
    paths below remain for providers that only populate those.
    """
    try:
        for batch in response.generations:
            for generation in batch:
                if not isinstance(generation, ChatGeneration):
                    continue
                usage_metadata = getattr(generation.message, "usage_metadata", None)
                if usage_metadata:
                    return int(usage_metadata["input_tokens"]), int(usage_metadata["output_tokens"])

        llm_output = response.llm_output
        if isinstance(llm_output, dict):
            usage = llm_output.get("token_usage") or llm_output.get("usage") or {}
            tin = (
                usage.get("prompt_tokens")
                or usage.get("input_tokens")
                or usage.get("prompt_eval_count")
                or 0
            )
            tout = (
                usage.get("completion_tokens")
                or usage.get("output_tokens")
                or usage.get("eval_count")
                or 0
            )
            if tin or tout:
                return int(tin), int(tout)

        for batch in response.generations:
            for gen in batch:
                info = gen.generation_info or {}
                tin = info.get("prompt_eval_count") or info.get("input_tokens") or 0
                tout = info.get("eval_count") or info.get("output_tokens") or 0
                if tin or tout:
                    return int(tin), int(tout)
    except Exception:
        return 0, 0
    return 0, 0

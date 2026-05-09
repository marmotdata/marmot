"""Callback handler that registers a LangChain agent in Marmot and writes
lineage edges for the data sources it touches during a run."""

from __future__ import annotations

import functools
import hashlib
import json
import logging
import re
from datetime import datetime, timezone
from typing import TYPE_CHECKING, Any
from uuid import UUID

if TYPE_CHECKING:
    from collections.abc import Callable

    from langchain_core.tools import BaseTool

    from marmot.client import Client

try:
    from langchain_core.callbacks import BaseCallbackHandler as _BaseCallbackHandler

    _LANGCHAIN_AVAILABLE = True
except ImportError:  # pragma: no cover - exercised only when extra missing
    _BaseCallbackHandler = object  # type: ignore[assignment,misc]
    _LANGCHAIN_AVAILABLE = False


_LOG = logging.getLogger("marmot.integrations.langchain")

_DEFAULT_SERVICE = "LangChain"
_AGENT_ASSET_TYPE = "Agent"
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
        from marmot import connect
        from marmot.integrations.langchain import MarmotCallbackHandler

        client = connect()
        handler = MarmotCallbackHandler(
            client,
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
        client: Client,
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
        self._client = client
        self._name = name
        self._service = service
        self._model = model
        self._version = version
        self._owner = owner
        self._tools = tools
        self._tool_names = [t.name for t in tools] if tools else None
        self._system_prompt_hash = (
            hashlib.sha256(system_prompt.encode()).hexdigest()[:16]
            if system_prompt
            else None
        )
        self._extra_metadata = extra_metadata or {}

        self._agent_mrn: str | None = None
        self._agent_id: str | None = None

        # Per-run accumulators, keyed by the root chain run_id.
        self._root_of: dict[UUID, UUID] = {}
        self._upstreams: dict[UUID, set[str]] = {}
        self._run_started: dict[UUID, datetime] = {}
        self._tool_traces: dict[UUID, list[dict[str, Any]]] = {}
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
        root = self._root_of.get(run_id, run_id) if run_id else next(
            iter(self._upstreams), None
        )
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
            for mrn in _extract_mrns(output):
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
        self._tool_traces.setdefault(root_run_id, []).append({
            "tool_name": opened["tool_name"],
            "target_mrn": opened["target_mrn"],
            "started_at": opened["started_at"],
            "duration_ms": duration_ms,
            "status": status,
        })

    def on_llm_end(
        self,
        response: Any,
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
            k: v for k, v in self._tool_open.items()
            if self._root_of.get(k) != root_run_id
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
        explicit = {tc.get("target_mrn") for tc in tool_calls if tc.get("target_mrn")}
        observed_extras = sorted((upstreams - explicit) - {self._agent_mrn}) if upstreams else []

        try:
            self._client.agent_runs.create(
                agent_mrn=self._agent_mrn,
                run_id=str(root_run_id),
                started_at=started_at,
                ended_at=ended_at,
                status=status,
                model=self._model,
                tokens_in=tokens[0],
                tokens_out=tokens[1],
                error=error or None,
                tool_calls=tool_calls or None,
                observed_assets=observed_extras or None,
            )
        except Exception as e:  # pragma: no cover - best-effort telemetry
            _LOG.warning("failed to record Marmot agent run: %s", e)

    def _ensure_agent_registered(self) -> None:
        if self._agent_mrn is not None:
            return
        try:
            existing = self._client.assets.find(
                type=_AGENT_ASSET_TYPE, service=self._service, name=self._name
            )
        except Exception as e:  # pragma: no cover - best-effort
            _LOG.warning("failed to look up Marmot agent asset: %s", e)
            return

        payload = self._build_asset_payload()
        try:
            if existing is None:
                created = self._client.assets.create(payload)
                self._agent_id = created.get("id")
                self._agent_mrn = created.get("mrn")
            else:
                self._agent_id = existing.get("id")
                self._agent_mrn = existing.get("mrn")
                if self._agent_id:
                    self._client.assets.update(self._agent_id, payload)
        except Exception as e:  # pragma: no cover - best-effort
            _LOG.warning("failed to upsert Marmot agent asset: %s", e)
            return

        self._emit_declared_invocations()

    def _emit_declared_invocations(self) -> None:
        """Emit one ``AGENT_INVOKES`` edge per tool that declares an upstream
        MRN at construction time. The server treats these as ``declared`` edges
        and they are stable across runs — repeated emission is a safe no-op via
        the existing ``(source, target, event_id)`` uniqueness.
        """
        if not self._agent_mrn or not self._tools:
            return
        edges: list[dict[str, Any]] = []
        for tool in self._tools:
            mrn = _tool_asset_mrn(tool)
            if not mrn:
                continue
            edges.append({
                "source": self._agent_mrn,
                "target": mrn,
                "type": "AGENT_INVOKES",
            })
        if not edges:
            return
        try:
            self._client.lineage.batch(edges)
        except Exception as e:  # pragma: no cover - best-effort
            _LOG.warning("failed to write AGENT_INVOKES edges: %s", e)

    def _build_asset_payload(self) -> dict[str, Any]:
        metadata: dict[str, Any] = {
            "framework": "LangChain",
        }
        if self._model:
            metadata["model"] = self._model
        if self._version:
            metadata["version"] = self._version
        if self._owner:
            metadata["owner"] = self._owner
        if self._tool_names:
            metadata["tool_names"] = self._tool_names
        if self._system_prompt_hash:
            metadata["system_prompt_sha256_16"] = self._system_prompt_hash
        metadata.update(self._extra_metadata)

        return {
            "name": self._name,
            "type": _AGENT_ASSET_TYPE,
            "providers": [self._service],
            "services": [self._service],
            "metadata": metadata,
        }


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
        functools.update_wrapper(tool, fn, updated=())
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


def _extract_mrns(output: Any) -> set[str]:
    """Best-effort extraction of asset MRNs from a tool's output.

    Recognises:
    - dicts with an ``mrn`` key (e.g. results from ``get_asset`` /
      ``lookup_asset``);
    - dicts with a ``results`` list of such dicts (e.g. ``search_catalog``);
    - dicts with a ``nodes`` / ``upstream`` list (lineage responses);
    - JSON-encoded strings of any of the above (LangChain often stringifies
      tool outputs before showing them to the LLM);
    - ``ToolMessage`` / similar wrappers — the JSON payload lives on
      ``output.content``.
    """
    # LangChain 1.x wraps structured tool output in ToolMessage; older agent
    # types may pass the raw return value. Normalise both into ``output``.
    content = getattr(output, "content", None)
    if isinstance(content, (str, list, dict)):
        output = content

    found: set[str] = set()
    _walk_for_mrns(output, found, depth=0)

    # ``content`` can be a list of content blocks (multimodal). Walk each.
    if isinstance(output, list):
        for item in output:
            text = getattr(item, "text", None) if not isinstance(item, dict) else item.get("text")
            if isinstance(text, str):
                _walk_string(text, found)

    if isinstance(output, str):
        _walk_string(output, found)

    return found


def _walk_string(s: str, out: set[str]) -> None:
    """Try JSON-decode first (most tool outputs are JSON-encoded structured
    data) then fall back to regex for prose.
    """
    try:
        parsed = json.loads(s)
        _walk_for_mrns(parsed, out, depth=0)
    except (ValueError, TypeError):
        pass
    for match in _MRN_PATTERN.findall(s):
        out.add(match)


# MRN schemes Marmot is known to emit. Conservative — anything matching
# ``<scheme>://`` would catch arbitrary URLs (HTTP, etc.); we only mine for
# schemes we control or know land in the catalog.
_MRN_PATTERN = re.compile(
    r"\b(?:mrn|postgres|mysql|kafka|s3|gcs|bigquery|snowflake|redis|"
    r"clickhouse|elasticsearch|opensearch|mongodb|dynamodb|airflow|"
    r"dbt|marmot)://[^\s\"'<>,)\]}]+"
)


def _walk_for_mrns(value: Any, out: set[str], *, depth: int) -> None:
    if depth > 4:
        return
    if isinstance(value, dict):
        mrn = value.get("mrn")
        if isinstance(mrn, str) and mrn:
            out.add(mrn)
        for v in value.values():
            if isinstance(v, (dict, list)):
                _walk_for_mrns(v, out, depth=depth + 1)
    elif isinstance(value, list):
        for v in value:
            if isinstance(v, (dict, list)):
                _walk_for_mrns(v, out, depth=depth + 1)


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


def _extract_tokens(response: Any) -> tuple[int, int]:
    """Best-effort extraction of (input_tokens, output_tokens) from a LangChain
    LLMResult. Different providers surface counts in different places — this
    handles the common ones (OpenAI/Anthropic via ``llm_output['token_usage']``,
    Ollama via ``generations[].generation_info``).
    """
    try:
        llm_output = getattr(response, "llm_output", None)
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

        generations = getattr(response, "generations", None) or []
        for batch in generations:
            for gen in batch:
                info = getattr(gen, "generation_info", None) or {}
                tin = info.get("prompt_eval_count") or info.get("input_tokens") or 0
                tout = info.get("eval_count") or info.get("output_tokens") or 0
                if tin or tout:
                    return int(tin), int(tout)
    except Exception:  # pragma: no cover — best-effort
        return 0, 0
    return 0, 0

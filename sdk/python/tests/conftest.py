"""Keep the suite hermetic.

The credential chain reads `MARMOT_API_KEY`, `MARMOT_TOKEN` and the token
`marmot login` cached under the config directory, all of which a developer's
shell is likely to have set. A test that expects a workload-identity exchange
would otherwise be served an ambient credential instead, and fail only on the
machines that have one.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from marmot.auth.credential import ENVIRONMENT_API_KEY, ENVIRONMENT_BEARER_TOKEN
from marmot.config import ENVIRONMENT_HOST, ENVIRONMENT_MARMOT_CONTEXT

ENVIRONMENT_VARIABLES = (
    ENVIRONMENT_API_KEY,
    ENVIRONMENT_BEARER_TOKEN,
    ENVIRONMENT_HOST,
    ENVIRONMENT_MARMOT_CONTEXT,
)


@pytest.fixture(autouse=True)
def isolated_environment(monkeypatch: pytest.MonkeyPatch, tmp_path: Path) -> None:
    """Never read the developer's real environment or config directory."""
    monkeypatch.setenv("XDG_CONFIG_HOME", str(tmp_path))
    for variable in ENVIRONMENT_VARIABLES:
        monkeypatch.delenv(variable, raising=False)

"""Exception hierarchy for the Marmot SDK."""

from __future__ import annotations


class MarmotError(Exception):
    """Base class for all SDK errors."""


class AuthError(MarmotError):
    """Raised when no credential source resolves, or when the server rejects the credential."""


class NotFoundError(MarmotError):
    """Raised when a requested resource does not exist."""


class ServerError(MarmotError):
    """Raised on 5xx responses or unexpected protocol errors."""

    def __init__(self, message: str, status_code: int | None = None) -> None:
        super().__init__(message)
        self.status_code = status_code

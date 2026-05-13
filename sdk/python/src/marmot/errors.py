"""Exception hierarchy for the Marmot SDK."""

from __future__ import annotations


class MarmotError(Exception):
    """Base class for all SDK errors."""

    def __init__(self, message: str, status_code: int | None = None) -> None:
        super().__init__(message)
        self.status_code = status_code


class AuthError(MarmotError):
    """Raised for 401/403 responses or when no credential resolves."""


class NotFoundError(MarmotError):
    """Raised for 404 responses."""


class ValidationError(MarmotError):
    """Raised for 400 responses (server rejected the request body)."""


class RateLimitError(MarmotError):
    """Raised for 429 responses."""


class ServerError(MarmotError):
    """Raised on 5xx responses or unexpected protocol errors."""


def is_not_found(err: BaseException) -> bool:
    """Return True if err is a NotFoundError. Mirrors Go SDK's IsNotFound."""
    return isinstance(err, NotFoundError)


def is_rate_limit(err: BaseException) -> bool:
    """Return True if err is a RateLimitError. Mirrors Go SDK's IsRateLimit."""
    return isinstance(err, RateLimitError)

"""Python SDK for Marmot."""

from marmot.client import Client, connect
from marmot.errors import (
    AuthError,
    MarmotError,
    NotFoundError,
    ServerError,
)

__all__ = [
    "AuthError",
    "Client",
    "MarmotError",
    "NotFoundError",
    "ServerError",
    "connect",
]

__version__ = "0.1.0"

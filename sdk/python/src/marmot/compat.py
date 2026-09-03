from __future__ import annotations

import sys

if sys.version_info < (3, 11):
    from strenum import StrEnum
else:
    from enum import StrEnum

__all__ = ["StrEnum"]

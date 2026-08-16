#!/usr/bin/env python3
"""Disabled historical availability-manipulation entry point.

The former implementation deleted recent ``probe_history`` rows and inserted
synthetic success/failure records, changing public availability data. It is not
part of the supported operations workflow. Git history retains the old code for
audit purposes; this file intentionally contains no database or SSH capability.

Restoring any write behavior requires separate authorization and the reviewed
design controls documented in AGENTS.md.
"""

import sys


def main() -> int:
    print(
        "adjust_availability.py is a disabled historical script; "
        "it cannot read or write production data.",
        file=sys.stderr,
    )
    return 2


if __name__ == "__main__":
    raise SystemExit(main())

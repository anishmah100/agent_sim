"""Event source — tails the historian's jsonl, yielding decoded records.

The historian writes a single line per event. The narrator process
follows the file (a la `tail -F`) and decodes each line into a
dict-shaped Event. Idle exit lets a one-shot post-run pass complete
when the file stops growing.
"""
from __future__ import annotations

import json
import os
import time
from pathlib import Path
from typing import Iterator, Optional


def iter_events(
    path: Path,
    *,
    follow: bool = True,
    idle_exit_seconds: Optional[float] = None,
    poll_interval: float = 0.5,
) -> Iterator[dict]:
    """Yield events from the jsonl file.

    ``follow=False`` reads to EOF and stops (post-run one-shot mode).
    ``follow=True`` blocks on EOF and re-polls, exiting only when no
    new bytes arrive for ``idle_exit_seconds``. Set ``idle_exit_seconds``
    to None for forever-tail.
    """
    if not path.exists():
        # The engine may not have written anything yet; treat as empty
        # and wait if we're in follow mode.
        path.parent.mkdir(parents=True, exist_ok=True)
        if not follow:
            return
        path.touch()

    last_size = 0
    idle_started = None
    with path.open("r", encoding="utf-8") as f:
        while True:
            line = f.readline()
            if line:
                idle_started = None
                line = line.strip()
                if not line:
                    continue
                try:
                    yield json.loads(line)
                except json.JSONDecodeError:
                    # Partial write — sleep and re-read.
                    continue
                continue
            if not follow:
                return
            # No new data. Check the file size as a sanity ping.
            size = path.stat().st_size
            if size != last_size:
                # File grew but we're at EOF — likely a buffered write
                # that hasn't flushed all the way.
                last_size = size
                idle_started = None
                time.sleep(poll_interval / 5)
                continue
            if idle_started is None:
                idle_started = time.monotonic()
            elif idle_exit_seconds is not None and (
                time.monotonic() - idle_started >= idle_exit_seconds
            ):
                return
            time.sleep(poll_interval)

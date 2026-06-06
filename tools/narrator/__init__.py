"""Live hierarchical narrator (D15).

A standalone process that tails the engine's event log and emits
NarratorSummary records at four levels of zoom (per-agent, per-cluster,
society, world). L1+L2 run on local Qwen; L3+L4 run on Claude. Budget
caps are enforced at every call site.
"""

from .config import NarratorConfig
from .buckets import Bucketizer
from .source import iter_events
from .emit import NarratorOutput

__all__ = ["NarratorConfig", "Bucketizer", "iter_events", "NarratorOutput"]

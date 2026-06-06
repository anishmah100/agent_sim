"""Rule-based archetype bots (D16). Each is a frozen-background
participant in emergence experiments. Pinned by commit SHA in
experiment.yaml.

Public surface:

    from agents.baselines import Survivor, Killer, Manipulator, Scavenger

Each archetype is constructed from `AgentCredentials` and exposes an
`async run()` coroutine that drives the obs→action loop until the
WS closes or `stop()` is called.
"""

from .survivor import Survivor
from .scavenger import Scavenger
from .killer import Killer
from .manipulator import Manipulator

__all__ = ["Survivor", "Scavenger", "Killer", "Manipulator"]

"""Layered observation renderer (Phase AGENT-A2).

Turns a structured ``Observation`` into a compact ASCII block the
tactical brain can read as a prompt prefix:

    self:
      pos: (24,17)  gold: 14  hunger: 0.62
      goal: "reach blacksmith, buy hammer"
      last: move_step DENIED (blocked_by_entity)

    nearby (within r=12, top 12 by salience):
    - cara   pos:(25,17)  facing:W   <- BLOCKING
    - gren   pos:(28,15)  holding:axe

    audible (last 5, 30s window):
      t=12:04  speak    cara->you           "watch it"
      t=12:04  speak    you->cara           "sorry"
      t=11:58  shout    mari (loud, NE)     "apples 2g!"

    map (11x11, you=@):
      . . . T T . . . . . .
      . . . T . . . s . . .
      ...
    legend: @=you ?=fog .=grass T=tree #=wall

Designed for both harnesses:
  - Claude: pass ``coord_style="absolute"`` for explicit (x,y).
  - Qwen:   pass ``coord_style="compass"`` for direction-only hints
    (saves tokens since direction matters more than absolute coords).

Pure Python, no engine imports. Lives in the SDK so both heuristic and
LLM agents share one renderer.
"""

from __future__ import annotations

import math
from dataclasses import dataclass, field
from typing import Optional

from .models import (
    Action, AudibleEvent, Observation, Pos, SelfState, VisibleEntity,
    VisibleObject,
)


# Coarse compass directions for token-frugal renderings.
COMPASS_8 = ["E", "NE", "N", "NW", "W", "SW", "S", "SE"]


def relative_compass(from_pos: Pos, to_pos: Pos) -> str:
    """Returns one of 8 compass headings from from_pos to to_pos.
    Equal positions return 'HERE'."""
    dx = to_pos[0] - from_pos[0]
    dy = to_pos[1] - from_pos[1]
    if dx == 0 and dy == 0:
        return "HERE"
    # math.atan2 has +x to the east, +y to the south on screen — invert dy
    # so positive angle means north.
    angle = math.atan2(-dy, dx)
    # angle is in [-pi, pi]; 0 = East, pi/2 = North. Map to 8 buckets
    # without the +pi offset so bucket 0 = East matches COMPASS_8[0].
    bucket = int(round(angle / (2 * math.pi) * 8)) % 8
    return COMPASS_8[bucket]


def chebyshev(a: Pos, b: Pos) -> int:
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


# --- Salience ---

@dataclass
class _RankedEntity:
    e: VisibleEntity
    distance: int
    score: int
    flags: list[str] = field(default_factory=list)


def rank_nearby(
    self_state: SelfState,
    visible: list[VisibleEntity],
    audible: list[AudibleEvent],
    top_k: int = 12,
) -> list[_RankedEntity]:
    """Sort visible entities by salience (higher = more interesting).

    Priority order (descending):
      1. Spoke to me (whisper / speak directed) in the audible window.
      2. Shouted recently (in audible window).
      3. Within 1 tile of me (could be blocking my next step).
      4. Holding something noted in extras_summary.
      5. By distance ascending.

    Returns at most top_k entries.
    """
    spoke_to_me = {e.from_entity for e in audible
                   if e.kind in ("speech", "whisper")
                   and e.text  # only "directed" lines have text the receiver heard
                   }
    shouted = {e.from_entity for e in audible if e.kind == "shout"}

    ranked: list[_RankedEntity] = []
    for v in visible:
        d = chebyshev(self_state.pos, v.pos)
        score = -d  # base: closer is higher
        flags: list[str] = []
        if v.entity_id in spoke_to_me:
            score += 1000
            flags.append("SPOKE_TO_YOU")
        if v.entity_id in shouted:
            score += 500
            flags.append("SHOUTED")
        if d <= 1:
            score += 100
            flags.append("ADJACENT")
        if v.extras_summary.get("holding"):
            score += 50
        ranked.append(_RankedEntity(e=v, distance=d, score=score, flags=flags))

    ranked.sort(key=lambda r: (-r.score, r.distance, r.e.entity_id))
    return ranked[:top_k]


# --- Render blocks ---

def render_self(s: SelfState, goal: Optional[str] = None) -> str:
    lines = [f"  pos: {tuple(s.pos)}  facing:{s.facing.value}"]
    if s.extras:
        parts = []
        for k in ("hp", "gold", "hunger"):
            if k in s.extras:
                v = s.extras[k]
                if isinstance(v, float):
                    parts.append(f"{k}:{v:.2f}")
                else:
                    parts.append(f"{k}:{v}")
        if parts:
            lines.append("  " + "  ".join(parts))
    if goal:
        lines.append(f"  goal: {goal!r}")
    if s.last_action_result:
        verb = s.last_action_result.get("verb", "?")
        accepted = s.last_action_result.get("accepted")
        if accepted is False:
            code = (
                s.last_action_result.get("reason_code")
                or s.last_action_result.get("reason")
                or "rejected"
            )
            lines.append(f"  last: {verb} DENIED ({code})")
        elif accepted is True:
            lines.append(f"  last: {verb} OK")
    return "self:\n" + "\n".join(lines)


def render_nearby(
    ranked: list[_RankedEntity],
    self_pos: Pos,
    coord_style: str = "absolute",
) -> str:
    if not ranked:
        return "nearby:\n  (none)"
    lines = [f"nearby (top {len(ranked)} by salience, r=chebyshev):"]
    for r in ranked:
        if coord_style == "compass":
            pos_str = f"{relative_compass(self_pos, tuple(r.e.pos))}+{r.distance}"
        else:
            pos_str = f"pos:{tuple(r.e.pos)}"
        face = f"facing:{r.e.facing.value}"
        flag = ""
        if r.flags:
            flag = "  <- " + ",".join(r.flags)
        lines.append(f"- {r.e.entity_id}  {pos_str}  {face}{flag}")
    return "\n".join(lines)


def render_audible(
    audible: list[AudibleEvent],
    self_entity_id: str,
    self_pos: Pos,
    coord_style: str = "absolute",
    limit: int = 5,
) -> str:
    if not audible:
        return "audible:\n  (none)"
    recent = audible[-limit:]
    lines = [f"audible (last {len(recent)}, rolling window):"]
    for ev in recent:
        # Format: tick  channel  from->you/from(loc)  "text" | sound_kind
        if coord_style == "compass":
            loc = relative_compass(self_pos, tuple(ev.from_pos))
        else:
            loc = str(tuple(ev.from_pos))
        if ev.kind == "whisper":
            arrow = f"{ev.from_entity}->you" if ev.from_entity != self_entity_id else "you->?"
            quoted = ev.text or "[private]"
            lines.append(f"  t={ev.tick:>5}  whisper  {arrow:<22}  \"{quoted}\"")
        elif ev.kind == "speech":
            arrow = f"{ev.from_entity}->you" if ev.from_entity != self_entity_id else "you"
            lines.append(f"  t={ev.tick:>5}  speak    {arrow:<22}  \"{ev.text}\"")
        elif ev.kind == "shout":
            lines.append(f"  t={ev.tick:>5}  shout    {ev.from_entity} ({loc})  \"{ev.text}\"")
        elif ev.kind == "sound":
            lines.append(f"  t={ev.tick:>5}  sound    {ev.sound_kind} ({loc})")
    return "\n".join(lines)


def render_minimap(
    self_pos: Pos,
    visible_entities: list[VisibleEntity],
    visible_objects: list[VisibleObject],
    radius: int = 5,
) -> str:
    """Render an ASCII minimap centered on the agent. Tiles the agent
    can't currently see render as '?'. Glyph priority: self > entities
    > objects > terrain.

    We don't have terrain in the Observation today; the floor is '.'
    until SUB-9 surfaces near-tile terrain. Agents using this renderer
    can still localize each other on the relative grid.
    """
    side = radius * 2 + 1
    grid = [["?"] * side for _ in range(side)]
    cx, cy = self_pos

    def place(x: int, y: int, glyph: str) -> None:
        gx = x - cx + radius
        gy = y - cy + radius
        if 0 <= gx < side and 0 <= gy < side:
            grid[gy][gx] = glyph

    # Visible terrain — we currently know nothing fine-grained, so mark
    # everything in vision as walkable until terrain surfaces.
    for v in visible_entities:
        place(v.pos[0], v.pos[1], "v")  # will be overwritten below by per-archetype glyph
    for v in visible_objects:
        glyph = {"tree": "T", "rock": "*", "stall_": "s", "sign": "i"}.get(v.kind[:5], "o")
        place(v.pos[0], v.pos[1], glyph)
    # Entities
    for v in visible_entities:
        glyph = v.entity_id[0].upper() if v.entity_id else "?"
        place(v.pos[0], v.pos[1], glyph)
    # Self last so it never gets stomped.
    place(cx, cy, "@")

    rows = [" ".join(row) for row in grid]
    legend = "legend: @=you ?=fog .=walkable T=tree *=rock s=stall i=sign  letter=entity"
    return f"map ({side}x{side}, you=@):\n" + "\n".join(rows) + "\n" + legend


# --- Top-level ---

def render_layered_observation(
    obs: Observation,
    *,
    goal: Optional[str] = None,
    coord_style: str = "absolute",
    nearby_top_k: int = 12,
    audible_limit: int = 5,
    minimap_radius: int = 5,
) -> str:
    """The full layered block, ready for either harness's prompt.

    Picks the absolute-coord rendering for Claude (rich), the compass
    style for Qwen (token-frugal).
    """
    ranked = rank_nearby(obs.self, obs.visible_entities, obs.audible, top_k=nearby_top_k)
    parts = [
        render_self(obs.self, goal=goal),
        render_nearby(ranked, tuple(obs.self.pos), coord_style=coord_style),
        render_audible(
            obs.audible, obs.self.entity_id, tuple(obs.self.pos),
            coord_style=coord_style, limit=audible_limit,
        ),
        render_minimap(
            tuple(obs.self.pos), obs.visible_entities, obs.visible_objects,
            radius=minimap_radius,
        ),
    ]
    return "\n\n".join(parts)

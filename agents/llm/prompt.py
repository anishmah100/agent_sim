"""Observation → prompt rendering for the focal agent.

Pure functions (no I/O, no LLM) so they're unit-testable. The prompt
is deliberately compact: Qwen-27B at reasoning-budget=0 does better
with a tight, structured context than a verbose one.
"""
from __future__ import annotations

from typing import Any


# The action menu shown to the model every cycle. Kept in sync with
# grammar.py's verb set. Each line is verb + when-to-use + params.
ACTION_MENU = """\
Actions you can take (pick 1-3 per turn):
- move {"verb":"move","target":[x,y]} — walk toward a tile
- speak {"verb":"speak","text":"..."} — say something out loud (nearby agents hear)
- whisper {"verb":"whisper","target":"<entity_id>","text":"..."} — private message to an ADJACENT agent
- shout {"verb":"shout","text":"..."} — loud, heard far away
- eat {"verb":"eat","item":"<item_id>"} — consume food from your inventory to reduce hunger
- pickup {"verb":"pickup","target":"<entity_id>"} — grab an adjacent ground item (coins/gems auto-convert to gold)
- equip {"verb":"equip","item":"<item_id>","slot":"weapon"} — wield a weapon from inventory
- give {"verb":"give","target":"<entity_id>","item":"<item_id>"} — hand an item to an adjacent agent
- pay {"verb":"pay","target":"<entity_id>","amount":N} — give gold to an adjacent agent
- trade {"verb":"trade","target":"<entity_id>","item":"<item_id>","price":N} — sell an item to an adjacent agent
- attack {"verb":"attack","target":"<entity_id>"} — strike an adjacent agent
- propose_task {"verb":"propose_task","target":"<entity_id>","terms":"...","reward":"..."} — offer a deal/contract to ANY agent you can see (no need to be near them)
- accept_task {"verb":"accept_task","id":"<contract_id>"} — accept a contract offered to you (works from ANYWHERE — you never need to move to accept)
- complete_task {"verb":"complete_task","id":"<contract_id>"} — mark a deal you've honored as done (works from anywhere; do this AFTER you've paid/given the promised reward)
- wait {"verb":"wait","ticks":N} — do nothing for a while

Rules:
- targets are ALWAYS the entity_id (e.g. "spawn_7"), never a display name.
- RANGE: propose_task and accept_task work at ANY distance — never chase someone just to propose or accept a deal; do it from where you are. speak reaches ~8 tiles, whisper ~2 tiles.
- pay and give work within ~3 tiles (a short reach) — you do NOT need to be exactly adjacent to pay gold or hand over an item. If a deal partner is within a few tiles, pay/give them NOW.
- ONLY trade/attack require the target to be strictly ADJACENT (within 1 tile) — move next to them first for those.
- you can only eat/equip items that are in YOUR inventory."""


def _item_kind(sprite: str) -> str:
    s = sprite or ""
    if s.startswith("item:"):
        s = s[5:]
    if "#" in s:
        s = s.split("#", 1)[0]
    return s


def render_self(obs: Any) -> str:
    s = obs.self
    extras = s.extras or {}
    hp = extras.get("hp", "?")
    hunger = extras.get("hunger", 0.0)
    gold = extras.get("gold", "?")
    inv = extras.get("inventory") or []
    inv_kinds: dict[str, int] = {}
    for it in inv:
        if isinstance(it, str):
            k = _item_kind(it)
            inv_kinds[k] = inv_kinds.get(k, 0) + 1
    inv_str = ", ".join(f"{k}x{v}" for k, v in inv_kinds.items()) or "(empty)"
    equipped = extras.get("equipped") or {}
    eq_str = ", ".join(f"{slot}={_item_kind(v)}"
                       for slot, v in equipped.items() if v) or "(none)"
    try:
        hunger_str = f"{float(hunger):.2f}"
    except (TypeError, ValueError):
        hunger_str = str(hunger)
    inside = getattr(s, "inside_building", None)
    lines = [
        f"YOU ({s.entity_id}) at {tuple(s.pos)}",
        f"  hp={hp}  hunger={hunger_str} (1.0=starving)  gold={gold}",
        f"  inventory: {inv_str}",
        f"  equipped: {eq_str}",
    ]
    if inside:
        lines.append(f"  inside building: {inside}")
    return "\n".join(lines)


def render_visible(obs: Any, self_pos) -> str:
    def cheb(p):
        return max(abs(p[0] - self_pos[0]), abs(p[1] - self_pos[1]))

    out = []
    ents = list(getattr(obs, "visible_entities", []) or [])
    if ents:
        out.append("Nearby agents:")
        for e in sorted(ents, key=lambda e: cheb(e.pos))[:8]:
            d = cheb(e.pos)
            summ = e.extras_summary or {}
            hpb = summ.get("hp_bucket", "")
            armed = "armed" if (summ.get("equipped_slot") or
                                summ.get("equipped_sprite")) else ""
            tags = " ".join(t for t in (hpb, armed) if t)
            adj = " ADJACENT" if d <= 1 else f" {d} tiles"
            out.append(f"  {e.entity_id} ({e.archetype}){adj}"
                       + (f" [{tags}]" if tags else ""))
    items = list(getattr(obs, "visible_items", []) or [])
    if items:
        out.append("Items on the ground:")
        for it in sorted(items, key=lambda it: cheb(it.pos))[:8]:
            d = cheb(it.pos)
            if d <= 1:
                hint = " ADJACENT — pickup NOW"
            else:
                hint = f" {d} tiles away — move to {tuple(it.pos)} first, " \
                       f"do NOT pickup until adjacent"
            out.append(f"  {it.entity_id} = {_item_kind(it.sprite)}{hint}")
    objs = list(getattr(obs, "visible_objects", []) or [])
    doors = [o for o in objs if o.kind == "door"]
    if doors:
        out.append("Doors (enterable):")
        for o in sorted(doors, key=lambda o: cheb(o.pos))[:4]:
            d = cheb(o.pos)
            out.append(f"  {o.object_id} at {tuple(o.pos)} ({d} tiles)")
    if not out:
        return "Nothing visible nearby."
    return "\n".join(out)


def render_contracts(obs: Any) -> str:
    """Surface pending verbal contracts where this agent is the target
    (can accept) or proposer (awaiting). Without this the LLM never
    sees a contract proposed to it and can't accept_task — the gap
    that produced '9 proposed / 0 accepted' in the first P7 run."""
    extras = obs.self.extras or {}
    contracts = extras.get("contracts") or []
    me = obs.self.entity_id
    if not contracts:
        return ""
    out = ["Contracts involving you:"]
    for c in contracts:
        if not isinstance(c, dict):
            continue
        cid = c.get("id", "?")
        status = c.get("status", "?")
        proposer = c.get("proposer", "?")
        target = c.get("target", "?")
        terms = c.get("terms", "")
        reward = c.get("reward", "")
        if target == me and status == "proposed":
            out.append(f"  [{cid}] {proposer} offers you: \"{terms}\" "
                       f"for {reward} — you can accept_task with id={cid}")
        elif proposer == me:
            out.append(f"  [{cid}] you offered {target}: \"{terms}\" "
                       f"(status: {status})")
        else:
            out.append(f"  [{cid}] {proposer}->{target}: \"{terms}\" "
                       f"({status})")
    return "\n".join(out) if len(out) > 1 else ""


def render_audible(obs: Any) -> str:
    aud = list(getattr(obs, "audible", []) or [])
    if not aud:
        return ""
    out = ["Recently heard:"]
    for ev in aud[-6:]:
        kind = ev.sound_kind or ev.kind
        txt = ev.text or ""
        if kind == "death_scream":
            out.append("  a death scream from nearby!")
        elif kind == "kill_witnessed":
            out.append(f"  you witnessed a killing: {txt}")
        elif txt:
            spk = getattr(ev, "from_entity", "") or "someone"
            out.append(f"  {spk} ({kind}): \"{txt}\"")
    return "\n".join(out) if len(out) > 1 else ""


def build_prompt(obs: Any, persona: str, goal: str,
                 last_results: list[str] | None = None,
                 intent: str = "") -> str:
    """Assemble the full tactical prompt for one decision cycle."""
    parts = [
        persona.strip(),
        f"\nYour current goal: {goal}",
    ]
    # If someone has proposed a contract TO this agent, surface it as a
    # top-priority decision. accept_task needs no movement and no
    # adjacency, so the agent should act on it immediately instead of
    # letting the offer rot while it chases gold (the cause of every run
    # ending 'N proposed / 0 accepted'). Reject is also free.
    _ex = obs.self.extras or {}
    _me = obs.self.entity_id
    _pending = [c for c in (_ex.get("contracts") or [])
                if isinstance(c, dict) and c.get("target") == _me
                and c.get("status") == "proposed"]
    if _pending:
        _c = _pending[0]
        parts.append(
            f"\nPRIORITY: {_c.get('proposer')} has offered you a deal "
            f"(id={_c.get('id')}): \"{_c.get('terms')}\" for "
            f"{_c.get('reward') or 'no stated reward'}. You can accept it "
            f"RIGHT NOW from where you stand with "
            f'{{"verb":"accept_task","id":"{_c.get("id")}"}} (no need to '
            f"move). If it helps your goal, accept this turn; otherwise "
            f"reject it. Don't ignore a pending offer.")
    # An ACCEPTED deal you're part of is a live obligation: either HONOR it
    # (pay the gold / give the item you promised — pay & give reach ~3
    # tiles, no need to be adjacent — then complete_task to close it) or
    # deliberately DEFECT (keep the reward, maybe even attack them). Both
    # are legitimate; what's NOT useful is forgetting the deal exists,
    # which is why accepted contracts produced 0 pays. Make the call.
    _active = [c for c in (_ex.get("contracts") or [])
               if isinstance(c, dict) and c.get("status") == "accepted"
               and (c.get("proposer") == _me or c.get("target") == _me)]
    if _active:
        _c = _active[0]
        _role = "proposer" if _c.get("proposer") == _me else "accepter"
        _other = _c.get("target") if _role == "proposer" else _c.get("proposer")
        parts.append(
            f"\nACTIVE DEAL (id={_c.get('id')}, you are the {_role}) with "
            f"{_other}: \"{_c.get('terms')}\", reward {_c.get('reward') or '?'}. "
            f"Decide NOW: HONOR it — pay {_other} gold "
            f'({{"verb":"pay","target":"{_other}","amount":N}}) or give them '
            f"the promised item (both reach ~3 tiles, no need to be adjacent), "
            f'then close it with {{"verb":"complete_task","id":"{_c.get("id")}"}} '
            f"— or DEFECT and keep everything (a betrayer might even attack "
            f"them). Your persona drives this choice; don't just forget the "
            f"deal exists.")
    if intent:
        parts.append(
            f"Last turn you decided: \"{intent}\". If that target is "
            f"still the best move and you haven't reached it, KEEP going "
            f"toward it — don't switch targets every turn or you'll never "
            f"arrive.")
    parts += [
        "",
        render_self(obs),
        "",
        render_visible(obs, obs.self.pos),
    ]
    contracts = render_contracts(obs)
    if contracts:
        parts += ["", contracts]
    aud = render_audible(obs)
    if aud:
        parts += ["", aud]
    if last_results:
        parts += ["", "Results of your last actions:"]
        parts += [f"  - {r}" for r in last_results[-4:]]
    parts += [
        "",
        ACTION_MENU,
        "",
        'Respond with JSON: {"reasoning":"<ONE short sentence, max 25 words>","actions":[<1-3 actions>]}.',
        "Pursue your goal. Be decisive — prefer concrete actions over waiting. Keep reasoning brief.",
        "IMPORTANT: pickup/eat/pay/trade/whisper/attack only work on an "
        "ADJACENT target (1 tile). If your target is farther, just move "
        "toward it this turn and act next turn — do NOT batch a move with "
        "an action on a far target, it will be rejected.",
    ]
    return "\n".join(parts)

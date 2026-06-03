// World Rulebook — renders the full affordance manifest as a
// browsable, searchable reference panel. The panel is the user-facing
// answer to "what can my agent do in this world?" — same JSON the
// agents fetch at register time.
//
// Layout: a left-rail TOC of systems, a content pane that lists each
// system's verbs (with params_schema + rejection reasons), its state
// fields (public vs private), its emitted sounds, and its archetypes.
// A search box filters verbs and state-field keys across all systems.

import { createSignal, onMount, For, Show } from "solid-js";
import { fetchAffordances, type AffordanceManifest, type SystemDeclaration, type VerbDeclaration, type StateFieldDecl, type SoundDecl, type ArchetypeDecl } from "../net/api";

const PALETTE = {
  bg: "rgba(24, 20, 37, 0.96)",
  border: "#3a4466",
  border2: "#5a6988",
  accent: "#feae34",
  ink: "#ead4aa",
  ink2: "#8b9bb4",
  good: "#63c74d",
  bad: "#e43b44",
};

export function WorldRulebook(props: { onClose: () => void }) {
  // Explicit fetch state so a hung or failed request shows the user
  // an actionable error instead of a forever "loading" spinner.
  const [manifest, setManifest] = createSignal<AffordanceManifest | null>(null);
  const [error, setError] = createSignal<string | null>(null);
  const [query, setQuery] = createSignal("");
  const [activeSystem, setActiveSystem] = createSignal<string | null>(null);

  onMount(() => {
    fetchAffordances()
      .then((m) => setManifest(m))
      .catch((e) => setError((e as Error).message ?? String(e)));
  });

  const matches = (s: string) => {
    const q = query().trim().toLowerCase();
    if (!q) return true;
    return s.toLowerCase().includes(q);
  };

  const visibleSystems = (): SystemDeclaration[] => {
    const m = manifest();
    if (!m) return [];
    if (!query().trim()) return m.systems;
    return m.systems.filter((s) =>
      matches(s.name) ||
      matches(s.description) ||
      s.verbs.some((v) => matches(v.verb) || matches(v.description)) ||
      s.state_fields.some((f) => matches(f.key) || matches(f.meaning))
    );
  };

  return (
    <div
      style={{
        position: "fixed",
        inset: "0",
        background: "rgba(0,0,0,0.6)",
        "z-index": "100",
        display: "flex",
        "align-items": "center",
        "justify-content": "center",
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) props.onClose();
      }}
    >
      <div
        style={{
          width: "min(1100px, 92vw)",
          height: "min(720px, 88vh)",
          background: PALETTE.bg,
          border: `1px solid ${PALETTE.border}`,
          "border-radius": "8px",
          display: "grid",
          "grid-template-rows": "auto 1fr",
          color: PALETTE.ink,
          "font-family": "system-ui, sans-serif",
          "font-size": "13px",
        }}
      >
        <header
          style={{
            display: "flex",
            "align-items": "center",
            gap: "16px",
            padding: "12px 18px",
            "border-bottom": `1px solid ${PALETTE.border}`,
          }}
        >
          <h2 style={{ margin: "0", color: PALETTE.accent, "font-size": "16px" }}>
            World Rulebook
          </h2>
          <Show when={manifest()}>
            <span style={{ color: PALETTE.ink2 }}>
              {manifest()!.scenario} · {manifest()!.systems.length} systems
            </span>
          </Show>
          <input
            type="text"
            placeholder="search verbs, state, descriptions…"
            value={query()}
            onInput={(e) => setQuery(e.currentTarget.value)}
            style={{
              "margin-left": "auto",
              padding: "5px 10px",
              background: "rgba(0,0,0,0.3)",
              border: `1px solid ${PALETTE.border2}`,
              "border-radius": "3px",
              color: PALETTE.ink,
              width: "260px",
              "font-size": "12px",
            }}
          />
          <button
            type="button"
            onClick={props.onClose}
            style={{
              padding: "4px 10px",
              background: PALETTE.border,
              color: PALETTE.ink,
              border: `1px solid ${PALETTE.border2}`,
              "border-radius": "3px",
              cursor: "pointer",
            }}
          >
            close
          </button>
        </header>

        <div style={{ display: "grid", "grid-template-columns": "200px 1fr", overflow: "hidden" }}>
          <nav
            style={{
              "border-right": `1px solid ${PALETTE.border}`,
              padding: "8px 0",
              overflow: "auto",
            }}
          >
            <For each={visibleSystems()}>
              {(s) => (
                <button
                  type="button"
                  onClick={() => setActiveSystem(s.name)}
                  style={{
                    display: "block",
                    width: "100%",
                    padding: "6px 14px",
                    "text-align": "left",
                    background: activeSystem() === s.name ? "rgba(254,174,52,0.12)" : "transparent",
                    border: "none",
                    "border-left": activeSystem() === s.name ? `2px solid ${PALETTE.accent}` : "2px solid transparent",
                    color: activeSystem() === s.name ? PALETTE.accent : PALETTE.ink,
                    cursor: "pointer",
                    "font-size": "12px",
                    "text-transform": "capitalize",
                  }}
                >
                  {s.name}
                  <span style={{ "margin-left": "8px", "font-size": "10px", color: PALETTE.ink2 }}>
                    {s.verbs.length} verbs
                  </span>
                </button>
              )}
            </For>
          </nav>

          <main style={{ padding: "16px 22px", overflow: "auto" }}>
            <Show when={error()}>
              <div style={{ color: PALETTE.bad, padding: "20px 0" }}>
                failed to load manifest: {error()}
                <div style={{ color: PALETTE.ink2, "font-size": "11px", "margin-top": "6px" }}>
                  is the engine running on {(import.meta as any).env?.VITE_ENGINE_URL ?? "http://127.0.0.1:8080"}?
                </div>
              </div>
            </Show>
            <Show
              when={manifest() || error()}
              fallback={
                <div style={{ color: PALETTE.ink2, padding: "40px 0", "text-align": "center" }}>
                  loading manifest…
                </div>
              }
            >
              <For each={visibleSystems()}>
                {(s) => (
                  <Show when={!activeSystem() || activeSystem() === s.name}>
                    <SystemSection system={s} matches={matches} />
                  </Show>
                )}
              </For>
            </Show>
          </main>
        </div>
      </div>
    </div>
  );
}

function SystemSection(props: { system: SystemDeclaration; matches: (s: string) => boolean }) {
  return (
    <section style={{ "margin-bottom": "28px" }}>
      <h3 style={{ margin: "0 0 4px 0", color: PALETTE.accent, "text-transform": "capitalize" }}>
        {props.system.name}
      </h3>
      <p style={{ margin: "0 0 14px 0", color: PALETTE.ink2 }}>{props.system.description}</p>

      <Show when={(props.system.verbs?.length ?? 0) > 0}>
        <h4 style={{ margin: "12px 0 6px 0", color: PALETTE.ink }}>verbs</h4>
        <For each={props.system.verbs ?? []}>{(v) => <VerbCard verb={v} />}</For>
      </Show>

      <Show when={(props.system.state_fields?.length ?? 0) > 0}>
        <h4 style={{ margin: "16px 0 6px 0", color: PALETTE.ink }}>state</h4>
        <For each={props.system.state_fields ?? []}>{(f) => <StateRow field={f} />}</For>
      </Show>

      <Show when={(props.system.sounds_emitted?.length ?? 0) > 0}>
        <h4 style={{ margin: "16px 0 6px 0", color: PALETTE.ink }}>sounds</h4>
        <For each={props.system.sounds_emitted ?? []}>{(s) => <SoundRow sound={s} />}</For>
      </Show>

      <Show when={(props.system.archetypes?.length ?? 0) > 0}>
        <h4 style={{ margin: "16px 0 6px 0", color: PALETTE.ink }}>archetypes</h4>
        <For each={props.system.archetypes ?? []}>{(a) => <ArchetypeRow arch={a} />}</For>
      </Show>
    </section>
  );
}

function VerbCard(props: { verb: VerbDeclaration }) {
  return (
    <div
      style={{
        background: "rgba(0,0,0,0.25)",
        border: `1px solid ${PALETTE.border}`,
        "border-radius": "4px",
        padding: "8px 12px",
        "margin-bottom": "6px",
      }}
    >
      <div style={{ display: "flex", "align-items": "baseline", gap: "12px" }}>
        <code style={{ color: PALETTE.accent, "font-weight": "600", "font-size": "14px" }}>{props.verb.verb}</code>
        <span style={{ color: PALETTE.ink2, "font-size": "12px" }}>{props.verb.description}</span>
      </div>
      <Show when={(props.verb.preconditions?.length ?? 0) > 0}>
        <div style={{ "margin-top": "6px", "font-size": "11px" }}>
          <span style={{ color: PALETTE.ink2 }}>requires: </span>
          <For each={props.verb.preconditions ?? []}>
            {(p, i) => (
              <span style={{ color: PALETTE.good }}>
                {p}{i() < (props.verb.preconditions?.length ?? 0) - 1 ? ", " : ""}
              </span>
            )}
          </For>
        </div>
      </Show>
      <Show when={(props.verb.rejection_reasons?.length ?? 0) > 0}>
        <div style={{ "margin-top": "3px", "font-size": "11px" }}>
          <span style={{ color: PALETTE.ink2 }}>rejects: </span>
          <For each={props.verb.rejection_reasons ?? []}>
            {(r, i) => (
              <span style={{ color: PALETTE.bad }}>
                {r}{i() < (props.verb.rejection_reasons?.length ?? 0) - 1 ? ", " : ""}
              </span>
            )}
          </For>
        </div>
      </Show>
      <Show when={(props.verb.emits_events?.length ?? 0) > 0}>
        <div style={{ "margin-top": "3px", "font-size": "11px" }}>
          <span style={{ color: PALETTE.ink2 }}>emits: </span>
          <For each={props.verb.emits_events ?? []}>
            {(e, i) => (
              <span style={{ color: PALETTE.ink }}>
                {e}{i() < (props.verb.emits_events?.length ?? 0) - 1 ? ", " : ""}
              </span>
            )}
          </For>
        </div>
      </Show>
    </div>
  );
}

function StateRow(props: { field: StateFieldDecl }) {
  return (
    <div style={{ display: "flex", gap: "10px", padding: "4px 0", "font-size": "12px" }}>
      <code style={{ color: PALETTE.accent, width: "140px" }}>{props.field.key}</code>
      <span style={{ width: "70px", color: PALETTE.ink2 }}>{props.field.type}</span>
      <span style={{ width: "70px", color: props.field.public_at_any_distance ? PALETTE.good : PALETTE.bad }}>
        {props.field.public_at_any_distance ? "public" : "private"}
      </span>
      <span style={{ flex: "1", color: PALETTE.ink2 }}>{props.field.meaning}</span>
    </div>
  );
}

function SoundRow(props: { sound: SoundDecl }) {
  return (
    <div style={{ display: "flex", gap: "10px", padding: "4px 0", "font-size": "12px" }}>
      <code style={{ color: PALETTE.accent, width: "140px" }}>{props.sound.kind}</code>
      <span style={{ flex: "1", color: PALETTE.ink2 }}>{props.sound.description}</span>
      <span style={{ color: PALETTE.ink2, "font-style": "italic" }}>{props.sound.emitted_by}</span>
    </div>
  );
}

function ArchetypeRow(props: { arch: ArchetypeDecl }) {
  return (
    <div style={{ "padding": "4px 0", "font-size": "12px" }}>
      <code style={{ color: PALETTE.accent }}>{props.arch.archetype}</code>
      <span style={{ "margin-left": "10px", color: PALETTE.ink2 }}>{props.arch.description}</span>
    </div>
  );
}

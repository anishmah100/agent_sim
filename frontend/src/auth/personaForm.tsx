// Persona form — what users fill in to attach an agent.
//
// Captures: name, bio, voice, terminal + instrumental goals, initial
// relationships, and the WS URL of their agent backend. Submitted to
// the engine's /api/v1/agent/register endpoint; engine returns
// agent_id + agent_secret + ws_url which the user feeds into their
// backend process.

import { createSignal, Show } from "solid-js";

interface PersonaFormData {
  name: string;
  bio: string;
  voice: string;
  terminal_goals: string;
  instrumental_goals: string;
  relationships: string;
  vision_mode: "structured" | "image" | "both";
  cadence_ms: number;
}

const initial: PersonaFormData = {
  name: "",
  bio: "",
  voice: "",
  terminal_goals: "",
  instrumental_goals: "",
  relationships: "",
  vision_mode: "structured",
  cadence_ms: 1000,
};

export function PersonaForm(props: { onSubmit: (data: PersonaFormData) => void }) {
  const [state, setState] = createSignal<PersonaFormData>({ ...initial });
  const [submitting, setSubmitting] = createSignal(false);

  const update = (k: keyof PersonaFormData) => (e: Event) => {
    const t = e.target as HTMLInputElement;
    const v = t.type === "number" ? Number(t.value) : t.value;
    setState({ ...state(), [k]: v });
  };

  return (
    <form
      class="persona-form"
      onSubmit={(e) => {
        e.preventDefault();
        setSubmitting(true);
        props.onSubmit(state());
      }}
    >
      <h2>Build your agent</h2>
      <label>Name<input value={state().name} onInput={update("name")} required /></label>
      <label>Bio<textarea value={state().bio} onInput={update("bio")} rows={3} /></label>
      <label>Voice style<input placeholder="e.g. terse, professorial, gleeful"
                               value={state().voice} onInput={update("voice")} /></label>
      <label>Terminal goals<textarea placeholder="What does this agent fundamentally want?"
                                     value={state().terminal_goals} onInput={update("terminal_goals")} rows={2} /></label>
      <label>Instrumental goals<textarea placeholder="Sub-goals that serve the terminal ones."
                                         value={state().instrumental_goals} onInput={update("instrumental_goals")} rows={2} /></label>
      <label>Initial relationships<textarea placeholder='e.g. {"Lyra": "rival", "Baker": "trusted friend"}'
                                            value={state().relationships} onInput={update("relationships")} rows={2} /></label>
      <fieldset>
        <legend>Vision mode</legend>
        <label><input type="radio" name="vm" value="structured" checked={state().vision_mode === "structured"}
                      onChange={update("vision_mode")} /> Structured (JSON only)</label>
        <label><input type="radio" name="vm" value="image" checked={state().vision_mode === "image"}
                      onChange={update("vision_mode")} /> Image (multimodal)</label>
        <label><input type="radio" name="vm" value="both" checked={state().vision_mode === "both"}
                      onChange={update("vision_mode")} /> Both</label>
      </fieldset>
      <label>Tick cadence (ms)<input type="number" min={200} step={100}
                                     value={state().cadence_ms} onInput={update("cadence_ms")} /></label>
      <Show when={submitting()}>
        <p>Registering…</p>
      </Show>
      <button type="submit" disabled={submitting()}>Attach my agent</button>
    </form>
  );
}

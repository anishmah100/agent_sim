// Persona form — what users fill in to attach an agent.
//
// Captures: name, bio, voice, terminal + instrumental goals, initial
// relationships, vision mode, cadence. Styled to match the rest of the
// HUD (dark plate, cream text, gold-yellow primary accent).

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

const PALETTE = {
  bg: "#181425",
  panel: "#262b44",
  border: "#3a4466",
  text: "#ead4aa",
  textDim: "#8b9bb4",
  accent: "#fee761",
  danger: "#e43b44",
};

const labelStyle = {
  display: "flex",
  "flex-direction": "column" as const,
  gap: "4px",
  "font-size": "12px",
  color: PALETTE.textDim,
  "font-weight": "500",
};

const inputStyle = {
  background: PALETTE.bg,
  color: PALETTE.text,
  border: `1px solid ${PALETTE.border}`,
  "border-radius": "3px",
  padding: "6px 8px",
  "font-size": "13px",
  "font-family": "ui-sans-serif, system-ui, sans-serif",
  width: "100%",
  "box-sizing": "border-box" as const,
};

const textareaStyle = {
  ...inputStyle,
  resize: "vertical" as const,
  "min-height": "52px",
  "font-family": "ui-sans-serif, system-ui, sans-serif",
  "line-height": "1.35",
};

export function PersonaForm(props: { onSubmit: (data: PersonaFormData) => void }) {
  const [state, setState] = createSignal<PersonaFormData>({ ...initial });
  const [submitting, setSubmitting] = createSignal(false);

  const update = (k: keyof PersonaFormData) => (e: Event) => {
    const t = e.target as HTMLInputElement | HTMLTextAreaElement;
    const v = (t as HTMLInputElement).type === "number" ? Number(t.value) : t.value;
    setState({ ...state(), [k]: v });
  };

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        setSubmitting(true);
        props.onSubmit(state());
      }}
      style={{
        display: "flex",
        "flex-direction": "column",
        gap: "12px",
        color: PALETTE.text,
        "font-family": "ui-sans-serif, system-ui, sans-serif",
      }}
    >
      <h2 style={{ color: PALETTE.accent, margin: "0 0 4px", "font-size": "18px" }}>
        Build your agent
      </h2>
      <p style={{ margin: "0 0 6px", color: PALETTE.textDim, "font-size": "12px" }}>
        Fill in a persona. The engine returns credentials you paste into the SDK to attach your bot.
      </p>

      <label style={labelStyle}>
        Name
        <input
          required
          value={state().name}
          onInput={update("name")}
          placeholder="e.g. Brakk the Forge-King"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Bio
        <textarea
          value={state().bio}
          onInput={update("bio") as unknown as (e: Event) => void}
          rows={3}
          placeholder="Who are they? What drives them?"
          style={textareaStyle}
        />
      </label>

      <label style={labelStyle}>
        Voice style
        <input
          value={state().voice}
          onInput={update("voice")}
          placeholder="e.g. terse, professorial, gleeful"
          style={inputStyle}
        />
      </label>

      <label style={labelStyle}>
        Terminal goals
        <textarea
          value={state().terminal_goals}
          onInput={update("terminal_goals") as unknown as (e: Event) => void}
          rows={2}
          placeholder="What does this agent fundamentally want?"
          style={textareaStyle}
        />
      </label>

      <label style={labelStyle}>
        Instrumental goals
        <textarea
          value={state().instrumental_goals}
          onInput={update("instrumental_goals") as unknown as (e: Event) => void}
          rows={2}
          placeholder="Sub-goals that serve the terminal ones"
          style={textareaStyle}
        />
      </label>

      <label style={labelStyle}>
        Initial relationships
        <textarea
          value={state().relationships}
          onInput={update("relationships") as unknown as (e: Event) => void}
          rows={2}
          placeholder='JSON, e.g. {"Lyra":"rival","Baker":"trusted friend"}'
          style={{ ...textareaStyle, "font-family": "ui-monospace, monospace" }}
        />
      </label>

      <fieldset
        style={{
          border: `1px solid ${PALETTE.border}`,
          "border-radius": "4px",
          padding: "8px 10px",
          margin: "0",
          color: PALETTE.text,
        }}
      >
        <legend style={{ color: PALETTE.textDim, "font-size": "12px", padding: "0 4px" }}>
          Vision mode
        </legend>
        <div style={{ display: "flex", gap: "16px", "flex-wrap": "wrap", "font-size": "13px" }}>
          <label style={{ display: "flex", "align-items": "center", gap: "6px" }}>
            <input
              type="radio"
              name="vm"
              value="structured"
              checked={state().vision_mode === "structured"}
              onChange={update("vision_mode")}
            />
            Structured (JSON)
          </label>
          <label style={{ display: "flex", "align-items": "center", gap: "6px" }}>
            <input
              type="radio"
              name="vm"
              value="image"
              checked={state().vision_mode === "image"}
              onChange={update("vision_mode")}
            />
            Image (multimodal)
          </label>
          <label style={{ display: "flex", "align-items": "center", gap: "6px" }}>
            <input
              type="radio"
              name="vm"
              value="both"
              checked={state().vision_mode === "both"}
              onChange={update("vision_mode")}
            />
            Both
          </label>
        </div>
      </fieldset>

      <label style={labelStyle}>
        Tick cadence (ms) — min 200, default 1000
        <input
          type="number"
          min={200}
          step={100}
          value={state().cadence_ms}
          onInput={update("cadence_ms")}
          style={{ ...inputStyle, "max-width": "140px" }}
        />
      </label>

      <Show when={submitting()}>
        <p style={{ color: PALETTE.textDim, "font-size": "12px", margin: "0" }}>
          Registering…
        </p>
      </Show>

      <button
        type="submit"
        disabled={submitting() || !state().name}
        style={{
          background: PALETTE.accent,
          color: PALETTE.bg,
          border: `1px solid ${PALETTE.accent}`,
          "border-radius": "3px",
          padding: "8px 16px",
          "font-weight": "600",
          "font-size": "13px",
          cursor: submitting() ? "not-allowed" : "pointer",
          opacity: submitting() ? "0.6" : "1",
          "margin-top": "4px",
          "align-self": "flex-start" as const,
        }}
      >
        Attach my agent
      </button>
    </form>
  );
}

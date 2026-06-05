// JoinAgent — modal entry point for users who want to attach their own
// agent to the running world. Wraps PersonaForm, calls the engine
// register endpoint, and renders the resulting credentials + a copy-
// paste-able SDK quickstart.
//
// Critical UX rule: the agent_secret appears ONCE here. If the user
// closes the modal without copying it, they have to re-register.

import { createSignal, Show } from "solid-js";
import { PersonaForm } from "../auth/personaForm";
import { registerAgent, type RegisterResponse } from "../auth/register";

interface PersonaSubmit {
  name: string;
  bio: string;
  voice: string;
  terminal_goals: string;
  instrumental_goals: string;
  relationships: string;
  vision_mode: "structured" | "image" | "both";
  cadence_ms: number;
}

export function JoinAgent(props: { open: boolean; onClose: () => void }) {
  const [creds, setCreds] = createSignal<RegisterResponse | null>(null);
  const [err, setErr] = createSignal<string | null>(null);

  const submit = async (data: PersonaSubmit) => {
    setErr(null);
    try {
      const resp = await registerAgent({
        user_token: "dev",   // empty engine accepts this; JWT-enabled deploy rejects
        persona_blob: {
          name: data.name, bio: data.bio, voice: data.voice,
          terminal_goals: data.terminal_goals,
          instrumental_goals: data.instrumental_goals,
          relationships: data.relationships,
        },
        vision_mode: data.vision_mode,
        cadence_ms: data.cadence_ms,
      });
      setCreds(resp);
    } catch (e) {
      setErr((e as Error).message);
    }
  };

  return (
    <Show when={props.open}>
      <div
        role="dialog"
        aria-label="Join as agent"
        style={{
          position: "fixed",
          inset: "0",
          background: "rgba(24, 20, 37, 0.75)",
          display: "flex",
          "align-items": "center",
          "justify-content": "center",
          "z-index": "20",
        }}
        onClick={(e) => {
          if (e.target === e.currentTarget) props.onClose();
        }}
      >
        <div
          style={{
            width: "560px",
            "max-width": "90vw",
            "max-height": "90vh",
            "overflow-y": "auto",
            background: "#262b44",
            color: "#ead4aa",
            padding: "20px",
            "border-radius": "6px",
            border: "1px solid #3a4466",
            "font-family": "ui-sans-serif, system-ui, sans-serif",
          }}
        >
          <Show
            when={creds() === null}
            fallback={<Credentials creds={creds()!} onClose={props.onClose} />}
          >
            <PersonaForm onSubmit={submit} />
            <Show when={err()}>
              <p style={{ color: "#e43b44" }}>register failed: {err()}</p>
            </Show>
          </Show>
        </div>
      </div>
    </Show>
  );
}

function Credentials(props: { creds: RegisterResponse; onClose: () => void }) {
  const code = () =>
    `from agent_sim_sdk import register_and_connect, VisionMode\n` +
    `import asyncio\n\n` +
    `async def main():\n` +
    `    agent = await register_and_connect(\n` +
    `        "${import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080"}",\n` +
    `        user_token="dev",\n` +
    `        persona={"name": "your bot"},\n` +
    `    )\n` +
    `    # agent_id    = "${props.creds.agent_id}"\n` +
    `    # agent_secret = "${props.creds.agent_secret}"\n` +
    `    # ws_url      = "${props.creds.ws_url}"\n` +
    `    # entity_id   = "${props.creds.entity_id}"\n` +
    `    async for obs in agent.observations():\n` +
    `        print(obs)\n\n` +
    `asyncio.run(main())\n`;

  const copy = () => navigator.clipboard.writeText(code());

  return (
    <div data-testid="join-credentials">
      <h2 style={{ color: "#fee761", "margin-top": "0" }}>Agent attached</h2>
      <p>
        <strong>Save these credentials</strong> — the secret appears only here.
      </p>
      <pre
        data-testid="join-creds-json"
        style={{
          background: "#181425",
          color: "#ead4aa",
          padding: "10px",
          "border-radius": "4px",
          "font-size": "11px",
          "white-space": "pre-wrap",
          "word-break": "break-all",
        }}
      >
        {JSON.stringify(props.creds, null, 2)}
      </pre>
      <h3 style={{ color: "#fee761" }}>Python quickstart</h3>
      <pre
        data-testid="join-creds-code"
        style={{
          background: "#181425",
          color: "#c0cbdc",
          padding: "10px",
          "border-radius": "4px",
          "font-size": "11px",
          "white-space": "pre-wrap",
          "overflow-x": "auto",
        }}
      >
        {code()}
      </pre>
      <p style={{ "font-size": "11px", color: "#8b9bb4" }}>
        Run <code>pip install agent-sim-sdk</code>, paste the snippet, and your
        agent connects to the live world.
      </p>
      <div style={{ display: "flex", gap: "8px", "margin-top": "12px" }}>
        <button
          type="button"
          onClick={copy}
          style={{
            background: "#3a4466",
            color: "#fee761",
            border: "1px solid #5a6988",
            padding: "6px 12px",
            "border-radius": "3px",
            cursor: "pointer",
          }}
        >
          Copy code
        </button>
        <button
          type="button"
          onClick={props.onClose}
          style={{
            background: "#262b44",
            color: "#ead4aa",
            border: "1px solid #3a4466",
            padding: "6px 12px",
            "border-radius": "3px",
            cursor: "pointer",
          }}
        >
          Close
        </button>
      </div>
    </div>
  );
}

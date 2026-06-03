// Dialogue box — renders speech events as styled bubbles + a bottom
// Pokemon-style dialogue strip. Subscribes to the audible event
// stream over WS.

import { For, Show, createMemo } from "solid-js";

export interface DialogueEvent {
  event_id: string;
  from_entity: string;
  speaker_label: string;
  text: string;
  kind: "speech" | "whisper" | "shout";
  tick: number;
}

export interface DialogueProps {
  events: DialogueEvent[];
  focused: DialogueEvent | null;
  onAdvance: () => void;
}

export function Dialogue(props: DialogueProps) {
  // The bottom strip shows the focused message (if any) plus pages
  // chunked by ~80 chars for readability.
  const pages = createMemo(() => {
    const ev = props.focused;
    if (!ev) return [] as string[];
    const out: string[] = [];
    const words = ev.text.split(/\s+/);
    let buf = "";
    for (const w of words) {
      if (buf.length + w.length + 1 > 80) {
        out.push(buf);
        buf = w;
      } else {
        buf = buf ? `${buf} ${w}` : w;
      }
    }
    if (buf) out.push(buf);
    return out;
  });

  return (
    <>
      <Show when={props.focused}>
        <div class="dialogue-strip" onClick={props.onAdvance}>
          <div class="speaker">{props.focused!.speaker_label}</div>
          <For each={pages()}>
            {(p) => <div class="line">{p}</div>}
          </For>
          <div class="hint">click to advance</div>
        </div>
      </Show>
      <div class="dialogue-feed">
        <For each={props.events.slice(-12).reverse()}>
          {(e) => (
            <div class={`bubble ${e.kind}`}>
              <span class="who">{e.speaker_label}:</span>
              <span class="what">{e.text}</span>
            </div>
          )}
        </For>
      </div>
    </>
  );
}

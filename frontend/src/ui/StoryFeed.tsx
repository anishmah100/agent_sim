// StoryFeed — D17 narrator-driven play-by-play.
//
// Polls /api/v1/narrator/recent every 3s and renders the L1-L4
// NarratorSummary records the narrator process emits to
// .runlog/narrator.jsonl. Newest entries appear at the top.
//
// Levels:
//   L1 — individual narrator (per-agent), cyan
//   L2 — group narrator (cluster), purple
//   L3 — society narrator, amber
//   L4 — world/era narrator, red
//
// The filter row lets the viewer scope to a single tier (or All).
// When a record was skipped (llm: "skipped"), we render a small
// dimmed "(skipped)" instead of the body so the gap is visible
// without being noisy.

import { createSignal, createMemo, onCleanup, onMount, For, Show } from 'solid-js'
import { fetchNarratorRecent, type NarratorRecord } from '../net/api'

type Filter = 'all' | 'L1' | 'L2' | 'L3L4'

const LEVEL_COLOR: Record<string, string> = {
  L1: '#5eead4',
  L2: '#a78bfa',
  L3: '#fbbf24',
  L4: '#f87171',
}

const FILTERS: Array<{ key: Filter; label: string }> = [
  { key: 'all',  label: 'All' },
  { key: 'L1',   label: 'L1' },
  { key: 'L2',   label: 'L2' },
  { key: 'L3L4', label: 'L3+L4' },
]

const POLL_MS = 3000

function truncate(s: string, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n - 1) + '…' : s
}

function matchesFilter(level: string, f: Filter): boolean {
  switch (f) {
    case 'all':  return true
    case 'L1':   return level === 'L1'
    case 'L2':   return level === 'L2'
    case 'L3L4': return level === 'L3' || level === 'L4'
  }
}

export function StoryFeed() {
  const [records, setRecords] = createSignal<NarratorRecord[]>([])
  const [filter, setFilter] = createSignal<Filter>('all')
  const [err, setErr] = createSignal<string | null>(null)
  let pollHandle: number | undefined
  let inflight = false

  const visible = createMemo(() =>
    records().filter((r) => matchesFilter(r.level, filter())),
  )

  const fetchOnce = async () => {
    if (inflight) return
    inflight = true
    try {
      const body = await fetchNarratorRecent(30)
      // Newest first; the endpoint is allowed to return either order
      // so we re-sort by tick descending to be safe.
      const list = (body.events ?? []).slice()
      list.sort((a, b) => b.tick - a.tick)
      setRecords(list)
      setErr(null)
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      inflight = false
    }
  }

  onMount(() => {
    void fetchOnce()
    pollHandle = window.setInterval(() => void fetchOnce(), POLL_MS)
  })
  onCleanup(() => {
    if (pollHandle) window.clearInterval(pollHandle)
  })

  return (
    <div
      data-testid='story-feed'
      style={{
        position: 'absolute',
        bottom: '16px',
        right: '16px',
        width: '380px',
        height: '180px',
        background: 'rgba(24, 20, 37, 0.95)',
        border: '1px solid #3a4466',
        'border-radius': '4px',
        padding: '8px 10px',
        color: '#ead4aa',
        'font-size': '12px',
        'z-index': '15',
        display: 'flex',
        'flex-direction': 'column',
        'box-shadow': '0 4px 18px rgba(0,0,0,0.45)',
      }}
    >
      {/* Header: title + filter pills */}
      <div
        style={{
          display: 'flex',
          'align-items': 'center',
          gap: '6px',
          'margin-bottom': '6px',
          'padding-bottom': '6px',
          'border-bottom': '1px solid #3a4466',
        }}
      >
        <strong style={{ color: '#fee761', 'font-size': '12px' }}>Story Feed</strong>
        <span style={{ 'margin-left': 'auto', display: 'flex', gap: '4px' }}>
          <For each={FILTERS}>
            {(f) => (
              <FilterPill
                label={f.label}
                active={filter() === f.key}
                testId={`story-feed-filter-${f.key}`}
                onClick={() => setFilter(f.key)}
              />
            )}
          </For>
        </span>
      </div>

      {/* Error strip */}
      <Show when={err()}>
        <div style={{ color: '#f87171', 'font-size': '10px', 'margin-bottom': '4px' }}>
          {err()}
        </div>
      </Show>

      {/* Body: newest-first scrollable list */}
      <div
        style={{
          flex: '1',
          'overflow-y': 'auto',
          'padding-right': '4px',
        }}
      >
        <Show
          when={visible().length > 0}
          fallback={
            <div style={{ color: '#5a6988', 'font-style': 'italic', 'font-size': '11px' }}>
              {records().length === 0
                ? 'waiting for narrator…'
                : 'no events at this filter'}
            </div>
          }
        >
          <For each={visible()}>
            {(rec) => <StoryRow rec={rec} />}
          </For>
        </Show>
      </div>
    </div>
  )
}

function FilterPill(props: {
  label: string
  active: boolean
  testId: string
  onClick: () => void
}) {
  return (
    <button
      type='button'
      data-testid={props.testId}
      onClick={() => props.onClick()}
      style={{
        background: props.active ? '#feae34' : 'transparent',
        color: props.active ? '#1f2238' : '#ead4aa',
        border: '1px solid ' + (props.active ? '#feae34' : '#3a4466'),
        'border-radius': '10px',
        padding: '1px 8px',
        cursor: 'pointer',
        'font-size': '10px',
        'font-family': 'ui-monospace, monospace',
      }}
    >
      {props.label}
    </button>
  )
}

function StoryRow(props: { rec: NarratorRecord }) {
  const [opacity, setOpacity] = createSignal(0)
  onMount(() => {
    // Force a tick so the transition fires from 0 → 1.
    requestAnimationFrame(() => setOpacity(1))
  })
  const color = () => LEVEL_COLOR[props.rec.level] ?? '#ead4aa'
  const scope = () => truncate(props.rec.scope ?? '', 18)
  const skipped = () => props.rec.llm === 'skipped'
  return (
    <div
      data-testid='story-feed-row'
      style={{
        opacity: String(opacity()),
        transition: 'opacity 250ms ease-in',
        'border-left': '2px solid ' + color(),
        'padding-left': '6px',
        'margin-bottom': '6px',
        'line-height': '1.35',
      }}
    >
      <div
        style={{
          display: 'flex',
          gap: '6px',
          'align-items': 'baseline',
          'font-size': '10px',
          color: '#8b9bb4',
          'font-family': 'ui-monospace, monospace',
        }}
      >
        <span
          style={{
            color: color(),
            'font-weight': '600',
          }}
        >
          {props.rec.level}
        </span>
        <span>t{props.rec.tick}</span>
        <Show when={scope().length > 0}>
          <span
            title={props.rec.scope}
            style={{
              overflow: 'hidden',
              'text-overflow': 'ellipsis',
              'white-space': 'nowrap',
              'max-width': '180px',
            }}
          >
            {scope()}
          </span>
        </Show>
      </div>
      <Show
        when={!skipped()}
        fallback={
          <div style={{ color: '#5a6988', 'font-style': 'italic', 'font-size': '10px' }}>
            (skipped)
          </div>
        }
      >
        <div
          style={{
            'font-size': '11px',
            color: '#ead4aa',
            display: '-webkit-box',
            '-webkit-line-clamp': '3',
            '-webkit-box-orient': 'vertical',
            overflow: 'hidden',
            'text-overflow': 'ellipsis',
          }}
        >
          {props.rec.text}
        </div>
      </Show>
    </div>
  )
}

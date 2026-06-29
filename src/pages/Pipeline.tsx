import { useState, useCallback, useRef, Fragment } from 'react';
import type { VideoPipeline, PipelineStatus, ExportTarget, QualityCheck } from '../types';
import { TOPICS } from '../data/topics';
import { PlatformDot } from '../components/PlatformDot';
import { riskMeta, trendColor } from '../lib/scoring';
import {
  createInitialPipeline,
  runPipeline,
  checkDuplicateTopic,
} from '../lib/pipeline/orchestrator';
import { resetQueue } from '../lib/pipeline/renderQueueService';
import { getPlatformLabel } from '../lib/pipeline/exportService';

// ─── Stage helpers ────────────────────────────────────────────

type StageKey = 'script' | 'storyboard' | 'assets' | 'render' | 'export';
type StageState = 'pending' | 'active' | 'done';

const STATUS_STAGE_IDX: Record<PipelineStatus, number> = {
  idle: 0,
  'generating-script': 1,
  'generating-storyboard': 2,
  'planning-assets': 3,
  'queued-render': 4,
  rendering: 5,
  exporting: 6,
  done: 7,
  error: -1,
};

function getStageStatuses(status: PipelineStatus): Record<StageKey, StageState> {
  const idx = STATUS_STAGE_IDX[status] ?? 0;
  const s = (active: number, doneFrom: number): StageState =>
    idx >= doneFrom ? 'done' : idx === active ? 'active' : 'pending';
  return {
    script:     s(1, 2),
    storyboard: s(2, 3),
    assets:     s(3, 4),
    render:     s(4, 6), // both queued-render (4) and rendering (5) are "active" for render stage
    export:     s(6, 7),
  };
}

// render stage special: idx 4 or 5 → active
function renderStageState(status: PipelineStatus): StageState {
  const idx = STATUS_STAGE_IDX[status] ?? 0;
  if (idx >= 6) return 'done';
  if (idx === 4 || idx === 5) return 'active';
  return 'pending';
}

const STAGES: { key: StageKey; label: string }[] = [
  { key: 'script',     label: 'Script' },
  { key: 'storyboard', label: 'Storyboard' },
  { key: 'assets',     label: 'Assets' },
  { key: 'render',     label: 'Render' },
  { key: 'export',     label: 'Export' },
];

// ─── Sub-components ───────────────────────────────────────────

function StageBar({ status }: { status: PipelineStatus }) {
  const baseStatuses = getStageStatuses(status);
  const statuses: Record<StageKey, StageState> = {
    ...baseStatuses,
    render: renderStageState(status),
  };

  return (
    <div style={{ display: 'flex', alignItems: 'flex-start', gap: 0, padding: '10px 0 4px' }}>
      {STAGES.map((stage, i) => {
        const st = statuses[stage.key];
        const dotColor =
          st === 'done'   ? 'var(--green)'  :
          st === 'active' ? 'var(--accent)' :
          'var(--border-strong)';
        const labelColor =
          st === 'done'   ? 'var(--green)'    :
          st === 'active' ? 'var(--accent)'   :
          'var(--text-dimmer)';

        return (
          <Fragment key={stage.key}>
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 5 }}>
              <div
                style={{
                  width: 9,
                  height: 9,
                  borderRadius: '50%',
                  background: dotColor,
                  flexShrink: 0,
                  animation: st === 'active' ? 'blip 1.2s infinite' : undefined,
                  border: st === 'pending' ? '1.5px solid var(--border-strong)' : 'none',
                  boxSizing: 'border-box',
                }}
              />
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 9,
                  color: labelColor,
                  letterSpacing: '0.04em',
                  whiteSpace: 'nowrap',
                }}
              >
                {stage.label}
              </span>
            </div>
            {i < STAGES.length - 1 && (
              <div
                style={{
                  height: 1,
                  flex: 1,
                  minWidth: 16,
                  marginTop: 4,
                  background:
                    statuses[stage.key] === 'done'
                      ? 'var(--green-border)'
                      : 'var(--border-inner)',
                }}
              />
            )}
          </Fragment>
        );
      })}
    </div>
  );
}

function QualityChip({ check }: { check: QualityCheck }) {
  const color =
    check.severity === 'error' && !check.passed   ? 'var(--red)'    :
    check.severity === 'warning' && !check.passed ? 'var(--yellow)' :
    check.severity === 'info'                      ? 'var(--accent)' :
    'var(--green)';
  const bg =
    check.severity === 'error' && !check.passed   ? 'rgba(232,115,107,.1)'   :
    check.severity === 'warning' && !check.passed ? 'rgba(227,181,78,.1)'    :
    check.severity === 'info'                      ? 'var(--accent-dim)'      :
    'var(--green-dim)';
  const icon =
    check.severity === 'error' && !check.passed   ? '✕' :
    check.severity === 'warning' && !check.passed ? '!' :
    check.severity === 'info'                      ? '~' :
    '✓';

  return (
    <span
      title={check.message}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        padding: '2px 8px',
        borderRadius: 20,
        background: bg,
        fontSize: 10.5,
        fontFamily: 'var(--font-mono)',
        color,
        cursor: 'default',
        whiteSpace: 'nowrap',
      }}
    >
      <span style={{ fontSize: 9 }}>{icon}</span>
      {check.name}
    </span>
  );
}

function ExportChip({ target, sizeKB, status }: { target: ExportTarget; sizeKB: number; status: 'pending' | 'ready' | 'failed' }) {
  const color = status === 'ready' ? 'var(--green)' : status === 'failed' ? 'var(--red)' : 'var(--text-dimmer)';
  const bg = status === 'ready' ? 'var(--green-dim)' : status === 'failed' ? 'var(--red-dim)' : 'var(--bg-subtle)';

  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        padding: '2px 8px',
        borderRadius: 20,
        background: bg,
        fontSize: 10,
        fontFamily: 'var(--font-mono)',
        color,
        whiteSpace: 'nowrap',
      }}
    >
      {getPlatformLabel(target)}
      {status === 'ready' && sizeKB > 0 && (
        <span style={{ color: 'var(--text-dim)', fontSize: 9 }}>
          {(sizeKB / 1024).toFixed(1)} MB
        </span>
      )}
      {status === 'failed' && <span style={{ fontSize: 9 }}>✕ over limit</span>}
    </span>
  );
}

function StatusLabel({ status }: { status: PipelineStatus }) {
  const map: Record<PipelineStatus, { label: string; color: string }> = {
    idle:                   { label: 'Idle',             color: 'var(--text-dimmer)' },
    'generating-script':    { label: 'Writing script…',  color: 'var(--accent)'      },
    'generating-storyboard':{ label: 'Storyboarding…',   color: 'var(--accent)'      },
    'planning-assets':      { label: 'Planning assets…', color: 'var(--accent)'      },
    'queued-render':        { label: 'Queued for render', color: 'var(--yellow)'     },
    rendering:              { label: 'Rendering…',       color: 'var(--yellow)'      },
    exporting:              { label: 'Exporting…',       color: '#6aa3f0'            },
    done:                   { label: 'Done',             color: 'var(--green)'       },
    error:                  { label: 'Error',            color: 'var(--red)'         },
  };
  const m = map[status];
  return (
    <span
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: 10.5,
        color: m.color,
        display: 'flex',
        alignItems: 'center',
        gap: 5,
      }}
    >
      {(status !== 'idle' && status !== 'done' && status !== 'error') && (
        <span
          style={{
            width: 6, height: 6, borderRadius: '50%',
            background: m.color,
            animation: 'blip 1.2s infinite',
            flexShrink: 0,
          }}
        />
      )}
      {m.label}
    </span>
  );
}

// ─── Detail panel ─────────────────────────────────────────────

function PipelineDetail({ p }: { p: VideoPipeline }) {
  if (p.status === 'idle') {
    return (
      <div style={{ padding: '10px 0', color: 'var(--text-dimmer)', fontSize: 12 }}>
        Ready to run — click Run to start this pipeline.
      </div>
    );
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 10, paddingTop: 10 }}>
      {/* Script */}
      {p.script && (
        <div
          style={{
            padding: '10px 14px',
            background: 'var(--bg-raised)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-inner)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 6 }}>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--accent)', letterSpacing: '0.08em' }}>
              SCRIPT
            </span>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)' }}>
              {p.script.estimatedDuration}s · {p.script.wordCount} words
            </span>
          </div>
          <div style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 8, lineHeight: 1.5 }}>
            <strong style={{ color: 'var(--text-primary)' }}>Hook: </strong>
            {p.script.hook}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
            {p.script.qualityChecks.map(c => (
              <QualityChip key={c.name} check={c} />
            ))}
          </div>
        </div>
      )}

      {/* Storyboard */}
      {p.storyboard && p.storyboard.length > 0 && (
        <div
          style={{
            padding: '10px 14px',
            background: 'var(--bg-raised)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-inner)',
          }}
        >
          <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: '#e87bc8', letterSpacing: '0.08em', marginBottom: 7 }}>
            STORYBOARD — {p.storyboard.length} scenes
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
            {p.storyboard.map(scene => (
              <div
                key={scene.index}
                style={{ display: 'flex', gap: 8, alignItems: 'flex-start' }}
              >
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: 9,
                    color:
                      scene.type === 'hook' ? 'var(--accent)' :
                      scene.type === 'cta'  ? 'var(--green)'  :
                      'var(--text-dimmer)',
                    width: 34,
                    flexShrink: 0,
                    paddingTop: 1,
                  }}
                >
                  {scene.timeRange}
                </span>
                <span style={{ fontSize: 11, color: 'var(--text-muted)', flex: 1, lineHeight: 1.4 }}>
                  {scene.visual}
                </span>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: 9,
                    padding: '1px 6px',
                    borderRadius: 10,
                    background:
                      scene.type === 'hook' ? 'var(--accent-dim)'  :
                      scene.type === 'cta'  ? 'var(--green-dim)'   :
                      'var(--bg-subtle)',
                    color:
                      scene.type === 'hook' ? 'var(--accent)' :
                      scene.type === 'cta'  ? 'var(--green)'  :
                      'var(--text-dim)',
                    flexShrink: 0,
                  }}
                >
                  {scene.type.toUpperCase()}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Asset plan */}
      {p.assetPlan && (
        <div
          style={{
            padding: '10px 14px',
            background: 'var(--bg-raised)',
            borderRadius: 'var(--radius-md)',
            border: `1px solid ${p.assetPlan.copyrightSafe ? 'var(--border-inner)' : 'rgba(227,181,78,.3)'}`,
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 7 }}>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: '#43e0d8', letterSpacing: '0.08em' }}>
              ASSETS
            </span>
            {!p.assetPlan.copyrightSafe && (
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 9.5, color: 'var(--yellow)' }}>
                ⚠ copyright review needed
              </span>
            )}
            {p.assetPlan.copyrightSafe && (
              <span style={{ fontFamily: 'var(--font-mono)', fontSize: 9.5, color: 'var(--green)' }}>
                ✓ copyright-safe
              </span>
            )}
          </div>
          <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
            {[
              ['Music', p.assetPlan.musicTrack],
              ['Voice', p.assetPlan.voiceStyle],
              ['Captions', p.assetPlan.captionStyle],
              ['Grade', p.assetPlan.colorGrade],
            ].map(([label, val]) => (
              <div key={label} style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span style={{ fontFamily: 'var(--font-mono)', fontSize: 9, color: 'var(--text-dimmer)', letterSpacing: '0.06em' }}>
                  {label}
                </span>
                <span style={{ fontSize: 11, color: 'var(--text-secondary)' }}>{val}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Render job */}
      {p.renderJob && (
        <div
          style={{
            padding: '10px 14px',
            background: 'var(--bg-raised)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-inner)',
          }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 7 }}>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--yellow)', letterSpacing: '0.08em' }}>
              RENDER
            </span>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)' }}>
              {p.renderJob.resolution} · {p.renderJob.fps}fps · {p.renderJob.format.toUpperCase()}
            </span>
          </div>
          <div style={{ display: 'flex', gap: 14, alignItems: 'center', flexWrap: 'wrap' }}>
            <div
              style={{
                flex: 1,
                minWidth: 120,
                height: 4,
                background: 'var(--border-strong)',
                borderRadius: 4,
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  height: '100%',
                  width: `${p.renderJob.progress}%`,
                  background:
                    p.renderJob.status === 'done'
                      ? 'var(--green)'
                      : 'var(--yellow)',
                  borderRadius: 4,
                  transition: 'width 0.4s ease',
                }}
              />
            </div>
            <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-muted)', flexShrink: 0 }}>
              {p.renderJob.status === 'done'
                ? '100% · Done'
                : p.renderJob.status === 'queued'
                  ? `Queue pos. ${p.renderJob.queuePosition}`
                  : `${p.renderJob.progress}%`}
            </span>
          </div>
        </div>
      )}

      {/* Export package */}
      {p.exportPackage && (
        <div
          style={{
            padding: '10px 14px',
            background: 'var(--bg-raised)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-inner)',
          }}
        >
          <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: '#6aa3f0', letterSpacing: '0.08em', marginBottom: 8 }}>
            EXPORT PACKAGE — {p.exportPackage.files.filter(f => f.status === 'ready').length}/{p.exportPackage.files.length} platforms ready
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5 }}>
            {p.exportPackage.files.map(f => (
              <ExportChip
                key={f.target}
                target={f.target}
                sizeKB={f.sizeKB}
                status={f.status}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Pipeline card ────────────────────────────────────────────

function PipelineCard({
  pipeline,
  onRun,
  isRunning,
  isDuplicate,
}: {
  pipeline: VideoPipeline;
  onRun: () => void;
  isRunning: boolean;
  isDuplicate: boolean;
}) {
  const topic = TOPICS.find(t => t.id === pipeline.topicId)!;
  const tc = trendColor(topic.trendScore);
  const rm = riskMeta(topic.riskScore);
  const rankLabel = String(topic.rank).padStart(2, '0');
  const canRun = !isRunning && pipeline.status === 'idle';

  return (
    <article
      style={{
        background: 'var(--bg-card)',
        border: `1px solid ${pipeline.status === 'done' ? 'rgba(95,211,154,.2)' : 'var(--border-card)'}`,
        borderRadius: 'var(--radius-lg)',
        padding: '16px 20px',
        display: 'flex',
        flexDirection: 'column',
        gap: 6,
      }}
    >
      {/* Header row */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 12 }}>
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 10,
            color: 'var(--text-dimmer)',
            paddingTop: 2,
            flexShrink: 0,
          }}
        >
          {rankLabel}
        </span>

        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            style={{
              fontSize: 13.5,
              fontWeight: 600,
              color: 'var(--text-primary)',
              lineHeight: 1.3,
              marginBottom: 2,
            }}
          >
            {topic.title}
          </div>
          <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
            <span
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 10,
                color: tc,
                fontWeight: 700,
              }}
            >
              {topic.trendScore}↑
            </span>
            <span
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 9.5,
                padding: '1px 6px',
                borderRadius: 10,
                background: rm.bg,
                color: rm.color,
              }}
            >
              {rm.label} risk
            </span>
            <div style={{ display: 'flex', gap: 4 }}>
              {topic.platforms.map(p => (
                <PlatformDot key={p} platform={p} size={15} />
              ))}
            </div>
            {isDuplicate && pipeline.status === 'idle' && (
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 9,
                  padding: '1px 6px',
                  borderRadius: 10,
                  background: 'rgba(227,181,78,.1)',
                  color: 'var(--yellow)',
                }}
              >
                ⚠ ran before
              </span>
            )}
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexShrink: 0 }}>
          <StatusLabel status={pipeline.status} />
          <button
            onClick={onRun}
            disabled={!canRun}
            style={{
              padding: '4px 12px',
              borderRadius: 'var(--radius-sm)',
              border: '1px solid var(--border-strong)',
              background: canRun ? 'var(--bg-raised)' : 'transparent',
              color: canRun ? 'var(--text-secondary)' : 'var(--text-dimmer)',
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
              cursor: canRun ? 'pointer' : 'not-allowed',
              transition: 'background 0.12s',
              flexShrink: 0,
            }}
          >
            {pipeline.status === 'done' ? '✓ Done' : isRunning ? 'Running…' : 'Run'}
          </button>
        </div>
      </div>

      {/* Stage bar */}
      <StageBar status={pipeline.status} />

      {/* Detail panel */}
      <PipelineDetail p={pipeline} />

      {/* Quality warnings */}
      {pipeline.qualityWarnings.length > 0 && (
        <div
          style={{
            display: 'flex',
            gap: 6,
            alignItems: 'center',
            padding: '6px 10px',
            borderRadius: 'var(--radius-sm)',
            background: 'rgba(227,181,78,.08)',
            border: '1px solid rgba(227,181,78,.2)',
            flexWrap: 'wrap',
          }}
        >
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 9.5, color: 'var(--yellow)', flexShrink: 0 }}>
            WARNINGS
          </span>
          {pipeline.qualityWarnings.map((w, i) => (
            <span key={i} style={{ fontSize: 11, color: 'var(--yellow)', lineHeight: 1.4 }}>
              {w}
            </span>
          ))}
        </div>
      )}
    </article>
  );
}

// ─── Main page ────────────────────────────────────────────────

export function PipelinePage() {
  const [pipelines, setPipelines] = useState<Record<string, VideoPipeline>>(() =>
    Object.fromEntries(TOPICS.map(t => [t.id, createInitialPipeline(t)])),
  );
  const running = useRef<Set<string>>(new Set());
  const prevTopicIds = useRef<Set<string>>(new Set());

  const updatePipeline = useCallback((updated: VideoPipeline) => {
    setPipelines(prev => ({ ...prev, [updated.topicId]: updated }));
  }, []);

  const startOne = useCallback(
    async (topicId: string) => {
      if (running.current.has(topicId)) return;
      running.current.add(topicId);

      const topic = TOPICS.find(t => t.id === topicId);
      if (!topic) { running.current.delete(topicId); return; }

      const fresh = createInitialPipeline(topic);
      updatePipeline(fresh);

      try {
        await runPipeline(fresh, updatePipeline);
        prevTopicIds.current.add(topicId);
      } finally {
        running.current.delete(topicId);
      }
    },
    [updatePipeline],
  );

  const startAll = useCallback(async () => {
    resetQueue();
    for (let i = 0; i < TOPICS.length; i++) {
      if (i > 0) await new Promise<void>(r => setTimeout(r, 180));
      startOne(TOPICS[i].id);
    }
  }, [startOne]);

  const resetAll = useCallback(() => {
    running.current.clear();
    resetQueue();
    setPipelines(
      Object.fromEntries(TOPICS.map(t => [t.id, createInitialPipeline(t)])),
    );
  }, []);

  const pipelineList = TOPICS.map(t => pipelines[t.id]).filter(Boolean);
  const anyRunning = running.current.size > 0;
  const countByStatus = pipelineList.reduce<Record<string, number>>(
    (acc, p) => {
      const bucket =
        p.status === 'idle' ? 'idle' :
        p.status === 'done' ? 'done' :
        'active';
      acc[bucket] = (acc[bucket] ?? 0) + 1;
      return acc;
    },
    {},
  );

  return (
    <section className="page-section">
      {/* Top bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          marginBottom: 18,
          flexWrap: 'wrap',
        }}
      >
        {/* Status counts */}
        <div style={{ display: 'flex', gap: 8, flex: 1, flexWrap: 'wrap' }}>
          {[
            { key: 'idle',   label: 'Idle',   color: 'var(--text-dim)',  bg: 'var(--bg-subtle)' },
            { key: 'active', label: 'Running', color: 'var(--accent)',   bg: 'var(--accent-dim)' },
            { key: 'done',   label: 'Done',    color: 'var(--green)',    bg: 'var(--green-dim)' },
          ].map(({ key, label, color, bg }) => (
            <div
              key={key}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 7,
                padding: '5px 12px',
                borderRadius: 'var(--radius-md)',
                background: bg,
                border: '1px solid var(--border-card)',
              }}
            >
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 13,
                  fontWeight: 700,
                  color,
                }}
              >
                {countByStatus[key] ?? 0}
              </span>
              <span style={{ fontSize: 11.5, color: 'var(--text-muted)' }}>{label}</span>
            </div>
          ))}
        </div>

        {/* Actions */}
        <div style={{ display: 'flex', gap: 8, flexShrink: 0 }}>
          <button
            onClick={resetAll}
            style={{
              padding: '6px 14px',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-strong)',
              background: 'transparent',
              color: 'var(--text-muted)',
              fontSize: 12,
              fontFamily: 'var(--font-mono)',
              cursor: 'pointer',
            }}
          >
            Reset
          </button>
          <button
            onClick={startAll}
            disabled={anyRunning}
            style={{
              padding: '6px 18px',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--accent-border)',
              background: anyRunning ? 'transparent' : 'var(--accent-dim)',
              color: anyRunning ? 'var(--text-dimmer)' : 'var(--accent)',
              fontSize: 12,
              fontWeight: 600,
              fontFamily: 'var(--font-mono)',
              cursor: anyRunning ? 'not-allowed' : 'pointer',
              transition: 'background 0.15s',
            }}
          >
            {anyRunning ? 'Generating…' : '▶ Generate All 6 Reels'}
          </button>
        </div>
      </div>

      {/* Batch info bar */}
      <div
        style={{
          padding: '8px 14px',
          borderRadius: 'var(--radius-md)',
          background: 'var(--bg-raised)',
          border: '1px solid var(--border-inner)',
          marginBottom: 16,
          display: 'flex',
          gap: 20,
          flexWrap: 'wrap',
          alignItems: 'center',
        }}
      >
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--text-dim)' }}>
          BATCH · 6 reels/day
        </span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--text-muted)' }}>
          Targets: YouTube Shorts · TikTok · IG Reels · FB Reels · X/Twitter · Threads
        </span>
        <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10.5, color: 'var(--text-muted)' }}>
          Format: 1080×1920 · 30fps · MP4 · max 60s
        </span>
        <span
          style={{
            marginLeft: 'auto',
            fontFamily: 'var(--font-mono)',
            fontSize: 10,
            padding: '2px 8px',
            borderRadius: 10,
            background: 'rgba(167,139,250,.08)',
            color: 'var(--accent)',
          }}
        >
          MOCK / DEMO — no real uploads
        </span>
      </div>

      {/* Pipeline cards */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
        {pipelineList.map(pipeline => (
          <PipelineCard
            key={pipeline.topicId}
            pipeline={pipeline}
            onRun={() => startOne(pipeline.topicId)}
            isRunning={running.current.has(pipeline.topicId)}
            isDuplicate={checkDuplicateTopic(pipeline.topicId, pipelines)}
          />
        ))}
      </div>

      <div style={{ marginTop: 16 }}>
        <span className="demo-badge">
          <span
            style={{
              width: 5,
              height: 5,
              borderRadius: '50%',
              background: 'var(--accent)',
              display: 'inline-block',
            }}
          />
          All pipeline stages are mock/demo — no real API calls, no uploads, no paid keys
        </span>
      </div>
    </section>
  );
}

import { useState } from 'react';
import type { DailyReel, WorkflowStatus } from '../types';
import { DAILY_WORKFLOW } from '../data/dailyWorkflow';
import { PlatformDot } from '../components/PlatformDot';
import { riskMeta, trendColor } from '../lib/scoring';
import {
  workflowStatusMeta,
  pipelineStatusMeta,
} from '../lib/dailyWorkflowEngine';

// ─── Sub-components ───────────────────────────────────────────

function WorkflowStatusPill({ status }: { status: WorkflowStatus }) {
  const m = workflowStatusMeta(status);
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 5,
        padding: '3px 9px',
        borderRadius: 20,
        fontSize: 10,
        fontFamily: 'var(--font-mono)',
        fontWeight: 600,
        letterSpacing: '0.04em',
        textTransform: 'uppercase',
        color: m.color,
        background: m.bg,
        border: `1px solid ${m.border}`,
        flexShrink: 0,
      }}
    >
      {m.label}
    </span>
  );
}

function PipelinePill({ status }: { status: DailyReel['pipelineStatus'] }) {
  const m = pipelineStatusMeta(status);
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        padding: '3px 8px',
        borderRadius: 20,
        fontSize: 10,
        fontFamily: 'var(--font-mono)',
        color: m.color,
        background: 'var(--bg-subtle)',
        border: '1px solid var(--border-card)',
        flexShrink: 0,
      }}
    >
      {m.dot && (
        <span
          style={{
            width: 5,
            height: 5,
            borderRadius: '50%',
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

interface ScoreChipProps {
  label: string;
  value: number;
  color?: string;
  subtext?: string;
}
function ScoreChip({ label, value, color, subtext }: ScoreChipProps) {
  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        padding: '7px 11px',
        borderRadius: 'var(--radius-md)',
        background: 'var(--bg-raised)',
        border: '1px solid var(--border-inner)',
        minWidth: 70,
      }}
    >
      <span style={{ fontSize: 9, fontFamily: 'var(--font-mono)', color: 'var(--text-dimmer)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
        {label}
      </span>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 4 }}>
        <span style={{ fontSize: 20, fontWeight: 700, color: color ?? 'var(--text-primary)', fontFamily: 'var(--font-mono)', lineHeight: 1 }}>
          {value}
        </span>
        {subtext && (
          <span style={{ fontSize: 9, color: color ?? 'var(--text-dim)', fontFamily: 'var(--font-mono)' }}>{subtext}</span>
        )}
      </div>
    </div>
  );
}

interface ReelCardProps {
  reel: DailyReel;
  expanded: boolean;
  onToggleExpand: () => void;
  onHandoff: () => void;
  onStatusChange: (s: WorkflowStatus) => void;
}

function ReelCard({ reel, expanded, onToggleExpand, onHandoff, onStatusChange }: ReelCardProps) {
  const tc = trendColor(reel.trendScore);
  const rm = riskMeta(reel.riskScore);
  const rankLabel = String(reel.rank).padStart(2, '0');
  const canHandoff = reel.pipelineStatus === 'not-started';

  return (
    <div
      style={{
        background: 'var(--bg-card)',
        border: '1px solid var(--border-card)',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
      }}
    >
      {/* Card header */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 10,
          padding: '14px 18px 0',
          flexWrap: 'wrap',
        }}
      >
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 10,
            color: 'var(--accent)',
            fontWeight: 600,
            flexShrink: 0,
          }}
        >
          {rankLabel}
        </span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            style={{
              fontSize: 13,
              fontWeight: 600,
              color: 'var(--text-primary)',
              lineHeight: 1.4,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            title={reel.title}
          >
            {reel.title}
          </div>
        </div>
        <WorkflowStatusPill status={reel.workflowStatus} />
        <PipelinePill status={reel.pipelineStatus} />
      </div>

      {/* Scores row */}
      <div style={{ display: 'flex', gap: 8, padding: '12px 18px 0', flexWrap: 'wrap' }}>
        <ScoreChip label="Trend"       value={reel.trendScore}        color={tc} />
        <ScoreChip label="Risk"        value={reel.riskScore}         color={rm.color} subtext={rm.label} />
        <ScoreChip label="Watch-Time"  value={reel.watchTimePotential} />
        <ScoreChip label="Platform Fit" value={reel.platformFit} />
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            padding: '7px 14px',
            borderRadius: 'var(--radius-md)',
            background: 'var(--accent-dim)',
            border: '1px solid var(--accent-border)',
            minWidth: 80,
          }}
        >
          <span style={{ fontSize: 9, fontFamily: 'var(--font-mono)', color: 'var(--accent)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>
            Composite
          </span>
          <span style={{ fontSize: 22, fontWeight: 700, color: 'var(--accent)', fontFamily: 'var(--font-mono)', lineHeight: 1.2 }}>
            {reel.compositeScore}
          </span>
        </div>
      </div>

      {/* Platforms row */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 6, padding: '10px 18px 0' }}>
        <span style={{ fontSize: 9, fontFamily: 'var(--font-mono)', color: 'var(--text-dimmer)', textTransform: 'uppercase', letterSpacing: '0.06em', marginRight: 2 }}>
          Targets
        </span>
        {reel.platforms.map(p => (
          <PlatformDot key={p} platform={p} size={18} />
        ))}
      </div>

      {/* Expanded detail */}
      {expanded && (
        <div style={{ padding: '14px 18px 0', display: 'flex', flexDirection: 'column', gap: 14 }}>
          {/* Selection reasons */}
          <div>
            <div
              style={{
                fontSize: 9,
                fontFamily: 'var(--font-mono)',
                color: 'var(--text-dimmer)',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                marginBottom: 8,
              }}
            >
              Why Selected
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 5 }}>
              {reel.selectionReasons.map((r, i) => (
                <div
                  key={i}
                  style={{
                    display: 'flex',
                    gap: 10,
                    padding: '7px 10px',
                    borderRadius: 'var(--radius-sm)',
                    background: 'var(--bg-raised)',
                    border: '1px solid var(--border-inner)',
                  }}
                >
                  <span
                    style={{
                      fontSize: 10,
                      fontFamily: 'var(--font-mono)',
                      color: 'var(--accent)',
                      fontWeight: 600,
                      flexShrink: 0,
                      minWidth: 90,
                    }}
                  >
                    {r.factor}
                  </span>
                  <span style={{ fontSize: 11, color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    {r.detail}
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Mock schedule */}
          <div>
            <div
              style={{
                fontSize: 9,
                fontFamily: 'var(--font-mono)',
                color: 'var(--text-dimmer)',
                textTransform: 'uppercase',
                letterSpacing: '0.06em',
                marginBottom: 8,
              }}
            >
              Mock Schedule · No real publishing
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
              {reel.scheduleSlots.map((slot, i) => (
                <div
                  key={i}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 10,
                    padding: '6px 10px',
                    borderRadius: 'var(--radius-sm)',
                    background: 'var(--bg-raised)',
                    border: '1px solid var(--border-inner)',
                  }}
                >
                  <span style={{ fontSize: 11, color: 'var(--text-secondary)', minWidth: 140 }}>
                    {slot.platformLabel}
                  </span>
                  <span style={{ fontSize: 11, fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                    Today · {slot.time}
                  </span>
                  <span
                    style={{
                      marginLeft: 'auto',
                      fontSize: 9,
                      fontFamily: 'var(--font-mono)',
                      color: '#9aa0a8',
                      background: 'rgba(154,160,168,.08)',
                      border: '1px solid rgba(154,160,168,.12)',
                      borderRadius: 4,
                      padding: '2px 6px',
                      flexShrink: 0,
                    }}
                  >
                    Mock · not live
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Action bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '12px 18px 14px',
          marginTop: 12,
          borderTop: '1px solid var(--border-inner)',
          flexWrap: 'wrap',
        }}
      >
        <button
          onClick={onToggleExpand}
          style={{
            padding: '5px 12px',
            fontSize: 11,
            fontFamily: 'var(--font-mono)',
            color: 'var(--text-muted)',
            background: 'var(--bg-raised)',
            border: '1px solid var(--border-strong)',
            borderRadius: 'var(--radius-sm)',
            cursor: 'pointer',
          }}
        >
          {expanded ? 'Collapse' : 'View Details'}
        </button>

        <button
          onClick={onHandoff}
          disabled={!canHandoff}
          title={canHandoff ? 'Send to mock pipeline' : 'Already in pipeline'}
          style={{
            padding: '5px 12px',
            fontSize: 11,
            fontFamily: 'var(--font-mono)',
            color: canHandoff ? 'var(--accent)' : 'var(--text-dimmer)',
            background: canHandoff ? 'var(--accent-dim)' : 'var(--bg-raised)',
            border: `1px solid ${canHandoff ? 'var(--accent-border)' : 'var(--border-strong)'}`,
            borderRadius: 'var(--radius-sm)',
            cursor: canHandoff ? 'pointer' : 'default',
            opacity: canHandoff ? 1 : 0.5,
          }}
        >
          {canHandoff ? 'Send to Pipeline' : 'In Pipeline'}
        </button>

        <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: 6 }}>
          <span style={{ fontSize: 10, fontFamily: 'var(--font-mono)', color: 'var(--text-dimmer)' }}>
            Status
          </span>
          <select
            value={reel.workflowStatus}
            onChange={e => onStatusChange(e.target.value as WorkflowStatus)}
            style={{
              padding: '4px 8px',
              fontSize: 11,
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-secondary)',
              background: 'var(--bg-input)',
              border: '1px solid var(--border-strong)',
              borderRadius: 'var(--radius-sm)',
              cursor: 'pointer',
            }}
          >
            <option value="draft">Draft</option>
            <option value="needs-review">Needs Review</option>
            <option value="approved">Approved</option>
            <option value="rejected">Rejected</option>
            <option value="scheduled">Scheduled</option>
          </select>
        </div>
      </div>
    </div>
  );
}

// ─── Main page ────────────────────────────────────────────────

export function DailyWorkflowPage() {
  const [activeTab, setActiveTab] = useState<'selected' | 'rejected'>('selected');
  const [reels, setReels] = useState<DailyReel[]>(DAILY_WORKFLOW.selectedReels);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [toast, setToast] = useState<string | null>(null);

  const { rejectedReels } = DAILY_WORKFLOW;

  function showToast(msg: string) {
    setToast(msg);
    setTimeout(() => setToast(null), 2400);
  }

  function handleHandoff(topicId: string) {
    setReels(prev =>
      prev.map(r =>
        r.topicId === topicId
          ? { ...r, pipelineStatus: 'queued' as const }
          : r,
      ),
    );
    showToast('Queued in pipeline (mock)');
  }

  function handleStatusChange(topicId: string, status: WorkflowStatus) {
    setReels(prev =>
      prev.map(r => (r.topicId === topicId ? { ...r, workflowStatus: status } : r)),
    );
  }

  const counts = {
    approved:    reels.filter(r => r.workflowStatus === 'approved').length,
    scheduled:   reels.filter(r => r.workflowStatus === 'scheduled').length,
    needsReview: reels.filter(r => r.workflowStatus === 'needs-review').length,
    draft:       reels.filter(r => r.workflowStatus === 'draft').length,
    rejected:    reels.filter(r => r.workflowStatus === 'rejected').length,
    inPipeline:  reels.filter(r => r.pipelineStatus === 'in-progress' || r.pipelineStatus === 'queued').length,
  };

  return (
    <section className="page-section">
      {/* Toast */}
      {toast && (
        <div
          style={{
            position: 'fixed',
            bottom: 24,
            right: 24,
            padding: '10px 18px',
            borderRadius: 'var(--radius-md)',
            background: 'var(--bg-raised)',
            border: '1px solid var(--accent-border)',
            color: 'var(--accent)',
            fontSize: 11,
            fontFamily: 'var(--font-mono)',
            zIndex: 1000,
            boxShadow: '0 4px 20px rgba(0,0,0,.4)',
          }}
        >
          {toast}
        </div>
      )}

      {/* Summary strip */}
      <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 20 }}>
        {[
          { label: '6 selected',                  color: 'var(--text-secondary)', bg: 'var(--bg-raised)',   border: 'var(--border-card)' },
          { label: `${counts.approved} approved`,  color: '#5fd39a',               bg: 'rgba(95,211,154,.10)',  border: 'rgba(95,211,154,.2)' },
          { label: `${counts.scheduled} scheduled`,color: '#6aa3f0',               bg: 'rgba(106,163,240,.10)', border: 'rgba(106,163,240,.2)' },
          { label: `${counts.needsReview} needs review`, color: '#e3b54e',         bg: 'rgba(227,181,78,.10)',  border: 'rgba(227,181,78,.2)' },
          { label: `${counts.draft} draft`,        color: '#9b9fa8',               bg: 'rgba(155,159,168,.08)', border: 'rgba(155,159,168,.12)' },
          { label: `${counts.inPipeline} in pipeline`, color: 'var(--accent)',     bg: 'var(--accent-dim)', border: 'var(--accent-border)' },
        ].map(chip => (
          <span
            key={chip.label}
            style={{
              padding: '5px 11px',
              borderRadius: 20,
              fontSize: 10,
              fontFamily: 'var(--font-mono)',
              color: chip.color,
              background: chip.bg,
              border: `1px solid ${chip.border}`,
              fontWeight: 600,
              letterSpacing: '0.02em',
            }}
          >
            {chip.label}
          </span>
        ))}
        <span
          style={{
            marginLeft: 'auto',
            padding: '5px 10px',
            fontSize: 9,
            fontFamily: 'var(--font-mono)',
            color: 'var(--text-dimmer)',
            background: 'var(--bg-subtle)',
            border: '1px solid var(--border-inner)',
            borderRadius: 20,
            alignSelf: 'center',
          }}
        >
          Generated · 09:00 AM · Mock only
        </span>
      </div>

      {/* Tabs */}
      <div
        style={{
          display: 'flex',
          gap: 0,
          marginBottom: 16,
          background: 'var(--bg-raised)',
          border: '1px solid var(--border-card)',
          borderRadius: 'var(--radius-md)',
          padding: 3,
          width: 'fit-content',
        }}
      >
        {(['selected', 'rejected'] as const).map(tab => {
          const active = activeTab === tab;
          const count = tab === 'selected' ? reels.length : rejectedReels.length;
          return (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              style={{
                padding: '6px 16px',
                fontSize: 11,
                fontFamily: 'var(--font-mono)',
                fontWeight: active ? 600 : 400,
                color: active ? 'var(--text-primary)' : 'var(--text-dim)',
                background: active ? 'var(--bg-card)' : 'transparent',
                border: active ? '1px solid var(--border-card)' : '1px solid transparent',
                borderRadius: 'var(--radius-sm)',
                cursor: 'pointer',
                textTransform: 'capitalize',
                display: 'flex',
                alignItems: 'center',
                gap: 6,
              }}
            >
              {tab === 'selected' ? "Today's 6" : 'Rejected'}
              <span
                style={{
                  fontSize: 9,
                  fontFamily: 'var(--font-mono)',
                  padding: '1px 5px',
                  borderRadius: 4,
                  background: active ? 'var(--accent-dim)' : 'var(--bg-subtle)',
                  color: active ? 'var(--accent)' : 'var(--text-dimmer)',
                  fontWeight: 600,
                }}
              >
                {count}
              </span>
            </button>
          );
        })}
      </div>

      {/* Selected reels list */}
      {activeTab === 'selected' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          {reels.map(reel => (
            <ReelCard
              key={reel.topicId}
              reel={reel}
              expanded={expandedId === reel.topicId}
              onToggleExpand={() => setExpandedId(prev => prev === reel.topicId ? null : reel.topicId)}
              onHandoff={() => handleHandoff(reel.topicId)}
              onStatusChange={status => handleStatusChange(reel.topicId, status)}
            />
          ))}
        </div>
      )}

      {/* Rejected reels */}
      {activeTab === 'rejected' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div
            style={{
              padding: '10px 14px',
              borderRadius: 'var(--radius-sm)',
              background: 'rgba(227,181,78,.06)',
              border: '1px solid rgba(227,181,78,.12)',
              fontSize: 11,
              color: '#e3b54e',
              fontFamily: 'var(--font-mono)',
              marginBottom: 4,
            }}
          >
            {rejectedReels.length} topic candidates were scored but did not make the daily cut.
          </div>
          {rejectedReels.map(reel => {
            const tc = trendColor(reel.trendScore);
            const rm = riskMeta(reel.riskScore);
            return (
              <div
                key={reel.topicId}
                style={{
                  background: 'var(--bg-card)',
                  border: '1px solid var(--border-card)',
                  borderRadius: 'var(--radius-lg)',
                  padding: '14px 18px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10, marginBottom: 10 }}>
                  <div style={{ flex: 1 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-secondary)', marginBottom: 4 }}>
                      {reel.title}
                    </div>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: tc }}>Trend {reel.trendScore}</span>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: rm.color }}>Risk {reel.riskScore} ({rm.label})</span>
                      <span style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)' }}>Composite {reel.compositeScore}</span>
                    </div>
                  </div>
                  <span
                    style={{
                      padding: '3px 9px',
                      borderRadius: 20,
                      fontSize: 9,
                      fontFamily: 'var(--font-mono)',
                      fontWeight: 600,
                      color: '#e8736b',
                      background: 'rgba(232,115,107,.10)',
                      border: '1px solid rgba(232,115,107,.2)',
                      flexShrink: 0,
                    }}
                  >
                    Not selected
                  </span>
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  {reel.rejectionReasons.map((reason, i) => (
                    <div
                      key={i}
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        gap: 8,
                        padding: '5px 8px',
                        borderRadius: 'var(--radius-sm)',
                        background: 'var(--bg-raised)',
                        border: '1px solid var(--border-inner)',
                      }}
                    >
                      <span style={{ color: '#e8736b', flexShrink: 0, fontSize: 10, marginTop: 1 }}>✕</span>
                      <span style={{ fontSize: 11, color: 'var(--text-muted)', lineHeight: 1.5 }}>{reason}</span>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Footer badge */}
      <div style={{ marginTop: 20, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data · mock only · no real publishing
        </span>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#5fd39a', display: 'inline-block' }} />
          Pipeline handoff is simulated · no video rendered
        </span>
      </div>
    </section>
  );
}

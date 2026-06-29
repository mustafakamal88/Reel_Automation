import { useEffect } from 'react';
import type { Topic } from '../types';
import { PLATFORMS } from '../data/platforms';
import { riskMeta, trendColor } from '../lib/scoring';
import { PlatformDot, SrcBadge } from './PlatformDot';

interface Props {
  topic: Topic;
  onClose: () => void;
  onApprove: (id: string) => void;
}

export function TopicDrawer({ topic, onClose, onApprove }: Props) {
  const tc = trendColor(topic.trendScore);
  const rm = riskMeta(topic.riskScore);

  // Close on Escape
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  // Prevent scroll on body
  useEffect(() => {
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = ''; };
  }, []);

  const rankLabel = String(topic.rank).padStart(2, '0');

  return (
    <>
      <div
        className="drawer-backdrop"
        onClick={onClose}
        role="presentation"
        aria-label="Close topic drawer"
      />
      <div
        className="drawer-panel"
        role="dialog"
        aria-modal="true"
        aria-label={topic.title}
      >
        <div className="drawer-inner">
          {/* Header */}
          <div className="drawer-header">
            <span className="drawer-tag">TOPIC {rankLabel} · Jun 29</span>
            <button
              className="drawer-close"
              onClick={onClose}
              aria-label="Close drawer"
            >
              ✕
            </button>
          </div>

          <h2 className="drawer-title">{topic.title}</h2>

          {/* Score chips */}
          <div className="drawer-scores">
            <div className="drawer-score-card">
              <div className="drawer-score-label">Trend score</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginTop: 5 }}>
                <span className="drawer-score-value" style={{ color: tc }}>{topic.trendScore}</span>
                <span className="drawer-score-sub">/100</span>
              </div>
            </div>
            <div className="drawer-score-card">
              <div className="drawer-score-label">Risk score</div>
              <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, marginTop: 5 }}>
                <span className="drawer-score-value" style={{ color: rm.color }}>{topic.riskScore}</span>
                <span className="drawer-score-sub">/100 · {rm.label}</span>
              </div>
            </div>
            <div className="drawer-score-card">
              <div className="drawer-score-label">Best fit</div>
              <div style={{ display: 'flex', gap: 4, marginTop: 8 }}>
                {topic.platforms.map(p => (
                  <PlatformDot key={p} platform={p} size={22} />
                ))}
              </div>
            </div>
          </div>

          {/* Hook */}
          <div className="hook-box">
            <div className="hook-label">⚡ Hook · first 2 seconds</div>
            <div className="hook-text">"{topic.hook}"</div>
          </div>

          {/* Script beats */}
          <div className="drawer-section-label">Script · {topic.scriptLength}</div>
          <div className="script-beats">
            {topic.scriptBeats.map((beat, i) => (
              <div key={i} className="script-beat">
                <span className="beat-time">{beat.timeRange}</span>
                <span className="beat-text">{beat.text}</span>
              </div>
            ))}
          </div>

          {/* Caption */}
          <div className="drawer-section-label">Caption</div>
          <div className="caption-box">{topic.caption}</div>

          {/* Hashtags */}
          <div className="drawer-section-label">Hashtags &amp; keywords</div>
          <div className="hashtag-list">
            {topic.hashtags.map(tag => (
              <span key={tag} className="hashtag">{tag}</span>
            ))}
          </div>

          {/* Visual direction */}
          <div className="drawer-section-label">Suggested visual</div>
          <div className="visual-box">
            <div className="visual-thumb" aria-hidden="true" />
            <span>{topic.visualDirection}</span>
          </div>

          {/* Sources */}
          <div className="drawer-section-label">Why selected · data signals</div>
          <div className="sources-list">
            {topic.sources.map((src, i) => {
              const meta = PLATFORMS[src.platform];
              return (
                <div key={i} className="source-row">
                  <SrcBadge platform={src.platform} />
                  <span className="source-label">{src.label}</span>
                  <span className="source-metric" style={{ color: meta.color }}>{src.metric}</span>
                </div>
              );
            })}
          </div>

          {/* Actions */}
          <div className="drawer-actions">
            <button
              className="btn-approve-full"
              onClick={() => onApprove(topic.id)}
              aria-label="Approve and schedule this topic"
            >
              Approve &amp; schedule
            </button>
            <button
              className="btn-close-drawer"
              onClick={onClose}
              aria-label="Close without approving"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </>
  );
}

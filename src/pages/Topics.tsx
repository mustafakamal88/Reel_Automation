import type { ApprovalStatus } from '../types';
import { TOPICS } from '../data/topics';
import { riskMeta, trendColor } from '../lib/scoring';
import { PlatformDot } from '../components/PlatformDot';
import { TopicDrawer } from '../components/TopicDrawer';
import { useState } from 'react';

interface Props {
  generated: boolean;
  onApprove: (id: string, status: ApprovalStatus) => void;
  onNavigateToApprovals: () => void;
  openTopicId?: string | null;
  onOpenTopic?: (id: string | null) => void;
}

export function TopicsPage({
  generated,
  onApprove,
  onNavigateToApprovals,
  openTopicId: externalOpenId,
  onOpenTopic,
}: Props) {
  const [internalOpenId, setInternalOpenId] = useState<string | null>(null);
  const openId = externalOpenId !== undefined ? externalOpenId : internalOpenId;

  const setOpenId = (id: string | null) => {
    setInternalOpenId(id);
    onOpenTopic?.(id);
  };

  const openTopic = openId ? TOPICS.find(t => t.id === openId) ?? null : null;

  if (!generated) {
    return (
      <section className="page-section">
        <div className="empty-state">
          <div className="empty-icon">✦</div>
          <div className="empty-title">No topics generated yet</div>
          <div className="empty-desc">
            Click "Generate today's 6" in the header to create AI-scored content ideas based on today's signals.
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="page-section">
      <div className="topics-grid">
        {TOPICS.map(topic => {
          const tc = trendColor(topic.trendScore);
          const rm = riskMeta(topic.riskScore);
          const rankLabel = String(topic.rank).padStart(2, '0');

          return (
            <article
              key={topic.id}
              className="topic-card"
              onClick={() => setOpenId(topic.id)}
              role="button"
              tabIndex={0}
              aria-label={`Topic ${rankLabel}: ${topic.title}`}
              onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') setOpenId(topic.id); }}
            >
              <div className="topic-card-top">
                <span className="topic-rank-badge">TOPIC {rankLabel}</span>
                <div className="topic-plats">
                  {topic.platforms.map(p => (
                    <PlatformDot key={p} platform={p} size={18} />
                  ))}
                </div>
              </div>

              <div className="topic-title">{topic.title}</div>

              <div className="topic-why">{topic.why}</div>

              <div className="topic-footer">
                <div className="topic-score-block">
                  <div className="topic-score-label">Trend</div>
                  <div className="topic-score-row">
                    <div
                      style={{
                        flex: 1,
                        height: 5,
                        background: 'var(--border-strong)',
                        borderRadius: 3,
                        overflow: 'hidden',
                      }}
                    >
                      <div
                        style={{
                          height: '100%',
                          width: `${topic.trendScore}%`,
                          background: tc,
                          borderRadius: 3,
                        }}
                      />
                    </div>
                    <span className="topic-score-value" style={{ color: tc }}>
                      {topic.trendScore}
                    </span>
                  </div>
                </div>
                <div style={{ textAlign: 'right' }}>
                  <div className="topic-score-label">Risk</div>
                  <span
                    className="risk-badge"
                    style={{ color: rm.color, background: rm.bg, padding: '2px 9px' }}
                  >
                    {rm.label}
                  </span>
                </div>
              </div>
            </article>
          );
        })}
      </div>

      <div style={{ marginTop: 16 }}>
        <span className="demo-badge">
          <span style={{ width: 5, height: 5, borderRadius: '50%', background: '#a78bfa', display: 'inline-block' }} />
          Demo data
        </span>
      </div>

      {openTopic && (
        <TopicDrawer
          topic={openTopic}
          onClose={() => setOpenId(null)}
          onApprove={id => {
            onApprove(id, 'approved');
            setOpenId(null);
            onNavigateToApprovals();
          }}
        />
      )}
    </section>
  );
}

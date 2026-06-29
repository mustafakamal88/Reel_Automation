import type { VideoTopic, ScriptDraft, QualityCheck, ScriptBeat } from '../../types';
import { TOPICS } from '../../data/topics';

// MOCK ONLY — no real AI calls. All data derived from existing topic fixtures.

function buildQualityChecks(
  hook: string,
  cta: string,
  durationSec: number,
): QualityCheck[] {
  const hookWords = hook.trim().split(/\s+/).length;
  const hookOk = hookWords >= 5 && hookWords <= 25;

  return [
    {
      name: 'Hook strength',
      passed: hookOk,
      message: hookOk
        ? `Hook is ${hookWords} words — ideal range`
        : hookWords < 5
          ? 'Hook too short — needs more punch'
          : 'Hook too long — trim for first-second retention',
      severity: 'warning',
    },
    {
      name: 'CTA present',
      passed: cta.length > 0,
      message:
        cta.length > 0
          ? 'CTA detected in final beat'
          : 'No CTA found — add follow / comment / save prompt',
      severity: 'error',
    },
    {
      name: 'Platform length limit',
      passed: durationSec <= 60,
      message:
        durationSec > 60
          ? `${durationSec}s exceeds 60s limit for YT Shorts / TikTok`
          : `${durationSec}s — within all six platform limits`,
      severity: 'warning',
    },
    {
      name: 'Watch-time score',
      passed: true,
      message: 'Predicted retention: pending model evaluation (placeholder)',
      severity: 'info',
    },
  ];
}

export async function generateScript(topic: VideoTopic): Promise<ScriptDraft> {
  // Simulate async generation delay
  await new Promise<void>(r => setTimeout(r, 650));

  const source = TOPICS.find(t => t.id === topic.id);
  const beats: ScriptBeat[] = source?.scriptBeats ?? [
    { timeRange: '0–2s', text: topic.hook },
    { timeRange: '2–20s', text: 'Main content body goes here.' },
    { timeRange: '20–24s', text: 'Follow for more.' },
  ];

  const lastBeat = beats[beats.length - 1];
  const cta = lastBeat?.text ?? '';
  const durationSec = parseInt(source?.scriptLength ?? '30', 10) || 30;
  const wordCount = beats.reduce((acc, b) => acc + b.text.split(/\s+/).length, 0);

  return {
    topicId: topic.id,
    title: topic.title,
    hook: topic.hook,
    body: beats
      .slice(1, -1)
      .map(b => b.text)
      .join(' '),
    cta,
    estimatedDuration: durationSec,
    wordCount,
    beats,
    qualityChecks: buildQualityChecks(topic.hook, cta, durationSec),
  };
}

import type { AssetPlan, RenderJob } from '../../types';

// MOCK ONLY — simulates a local render queue with no real video encoding.

let _queueCounter = 0;

export function resetQueue(): void {
  _queueCounter = 0;
}

export function getQueueLength(): number {
  return _queueCounter;
}

export async function enqueueRender(assetPlan: AssetPlan): Promise<RenderJob> {
  await new Promise<void>(r => setTimeout(r, 320));

  _queueCounter += 1;
  const position = _queueCounter;

  return {
    id: `rj-${assetPlan.topicId}-${Date.now()}`,
    topicId: assetPlan.topicId,
    status: 'queued',
    progress: 0,
    estimatedSeconds: 30 + position * 6,
    format: 'mp4',
    resolution: '1080x1920',
    fps: 30,
    queuePosition: position,
    enqueuedAt: new Date().toISOString(),
  };
}

export async function simulateRender(job: RenderJob): Promise<RenderJob> {
  // Simulate render time proportional to queue position (capped at 900ms for demo)
  const delay = Math.min(400 + job.queuePosition * 80, 900);
  await new Promise<void>(r => setTimeout(r, delay));

  return {
    ...job,
    status: 'done',
    progress: 100,
    completedAt: new Date().toISOString(),
  };
}

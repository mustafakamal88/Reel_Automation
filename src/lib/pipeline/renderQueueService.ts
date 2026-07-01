import type { AssetPlan, RenderJob } from '../../types';

let _queueCounter = 0;

export function resetQueue(): void {
  _queueCounter = 0;
}

export function getQueueLength(): number {
  return _queueCounter;
}

export async function enqueueRender(assetPlan: AssetPlan): Promise<RenderJob> {
  _queueCounter += 1;
  const position = _queueCounter;

  return {
    id: `rj-${assetPlan.topicId}-${Date.now()}`,
    topicId: assetPlan.topicId,
    status: 'failed',
    progress: 0,
    estimatedSeconds: 0,
    format: 'mp4',
    resolution: '1080x1920',
    fps: 30,
    queuePosition: position,
    enqueuedAt: new Date().toISOString(),
  };
}

export async function simulateRender(job: RenderJob): Promise<RenderJob> {
  return {
    ...job,
    status: 'failed',
    progress: 0,
  };
}

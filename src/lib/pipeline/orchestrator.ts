import type { Topic, VideoPipeline, VideoTopic } from '../../types';
import { generateScript } from './scriptGeneratorService';
import { generateStoryboard } from './storyboardService';
import { planAssets } from './assetPlannerService';
import { enqueueRender, simulateRender } from './renderQueueService';
import { exportVideo, ALL_TARGETS } from './exportService';

// MOCK ONLY — orchestrates pipeline stages with no real API calls.

function topicToVideoTopic(t: Topic): VideoTopic {
  return {
    id: t.id,
    title: t.title,
    hook: t.hook,
    platforms: t.platforms,
    trendScore: t.trendScore,
    riskScore: t.riskScore,
  };
}

function ts(): string {
  return new Date().toISOString();
}

export function createInitialPipeline(topic: Topic): VideoPipeline {
  return {
    id: `pipe-${topic.id}`,
    topicId: topic.id,
    status: 'idle',
    topic: topicToVideoTopic(topic),
    qualityWarnings: [],
    createdAt: ts(),
    updatedAt: ts(),
  };
}

export function checkDuplicateTopic(
  topicId: string,
  pipelines: Record<string, VideoPipeline>,
): boolean {
  const p = pipelines[topicId];
  return !!p && p.status !== 'idle';
}

export async function runPipeline(
  initial: VideoPipeline,
  onUpdate: (updated: VideoPipeline) => void,
): Promise<VideoPipeline> {
  let state: VideoPipeline = { ...initial, updatedAt: ts() };

  const push = (patch: Partial<VideoPipeline>): VideoPipeline => {
    state = { ...state, ...patch, updatedAt: ts() };
    onUpdate(state);
    return state;
  };

  // Stage 1 — Script generation
  push({ status: 'generating-script' });
  const script = await generateScript(state.topic);
  push({ script });

  // Stage 2 — Storyboard
  push({ status: 'generating-storyboard' });
  const storyboard = await generateStoryboard(script);
  push({ storyboard });

  // Stage 3 — Asset planning
  push({ status: 'planning-assets' });
  const assetPlan = await planAssets(state.topic, storyboard);
  const warnings: string[] = [];
  if (assetPlan.copyrightWarning) warnings.push(assetPlan.copyrightWarning);

  // Duplicate topic check happens before enqueue (logged as warning only)
  push({ assetPlan, qualityWarnings: warnings });

  // Stage 4 — Render queue
  push({ status: 'queued-render' });
  const renderJob = await enqueueRender(assetPlan);
  push({ renderJob, status: 'rendering' });
  const completedJob = await simulateRender(renderJob);
  push({ renderJob: completedJob });

  // Stage 5 — Export package
  push({ status: 'exporting' });
  const durationSec = script.estimatedDuration;
  const exportPackage = await exportVideo(completedJob, ALL_TARGETS, durationSec);
  push({ exportPackage, status: 'done' });

  return state;
}

export async function createVideoPipeline(
  topicId: string,
  allTopics: Topic[],
  onUpdate: (updated: VideoPipeline) => void,
): Promise<VideoPipeline | null> {
  const topic = allTopics.find(t => t.id === topicId);
  if (!topic) return null;
  const pipeline = createInitialPipeline(topic);
  return runPipeline(pipeline, onUpdate);
}

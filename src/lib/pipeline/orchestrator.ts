import type { Topic, VideoPipeline } from '../../types';

function notConfigured(): never {
  throw new Error('Pipeline automation is not configured. Use the backend Phase 4D render + ZIP test or connect real providers.');
}

export function createInitialPipeline(_topic: Topic): VideoPipeline {
  return notConfigured();
}

export function checkDuplicateTopic(): boolean {
  return false;
}

export async function runPipeline(): Promise<VideoPipeline> {
  return notConfigured();
}

export async function createVideoPipeline(): Promise<VideoPipeline | null> {
  return notConfigured();
}

import type { VideoTopic, ScriptDraft } from '../../types';

export async function generateScript(topic: VideoTopic): Promise<ScriptDraft> {
  return {
    topicId: topic.id,
    title: topic.title,
    hook: '',
    body: '',
    cta: '',
    estimatedDuration: 0,
    wordCount: 0,
    beats: [],
    qualityChecks: [{
      name: 'Provider not connected',
      passed: false,
      message: 'Connect a real script provider before generating copy.',
      severity: 'error',
    }],
  };
}

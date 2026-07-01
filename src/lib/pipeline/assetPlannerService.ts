import type { VideoTopic, StoryboardScene, AssetPlan } from '../../types';

export async function planAssets(
  topic: VideoTopic,
  scenes: StoryboardScene[],
): Promise<AssetPlan> {
  return {
    topicId: topic.id,
    musicTrack: 'Provider not connected',
    voiceStyle: 'Provider not connected',
    captionStyle: 'Provider not connected',
    colorGrade: 'Provider not connected',
    requiredBRoll: scenes
      .filter(s => s.type !== 'hook')
      .map(s => s.bRoll),
    copyrightSafe: false,
    copyrightWarning: 'Connect real media providers before planning assets.',
  };
}

import type { RenderJob, ExportPackage, ExportFile, ExportTarget } from '../../types';

// MOCK ONLY — builds export packages locally, no uploads to any platform.

interface PlatformSpec {
  label: string;
  resolution: string;
  maxDurationSec: number;
  avgBitrateKbps: number;
}

const PLATFORM_SPECS: Record<ExportTarget, PlatformSpec> = {
  'yt-shorts':  { label: 'YouTube Shorts', resolution: '1080x1920', maxDurationSec: 60,  avgBitrateKbps: 8000  },
  'tiktok':     { label: 'TikTok',          resolution: '1080x1920', maxDurationSec: 60,  avgBitrateKbps: 7500  },
  'ig-reels':   { label: 'Instagram Reels', resolution: '1080x1920', maxDurationSec: 90,  avgBitrateKbps: 8000  },
  'fb-reels':   { label: 'Facebook Reels',  resolution: '1080x1920', maxDurationSec: 60,  avgBitrateKbps: 7000  },
  'x-twitter':  { label: 'X / Twitter',     resolution: '1080x1920', maxDurationSec: 140, avgBitrateKbps: 6000  },
  'threads':    { label: 'Threads',          resolution: '1080x1920', maxDurationSec: 90,  avgBitrateKbps: 6500  },
};

export const ALL_TARGETS: ExportTarget[] = [
  'yt-shorts',
  'tiktok',
  'ig-reels',
  'fb-reels',
  'x-twitter',
  'threads',
];

export function getPlatformLabel(target: ExportTarget): string {
  return PLATFORM_SPECS[target].label;
}

export async function exportVideo(
  renderJob: RenderJob,
  targets: ExportTarget[],
  durationSec: number,
): Promise<ExportPackage> {
  await new Promise<void>(r => setTimeout(r, 480));

  const files: ExportFile[] = targets.map(target => {
    const spec = PLATFORM_SPECS[target];
    const fits = durationSec <= spec.maxDurationSec;
    const sizeKB = fits
      ? Math.round((durationSec * spec.avgBitrateKbps) / 8)
      : 0;

    return {
      target,
      filename: `${renderJob.topicId}_${target}.mp4`,
      resolution: spec.resolution,
      durationSec,
      sizeKB,
      status: fits ? 'ready' : 'failed',
    };
  });

  const allReady = files.every(f => f.status === 'ready');

  return {
    id: `exp-${renderJob.topicId}-${Date.now()}`,
    topicId: renderJob.topicId,
    renderJobId: renderJob.id,
    targets,
    status: allReady ? 'ready' : 'failed',
    files,
    exportedAt: new Date().toISOString(),
  };
}

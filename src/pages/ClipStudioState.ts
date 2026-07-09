import type { ClipStudioSourceResponse } from '../lib/api/client';
import type { ClipStudioGenerateResponse } from '../lib/api/client';
import type { AISceneGenerationStatusResponse } from '../lib/api/client';
import type { AISceneWorkerStatusResponse } from '../lib/api/client';

export type ClipSourceStatusState =
  | 'none'
  | 'uploaded_ready'
  | 'url_reference_only'
  | 'direct_video_imported'
  | 'unsupported_url'
  | 'rights_required';

export interface ClipSourceStatusView {
  state: ClipSourceStatusState;
  label: string;
  message: string;
  tone: 'neutral' | 'ready' | 'warning' | 'danger';
}

export interface ClipGenerateAvailabilityInput {
  source: ClipStudioSourceResponse | null;
  rightsConfirmed: boolean;
  prompt: string;
  busy: boolean;
  uploadBusy: boolean;
  urlImportBusy?: boolean;
}

const referenceOnlyMessage = 'URL saved as reference only. Upload the source video file or connect an approved source before generating clips.';
const watchURLMessage = 'This is a platform watch URL. Upload the source file or provide a direct downloadable video URL.';

export function isYouTubeURL(value: string): boolean {
  return isPlatformWatchURL(value);
}

export function isPlatformWatchURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    const host = parsed.hostname.toLowerCase().replace(/^www\./, '');
    if (host === 'youtube.com' || host.endsWith('.youtube.com') || host === 'youtu.be') return true;
    if (host === 'vimeo.com' || host.endsWith('.vimeo.com')) return true;
    if (parsed.pathname.toLowerCase().includes('/watch')) return true;
    return ['tiktok.com', 'instagram.com', 'facebook.com', 'fb.watch', 'x.com', 'twitter.com', 'threads.net']
      .some(socialHost => host === socialHost || host.endsWith(`.${socialHost}`));
  } catch {
    return false;
  }
}

export function sourceCanGenerate(source: ClipStudioSourceResponse | null): boolean {
  return Boolean(source?.can_render && source.download_ready);
}

export function clipPackageReady(result: ClipStudioGenerateResponse | null): boolean {
  return Boolean(result?.download_url && result.zip_filename);
}

export function clipGenerateDisabledReason(input: ClipGenerateAvailabilityInput): string | null {
  if (input.busy) return 'Generating clips...';
  if (input.uploadBusy) return 'Uploading source video...';
  if (input.urlImportBusy) return 'Checking source URL...';
  if (!sourceCanGenerate(input.source)) return 'Upload a source video or import a direct downloadable video URL before generating.';
  if (!input.rightsConfirmed) return 'Confirm source rights before generating clips.';
  if (!input.prompt.trim()) return 'Describe what clips you want before generating.';
  return null;
}

export function getClipSourceStatus(source: ClipStudioSourceResponse | null, sourceUrl: string): ClipSourceStatusView {
  const trimmedURL = sourceUrl.trim();
  if (!source && !trimmedURL) {
    return {
      state: 'none',
      label: 'No source selected',
      message: 'Choose a video file, or import a direct downloadable video URL.',
      tone: 'neutral',
    };
  }

  if (!source && isPlatformWatchURL(trimmedURL)) {
    return {
      state: 'unsupported_url',
      label: 'Unsupported URL',
      message: watchURLMessage,
      tone: 'danger',
    };
  }

  if (!source && trimmedURL) {
    return {
      state: 'unsupported_url',
      label: 'Unsupported URL',
      message: 'Import the URL first. Only direct downloadable video files can be used for generation.',
      tone: 'warning',
    };
  }

  if (source?.status === 'rights_required') {
    return {
      state: 'rights_required',
      label: 'Rights confirmation required',
      message: source.message || 'Confirm source rights before importing this direct video URL.',
      tone: 'warning',
    };
  }

  if (source?.can_render && source.metadata.kind === 'upload') {
    return {
      state: 'uploaded_ready',
      label: 'Source uploaded and ready',
      message: 'Confirm rights, then generate clips.',
      tone: 'ready',
    };
  }

  if (source?.can_render && source.metadata.kind === 'url') {
    return {
      state: 'direct_video_imported',
      label: 'Direct video imported',
      message: 'Video imported and ready',
      tone: 'ready',
    };
  }

  if (source?.status === 'metadata_only') {
    return {
      state: 'url_reference_only',
      label: 'URL reference only',
      message: isPlatformWatchURL(source.metadata.url || '') ? watchURLMessage : referenceOnlyMessage,
      tone: 'warning',
    };
  }

  return {
    state: 'unsupported_url',
    label: 'Unsupported URL',
    message: source?.message || referenceOnlyMessage,
    tone: 'danger',
  };
}

export function aiSceneProgressLabel(status: AISceneGenerationStatusResponse | null, busy: boolean): string {
  if (status?.status === 'completed') return 'Video ready';
  if (status?.status === 'generator_not_configured') {
    return 'AI video generator is connected but automatic model generation is not configured yet.';
  }
  if (status?.current_step) return status.current_step;
  return busy ? 'Creating scenes' : 'Preparing scene plan';
}

export function aiScenePrimaryButtonLabel(status: AISceneGenerationStatusResponse | null, busy: boolean): string {
  if (status?.status === 'generator_not_configured') return 'Configure generator';
  return busy ? 'Generating video...' : 'Generate Video';
}

export function aiSceneWorkerCanRun(status: AISceneWorkerStatusResponse | null): boolean {
  if (!status?.configured || status.status === 'error') return false;
  if (status.generator_mode === 'dev_stub') return true;
  return status.generator_mode === 'auto_command' && status.auto_command_configured === true;
}

export function aiSceneWorkerConfigurationMessage(status: AISceneWorkerStatusResponse | null): string {
  if (!status) return '';
  if (!status.configured) return 'Local AI worker not connected.';
  if (status.status === 'error') return status.message || 'Local AI worker status unavailable.';
  if (status.generator_mode === 'manual' || (status.generator_mode === 'auto_command' && !status.auto_command_configured)) {
    return 'Worker connected, but automatic generator command is not configured.';
  }
  if (status.generator_mode === 'dev_stub') return 'Worker connected in dev stub mode.';
  return '';
}

export function aiSceneDownloadsReady(status: AISceneGenerationStatusResponse | null): { video: boolean; zip: boolean } {
  return {
    video: Boolean(status?.downloadable && status.video_url && status.video_filename),
    zip: Boolean(status?.downloadable && status.zip_url && status.zip_filename),
  };
}

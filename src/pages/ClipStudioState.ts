import type { ClipStudioSourceResponse } from '../lib/api/client';
import type { ClipStudioGenerateResponse } from '../lib/api/client';

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
const youtubeMessage = 'For YouTube links, upload the source video file or connect your own/approved channel source. This app does not auto-rip YouTube videos.';

export function isYouTubeURL(value: string): boolean {
  try {
    const parsed = new URL(value);
    const host = parsed.hostname.toLowerCase().replace(/^www\./, '');
    return host === 'youtube.com' || host.endsWith('.youtube.com') || host === 'youtu.be';
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

  if (!source && isYouTubeURL(trimmedURL)) {
    return {
      state: 'unsupported_url',
      label: 'Unsupported URL',
      message: youtubeMessage,
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
      message: 'Confirm rights, then generate clips.',
      tone: 'ready',
    };
  }

  if (source?.status === 'metadata_only') {
    return {
      state: 'url_reference_only',
      label: 'URL reference only',
      message: isYouTubeURL(source.metadata.url || '') ? youtubeMessage : referenceOnlyMessage,
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

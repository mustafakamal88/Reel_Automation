import type { ClipStudioSourceMetadata, ClipStudioSourceResponse } from '../lib/api/client';
import { clipGenerateDisabledReason, clipPackageReady, getClipSourceStatus } from './ClipStudioState';

function assert(condition: boolean, message: string) {
  if (!condition) throw new Error(message);
}

function source(overrides: Partial<Omit<ClipStudioSourceResponse, 'metadata'>> & { metadata?: Partial<ClipStudioSourceMetadata> }): ClipStudioSourceResponse {
  const { metadata: metadataOverrides, ...sourceOverrides } = overrides;
  const metadata: ClipStudioSourceMetadata = {
    source_id: 'src-test',
    kind: 'upload',
    rights: { user_confirmed_rights: false },
    status: 'ready',
    created_at: '2026-01-01T00:00:00Z',
    direct_video: true,
    supported_type: true,
    ...metadataOverrides,
  };
  return {
    source_id: 'src-test',
    status: 'ready',
    can_render: true,
    direct_video: true,
    download_ready: true,
    ...sourceOverrides,
    metadata,
  };
}

const youtubeSource = source({
  status: 'metadata_only',
  can_render: false,
  direct_video: false,
  download_ready: false,
  metadata: {
    kind: 'url',
    url: 'https://youtu.be/abc123',
    status: 'metadata_only',
    direct_video: false,
    supported_type: false,
  },
});
assert(getClipSourceStatus(youtubeSource, '').state === 'url_reference_only', 'YouTube source should be reference only');
assert(
  clipGenerateDisabledReason({
    source: youtubeSource,
    rightsConfirmed: true,
    prompt: 'make clips',
    busy: false,
    uploadBusy: false,
  }) !== null,
  'Generate must be disabled for metadata-only source',
);

const uploadedSource = source({});
assert(
  clipGenerateDisabledReason({
    source: uploadedSource,
    rightsConfirmed: true,
    prompt: 'make clips',
    busy: false,
    uploadBusy: false,
  }) === null,
  'Uploaded source should enable Generate after rights confirmation',
);

const importedSource = source({
  metadata: {
    kind: 'url',
    url: 'https://cdn.example.com/source.mp4',
  },
});
assert(
  getClipSourceStatus(importedSource, '').message === 'Video imported and ready',
  'Direct URL import should show ready message',
);

assert(getClipSourceStatus(null, '').state === 'none', 'Empty state should be no source selected');
assert(!clipPackageReady(null), 'Download must be disabled until a package exists');
assert(
  !clipPackageReady({
    success: false,
    render_status: 'unsupported_source',
    highlight_detection: 'not_run',
    generated_clip_jobs: [],
    included_files: [],
  }),
  'Download must stay disabled when generation has no ZIP package',
);
assert(
  clipPackageReady({
    success: true,
    render_status: 'completed',
    highlight_detection: 'not_run',
    generated_clip_jobs: [],
    included_files: ['manifest.json'],
    zip_filename: 'clips.zip',
    download_url: '/api/clip-studio/download/clips.zip',
  }),
  'Download should enable when a ZIP package is ready',
);

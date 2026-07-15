import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { apiUrl, archiveAsset, listAssets, reconcileAsset, restoreAsset, type AssetListResponse, type MediaAsset } from '../lib/api/client';
import type { View } from '../types';
import { ConfirmationDialog } from '../components/ConfirmationDialog';

interface AssetLibraryPageProps {
  onNavigate: (view: View) => void;
}

const PAGE_SIZE = 24;

export function AssetLibraryPage({ onNavigate }: AssetLibraryPageProps) {
  const [searchDraft, setSearchDraft] = useState('');
  const [search, setSearch] = useState('');
  const [type, setType] = useState('');
  const [projectID, setProjectID] = useState('');
  const [status, setStatus] = useState('');
  const [workflow, setWorkflow] = useState('');
  const [sort, setSort] = useState('newest');
  const [includeArchived, setIncludeArchived] = useState(false);
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<AssetListResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<MediaAsset | null>(null);
  const [confirmAsset, setConfirmAsset] = useState<MediaAsset | null>(null);
  const [mutationError, setMutationError] = useState<string | null>(null);
  const [mutating, setMutating] = useState(false);

  useEffect(() => {
    const id = window.setTimeout(() => {
      setSearch(searchDraft.trim());
      setOffset(0);
    }, 250);
    return () => window.clearTimeout(id);
  }, [searchDraft]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    listAssets({
      search,
      asset_type: type,
      project_id: projectID,
      status,
      source_workflow: workflow,
      sort,
      include_archived: includeArchived,
      limit: PAGE_SIZE,
      offset,
    }).then(next => {
      if (!controller.signal.aborted) setData(next);
    }).catch(err => {
      if (!controller.signal.aborted) setError(err instanceof Error ? err.message : 'Asset Library is unavailable.');
    }).finally(() => {
      if (!controller.signal.aborted) setLoading(false);
    });
    return () => controller.abort();
  }, [includeArchived, offset, projectID, search, sort, status, type, workflow]);

  const refresh = useCallback(async () => {
    const next = await listAssets({ search, asset_type: type, project_id: projectID, status, source_workflow: workflow, sort, include_archived: includeArchived, limit: PAGE_SIZE, offset });
    setData(next);
    return next;
  }, [includeArchived, offset, projectID, search, sort, status, type, workflow]);

  const assets = data?.assets ?? [];
  const summary = data?.summary ?? {};
  const filtersActive = Boolean(search || type || projectID || status || workflow || includeArchived || sort !== 'newest');

  async function verify(asset: MediaAsset) {
    setMutationError(null);
    setMutating(true);
    try {
      const { asset: updated } = await reconcileAsset(asset.id);
      setSelected(updated);
      await refresh();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : 'Storage verification failed.');
    } finally {
      setMutating(false);
    }
  }

  async function confirmArchive() {
    if (!confirmAsset) return;
    setMutationError(null);
    setMutating(true);
    try {
      const { asset: updated } = await archiveAsset(confirmAsset.id);
      setConfirmAsset(null);
      setSelected(current => current?.id === updated.id ? updated : current);
      await refresh();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : 'Archive failed.');
    } finally {
      setMutating(false);
    }
  }

  async function restore(asset: MediaAsset) {
    setMutationError(null);
    setMutating(true);
    try {
      const { asset: updated } = await restoreAsset(asset.id);
      setSelected(updated);
      await refresh();
    } catch (err) {
      setMutationError(err instanceof Error ? err.message : 'Restore failed.');
    } finally {
      setMutating(false);
    }
  }

  return (
    <div className="asset-library-page">
      <section className="asset-library-header">
        <div>
          <div className="page-eyebrow">CONTENT</div>
          <h1>Asset Library</h1>
          <p>Manage uploaded sources, generated videos, thumbnails, and downloadable project packages.</p>
        </div>
        <button className="generate-btn idle" type="button" onClick={() => onNavigate('clipStudio')}>Upload media</button>
      </section>

      <section className="asset-summary-strip" aria-label="Asset summary">
        <SummaryCard label="Total assets" value={summary.total ?? 0} />
        <SummaryCard label="Videos" value={summary.videos ?? 0} />
        <SummaryCard label="Thumbnails" value={summary.thumbnails ?? 0} />
        <SummaryCard label="Packages" value={summary.packages ?? 0} />
        <SummaryCard label="Needs attention" value={summary.needs_attention ?? 0} tone="attention" />
      </section>

      <section className="asset-filter-toolbar" aria-label="Asset filters">
        <label className="asset-search">
          <span>Search</span>
          <input value={searchDraft} onChange={event => setSearchDraft(event.target.value)} placeholder="Search assets" maxLength={120} />
        </label>
        <FilterSelect label="Type" value={type} onChange={value => { setType(value); setOffset(0); }} options={data?.facets.asset_type ?? []} />
        <FilterSelect label="Project" value={projectID} onChange={value => { setProjectID(value); setOffset(0); }} options={data?.facets.project ?? []} />
        <FilterSelect label="Status" value={status} onChange={value => { setStatus(value); setOffset(0); }} options={data?.facets.status ?? []} />
        <FilterSelect label="Workflow" value={workflow} onChange={value => { setWorkflow(value); setOffset(0); }} options={data?.facets.source_workflow ?? []} />
        <label className="asset-filter-field">
          <span>Sort</span>
          <select value={sort} onChange={event => { setSort(event.target.value); setOffset(0); }}>
            <option value="newest">Newest first</option>
            <option value="oldest">Oldest first</option>
            <option value="name">Name</option>
            <option value="size_desc">Largest</option>
            <option value="size_asc">Smallest</option>
          </select>
        </label>
        <label className="asset-archive-toggle">
          <input type="checkbox" checked={includeArchived} onChange={event => { setIncludeArchived(event.target.checked); setOffset(0); }} />
          <span>Include archived</span>
        </label>
        <button className="generate-btn secondary" type="button" disabled={!filtersActive} onClick={() => {
          setSearchDraft('');
          setSearch('');
          setType('');
          setProjectID('');
          setStatus('');
          setWorkflow('');
          setSort('newest');
          setIncludeArchived(false);
          setOffset(0);
        }}>Clear filters</button>
      </section>

      {loading && <AssetState title="Loading assets..." desc="Fetching the durable media index." />}
      {!loading && error && <AssetState tone="error" title="Asset Library unavailable" desc={error} />}
      {!loading && !error && assets.length === 0 && (
        <AssetState
          title={filtersActive ? 'No results match these filters' : includeArchived ? 'No archived assets' : 'No assets created yet'}
          desc={filtersActive ? 'Clear filters or broaden your search.' : 'Upload a source or generate a package from Clip Generator to create the first durable asset.'}
          action={<button className="generate-btn idle" type="button" onClick={() => onNavigate('clipStudio')}>Open Clip Generator</button>}
        />
      )}

      {!loading && !error && assets.length > 0 && (
        <>
          <section className="asset-grid" aria-label="Assets">
            {assets.map(asset => (
              <AssetCard
                key={asset.id}
                asset={asset}
                onOpen={() => setSelected(asset)}
                onArchive={() => setConfirmAsset(asset)}
              />
            ))}
          </section>
          <div className="asset-pagination">
            <button className="generate-btn secondary" type="button" disabled={offset === 0} onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}>Previous</button>
            <span>{Math.min(offset + 1, data?.pagination.total ?? 0)}-{Math.min(offset + assets.length, data?.pagination.total ?? 0)} of {data?.pagination.total ?? 0}</span>
            <button className="generate-btn secondary" type="button" disabled={!data?.pagination.has_more} onClick={() => setOffset(offset + PAGE_SIZE)}>Next</button>
          </div>
        </>
      )}

      {selected && (
        <AssetDetailPanel
          asset={selected}
          pending={mutating}
          error={mutationError}
          onClose={() => setSelected(null)}
          onVerify={() => verify(selected)}
          onArchive={() => setConfirmAsset(selected)}
          onRestore={() => restore(selected)}
          onOpenProject={() => selected.project && onNavigate('contentProjects')}
        />
      )}

      <ConfirmationDialog
        open={Boolean(confirmAsset)}
        title="Archive asset?"
        description="This hides the asset from the default library but keeps the durable media object and related project history."
        confirmLabel="Archive"
        intent="destructive"
        pending={mutating}
        pendingLabel="Archiving..."
        errorMessage={mutationError}
        onConfirm={confirmArchive}
        onCancel={() => { if (!mutating) setConfirmAsset(null); }}
      />
    </div>
  );
}

function SummaryCard({ label, value, tone }: { label: string; value: number; tone?: 'attention' }) {
  return (
    <div className={`asset-summary-card${tone ? ` is-${tone}` : ''}`}>
      <span>{label}</span>
      <strong>{value.toLocaleString()}</strong>
    </div>
  );
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string; count: number }[]; onChange: (value: string) => void }) {
  return (
    <label className="asset-filter-field">
      <span>{label}</span>
      <select value={value} onChange={event => onChange(event.target.value)}>
        <option value="">All</option>
        {options.map(option => (
          <option key={option.value} value={option.value}>{option.label} ({option.count})</option>
        ))}
      </select>
    </label>
  );
}

function AssetCard({ asset, onOpen, onArchive }: { asset: MediaAsset; onOpen: () => void; onArchive: () => void }) {
  const meta = compactAssetMeta(asset);
  return (
    <article className={`asset-card is-${asset.status}`} tabIndex={0} onKeyDown={event => { if (event.key === 'Enter') onOpen(); }}>
      <button className="asset-thumb" type="button" onClick={onOpen} aria-label={`Open ${asset.display_name}`}>
        {asset.preview_url && asset.asset_type === 'thumbnail'
          ? <img src={apiUrl(asset.preview_url)} alt="" loading="lazy" />
          : asset.mime_type?.startsWith('audio/')
            ? <span aria-hidden="true">AUD</span>
          : <span aria-hidden="true">{assetIcon(asset)}</span>}
      </button>
      <div className="asset-card-body">
        <div className="asset-card-top">
          <span className="asset-type">{readableAssetType(asset.asset_type)}</span>
          <span className={`asset-status is-${asset.status}`}>{readableStatus(asset.status)}</span>
        </div>
        <button className="asset-name" type="button" onClick={onOpen}>{asset.display_name}</button>
        <div className="asset-context">{asset.project?.title || 'Workspace asset'}</div>
        <div className="asset-meta">{meta}</div>
      </div>
      <div className="asset-card-actions">
        <button type="button" onClick={onOpen}>Details</button>
        {asset.capabilities.can_download && <a href={apiUrl(asset.download_url || '')}>Download</a>}
        {asset.capabilities.can_archive && <button type="button" onClick={onArchive}>Archive</button>}
      </div>
    </article>
  );
}

function AssetDetailPanel({ asset, pending, error, onClose, onVerify, onArchive, onRestore, onOpenProject }: {
  asset: MediaAsset;
  pending: boolean;
  error: string | null;
  onClose: () => void;
  onVerify: () => void;
  onArchive: () => void;
  onRestore: () => void;
  onOpenProject: () => void;
}) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const previousFocus = useRef<HTMLElement | null>(null);
  const fields = useMemo(() => assetFields(asset), [asset]);

  useEffect(() => {
    previousFocus.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    panelRef.current?.focus();
    function onKey(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose();
    }
    document.addEventListener('keydown', onKey);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', onKey);
      previousFocus.current?.focus();
    };
  }, [onClose]);

  return (
    <div className="asset-detail-layer">
      <button className="modal-backdrop" type="button" aria-label="Close asset details" onClick={onClose} />
      <aside ref={panelRef} className="asset-detail-panel" role="dialog" aria-modal="true" aria-label="Asset details" tabIndex={-1}>
        <div className="asset-detail-header">
          <div>
            <span className="asset-type">{readableAssetType(asset.asset_type)}</span>
            <h2>{asset.display_name}</h2>
            <span className={`asset-status is-${asset.status}`}>{readableStatus(asset.status)}</span>
          </div>
          <button className="modal-close" type="button" onClick={onClose} aria-label="Close asset details">x</button>
        </div>
        <div className="asset-preview-box">
          {asset.preview_url && asset.mime_type?.startsWith('image/')
            ? <img src={apiUrl(asset.preview_url)} alt={asset.display_name} />
            : asset.preview_url && asset.mime_type?.startsWith('video/')
              ? <video src={apiUrl(asset.preview_url)} controls preload="metadata" />
              : asset.preview_url && asset.mime_type?.startsWith('audio/')
                ? <audio src={apiUrl(asset.preview_url)} controls preload="metadata" />
              : <span aria-hidden="true">{assetIcon(asset)}</span>}
        </div>
        {asset.failure && <div className="confirmation-error" role="alert">{asset.failure.message}</div>}
        {error && <div className="confirmation-error" role="alert">{error}</div>}
        <dl className="asset-detail-fields">
          {fields.map(field => (
            <div key={field.label}>
              <dt>{field.label}</dt>
              <dd>{field.value}</dd>
            </div>
          ))}
        </dl>
        {asset.checksum_sha256 && (
          <details className="asset-checksum">
            <summary>SHA-256 checksum</summary>
            <code>{asset.checksum_sha256}</code>
          </details>
        )}
        <div className="asset-detail-actions">
          {asset.capabilities.can_download && <a className="generate-btn idle" href={apiUrl(asset.download_url || '')}>Download</a>}
          <button className="generate-btn secondary" type="button" onClick={onVerify} disabled={pending}>Verify storage</button>
          {asset.project && <button className="generate-btn secondary" type="button" onClick={onOpenProject}>Open project</button>}
          {asset.capabilities.can_archive && <button className="generate-btn danger" type="button" onClick={onArchive}>Archive</button>}
          {asset.capabilities.can_restore && <button className="generate-btn idle" type="button" onClick={onRestore} disabled={pending}>Restore</button>}
        </div>
      </aside>
    </div>
  );
}

function AssetState({ title, desc, action, tone = 'empty' }: { title: string; desc: string; action?: ReactNode; tone?: 'empty' | 'error' }) {
  return (
    <div className={`empty-state system-state is-${tone}`} role={tone === 'error' ? 'alert' : 'status'}>
      <div className="empty-icon" aria-hidden="true">{tone === 'error' ? '!' : 'i'}</div>
      <div className="empty-title">{title}</div>
      <div className="empty-desc">{desc}</div>
      {action && <div className="empty-action">{action}</div>}
    </div>
  );
}

function compactAssetMeta(asset: MediaAsset): string {
  return [formatBytes(asset.size_bytes), asset.duration_seconds ? formatDuration(asset.duration_seconds) : resolution(asset), formatDate(asset.created_at)].filter(Boolean).join(' · ');
}

function assetFields(asset: MediaAsset): { label: string; value: string }[] {
  return [
    { label: 'Status', value: readableStatus(asset.status) },
    { label: 'Project', value: asset.project?.title || 'Workspace asset' },
    { label: 'Scene', value: asset.scene?.label || 'None' },
    { label: 'Source workflow', value: readableWorkflow(asset.source_workflow) },
    { label: 'MIME type', value: asset.mime_type || 'Unknown' },
    { label: 'File size', value: formatBytes(asset.size_bytes) || 'Unknown' },
    { label: 'Duration', value: asset.duration_seconds ? formatDuration(asset.duration_seconds) : 'Unknown' },
    { label: 'Resolution', value: resolution(asset) || 'Unknown' },
    { label: 'Created', value: formatDate(asset.created_at) },
    { label: 'Last verified', value: asset.last_verified_at ? formatDate(asset.last_verified_at) : 'Not verified' },
    { label: 'Storage', value: asset.storage_label },
  ];
}

function assetIcon(asset: MediaAsset): string {
  if (asset.asset_type === 'package') return 'ZIP';
  if (asset.asset_type === 'thumbnail') return 'IMG';
  if (asset.asset_type === 'audio' || asset.asset_type === 'voiceover' || asset.mime_type?.startsWith('audio/')) return 'AUD';
  if (asset.asset_type.includes('video')) return 'VID';
  return 'FILE';
}

function readableAssetType(type: string): string {
  return type.split('_').map(part => part[0].toUpperCase() + part.slice(1)).join(' ');
}

function readableStatus(status: string): string {
  return status[0].toUpperCase() + status.slice(1);
}

function readableWorkflow(workflow: string): string {
  return workflow.split('_').map(part => part[0].toUpperCase() + part.slice(1)).join(' ');
}

function formatBytes(value?: number): string {
  if (value == null) return '';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 * 1024 * 1024) return `${(value / 1024 / 1024).toFixed(1)} MB`;
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GB`;
}

function formatDuration(value: number): string {
  const minutes = Math.floor(value / 60);
  const seconds = Math.round(value % 60).toString().padStart(2, '0');
  return minutes ? `${minutes}:${seconds}` : `${Math.round(value)}s`;
}

function resolution(asset: MediaAsset): string {
  return asset.width && asset.height ? `${asset.width} x ${asset.height}` : '';
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(new Date(value));
}

import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import {
  ApiError,
  createContentProject,
  listContentProjects,
  type ContentProject,
  type ContentProjectPayload,
  type ContentProjectStatus,
} from '../lib/api/client';

interface Props {
  onOpenProject: (projectID: string) => void;
}

const STATUS_LABELS: Record<string, string> = {
  idea: 'Idea',
  brief_ready: 'Brief ready',
  draft: 'Draft',
  needs_review: 'Needs review',
  approved: 'Approved',
  in_production: 'In production',
  rendered: 'Rendered',
  scheduled: 'Scheduled',
  published: 'Published',
  archived: 'Archived',
};

const STAGE_LABELS: Record<string, string> = {
  idea: 'Idea',
  brief: 'Brief',
  script: 'Script',
  scenes: 'Scenes',
  voice: 'Voice',
  video: 'Video',
  thumbnail: 'Thumbnail',
  publishing: 'Publishing',
  complete: 'Complete',
};

const PLATFORM_OPTIONS = [
  { value: 'youtube', label: 'YouTube' },
  { value: 'tiktok', label: 'TikTok' },
  { value: 'instagram', label: 'Instagram' },
  { value: 'facebook', label: 'Facebook' },
  { value: 'x', label: 'X' },
];

const FORMAT_OPTIONS = [
  { value: 'short_video', label: 'Short video' },
  { value: 'long_video', label: 'Long video' },
  { value: 'carousel', label: 'Carousel' },
  { value: 'post', label: 'Post' },
  { value: 'thread', label: 'Thread' },
];

const STATUS_FILTERS: Array<{ value: '' | ContentProjectStatus; label: string }> = [
  { value: '', label: 'All statuses' },
  { value: 'idea', label: 'Idea' },
  { value: 'draft', label: 'Draft' },
  { value: 'needs_review', label: 'Needs review' },
  { value: 'approved', label: 'Approved' },
  { value: 'in_production', label: 'In production' },
  { value: 'published', label: 'Published' },
];

function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return 'Request failed.';
}

function formatDate(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function defaultForm(): ContentProjectPayload {
  return {
    title: '',
    topic: '',
    target_platforms: ['youtube', 'tiktok', 'instagram'],
    content_format: 'short_video',
    target_duration_seconds: 30,
    language: 'en-US',
    source_type: 'manual',
    status: 'idea',
    current_stage: 'idea',
  };
}

export function ContentProjectsPage({ onOpenProject }: Props) {
  const [projects, setProjects] = useState<ContentProject[]>([]);
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showNewProject, setShowNewProject] = useState(false);
  const [form, setForm] = useState<ContentProjectPayload>(() => defaultForm());
  const [formError, setFormError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const hasFilters = search.trim() !== '' || status !== '';

  const loadProjects = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await listContentProjects({ q: search.trim(), status });
      setProjects(response.projects);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, [search, status]);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      void loadProjects();
    }, 160);
    return () => window.clearTimeout(timeout);
  }, [loadProjects]);

  const titleLength = useMemo(() => form.title.trim().length, [form.title]);

  function updatePlatform(value: string, checked: boolean) {
    setForm(current => ({
      ...current,
      target_platforms: checked
        ? Array.from(new Set([...current.target_platforms, value]))
        : current.target_platforms.filter(platform => platform !== value),
    }));
  }

  async function submitProject(event: FormEvent) {
    event.preventDefault();
    setFormError(null);
    if (!form.title.trim()) {
      setFormError('Project title is required.');
      return;
    }
    if (!form.topic.trim()) {
      setFormError('Topic is required.');
      return;
    }
    if (form.target_platforms.length === 0) {
      setFormError('Choose at least one target platform.');
      return;
    }
    if (form.target_duration_seconds < 5) {
      setFormError('Target duration must be at least 5 seconds.');
      return;
    }
    setCreating(true);
    try {
      const created = await createContentProject({
        ...form,
        title: form.title.trim(),
        topic: form.topic.trim(),
      });
      setProjects(current => [created, ...current.filter(project => project.id !== created.id)]);
      setShowNewProject(false);
      setForm(defaultForm());
      onOpenProject(created.id);
    } catch (err) {
      setFormError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  }

  return (
    <section className="page-section content-projects-page">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Content</div>
          <h1>Projects</h1>
          <p>Persistent production spaces for scripts, briefs, handoffs, and the next creator workflow stages.</p>
        </div>
        <button className="generate-btn idle" type="button" onClick={() => setShowNewProject(true)}>New project</button>
      </div>

      <div className="project-toolbar settings-card">
        <label className="form-group">
          <span className="form-label">Search projects</span>
          <input className="form-input" value={search} onChange={event => setSearch(event.target.value)} placeholder="Search by title or topic" />
        </label>
        <label className="form-group">
          <span className="form-label">Status</span>
          <select className="form-input" value={status} onChange={event => setStatus(event.target.value)}>
            {STATUS_FILTERS.map(item => <option key={item.value} value={item.value}>{item.label}</option>)}
          </select>
        </label>
        <button className="generate-btn secondary" type="button" onClick={() => void loadProjects()} disabled={loading}>
          {loading ? 'Loading...' : 'Retry'}
        </button>
      </div>

      {showNewProject && (
        <div className="project-modal-backdrop" role="presentation">
          <form className="project-modal settings-card" onSubmit={event => void submitProject(event)} aria-label="Create content project">
            <div className="project-modal-header">
              <div>
                <div className="settings-card-title">New content project</div>
                <p>Create the persistent container before script, clip, and asset work starts.</p>
              </div>
              <button className="mini-copy-btn" type="button" onClick={() => setShowNewProject(false)} aria-label="Close new project form">Close</button>
            </div>

            {formError && <div className="clip-error">{formError}</div>}

            <div className="form-grid two">
              <label className="form-group">
                <span className="form-label">Project title</span>
                <input className="form-input" value={form.title} maxLength={180} onChange={event => setForm(current => ({ ...current, title: event.target.value }))} />
                <span className="project-field-hint">{titleLength}/180</span>
              </label>
              <label className="form-group">
                <span className="form-label">Topic</span>
                <input className="form-input" value={form.topic} maxLength={220} onChange={event => setForm(current => ({ ...current, topic: event.target.value }))} />
              </label>
            </div>

            <fieldset className="project-platform-fieldset">
              <legend className="form-label">Target platforms</legend>
              <div className="project-platform-options">
                {PLATFORM_OPTIONS.map(platform => (
                  <label key={platform.value} className="project-checkbox">
                    <input
                      type="checkbox"
                      checked={form.target_platforms.includes(platform.value)}
                      onChange={event => updatePlatform(platform.value, event.target.checked)}
                    />
                    <span>{platform.label}</span>
                  </label>
                ))}
              </div>
            </fieldset>

            <div className="form-grid three">
              <label className="form-group">
                <span className="form-label">Content format</span>
                <select className="form-input" value={form.content_format} onChange={event => setForm(current => ({ ...current, content_format: event.target.value }))}>
                  {FORMAT_OPTIONS.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </label>
              <label className="form-group">
                <span className="form-label">Target duration seconds</span>
                <input className="form-input" type="number" min={5} max={7200} value={form.target_duration_seconds} onChange={event => setForm(current => ({ ...current, target_duration_seconds: Number(event.target.value) }))} />
              </label>
              <label className="form-group">
                <span className="form-label">Language</span>
                <input className="form-input" value={form.language} onChange={event => setForm(current => ({ ...current, language: event.target.value }))} />
              </label>
            </div>

            <div className="clip-action-row">
              <button className="generate-btn idle" type="submit" disabled={creating}>{creating ? 'Creating...' : 'Create project'}</button>
              <button className="generate-btn secondary" type="button" onClick={() => setShowNewProject(false)} disabled={creating}>Cancel</button>
            </div>
          </form>
        </div>
      )}

      {error && (
        <div className="system-state is-error" role="alert">
          <div className="empty-title">Projects could not load.</div>
          <div className="empty-desc">{error}</div>
          <button className="generate-btn secondary" type="button" onClick={() => void loadProjects()}>Retry</button>
        </div>
      )}

      {!error && loading && (
        <div className="project-grid">
          {[0, 1, 2].map(item => <div className="project-card project-card-loading" key={item} />)}
        </div>
      )}

      {!error && !loading && projects.length === 0 && (
        <div className="system-state is-empty">
          <div className="empty-title">{hasFilters ? 'No matching projects.' : 'No content projects yet.'}</div>
          <div className="empty-desc">{hasFilters ? 'Adjust search or status filters to see more projects.' : 'Create a project to persist scripts and production state.'}</div>
          <button className="generate-btn idle" type="button" onClick={() => setShowNewProject(true)}>New project</button>
        </div>
      )}

      {!error && !loading && projects.length > 0 && (
        <div className="project-grid">
          {projects.map(project => (
            <article className="project-card script-card" key={project.id}>
              <div className="project-card-topline">
                <span>{STATUS_LABELS[project.status] || project.status}</span>
                <span>{STAGE_LABELS[project.current_stage] || project.current_stage}</span>
              </div>
              <h2>{project.title}</h2>
              <p>{project.topic || 'No topic recorded yet.'}</p>
              <div className="script-badge-row">
                {project.target_platforms.map(platform => <span className="script-badge" key={platform}>{platformLabel(platform)}</span>)}
              </div>
              <div className="project-card-meta">
                <span>{formatFormat(project.content_format)}</span>
                <span>{project.target_duration_seconds}s</span>
                <span>Updated {formatDate(project.updated_at)}</span>
              </div>
              <button className="generate-btn secondary" type="button" onClick={() => onOpenProject(project.id)}>Resume</button>
            </article>
          ))}
        </div>
      )}
    </section>
  );
}

function platformLabel(value: string): string {
  return PLATFORM_OPTIONS.find(option => option.value === value)?.label || value;
}

function formatFormat(value: string): string {
  return FORMAT_OPTIONS.find(option => option.value === value)?.label || value.replaceAll('_', ' ');
}

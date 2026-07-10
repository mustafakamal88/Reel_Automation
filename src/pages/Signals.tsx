import { useEffect, useMemo, useState, type ReactNode } from 'react';
import type { Platform } from '../types';
import { PLATFORMS } from '../data/platforms';
import {
  ApiError,
  analyzeYouTubeChannel,
  analyzeYouTubeVideo,
  discoverTrendCandidates,
  generateReelScript,
  getResearchProviderStatus,
  type ReelContentPackage,
  type ResearchProviderStatus,
  type TrendCandidate,
  type TrendDiscoveryResponse,
  type YouTubeChannelAnalysisResponse,
  type YouTubeVideoAnalysisResponse,
} from '../lib/api/client';
import { storage } from '../lib/storage';

type ResearchTab = 'keywords' | 'platforms' | 'video' | 'channel' | 'niche';

const TABS: { id: ResearchTab; label: string }[] = [
  { id: 'keywords', label: 'Trending Keywords' },
  { id: 'platforms', label: 'Platform Trends' },
  { id: 'video', label: 'YouTube Video Analyzer' },
  { id: 'channel', label: 'YouTube Channel Analyzer' },
  { id: 'niche', label: 'Niche Finder' },
];

const REGION_OPTIONS = [
  { label: 'Global', value: 'US' },
  { label: 'US', value: 'US' },
  { label: 'UK', value: 'GB' },
  { label: 'PK', value: 'PK' },
  { label: 'IN', value: 'IN' },
  { label: 'Custom', value: 'custom' },
];

const LANGUAGE_OPTIONS = [
  { label: 'English', value: 'en-US' },
  { label: 'Urdu', value: 'ur-PK' },
  { label: 'Hindi', value: 'hi-IN' },
  { label: 'Arabic', value: 'ar' },
  { label: 'Spanish', value: 'es' },
  { label: 'Custom', value: 'custom' },
];

const AUDIENCE_OPTIONS = ['Global', 'South Asian', 'UK Pakistani', 'US Gen Z', 'Muslim audience', 'Tech creators', 'Finance creators', 'Entertainment creators', 'custom text'];

const PLATFORM_FILTERS: { id: Platform | 'all'; label: string; providerID?: string }[] = [
  { id: 'all', label: 'All available' },
  { id: 'gt', label: 'Google Trends', providerID: 'google_trends_rss' },
  { id: 'yt', label: 'YouTube', providerID: 'youtube_data_api' },
  { id: 'tt', label: 'TikTok', providerID: 'tiktok_research_api' },
  { id: 'ig', label: 'Instagram', providerID: 'instagram_graph_api' },
  { id: 'x', label: 'X', providerID: 'x_api' },
  { id: 'fb', label: 'Facebook', providerID: 'facebook_graph_api' },
];

interface Props {
  initialFilter?: Platform | 'all';
  onFilterChange?: (f: Platform | 'all') => void;
  onStatusChange?: (status: string) => void;
  onScriptGenerated?: (candidate: TrendCandidate, pkg: ReelContentPackage) => void;
}

export function TrendFinderPage({ initialFilter = 'all', onFilterChange, onStatusChange, onScriptGenerated }: Props) {
  const [tab, setTab] = useState<ResearchTab>('keywords');
  const [platformFilter, setPlatformFilter] = useState<Platform | 'all'>(initialFilter);
  const [regionChoice, setRegionChoice] = useState('US');
  const [customRegion, setCustomRegion] = useState('');
  const [languageChoice, setLanguageChoice] = useState('en-US');
  const [customLanguage, setCustomLanguage] = useState('');
  const [audience, setAudience] = useState('Global');
  const [customAudience, setCustomAudience] = useState('');
  const [response, setResponse] = useState<TrendDiscoveryResponse | null>(null);
  const [providers, setProviders] = useState<ResearchProviderStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [generatingID, setGeneratingID] = useState<string | null>(null);
  const [generated, setGenerated] = useState<Record<string, ReelContentPackage>>({});
  const [generationErrors, setGenerationErrors] = useState<Record<string, string>>({});
  const [videoURL, setVideoURL] = useState('');
  const [videoResult, setVideoResult] = useState<YouTubeVideoAnalysisResponse | null>(null);
  const [videoLoading, setVideoLoading] = useState(false);
  const [channelURL, setChannelURL] = useState('');
  const [channelResult, setChannelResult] = useState<YouTubeChannelAnalysisResponse | null>(null);
  const [channelLoading, setChannelLoading] = useState(false);

  const region = regionChoice === 'custom' ? customRegion.trim() : regionChoice;
  const language = languageChoice === 'custom' ? customLanguage.trim() : languageChoice;
  const audienceText = audience === 'custom text' ? customAudience.trim() : audience;

  useEffect(() => {
    let cancelled = false;
    getResearchProviderStatus()
      .then(data => {
        if (!cancelled) setProviders(data.providers);
      })
      .catch(() => {
        if (!cancelled) setProviders([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    discoverTrendCandidates({ region: region || 'US', language: language || 'en-US', limit: 20 })
      .then(data => {
        if (cancelled) return;
        setResponse(data);
        if (data.provider_status === 'ok') {
          storage.updateActivity(current => ({
            ...current,
            trendsFoundToday: data.candidates.length,
            latestTrendPulled: new Date().toISOString(),
          }));
        }
      })
      .catch(err => {
        if (cancelled) return;
        setError(err instanceof ApiError ? err.message : 'Trend discovery request failed.');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [region, language]);

  useEffect(() => {
    if (!onStatusChange) return;
    if (loading) onStatusChange('Checking live trend provider status');
    else if (error) onStatusChange('Trend discovery unavailable');
    else onStatusChange(statusSubtitle(response));
  }, [error, loading, onStatusChange, response]);

  const filteredCandidates = useMemo(() => {
    const candidates = response?.candidates ?? [];
    if (platformFilter === 'all') return candidates;
    if (platformFilter === 'gt') return candidates.filter(c => c.source === 'google_trends_rss');
    return [];
  }, [platformFilter, response]);

  function setFilter(next: Platform | 'all') {
    setPlatformFilter(next);
    onFilterChange?.(next);
  }

  function handleGenerate(candidate: TrendCandidate) {
    setGeneratingID(candidate.id);
    setGenerationErrors(prev => {
      const next = { ...prev };
      delete next[candidate.id];
      return next;
    });
    generateReelScript({
      trend_candidate_id: candidate.id,
      trend_candidate: candidate,
      platform_targets: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
      duration_target: '30s',
      language: candidate.language || language || 'en-US',
      region: candidate.region || region || 'US',
    })
      .then(data => {
        setGenerated(prev => ({ ...prev, [candidate.id]: data.package }));
        onScriptGenerated?.(candidate, data.package);
      })
      .catch(err => {
        setGenerationErrors(prev => ({ ...prev, [candidate.id]: err instanceof ApiError ? err.message : 'Script generation request failed.' }));
      })
      .finally(() => setGeneratingID(current => (current === candidate.id ? null : current)));
  }

  function analyzeVideo() {
    if (!videoURL.trim()) return;
    setVideoLoading(true);
    setVideoResult(null);
    analyzeYouTubeVideo(videoURL.trim())
      .then(res => {
        setVideoResult(res);
        if (res.status === 'ok') {
          storage.updateActivity(current => ({
            ...current,
            youtubeAnalysesRun: current.youtubeAnalysesRun + 1,
            latestYouTubeAnalysis: new Date().toISOString(),
          }));
        }
      })
      .catch(err => setVideoResult({ status: 'provider_error', message: err instanceof Error ? err.message : 'YouTube video analysis is unavailable.' }))
      .finally(() => setVideoLoading(false));
  }

  function analyzeChannel() {
    if (!channelURL.trim()) return;
    setChannelLoading(true);
    setChannelResult(null);
    analyzeYouTubeChannel(channelURL.trim())
      .then(res => {
        setChannelResult(res);
        if (res.status === 'ok') {
          storage.updateActivity(current => ({
            ...current,
            channelAnalysesRun: current.channelAnalysesRun + 1,
            latestChannelAnalysis: new Date().toISOString(),
          }));
        }
      })
      .catch(err => setChannelResult({ status: 'provider_error', message: err instanceof Error ? err.message : 'YouTube channel analysis is unavailable.' }))
      .finally(() => setChannelLoading(false));
  }

  function generateFromVideo() {
    if (!videoResult || videoResult.status !== 'ok') return;
    handleGenerate(candidateFromVideo(videoResult, region || 'US', language || 'en-US'));
  }

  function generateFromChannelIdea(idea: string) {
    if (!channelResult || channelResult.status !== 'ok') return;
    handleGenerate(candidateFromChannel(channelResult, idea, region || 'US', language || 'en-US'));
  }

  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Trend Intelligence</div>
          <h1>Daily creator research from connected sources.</h1>
          <p>TrendCortex shows real provider data when available and clear not-configured states when a source is not connected.</p>
        </div>
      </div>

      <div className="research-tabs" role="tablist" aria-label="Research mode">
        {TABS.map(item => (
          <button key={item.id} className={tab === item.id ? 'active' : ''} type="button" onClick={() => setTab(item.id)}>
            {item.label}
          </button>
        ))}
      </div>

      {tab === 'keywords' && (
        <TrendingKeywordsTab
          regionChoice={regionChoice}
          setRegionChoice={setRegionChoice}
          customRegion={customRegion}
          setCustomRegion={setCustomRegion}
          languageChoice={languageChoice}
          setLanguageChoice={setLanguageChoice}
          customLanguage={customLanguage}
          setCustomLanguage={setCustomLanguage}
          audience={audience}
          setAudience={setAudience}
          customAudience={customAudience}
          setCustomAudience={setCustomAudience}
          audienceText={audienceText}
          providers={providers}
          platformFilter={platformFilter}
          setPlatformFilter={setFilter}
          response={response}
          loading={loading}
          error={error}
          filteredCandidates={filteredCandidates}
          generated={generated}
          generationErrors={generationErrors}
          generatingID={generatingID}
          onGenerate={handleGenerate}
        />
      )}

      {tab === 'platforms' && <PlatformTrendsTab providers={providers} />}

      {tab === 'video' && (
        <YouTubeVideoTab
          value={videoURL}
          onChange={setVideoURL}
          onAnalyze={analyzeVideo}
          loading={videoLoading}
          result={videoResult}
          onGenerate={generateFromVideo}
        />
      )}

      {tab === 'channel' && (
        <YouTubeChannelTab
          value={channelURL}
          onChange={setChannelURL}
          onAnalyze={analyzeChannel}
          loading={channelLoading}
          result={channelResult}
          onGenerateIdea={generateFromChannelIdea}
        />
      )}

      {tab === 'niche' && (
        <NicheFinderTab
          providers={providers}
          candidates={response?.candidates ?? []}
          region={region || 'US'}
          language={language || 'en-US'}
          audience={audienceText || 'Global'}
          onGenerate={handleGenerate}
          generated={generated}
          generationErrors={generationErrors}
          generatingID={generatingID}
        />
      )}
    </section>
  );
}

function TrendingKeywordsTab(props: {
  regionChoice: string;
  setRegionChoice: (value: string) => void;
  customRegion: string;
  setCustomRegion: (value: string) => void;
  languageChoice: string;
  setLanguageChoice: (value: string) => void;
  customLanguage: string;
  setCustomLanguage: (value: string) => void;
  audience: string;
  setAudience: (value: string) => void;
  customAudience: string;
  setCustomAudience: (value: string) => void;
  audienceText: string;
  providers: ResearchProviderStatus[];
  platformFilter: Platform | 'all';
  setPlatformFilter: (value: Platform | 'all') => void;
  response: TrendDiscoveryResponse | null;
  loading: boolean;
  error: string | null;
  filteredCandidates: TrendCandidate[];
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onGenerate: (candidate: TrendCandidate) => void;
}) {
  return (
    <>
      <div className="settings-card" style={{ marginBottom: 14 }}>
        <div className="settings-card-title">Research filters</div>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(170px, 1fr))', gap: 10 }}>
          <Select label="Region" value={props.regionChoice} onChange={props.setRegionChoice} options={REGION_OPTIONS} />
          {props.regionChoice === 'custom' && <TextInput label="Custom region code" value={props.customRegion} onChange={props.setCustomRegion} placeholder="e.g. CA, AE" />}
          <Select label="Language" value={props.languageChoice} onChange={props.setLanguageChoice} options={LANGUAGE_OPTIONS} />
          {props.languageChoice === 'custom' && <TextInput label="Custom language" value={props.customLanguage} onChange={props.setCustomLanguage} placeholder="e.g. en-GB" />}
          <Select label="Culture / audience" value={props.audience} onChange={props.setAudience} options={AUDIENCE_OPTIONS.map(value => ({ label: value, value }))} />
          {props.audience === 'custom text' && <TextInput label="Custom audience" value={props.customAudience} onChange={props.setCustomAudience} placeholder="Audience or subculture" />}
        </div>
        <div className="muted-note">Audience fit is used only for script/niche context unless a connected provider returns matching real data.</div>
      </div>

      <ProviderStatusGrid providers={props.providers} response={props.response} loading={props.loading} />

      <div className="filter-bar" role="group" aria-label="Filter by source">
        {PLATFORM_FILTERS.map(filter => {
          const active = filter.id === props.platformFilter;
          const count = countForFilter(filter.id, props.response?.candidates ?? []);
          return (
            <button
              key={filter.id}
              className="filter-chip"
              onClick={() => props.setPlatformFilter(filter.id)}
              aria-pressed={active}
              style={{
                background: active ? '#1c2026' : 'var(--bg-card)',
                borderColor: active ? '#343942' : 'var(--border-card)',
                color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                fontFamily: 'inherit',
                cursor: 'pointer',
              }}
              type="button"
            >
              <span className="filter-chip-dot" style={{ background: dotForFilter(filter.id) }} />
              {filter.label}
              <span className="filter-chip-count">{count}</span>
            </button>
          );
        })}
      </div>

      <DiscoveryState loading={props.loading} error={props.error} response={props.response} filteredCount={props.filteredCandidates.length} />

      {!props.loading && !props.error && props.response?.provider_status === 'ok' && props.filteredCandidates.length > 0 && (
        <div style={{ display: 'grid', gap: 10 }}>
          {props.filteredCandidates.map(candidate => (
            <TrendCandidateCard
              key={candidate.id}
              candidate={candidate}
              audience={props.audienceText}
              generated={props.generated[candidate.id]}
              generationError={props.generationErrors[candidate.id]}
              generating={props.generatingID === candidate.id}
              onGenerate={() => props.onGenerate(candidate)}
            />
          ))}
        </div>
      )}
    </>
  );
}

function PlatformTrendsTab({ providers }: { providers: ResearchProviderStatus[] }) {
  const rows = PLATFORM_FILTERS.filter(p => p.id !== 'all');
  return (
    <div className="settings-card">
      <div className="settings-card-title">Per-platform availability</div>
      <div className="status-list">
        {rows.map(row => {
          const provider = providers.find(p => p.id === row.providerID);
          const status = provider?.status ?? 'not_configured';
          const active = status === 'active';
          return (
            <div className="status-row" key={row.id}>
              <span>{row.label}</span>
              <strong style={{ color: active ? 'var(--green)' : undefined }}>
                {active ? 'Active' : 'Connect API credentials in Settings to enable this source.'}
              </strong>
            </div>
          );
        })}
      </div>
      <div className="muted-note">No fake platform counts are shown. Platform trend tables appear only when official provider data is connected and returned.</div>
    </div>
  );
}

function YouTubeVideoTab({ value, onChange, onAnalyze, loading, result, onGenerate }: {
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  result: YouTubeVideoAnalysisResponse | null;
  onGenerate: () => void;
}) {
  return (
    <AnalyzerShell
      title="YouTube Video Analyzer"
      description="Paste a YouTube video URL. Configured analysis uses official YouTube Data API public metadata only."
      inputLabel="YouTube video URL"
      value={value}
      onChange={onChange}
      onAnalyze={onAnalyze}
      loading={loading}
      buttonLabel="Analyze Video"
    >
      {!result && <div className="neutral-callout">Add YOUTUBE_API_KEY in Settings/Railway variables to analyze YouTube videos.</div>}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <div style={{ display: 'grid', gap: 10 }}>
          <ResultCard title="Summary">
            <TextBlock label="Title" value={result.title} />
            <TextBlock label="Channel" value={[result.channel_title, result.channel_id].filter(Boolean).join(' · ')} />
            <TextBlock label="Published" value={result.published_at} />
            <TextBlock label="Category" value={result.category} />
            <TextBlock label="Duration" value={result.duration} />
            <TextBlock label="Evidence" value={result.metadata?.score_reason} />
          </ResultCard>
          <ResultCard title="Keywords">
            <ChipList items={result.extracted_keywords ?? []} />
            <TextBlock label="Tags" value={(result.tags ?? []).join(', ')} />
          </ResultCard>
          <ResultCard title="Niche">
            <TextBlock label="Inferred niche" value={result.inferred_niche} />
            <TextBlock label="Inferred content angle" value={result.inferred_content_angle} />
          </ResultCard>
          <ResultCard title="Hook / title breakdown">
            <TextBlock label="Hook" value={result.hook_analysis} />
            <TextBlock label="Title structure" value={result.title_structure_analysis} />
            <TextBlock label="Description / hashtags" value={result.description_hashtag_analysis} />
          </ResultCard>
          <ResultCard title="Performance signals">
            <MetricGrid values={{ Views: result.views, Likes: result.likes, Comments: result.comments, ...(result.performance_signals ?? {}) }} />
          </ResultCard>
          <ResultCard title="Suggested remake angles">
            <List items={result.suggested_remake_angles ?? []} />
            <button className="generate-btn idle" type="button" onClick={onGenerate}>Generate Script from this analysis</button>
          </ResultCard>
          <Limitations items={result.limitations ?? []} />
        </div>
      )}
    </AnalyzerShell>
  );
}

function YouTubeChannelTab({ value, onChange, onAnalyze, loading, result, onGenerateIdea }: {
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  result: YouTubeChannelAnalysisResponse | null;
  onGenerateIdea: (idea: string) => void;
}) {
  return (
    <AnalyzerShell
      title="YouTube Channel Analyzer"
      description="Paste a YouTube channel URL, @handle, /channel/ID, /c/Name, or query. Resolution uses official YouTube Data API where possible."
      inputLabel="YouTube channel URL or handle"
      value={value}
      onChange={onChange}
      onAnalyze={onAnalyze}
      loading={loading}
      buttonLabel="Analyze Channel"
    >
      {!result && <div className="neutral-callout">Add YOUTUBE_API_KEY in Settings/Railway variables to analyze YouTube channels.</div>}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <div style={{ display: 'grid', gap: 10 }}>
          <ResultCard title="Channel snapshot">
            <TextBlock label="Channel" value={[result.channel_title, result.channel_id].filter(Boolean).join(' · ')} />
            <MetricGrid values={{ Subscribers: result.subscribers, Views: result.views, Videos: result.video_count, Country: result.country }} />
          </ResultCard>
          <ResultCard title="Niche diagnosis">
            <TextBlock label="Inferred niche" value={result.channel_niche} />
            <TextBlock label="Likely strategy" value={result.likely_strategy} />
          </ResultCard>
          <ResultCard title="Top content pillars"><ChipList items={result.content_pillars ?? []} /></ResultCard>
          <ResultCard title="Best-performing topic patterns"><List items={result.title_patterns ?? []} /></ResultCard>
          <ResultCard title="Keyword clusters"><ChipList items={result.repeated_keywords ?? []} /></ResultCard>
          <ResultCard title="View distribution"><MetricGrid values={result.view_distribution ?? {}} /></ResultCard>
          <ResultCard title="Content opportunities"><List items={result.opportunities ?? []} /></ResultCard>
          <ResultCard title="Suggested next 10 video ideas">
            <List items={result.suggested_content_ideas ?? []} />
            {(result.suggested_content_ideas ?? []).slice(0, 3).map(idea => (
              <button key={idea} className="generate-btn idle" type="button" onClick={() => onGenerateIdea(idea)}>Generate Script: {idea}</button>
            ))}
          </ResultCard>
          <Limitations items={result.limitations ?? []} />
        </div>
      )}
    </AnalyzerShell>
  );
}

function NicheFinderTab({ providers, candidates, region, language, audience, onGenerate, generated, generationErrors, generatingID }: {
  providers: ResearchProviderStatus[];
  candidates: TrendCandidate[];
  region: string;
  language: string;
  audience: string;
  onGenerate: (candidate: TrendCandidate) => void;
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
}) {
  const activeProviders = providers.filter(p => p.status === 'active').map(p => p.id);
  const hasEnough = activeProviders.length >= 2 && candidates.length >= 3;
  const ideas = candidates.slice(0, 5).map((candidate, idx) => ({
    candidate,
    score: Math.max(1, Math.round(candidate.score * (idx === 0 ? 1 : 0.92))),
  }));
  return (
    <div style={{ display: 'grid', gap: 12 }}>
      <div className="settings-card">
        <div className="settings-card-title">Niche Finder inputs</div>
        <MetricGrid values={{ Platform: activeProviders.length ? activeProviders.join(', ') : 'Need connected sources', Country: region, Language: language, Audience: audience, 'Content style': 'From workspace defaults', 'Monetization goal': 'Not set' }} />
      </div>
      {!hasEnough && (
        <div className="empty-state">
          <div className="empty-icon">ND</div>
          <div className="empty-title">Need more connected sources.</div>
          <div className="empty-desc">Niche ideas require enough real cross-source provider data. Google Trends can seed research, but connect YouTube/TikTok/Instagram/X/Facebook before TrendCortex calls this a niche opportunity.</div>
        </div>
      )}
      {hasEnough && (
        <div style={{ display: 'grid', gap: 10 }}>
          {ideas.map(({ candidate, score }) => (
            <TrendCandidateCard
              key={`niche-${candidate.id}`}
              candidate={{ ...candidate, score }}
              audience={audience}
              generated={generated[candidate.id]}
              generationError={generationErrors[candidate.id]}
              generating={generatingID === candidate.id}
              onGenerate={() => onGenerate(candidate)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

function ProviderStatusGrid({ providers, response, loading }: { providers: ResearchProviderStatus[]; response: TrendDiscoveryResponse | null; loading: boolean }) {
  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: 10, marginBottom: 16 }}>
      {PLATFORM_FILTERS.filter(p => p.id !== 'all').map(filter => {
        const provider = providers.find(p => p.id === filter.providerID);
        const status = filter.id === 'gt' && loading ? 'checking' : provider?.status ?? 'not_configured';
        const good = status === 'active' || (filter.id === 'gt' && response?.provider_status === 'ok');
        return (
          <div key={filter.id} className="settings-card" style={{ padding: '12px 14px', borderRadius: 8 }}>
            <div style={{ fontSize: 12, fontWeight: 800, color: 'var(--text-primary)' }}>{provider?.name ?? filter.label}</div>
            <div style={{ fontSize: 11, color: good ? 'var(--green)' : 'var(--text-dim)', marginTop: 5 }}>{statusLabel(status)}</div>
            <div style={{ fontSize: 11, color: 'var(--text-dim)', marginTop: 5 }}>{provider?.message ?? 'Connect API credentials in Settings to enable this source.'}</div>
          </div>
        );
      })}
    </div>
  );
}

function TrendCandidateCard({ candidate, audience, generated, generationError, generating, onGenerate }: {
  candidate: TrendCandidate;
  audience: string;
  generated?: ReelContentPackage;
  generationError?: string;
  generating: boolean;
  onGenerate: () => void;
}) {
  const scoreReason = `Why this scored high: ${candidate.source === 'google_trends_rss' ? 'Google Trends RSS provided current evidence; score reflects provider traffic/recency, evidence quality, and selected audience context.' : 'Score reflects connected public metadata, source confidence, evidence quality, and audience fit.'} Scores are directional, not exact rankings.`;
  return (
    <article style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) auto', gap: 14, padding: '14px 16px', background: 'var(--bg-card)', border: '1px solid var(--border-card)', borderRadius: 8 }}>
      <div style={{ minWidth: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 6 }}>
          <span className="filter-chip-dot" style={{ background: PLATFORMS.gt.color }} />
          <span style={{ fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
            {candidate.source.replaceAll('_', ' ')} · {candidate.region} · {candidate.language}
          </span>
        </div>
        <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--text-primary)', overflowWrap: 'anywhere' }}>{candidate.title || candidate.keyword}</div>
        <TextBlock label="Suggested angle" value={`For ${audience || 'Global'}: explain why "${candidate.keyword}" matters today and make the first three seconds concrete.`} />
        {candidate.evidence && <TextBlock label="Evidence" value={candidate.evidence} />}
        <TextBlock label="Score explanation" value={scoreReason} />
        {candidate.source_url && <a href={candidate.source_url} target="_blank" rel="noreferrer" style={{ display: 'inline-block', marginTop: 8, fontSize: 12, color: 'var(--accent)' }}>Source evidence</a>}
        <div style={{ marginTop: 12, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <button className="generate-btn idle" type="button" onClick={onGenerate} disabled={generating}>{generating ? 'Generating...' : 'Generate Script'}</button>
          {generated && <span style={{ alignSelf: 'center', fontSize: 12, color: 'var(--green)' }}>Script ready in Script Studio</span>}
        </div>
        {generationError && <div style={{ marginTop: 10, fontSize: 12, color: 'var(--red)', overflowWrap: 'anywhere' }}>{generationError}</div>}
        {generated && <GeneratedPackageView pkg={generated} />}
      </div>
      <div style={{ textAlign: 'right', minWidth: 88 }}>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 20, fontWeight: 700, color: 'var(--green)' }}>{Math.round(candidate.score)}</div>
        <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)', textTransform: 'uppercase' }}>score</div>
      </div>
    </article>
  );
}

function AnalyzerShell({ title, description, inputLabel, value, onChange, onAnalyze, loading, buttonLabel, children }: {
  title: string;
  description: string;
  inputLabel: string;
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  buttonLabel: string;
  children: ReactNode;
}) {
  return (
    <div className="settings-card analyzer-panel">
      <div>
        <div className="settings-card-title">{title}</div>
        <div className="muted-note">{description}</div>
      </div>
      <label className="form-group">
        <span className="form-label">{inputLabel}</span>
        <input className="form-input" value={value} onChange={event => onChange(event.target.value)} placeholder="https://www.youtube.com/..." />
      </label>
      <button className="generate-btn idle" type="button" onClick={onAnalyze} disabled={!value.trim() || loading}>{loading ? 'Analyzing...' : buttonLabel}</button>
      {children}
    </div>
  );
}

function DiscoveryState({ loading, error, response, filteredCount }: { loading: boolean; error: string | null; response: TrendDiscoveryResponse | null; filteredCount: number }) {
  if (loading) return <EmptyState icon="ST" title="Loading real trend candidates." desc="Checking Google Trends RSS through the backend." />;
  if (error) return <EmptyState icon="ER" title="Trend discovery is unavailable." desc={error} />;
  if (response?.provider_status === 'provider_not_configured') return <EmptyState icon="NC" title="No trend provider configured." desc={response.message || 'Configure a real backend trend provider to collect trends.'} />;
  if (response?.provider_status === 'no_data') return <EmptyState icon="ND" title="No real trend data found." desc={response.message || 'The configured provider returned no candidates for this request.'} />;
  if (response?.provider_status === 'provider_error') return <EmptyState icon="PE" title="Trend provider error." desc={response.message || 'The backend provider request failed.'} />;
  if (response?.provider_status === 'ok' && filteredCount === 0) return <EmptyState icon="ST" title="No candidates for this source." desc="The selected source has no real provider data connected or returned for this request." />;
  return null;
}

function EmptyState({ icon, title, desc }: { icon: string; title: string; desc: string }) {
  return (
    <div className="empty-state">
      <div className="empty-icon">{icon}</div>
      <div className="empty-title">{title}</div>
      <div className="empty-desc">{desc}</div>
    </div>
  );
}

function HonestResultState({ result }: { result: { status: string; message: string; limitations?: string[] } }) {
  return (
    <div className="neutral-callout">
      <strong>{statusLabel(result.status)}:</strong> {result.message}
      <Limitations items={result.limitations ?? []} />
    </div>
  );
}

function ResultCard({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="settings-card" style={{ padding: '12px 14px' }}>
      <div className="settings-card-title">{title}</div>
      {children}
    </div>
  );
}

function Select({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: { label: string; value: string }[] }) {
  return (
    <label className="form-group">
      <span className="form-label">{label}</span>
      <select className="form-input" value={value} onChange={event => onChange(event.target.value)}>
        {options.map(option => <option key={`${option.label}-${option.value}`} value={option.value}>{option.label}</option>)}
      </select>
    </label>
  );
}

function TextInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return (
    <label className="form-group">
      <span className="form-label">{label}</span>
      <input className="form-input" value={value} onChange={event => onChange(event.target.value)} placeholder={placeholder} />
    </label>
  );
}

function MetricGrid({ values }: { values: Record<string, unknown> }) {
  return (
    <div className="status-list">
      {Object.entries(values).map(([label, value]) => (
        <div className="status-row" key={label}>
          <span>{label}</span>
          <strong>{formatValue(value)}</strong>
        </div>
      ))}
    </div>
  );
}

function ChipList({ items }: { items: string[] }) {
  if (!items.length) return <div className="muted-note">No public metadata returned for this field.</div>;
  return <div className="platform-toggles">{items.map(item => <span key={item} className="platform-toggle">{item}</span>)}</div>;
}

function List({ items }: { items: string[] }) {
  if (!items.length) return <div className="muted-note">No public metadata returned for this field.</div>;
  return <div style={{ display: 'grid', gap: 8 }}>{items.map(item => <div key={item} className="small-capability">{item}</div>)}</div>;
}

function Limitations({ items }: { items: string[] }) {
  if (!items.length) return null;
  return (
    <div className="muted-note" style={{ marginTop: 8 }}>
      <strong>Limitations:</strong> {items.join(' ')}
    </div>
  );
}

function countForFilter(filter: Platform | 'all', candidates: TrendCandidate[]): number {
  if (filter === 'all') return candidates.length;
  if (filter === 'gt') return candidates.filter(c => c.source === 'google_trends_rss').length;
  return 0;
}

function dotForFilter(filter: Platform | 'all'): string {
  if (filter === 'all') return '#a78bfa';
  return PLATFORMS[filter]?.color ?? '#a78bfa';
}

function statusSubtitle(response: TrendDiscoveryResponse | null): string {
  if (!response) return 'No real trend data found';
  if (response.provider_status === 'provider_not_configured') return 'No trend provider configured';
  if (response.provider_status === 'provider_error') return 'Trend provider error';
  if (response.provider_status === 'no_data') return 'No real trend data found';
  if ((response.candidates?.length ?? 0) > 0) return 'Live Google Trends RSS data';
  return 'No real trend data found';
}

function statusLabel(status: string): string {
  return status.replaceAll('_', ' ');
}

function candidateFromVideo(result: YouTubeVideoAnalysisResponse, region: string, language: string): TrendCandidate {
  const keyword = result.extracted_keywords?.[0] || result.title || 'YouTube video analysis';
  return {
    id: `youtube-video-${result.video_id || Date.now()}`,
    source: 'youtube_data_api',
    region,
    language,
    keyword,
    title: result.title || keyword,
    score: result.metadata?.score ?? 50,
    discovered_at: new Date().toISOString(),
    source_url: result.metadata?.source_url,
    evidence: result.metadata?.score_reason || result.message,
    status: 'discovered',
  };
}

function candidateFromChannel(result: YouTubeChannelAnalysisResponse, idea: string, region: string, language: string): TrendCandidate {
  return {
    id: `youtube-channel-${result.channel_id || Date.now()}-${idea}`,
    source: 'youtube_data_api',
    region,
    language,
    keyword: idea,
    title: idea,
    score: result.metadata?.score ?? 50,
    discovered_at: new Date().toISOString(),
    source_url: result.metadata?.source_url,
    evidence: result.metadata?.score_reason || result.message,
    status: 'discovered',
  };
}

function formatValue(value: unknown): string {
  if (value == null || value === '') return 'Unavailable';
  if (typeof value === 'number') return Number.isInteger(value) ? value.toLocaleString() : value.toFixed(3);
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
}

function GeneratedPackageView({ pkg }: { pkg: ReelContentPackage }) {
  return (
    <div style={{ marginTop: 12, padding: 12, border: '1px solid var(--border-card)', borderRadius: 8, background: '#10141a', display: 'grid', gap: 10 }}>
      <div style={{ fontSize: 13, fontWeight: 800, color: 'var(--text-primary)', overflowWrap: 'anywhere' }}>{pkg.title}</div>
      <TextBlock label="Hook" value={pkg.hook} />
      <TextBlock label="Script" value={pkg.script} />
      <TextBlock label="Caption" value={pkg.caption} />
      {pkg.hashtags?.length > 0 && <TextBlock label="Hashtags" value={pkg.hashtags.join(' ')} />}
      <TextBlock label="Thumbnail brief" value={pkg.thumbnail_brief} />
      <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
        {pkg.provider_metadata.provider} · {pkg.provider_metadata.model} · {pkg.provider_metadata.source}
      </div>
    </div>
  );
}

function TextBlock({ label, value }: { label: string; value?: string }) {
  if (!value) return null;
  return (
    <div style={{ fontSize: 12, color: 'var(--text-muted)', overflowWrap: 'anywhere', lineHeight: 1.5, marginTop: 6 }}>
      <strong style={{ color: 'var(--text-primary)' }}>{label}:</strong> {value}
    </div>
  );
}

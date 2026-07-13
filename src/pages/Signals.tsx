import { useCallback, useEffect, useId, useMemo, useRef, useState, type CSSProperties, type KeyboardEvent, type MutableRefObject, type ReactNode, type RefObject } from 'react';
import type { Platform, View } from '../types';
import {
  ApiError,
  analyzeYouTubeChannel,
  analyzeYouTubeVideo,
  generateResearchScript,
  getTrendFilters,
  getResearchProviderStatus,
  researchNiches,
  searchTrendIntelligence,
  type ContentPillar,
  type CreatorNicheProfile,
  type NicheCandidate,
  type OutlierEvidence,
  type NicheReport,
  type ReelContentPackage,
  type ResearchScriptGenerationRequest,
  type ResearchScriptSourceType,
  type ResearchProviderStatus,
  type ScoreDimension,
  type VideoTopic,
  type PerformanceMetric,
  type TrendCandidate,
  type TrendDiscoveryResponse,
  type TrendFilterMetadata,
  type TrendIntelligenceResult,
  type TrendIntelligenceResponse,
  type YouTubeChannelAnalysisResponse,
  type YouTubeVideoAnalysisResponse,
} from '../lib/api/client';
import { formatLabel, formatMetric } from '../lib/metricFormat';
import { storage } from '../lib/storage';

type AIToolView = 'discoverTrends' | 'trendingKeywords' | 'youtubeVideoAnalyzer' | 'youtubeChannelAnalyzer' | 'nicheFinder';

const AI_TOOL_PAGE_META: Record<AIToolView, { title: string; eyebrow: string; description: string; action: string }> = {
  discoverTrends: {
    eyebrow: 'LIVE OPPORTUNITIES',
    title: 'Latest Trends',
    description: 'Browse current rising topics ranked by momentum, demand, competition, and opportunity.',
    action: 'Refresh',
  },
  trendingKeywords: {
    eyebrow: 'Keyword Research',
    title: 'Keyword Discovery',
    description: 'Discover, search, filter, and rank current content opportunities by market, language, niche, and recency.',
    action: 'Generate Script',
  },
  youtubeVideoAnalyzer: {
    eyebrow: 'YouTube research',
    title: 'YouTube Video Analyzer',
    description: 'Paste a YouTube video URL to review hooks, keywords, audience fit, and script angles.',
    action: 'Analyze Video',
  },
  youtubeChannelAnalyzer: {
    eyebrow: 'YouTube research',
    title: 'YouTube Channel Analyzer',
    description: 'Enter a channel URL, handle, or search term to review positioning, formats, and next-video ideas.',
    action: 'Analyze Channel',
  },
  nicheFinder: {
    eyebrow: 'Opportunity research',
    title: 'Niche Finder',
    description: 'Evaluate creator fit, demand, competition opportunity, sustainability, and differentiation.',
    action: 'Analyze niche opportunity',
  },
};

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
const TREND_MARKET_PREFS_KEY = 'trendcortex_trend_market_preferences';
const NICHE_LOADING_STAGES = [
  'Understanding creator profile',
  'Generating niche strategies',
  'Validating structured scores',
  'Checking available public evidence',
  'Building the content runway',
  'Preparing the dashboard',
];
const DEFAULT_TREND_MARKET_PREFS = {
  country: 'GB',
  language: 'en',
  windowValue: '24h',
  category: 'all',
};
type TrendMarketPrefs = typeof DEFAULT_TREND_MARKET_PREFS;

function getTrendMarketPrefs(): TrendMarketPrefs {
  try {
    const raw = localStorage.getItem(TREND_MARKET_PREFS_KEY);
    if (!raw) return DEFAULT_TREND_MARKET_PREFS;
    const parsed = JSON.parse(raw) as Partial<typeof DEFAULT_TREND_MARKET_PREFS>;
    return {
      country: parsed.country || DEFAULT_TREND_MARKET_PREFS.country,
      language: parsed.language || DEFAULT_TREND_MARKET_PREFS.language,
      windowValue: parsed.windowValue || DEFAULT_TREND_MARKET_PREFS.windowValue,
      category: parsed.category || DEFAULT_TREND_MARKET_PREFS.category,
    };
  } catch {
    return DEFAULT_TREND_MARKET_PREFS;
  }
}

function setTrendMarketPrefs(prefs: TrendMarketPrefs): void {
  try {
    localStorage.setItem(TREND_MARKET_PREFS_KEY, JSON.stringify(prefs));
  } catch {
    // localStorage might be unavailable in some environments
  }
}

interface Props {
  initialFilter?: Platform | 'all';
  onFilterChange?: (f: Platform | 'all') => void;
  onScriptGenerated?: (candidate: TrendCandidate, pkg: ReelContentPackage) => void;
  onOpenScriptStudio?: () => void;
  onManageDataSources?: () => void;
}

export function AIToolsLandingPage({ onNavigate }: { onNavigate: (view: View) => void }) {
  return (
    <section className="page-section">
      <div className="page-hero compact">
        <div>
          <div className="page-eyebrow">Research</div>
          <h1>Find trends, channels, and niches worth building around.</h1>
          <p>Choose a workflow, review the evidence, and turn strong trends into scripts or content plans.</p>
        </div>
      </div>
      <div className="ai-tools-grid">
        {(Object.keys(AI_TOOL_PAGE_META) as AIToolView[]).map(view => {
          const meta = AI_TOOL_PAGE_META[view];
          return (
            <button key={view} className="ai-tool-card" type="button" onClick={() => onNavigate(view)}>
              <span>{meta.eyebrow}</span>
              <strong>{meta.title}</strong>
              <small>{meta.description}</small>
            </button>
          );
        })}
      </div>
    </section>
  );
}

export function AIToolPage({ tool, initialFilter = 'all', onFilterChange, onScriptGenerated, onOpenScriptStudio, onManageDataSources }: Props & { tool: AIToolView }) {
  const [platformFilter, setPlatformFilter] = useState<Platform | 'all'>(initialFilter);
  const [regionChoice, setRegionChoice] = useState('US');
  const [customRegion, setCustomRegion] = useState('');
  const [languageChoice, setLanguageChoice] = useState('en-US');
  const [customLanguage, setCustomLanguage] = useState('');
  const [audience, setAudience] = useState('Global');
  const [customAudience, setCustomAudience] = useState('');
  const response: TrendDiscoveryResponse | null = null;
  const [providers, setProviders] = useState<ResearchProviderStatus[]>([]);
  const loading = false;
  const error: string | null = null;
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

  const filteredCandidates = useMemo(() => {
    void platformFilter;
    return [] as TrendCandidate[];
  }, [platformFilter]);

  function setFilter(next: Platform | 'all') {
    setPlatformFilter(next);
    onFilterChange?.(next);
  }

  function handleGenerate(candidate: TrendCandidate, sourceType: ResearchScriptSourceType = 'google_trend') {
    const generationKey = sourceType === 'niche_idea' ? `niche-${candidate.id}` : candidate.id;
    setGeneratingID(generationKey);
    setGenerationErrors(prev => {
      const next = { ...prev };
      delete next[generationKey];
      return next;
    });
    generateResearchScript({
      source_type: sourceType,
      source_id: candidate.id,
      source_url: candidate.source_url,
      topic: candidate.keyword,
      title: candidate.title || candidate.keyword,
      summary: candidate.evidence,
      keywords: [candidate.keyword].filter(Boolean),
      suggested_angle: sourceType === 'niche_idea' ? `For ${audienceText || 'Global'}: turn this into a focused niche creator video.` : undefined,
      target_platforms: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
      content_style: 'Short-form creator script',
      duration_seconds: 30,
      language: candidate.language || language || 'en-US',
      region: candidate.region || region || 'US',
      evidence: {
        score: candidate.score,
        source: candidate.source,
        evidence: candidate.evidence,
      },
      metadata: {
        score: candidate.score,
        source_provider: candidate.source,
        source_url: candidate.source_url,
      },
    })
      .then(data => {
        const storedCandidate = { ...candidate, source: sourceType };
        setGenerated(prev => ({ ...prev, [generationKey]: data.package }));
        onScriptGenerated?.(storedCandidate, data.package);
      })
      .catch(err => {
        setGenerationErrors(prev => ({ ...prev, [generationKey]: err instanceof ApiError ? err.message : 'Script generation request failed.' }));
      })
      .finally(() => setGeneratingID(current => (current === generationKey ? null : current)));
  }

  function handleGenerateResearch(generationKey: string, candidate: TrendCandidate, body: ResearchScriptGenerationRequest) {
    setGeneratingID(generationKey);
    setGenerationErrors(prev => {
      const next = { ...prev };
      delete next[generationKey];
      return next;
    });
    generateResearchScript(body)
      .then(data => {
        setGenerated(prev => ({ ...prev, [generationKey]: data.package }));
        onScriptGenerated?.(candidate, data.package);
      })
      .catch(err => {
        setGenerationErrors(prev => ({ ...prev, [generationKey]: err instanceof ApiError ? err.message : 'Script generation request failed.' }));
      })
      .finally(() => setGeneratingID(current => (current === generationKey ? null : current)));
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
    const candidate = candidateFromVideo(videoResult, region || 'US', language || 'en-US');
    handleGenerateResearch(candidate.id, candidate, researchScriptFromVideo(videoResult, region || 'US', language || 'en-US'));
  }

  function generateFromChannelIdea(idea: string) {
    if (!channelResult || channelResult.status !== 'ok') return;
    const candidate = candidateFromChannel(channelResult, idea, region || 'US', language || 'en-US');
    handleGenerateResearch(candidate.id, candidate, researchScriptFromChannelIdea(channelResult, idea, region || 'US', language || 'en-US'));
  }

  function generateFromNicheCandidate(candidate: NicheCandidate) {
    const key = nicheCandidateKey(candidate);
    const trendCandidate = trendCandidateFromNicheCandidate(candidate, region || 'GB', language || 'en');
    handleGenerateResearch(key, trendCandidate, researchScriptFromNicheCandidate(candidate, region || 'GB', language || 'en'));
  }

  const meta = AI_TOOL_PAGE_META[tool];
  const analyzerTool = tool === 'youtubeVideoAnalyzer' || tool === 'youtubeChannelAnalyzer';
  const videoAnalysisReady = tool === 'youtubeVideoAnalyzer' && videoResult?.status === 'ok';

  return (
    <section className={`page-section${analyzerTool ? ' analyzer-page' : ''}${videoAnalysisReady ? ' video-analysis-ready-page' : ''}`}>
      <div className={analyzerTool ? 'analyzer-workspace' : undefined}>
        <div className="page-hero compact">
          <div>
            <div className="page-eyebrow">{meta.eyebrow}</div>
            <h1>{meta.title}</h1>
            <p>{meta.description}</p>
          </div>
        </div>

        {tool === 'discoverTrends' && (
          <DiscoverTrendsTab
            generated={generated}
            generationErrors={generationErrors}
            generatingID={generatingID}
            onGenerate={handleGenerate}
            onOpenScriptStudio={onOpenScriptStudio}
          />
        )}

        {tool === 'trendingKeywords' && (
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
            onOpenScriptStudio={onOpenScriptStudio}
            onManageDataSources={onManageDataSources}
          />
        )}

        {tool === 'youtubeVideoAnalyzer' && (
          <YouTubeVideoTab
            value={videoURL}
            onChange={setVideoURL}
            onAnalyze={analyzeVideo}
            loading={videoLoading}
            result={videoResult}
            onGenerate={generateFromVideo}
            generationKey={videoResult?.status === 'ok' ? candidateFromVideo(videoResult, region || 'US', language || 'en-US').id : null}
            generated={generated}
            generationErrors={generationErrors}
            generatingID={generatingID}
            onOpenScriptStudio={onOpenScriptStudio}
          />
        )}

        {tool === 'youtubeChannelAnalyzer' && (
          <YouTubeChannelTab
            value={channelURL}
            onChange={setChannelURL}
            onAnalyze={analyzeChannel}
            loading={channelLoading}
            result={channelResult}
            onGenerateIdea={generateFromChannelIdea}
            generated={generated}
            generationErrors={generationErrors}
            generatingID={generatingID}
            region={region || 'US'}
            language={language || 'en-US'}
            onOpenScriptStudio={onOpenScriptStudio}
          />
        )}

        {tool === 'nicheFinder' && (
          <NicheFinderTab
            providers={providers}
            region={region || 'US'}
            language={language || 'en-US'}
            audience={audienceText || 'Global'}
            onGenerate={generateFromNicheCandidate}
            generated={generated}
            generationErrors={generationErrors}
            generatingID={generatingID}
            onOpenScriptStudio={onOpenScriptStudio}
          />
        )}
      </div>
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
  onOpenScriptStudio?: () => void;
  onManageDataSources?: () => void;
}) {
  const initialPrefs = useMemo(() => getTrendMarketPrefs(), []);
  const [filters, setFilters] = useState<TrendFilterMetadata | null>(null);
  const [query, setQuery] = useState('');
  const [country, setCountry] = useState(initialPrefs.country);
  const [language, setLanguage] = useState(initialPrefs.language);
  const [windowValue, setWindowValue] = useState(initialPrefs.windowValue);
  const [category, setCategory] = useState(initialPrefs.category);
  const [postcode, setPostcode] = useState('');
  const [includeWords, setIncludeWords] = useState('');
  const [excludeWords, setExcludeWords] = useState('');
  const [exactPhrase, setExactPhrase] = useState('');
  const [customNiche, setCustomNiche] = useState('');
  const [localOnly, setLocalOnly] = useState(false);
  const [radius, setRadius] = useState(25);
  const [videoDuration, setVideoDuration] = useState('any');
  const [minViews, setMinViews] = useState('');
  const [maxCompetition, setMaxCompetition] = useState('');
  const [minOpportunity, setMinOpportunity] = useState('');
  const [sort, setSort] = useState('opportunity');
  const [showMore, setShowMore] = useState(false);
  const [trendResponse, setTrendResponse] = useState<TrendIntelligenceResponse | null>(null);
  const [trendLoading, setTrendLoading] = useState(false);
  const [trendError, setTrendError] = useState<string | null>(null);
  const [validationMessage, setValidationMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getTrendFilters()
      .then(data => {
        if (cancelled) return;
        setFilters(data);
      })
      .catch(() => {
        if (!cancelled) setFilters(null);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setTrendMarketPrefs({ country, language, windowValue, category });
  }, [category, country, language, windowValue]);

  function runSearch() {
    const trimmedQuery = query.trim();
    if (!trimmedQuery && !exactPhrase.trim()) {
      setValidationMessage('Enter a keyword before searching.');
      return;
    }
    setTrendLoading(true);
    setTrendError(null);
    setValidationMessage(null);
    searchTrendIntelligence({
      q: trimmedQuery,
      country,
      language,
      window: windowValue,
      category,
      niche: customNiche,
      postcode,
      local_videos_only: localOnly,
      radius_km: radius,
      include_words: includeWords,
      exclude_words: excludeWords,
      exact_phrase: exactPhrase,
      video_duration: videoDuration,
      min_views: minViews,
      max_competition: maxCompetition,
      min_opportunity: minOpportunity,
      sort,
      limit: 20,
    })
      .then(data => {
        setTrendResponse(data);
        storage.updateActivity(current => ({
          ...current,
          trendsFoundToday: data.results.length,
          latestTrendPulled: new Date().toISOString(),
        }));
      })
      .catch(() => setTrendError('Trend intelligence is temporarily unavailable.'))
      .finally(() => setTrendLoading(false));
  }

  function handleSearchFormKeyDown(event: KeyboardEvent<HTMLFormElement>) {
    if (event.key !== 'Enter') return;
    event.preventDefault();
    runSearch();
  }

  const countryOptions = filters?.countries ?? REGION_OPTIONS.filter(option => option.value !== 'custom').map(option => ({ label: option.label === 'UK' ? 'United Kingdom' : option.label, value: option.value }));
  const languageOptions = filters?.languages ?? LANGUAGE_OPTIONS.filter(option => option.value !== 'custom').map(option => ({ label: option.label, value: option.value.split('-')[0] }));
  const timeOptions = filters?.time_windows ?? [{ label: 'Last 24 hours', value: '24h' }];
  const categoryOptions = filters?.categories ?? [{ label: 'All', value: 'all' }];
  const durationOptions = filters?.video_durations ?? [{ label: 'Any duration', value: 'any' }];
  const sortOptions = filters?.sort_orders ?? [{ label: 'Best opportunity', value: 'opportunity' }];

  return (
    <>
      <div className="settings-card research-filter-card">
        <div className="settings-card-title">Search & Filters</div>
        <form onSubmit={event => { event.preventDefault(); runSearch(); }} onKeyDown={handleSearchFormKeyDown}>
          <div className="form-grid four">
            <TextInput label="Search keyword" value={query} onChange={setQuery} placeholder="artificial intelligence, football highlights" />
            <Select label="Country" value={country} onChange={setCountry} options={countryOptions} />
            <Select label="Language" value={language} onChange={setLanguage} options={languageOptions} />
            <TextInput label="Postcode or location" value={postcode} onChange={setPostcode} placeholder="Optional" />
            <Select label="Time window" value={windowValue} onChange={setWindowValue} options={timeOptions} />
            <Select label="Category" value={category} onChange={setCategory} options={categoryOptions} />
            <Select label="Sort" value={sort} onChange={setSort} options={sortOptions} />
            <TextInput label="Exclude words" value={excludeWords} onChange={setExcludeWords} placeholder="comma separated" />
          </div>
          {showMore && (
            <div className="form-grid four" style={{ marginTop: 12 }}>
              <TextInput label="Include words" value={includeWords} onChange={setIncludeWords} placeholder="comma separated" />
              <TextInput label="Exact phrase" value={exactPhrase} onChange={setExactPhrase} placeholder="Optional" />
              <TextInput label="Custom niche" value={customNiche} onChange={setCustomNiche} placeholder="Optional" />
              <Select label="Video duration" value={videoDuration} onChange={setVideoDuration} options={durationOptions} />
              <TextInput label="Minimum views" value={minViews} onChange={setMinViews} placeholder="Optional" />
              <TextInput label="Max competition" value={maxCompetition} onChange={setMaxCompetition} placeholder="0-100" />
              <TextInput label="Minimum opportunity" value={minOpportunity} onChange={setMinOpportunity} placeholder="0-100" />
              <Select label="Local radius" value={String(radius)} onChange={value => setRadius(Number(value))} options={(filters?.local_radii_km ?? [10, 25, 50, 100]).map(value => ({ label: `${value} km`, value: String(value) }))} />
              <label className="checkbox-row">
                <input type="checkbox" checked={localOnly} onChange={event => setLocalOnly(event.target.checked)} />
                <span>Local videos only</span>
              </label>
            </div>
          )}
          <div className="trend-filter-footer">
            <span>{trendResponse?.resolved_location?.status === 'resolved' ? `Resolved locality: ${trendResponse.resolved_location.city || trendResponse.resolved_location.region || trendResponse.resolved_location.input}` : 'Enter a keyword to search content opportunities.'}</span>
            <button type="button" className="link-button" onClick={() => setShowMore(value => !value)}>More Filters</button>
          </div>
          <div className="trend-search-actions">
            <button type="submit" className="generate-btn idle trend-search-button" disabled={trendLoading || (!query.trim() && !exactPhrase.trim())}>Search</button>
          </div>
          {validationMessage && <div className="inline-error centered" role="alert">{validationMessage}</div>}
        </form>
        {localOnly && <div className="neutral-callout">Local videos only matches supporting videos with available geographic metadata. Country-level trend geography is still used.</div>}
      </div>

      {trendLoading && <EmptyState tone="loading" title="Loading keyword search." desc="Checking current demand, recency, and public engagement signals." />}
      {!trendLoading && trendError && <EmptyState tone="error" title="Keyword search is temporarily unavailable." desc={trendError} />}
      {!trendLoading && !trendError && !trendResponse && <EmptyState tone="empty" title="Enter a keyword to search content opportunities." desc="Choose country, language, time window, and niche filters to rank opportunities." />}
      {!trendLoading && !trendError && trendResponse?.message && trendResponse.results.length === 0 && (
        <EmptyState tone="empty" title={trendResponse.message} desc="Try a broader country, language, or time window." />
      )}

      {!trendLoading && !trendError && trendResponse && trendResponse.results.length > 0 && (
        <TrendOpportunityList
          results={trendResponse.results}
          generated={props.generated}
          generationErrors={props.generationErrors}
          generatingID={props.generatingID}
          onGenerate={result => props.onGenerate(candidateFromTrendIntelligence(result, country, language))}
          onOpenScriptStudio={props.onOpenScriptStudio}
        />
      )}
    </>
  );
}

function DiscoverTrendsTab(props: {
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onGenerate: (candidate: TrendCandidate) => void;
  onOpenScriptStudio?: () => void;
}) {
  const initialPrefs = useMemo(() => getTrendMarketPrefs(), []);
  const [filters, setFilters] = useState<TrendFilterMetadata | null>(null);
  const [country, setCountry] = useState(initialPrefs.country);
  const [language, setLanguage] = useState(initialPrefs.language);
  const [windowValue, setWindowValue] = useState(initialPrefs.windowValue);
  const [category, setCategory] = useState(initialPrefs.category);
  const [trendResponse, setTrendResponse] = useState<TrendIntelligenceResponse | null>(null);
  const [trendLoading, setTrendLoading] = useState(false);
  const [trendError, setTrendError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getTrendFilters()
      .then(data => {
        if (!cancelled) setFilters(data);
      })
      .catch(() => {
        if (!cancelled) setFilters(null);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setTrendMarketPrefs({ country, language, windowValue, category });
  }, [category, country, language, windowValue]);

  const requestDiscovery = useCallback((values: TrendMarketPrefs, reason: 'auto' | 'manual') => {
    void reason;
    const controller = new AbortController();
    setTrendLoading(true);
    setTrendError(null);
    searchTrendIntelligence({
      country: values.country,
      language: values.language,
      window: values.windowValue,
      category: values.category,
      sort: 'opportunity',
      limit: 20,
    }, { signal: controller.signal })
      .then(data => {
        if (controller.signal.aborted) return;
        setTrendResponse(data);
        storage.updateActivity(current => ({
          ...current,
          trendsFoundToday: data.results.length,
          latestTrendPulled: new Date().toISOString(),
        }));
      })
      .catch(err => {
        if (controller.signal.aborted) return;
        setTrendError(err instanceof Error ? err.message : 'Trend intelligence is temporarily unavailable.');
      })
      .finally(() => {
        if (!controller.signal.aborted) setTrendLoading(false);
      });
    return controller;
  }, []);

  useEffect(() => {
    const controller = requestDiscovery(initialPrefs, 'auto');
    return () => controller?.abort();
  }, [initialPrefs, requestDiscovery]);

  const countryOptions = filters?.countries ?? REGION_OPTIONS.filter(option => option.value !== 'custom').map(option => ({ label: option.label === 'UK' ? 'United Kingdom' : option.label, value: option.value }));
  const languageOptions = filters?.languages ?? LANGUAGE_OPTIONS.filter(option => option.value !== 'custom').map(option => ({ label: option.label, value: option.value.split('-')[0] }));
  const timeOptions = filters?.time_windows ?? [{ label: 'Last 24 hours', value: '24h' }];
  const categoryOptions = filters?.categories ?? [{ label: 'All', value: 'all' }];

  return (
    <>
      <div className="settings-card research-filter-card discover-filter-card">
        <div className="settings-card-title">Market Filters</div>
        <div className="form-grid four">
          <Select label="Country" value={country} onChange={setCountry} options={countryOptions} />
          <Select label="Language" value={language} onChange={setLanguage} options={languageOptions} />
          <Select label="Time window" value={windowValue} onChange={setWindowValue} options={timeOptions} />
          <Select label="Category" value={category} onChange={setCategory} options={categoryOptions} />
        </div>
        <div className="discover-filter-actions">
          <button
            type="button"
            className="generate-btn idle secondary discover-refresh-button"
            onClick={() => requestDiscovery({ country, language, windowValue, category }, 'manual')}
            disabled={trendLoading}
          >
            Refresh
          </button>
        </div>
      </div>

      {trendLoading && <EmptyState tone="loading" title="Loading trend intelligence." desc="Checking current demand, recency, and public engagement signals." />}
      {!trendLoading && trendError && <EmptyState tone="error" title="Trend intelligence is temporarily unavailable." desc={trendError} />}
      {!trendLoading && !trendError && trendResponse?.message && trendResponse.results.length === 0 && (
        <EmptyState tone="empty" title={trendResponse.message} desc="Try a broader country, language, or time window." />
      )}
      {!trendLoading && !trendError && trendResponse && trendResponse.results.length === 0 && !trendResponse.message && (
        <EmptyState tone="empty" title="No trends found." desc="Try a broader country, language, or time window." />
      )}
      {!trendLoading && !trendError && trendResponse && trendResponse.results.length > 0 && (
        <TrendOpportunityList
          results={trendResponse.results}
          generated={props.generated}
          generationErrors={props.generationErrors}
          generatingID={props.generatingID}
          onGenerate={result => props.onGenerate(candidateFromTrendIntelligence(result, country, language))}
          onOpenScriptStudio={props.onOpenScriptStudio}
        />
      )}
    </>
  );
}

function TrendOpportunityList({ results, generated, generationErrors, generatingID, onGenerate, onOpenScriptStudio }: {
  results: TrendIntelligenceResult[];
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onGenerate: (result: TrendIntelligenceResult) => void;
  onOpenScriptStudio?: () => void;
}) {
  const [expandedResultId, setExpandedResultId] = useState<string | null>(null);
  const resultIdsKey = useMemo(() => results.map(result => result.id).join('|'), [results]);

  useEffect(() => {
    setExpandedResultId(results[0]?.id ?? null);
  }, [resultIdsKey, results]);

  return (
    <div className="trend-opportunity-list">
      {results.map(result => (
        <TrendOpportunityCard
          key={result.id}
          result={result}
          expanded={expandedResultId === result.id}
          onToggle={() => setExpandedResultId(current => current === result.id ? null : result.id)}
          generated={generated[result.id]}
          generationError={generationErrors[result.id]}
          generating={generatingID === result.id}
          onGenerate={() => onGenerate(result)}
          onOpenScriptStudio={onOpenScriptStudio}
        />
      ))}
    </div>
  );
}

function YouTubeVideoTab({ value, onChange, onAnalyze, loading, result, onGenerate, generationKey, generated, generationErrors, generatingID, onOpenScriptStudio }: {
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  result: YouTubeVideoAnalysisResponse | null;
  onGenerate: () => void;
  generationKey: string | null;
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onOpenScriptStudio?: () => void;
}) {
  const generatedPackage = generationKey ? generated[generationKey] : undefined;
  const generationError = generationKey ? generationErrors[generationKey] : undefined;
  const generating = Boolean(generationKey && generatingID === generationKey);
  return (
    <AnalyzerShell
      title="YouTube Video Analyzer"
      description="Paste a YouTube video URL to review the title, hook, keywords, audience fit, and remake angles."
      inputLabel="YouTube video URL"
      value={value}
      onChange={onChange}
      onAnalyze={onAnalyze}
      loading={loading}
      buttonLabel="Analyze Video"
      compactResult={result?.status === 'ok'}
    >
      {!result && <div className="neutral-callout">Connect YouTube in Connections to analyze videos.</div>}
      {loading && !result && <VideoAnalyzerLoading />}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <VideoAnalysisResultView
          result={result}
          generating={generating}
          generated={Boolean(generatedPackage)}
          generationError={generationError}
          onGenerate={onGenerate}
          onOpenScriptStudio={onOpenScriptStudio}
        />
      )}
    </AnalyzerShell>
  );
}

function VideoAnalysisResultView({ result, generating, generated, generationError, onGenerate, onOpenScriptStudio }: {
  result: YouTubeVideoAnalysisResponse;
  generating: boolean;
  generated: boolean;
  generationError?: string;
  onGenerate: () => void;
  onOpenScriptStudio?: () => void;
}) {
  return (
    <div className="video-analysis-results launch-video-analysis">
      <VideoAnalysisHero result={result} onGenerate={onGenerate} generating={generating} />
      <ExecutiveTakeaways result={result} />
      <PerformanceStory result={result} />
      <RevenuePotentialCard estimate={result.revenue_estimate} />
      <TopicIntelligence result={result} />
      <HookPackagingLab result={result} />
      <CreativeDirectionTabs result={result} onGenerate={onGenerate} generating={generating} />
      <ScriptGenerationPanel
        result={result}
        generating={generating}
        generated={generated}
        error={generationError}
        onGenerate={onGenerate}
        onOpenScriptStudio={onOpenScriptStudio}
      />
      <AnalysisDetailsAccordion result={result} />
    </div>
  );
}

function VideoAnalysisHero({ result, onGenerate, generating }: { result: YouTubeVideoAnalysisResponse; onGenerate: () => void; generating: boolean }) {
  const score = summaryOpportunityScore(result);
  const topic = videoCoreTopic(result);
  const thumbnail = result.thumbnail_url;
  const [imageFailed, setImageFailed] = useState(false);
  const sourceURL = result.metadata?.source_url || result.video_url;
  return (
    <section className="video-launch-hero" aria-labelledby="video-analysis-title">
      <div className="video-hero-media">
        {thumbnail && !imageFailed ? (
          <img src={thumbnail} alt={`Thumbnail for ${result.title || 'analysed video'}`} loading="eager" onError={() => setImageFailed(true)} />
        ) : (
          <div className="video-thumbnail-fallback" role="img" aria-label="YouTube thumbnail unavailable">
            <span>{initialsForTitle(result.title || 'Video')}</span>
          </div>
        )}
        {sourceURL && <a className="video-play-link" href={sourceURL} target="_blank" rel="noopener noreferrer" aria-label="Open source video">Open source</a>}
      </div>
      <div className="video-hero-content">
        <div className="video-hero-kicker">
          <span>{formatVideoFormat(result.formatted_metadata?.format)}</span>
          {topic.category && <span>{topic.category}</span>}
          {topic.main && <span>{topic.main}</span>}
        </div>
        <h2 id="video-analysis-title">{result.title || 'YouTube video analysis'}</h2>
        <div className="video-source-line">
          <strong>{result.channel_title || 'Channel unavailable'}</strong>
          <span>{[result.formatted_metadata?.published_date || formatDateLabel(result.published_at), result.formatted_metadata?.published_relative, result.formatted_metadata?.duration || result.duration].filter(Boolean).join(' · ') || 'Public metadata'}</span>
        </div>
        <div className="video-hero-metrics">
          <OpportunityScoreRing score={score} label={ratingForScore(score)} />
          <div className="video-confidence-card">
            <span>Confidence</span>
            <strong>{result.analysis_confidence ? `${clampScore(result.analysis_confidence.score)}%` : 'Unavailable'}</strong>
            <small>{result.analysis_confidence?.rating || 'Public metadata scale'}</small>
          </div>
          <div className="video-revenue-card">
            <span>Revenue potential</span>
            <strong>{displayCompactRevenueRange(result.revenue_estimate)}</strong>
            <small>{result.revenue_estimate?.confidence ? `${result.revenue_estimate.confidence} confidence` : 'Public estimate'}</small>
          </div>
        </div>
        <p>{analysisSummary(result)}</p>
        <div className="video-hero-actions">
          <button className="generate-btn idle" type="button" onClick={onGenerate} disabled={generating}>{generating ? 'Generating...' : 'Generate script'}</button>
          {sourceURL && <a className="link-button" href={sourceURL} target="_blank" rel="noopener noreferrer">Open source video</a>}
        </div>
      </div>
    </section>
  );
}

function OpportunityScoreRing({ score, label }: { score: number; label: string }) {
  const background = `conic-gradient(from -90deg, #42d392 ${score * 3.6}deg, rgba(255,255,255,0.12) 0deg)`;
  return (
    <div className="opportunity-score-ring" style={{ '--score-ring': background } as CSSProperties} role="img" aria-label={`Opportunity score ${score} out of 100, ${label}`}>
      <div>
        <span>Opportunity</span>
        <strong>{score}</strong>
        <small>{label}</small>
      </div>
    </div>
  );
}

function ExecutiveTakeaways({ result }: { result: YouTubeVideoAnalysisResponse }) {
  const insight = strongestDimension(result.score_dimensions ?? []);
  const weak = weakestDimension(result.score_dimensions ?? []);
  const opportunity = firstUseful(result.creator_opportunities?.suggested_remake_angles, result.suggested_remake_angles, [result.niche_analysis?.inferred_content_angle, result.inferred_content_angle]);
  return (
    <section className="executive-takeaways" aria-label="Executive takeaways">
      <ExecutiveInsightCard tone="good" icon="W" title="Why it works" text={concise(insight?.explanation || result.hook_intelligence?.explanation || result.message)} tag={insight ? `${insight.label} ${clampScore(insight.score)}` : ratingForScore(summaryOpportunityScore(result))} />
      <ExecutiveInsightCard tone="risk" icon="R" title="Biggest weakness" text={concise(weak?.explanation || hookWeakness(result))} tag={weak ? `${weak.label} ${clampScore(weak.score)}` : 'Packaging'} />
      <ExecutiveInsightCard tone="creative" icon="C" title="Best creator opportunity" text={concise(opportunity || topicOpportunity(result))} tag={videoAudience(result)} />
    </section>
  );
}

function ExecutiveInsightCard({ tone, icon, title, text, tag }: { tone: string; icon: string; title: string; text: string; tag: string }) {
  return (
    <article className={`executive-insight-card is-${tone}`}>
      <span className="insight-icon" aria-hidden="true">{icon}</span>
      <div>
        <h3>{title}</h3>
        <p>{text}</p>
        <small>{tag}</small>
      </div>
    </article>
  );
}

function PerformanceStory({ result }: { result: YouTubeVideoAnalysisResponse }) {
  const dimensions = videoScoreGroups(result);
  const metrics = result.performance_profile ?? [];
  return (
    <section className="video-section performance-story" aria-labelledby="performance-story-title">
      <div className="video-section-heading">
        <span>Performance story</span>
        <h3 id="performance-story-title">What the public signals say</h3>
      </div>
      <div className="performance-story-grid">
        <div className="score-profile-panel">
          {dimensions.map(dimension => <ScoreDimensionRow key={dimension.id} dimension={dimension} />)}
        </div>
        <div className="public-signal-panel">
          <div className="video-chart-note">TrendCortex public-signal scale</div>
          <div className="public-signal-grid">
            {metrics.length ? metrics.slice(0, 4).map(metric => <PublicSignalMetric key={metric.id || metric.label} metric={metric} />) : <div className="muted-note">Public performance signals are unavailable for this video.</div>}
          </div>
        </div>
      </div>
    </section>
  );
}

function ScoreDimensionRow({ dimension }: { dimension: ScoreDimension }) {
  const score = clampScore(dimension.score);
  return (
    <div className="score-dimension-row" aria-label={`${dimension.label}: ${score} out of 100, ${dimension.rating}`}>
      <div className="score-dimension-main">
        <span>{dimension.label}</span>
        <strong>{score}</strong>
      </div>
      <div className="score-dimension-track" aria-hidden="true"><span style={{ width: `${score}%` }} /></div>
      <p>{concise(dimension.explanation)}</p>
    </div>
  );
}

function PublicSignalMetric({ metric }: { metric: PerformanceMetric }) {
  const score = clampScore(metric.score);
  return (
    <div className="public-signal-metric">
      <span>{metric.label}</span>
      <strong>{metric.value}</strong>
      <div className="mini-strength-track" aria-hidden="true"><span style={{ width: `${score}%` }} /></div>
      <small>{concise(metric.explanation, 92)}</small>
    </div>
  );
}

function RevenuePotentialCard({ estimate }: { estimate?: YouTubeVideoAnalysisResponse['revenue_estimate'] }) {
  if (!estimate) {
    return (
      <section className="video-section revenue-potential-card">
        <div className="video-section-heading"><span>Revenue potential</span><h3>Estimated range unavailable</h3></div>
        <p className="muted-note">The public metadata returned for this video is not enough to create a revenue estimate.</p>
      </section>
    );
  }
  const midpointPercent = estimate.high > estimate.low ? ((estimate.midpoint - estimate.low) / (estimate.high - estimate.low)) * 100 : 50;
  return (
    <section className="video-section revenue-potential-card" aria-labelledby="revenue-title">
      <div className="video-section-heading">
        <span>Revenue potential</span>
        <h3 id="revenue-title">{displayRevenueRange(estimate)}</h3>
      </div>
      <div className="revenue-premium-grid">
        <div className="revenue-range-visual">
          <div className="revenue-range-track premium" role="img" aria-label={`Low ${formatCurrencyEstimate(estimate.low)}, midpoint ${formatCurrencyEstimate(estimate.midpoint)}, high ${formatCurrencyEstimate(estimate.high)}`}>
            <span className="revenue-range-fill" />
            <span className="revenue-range-midpoint" style={{ left: `${clampScore(midpointPercent)}%` }} />
          </div>
          <div className="revenue-range-labels">
            <span>Low {formatCurrencyEstimate(estimate.low)}</span>
            <span>Mid {formatCurrencyEstimate(estimate.midpoint)}</span>
            <span>High {formatCurrencyEstimate(estimate.high)}</span>
          </div>
        </div>
        <div className="revenue-facts">
          <div><span>RPM range</span><strong>{estimate.estimated_revenue_per_1000_views}</strong></div>
          <div><span>Confidence</span><strong>{estimate.confidence}</strong></div>
          <div><span>Model</span><strong>{humanizeLabel(estimate.model_type || estimate.source)}</strong></div>
        </div>
      </div>
      <p>{concise(estimate.calculation_basis, 140)}</p>
      {estimate.future_revenue_scenario && <div className="future-scenario">{concise(estimate.future_revenue_scenario, 130)}</div>}
      <details className="video-details-accordion">
        <summary>How this estimate was calculated</summary>
        <SectionList label="Assumptions" items={estimate.assumptions ?? []} />
        <SectionList label="Exclusions" items={estimate.exclusions ?? []} />
        <MetricGrid values={{ Source: humanizeLabel(estimate.source), 'Model type': humanizeLabel(estimate.model_type), 'Exact analytics unavailable': estimate.actual_analytics_unavailable ? 'Yes' : undefined, 'Monetisation eligibility': estimate.monetisation_eligibility }} />
      </details>
    </section>
  );
}

function TopicIntelligence({ result }: { result: YouTubeVideoAnalysisResponse }) {
  const topic = videoCoreTopic(result);
  const themes = uniqueStrings([...(result.keyword_intelligence?.primary_topics ?? []), ...(result.keyword_intelligence?.supporting_terms ?? []), ...(result.keyword_intelligence?.secondary_keywords ?? [])]).slice(0, 12);
  const search = uniqueStrings([...(result.keyword_intelligence?.search_phrases ?? []), ...(result.keyword_intelligence?.long_tail_phrases ?? [])]).slice(0, 8);
  return (
    <section className="video-section topic-intelligence" aria-labelledby="topic-title">
      <div className="video-section-heading"><span>Topic intelligence</span><h3 id="topic-title">{topic.main}</h3></div>
      <div className="topic-intel-grid">
        <div className="core-topic-card">
          <span>Core topic</span>
          <strong>{topic.specific}</strong>
          <div className="topic-meta-grid">
            <small>{topic.category}</small>
            <small>{topic.niche}</small>
            <small>{videoAudience(result)}</small>
            <small>{topic.format}</small>
          </div>
        </div>
        <TopicCluster core={topic.main} themes={themes.slice(0, 6)} />
      </div>
      <LabelledChips label="Supporting themes" items={themes.slice(0, 10)} />
      <div className="search-opportunity-list">
        {search.slice(0, 5).map((phrase, index) => <div key={phrase} className="search-opportunity-row"><span>{index + 1}</span><strong>{phrase}</strong></div>)}
      </div>
      {search.length > 5 && (
        <details className="video-details-accordion">
          <summary>Show more search opportunities</summary>
          <ChipList items={search.slice(5)} />
        </details>
      )}
    </section>
  );
}

function TopicCluster({ core, themes }: { core: string; themes: string[] }) {
  return (
    <div className="topic-cluster" aria-label={`Topic cluster centred on ${core}`}>
      <strong>{core}</strong>
      {themes.map((theme, index) => <span key={theme} className={`topic-node node-${index + 1}`}>{theme}</span>)}
    </div>
  );
}

function HookPackagingLab({ result }: { result: YouTubeVideoAnalysisResponse }) {
  const hook = result.hook_intelligence;
  const rows = [
    { id: 'clarity', label: 'Clarity', score: hook?.clarity_score ?? 0 },
    { id: 'specificity', label: 'Specificity', score: hook?.specificity_score ?? 0 },
    { id: 'curiosity', label: 'Curiosity', score: hook?.curiosity_score ?? 0 },
    { id: 'audience', label: 'Audience signal', score: hook?.audience_signal_score ?? 0 },
    { id: 'value', label: 'Value promise', score: hook?.value_promise_score ?? 0 },
  ];
  return (
    <section className="video-section hook-packaging-lab" aria-labelledby="hook-title">
      <div className="video-section-heading"><span>Hook & packaging lab</span><h3 id="hook-title">{humanizeLabel(hook?.title_pattern || hook?.hook_type || 'Title packaging')}</h3></div>
      <div className="hook-lab-grid">
        <div className="hook-score-strips">
          {rows.map(row => <ScoreDimensionRow key={row.id} dimension={{ id: row.id, label: row.label, score: row.score, rating: ratingForScore(row.score), explanation: hookExplanation(row.id) }} />)}
        </div>
        <div className="hook-copy-panel">
          <div><span>What the title does well</span><p>{concise(hook?.explanation || result.hook_analysis || result.title_structure_analysis || 'The public title gives enough signal to evaluate packaging.')}</p></div>
          <div><span>What weakens it</span><p>{concise(hookWeakness(result), 120)}</p></div>
          <div><span>Suggested improvement</span><p>{improvedTitle(result)}</p></div>
        </div>
      </div>
      <details className="video-details-accordion">
        <summary>Title score methodology</summary>
        <HookProfile hook={hook} />
      </details>
    </section>
  );
}

function CreativeDirectionTabs({ result, onGenerate, generating }: { result: YouTubeVideoAnalysisResponse; onGenerate: () => void; generating: boolean }) {
  const [active, setActive] = useState<'angles' | 'titles' | 'shorts' | 'scripts'>('angles');
  const groups = {
    angles: creativeCards(result.creator_opportunities?.suggested_remake_angles ?? result.suggested_remake_angles ?? [], 'Remake angle', videoAudience(result)),
    titles: creativeCards(result.creator_opportunities?.title_ideas ?? [], 'Title idea', formatVideoFormat(result.formatted_metadata?.format)),
    shorts: creativeCards(result.creator_opportunities?.short_form_clip_ideas ?? [], 'Short-form idea', 'Short-form'),
    scripts: creativeCards(result.creator_opportunities?.script_prompts ?? [], 'Script starter', videoAudience(result)),
  };
  const current = groups[active];
  return (
    <section className="video-section creative-directions" aria-labelledby="creative-title">
      <div className="video-section-heading"><span>Creative directions</span><h3 id="creative-title">Content to create from this insight</h3></div>
      <div className="creative-tabs" role="tablist" aria-label="Creative direction categories">
        {[
          ['angles', 'Remake Angles'],
          ['titles', 'Title Ideas'],
          ['shorts', 'Short-Form Ideas'],
          ['scripts', 'Script Starters'],
        ].map(([key, label]) => (
          <button key={key} type="button" role="tab" aria-selected={active === key} className={active === key ? 'is-active' : ''} onClick={() => setActive(key as typeof active)}>{label}</button>
        ))}
      </div>
      <div className="creative-card-grid">
        {current.length ? current.slice(0, 6).map(card => <CreativeDirectionCard key={card.id} card={card} onGenerate={onGenerate} generating={generating} />) : <div className="muted-note">No creative recommendations returned for this category.</div>}
      </div>
    </section>
  );
}

function CreativeDirectionCard({ card, onGenerate, generating }: { card: CreativeCard; onGenerate: () => void; generating: boolean }) {
  return (
    <article className="creative-direction-card">
      <div className="creative-card-top">
        <span aria-hidden="true">{card.icon}</span>
        <small>{card.tag}</small>
      </div>
      <h4>{card.title}</h4>
      <p>{card.description}</p>
      <button type="button" className="link-button" onClick={onGenerate} disabled={generating}>{generating ? 'Generating...' : 'Generate script'}</button>
    </article>
  );
}

function ScriptGenerationPanel({ result, generating, generated, error, onGenerate, onOpenScriptStudio }: {
  result: YouTubeVideoAnalysisResponse;
  generating: boolean;
  generated: boolean;
  error?: string;
  onGenerate: () => void;
  onOpenScriptStudio?: () => void;
}) {
  return (
    <section className="script-generation-panel" aria-labelledby="script-generation-title">
      <div>
        <span>Next creator action</span>
        <h3 id="script-generation-title">Generate a script from this analysis</h3>
        <p>{[firstUseful(result.creator_opportunities?.suggested_remake_angles, result.suggested_remake_angles, [result.inferred_content_angle]), videoAudience(result), formatVideoFormat(result.formatted_metadata?.format)].filter(Boolean).join(' · ')}</p>
      </div>
      <ScriptAction
        label="Generate Script From This Analysis"
        generating={generating}
        generated={generated}
        error={error}
        onGenerate={onGenerate}
        onOpenScriptStudio={onOpenScriptStudio}
      />
    </section>
  );
}

function AnalysisDetailsAccordion({ result }: { result: YouTubeVideoAnalysisResponse }) {
  return (
    <section className="analysis-details-bottom">
      <details className="video-details-accordion">
        <summary>Analysis details</summary>
        <TextBlock label="Evidence boundary" value="Analysis uses public YouTube Data API metadata. It does not include transcript content, retention, impressions, CTR, private audience data, exact RPM, or actual revenue." />
        <MetricGrid values={{
          Title: result.title,
          Channel: result.channel_title,
          Published: result.formatted_metadata?.published_date || result.published_at,
          Duration: result.formatted_metadata?.duration || result.duration,
          Views: result.formatted_metadata?.views ?? result.views,
          Likes: result.formatted_metadata?.likes ?? result.likes,
          Comments: result.formatted_metadata?.comments ?? result.comments,
          Provider: result.metadata?.source_provider,
          'Schema version': result.schema_version,
        }} />
        <SectionList label="Limitations" items={result.limitations ?? []} />
        <SectionList label="Evidence fields" items={(result.evidence_basis ?? []).map(item => `${formatLabel(item.field)}: ${item.basis}`)} />
        <SectionList label="Public hashtags" items={result.keyword_intelligence?.hashtags ?? []} />
      </details>
    </section>
  );
}

function VideoAnalyzerLoading() {
  return (
    <div className="video-analyzer-loading" role="status" aria-live="polite">
      <div className="loading-thumb skeleton-block" />
      <div className="loading-copy">
        <span className="skeleton-line wide" />
        <span className="skeleton-line" />
        <div className="loading-metrics">
          <span className="skeleton-block" />
          <span className="skeleton-block" />
          <span className="skeleton-block" />
        </div>
      </div>
    </div>
  );
}

function YouTubeChannelTab({ value, onChange, onAnalyze, loading, result, onGenerateIdea, generated, generationErrors, generatingID, region, language, onOpenScriptStudio }: {
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  result: YouTubeChannelAnalysisResponse | null;
  onGenerateIdea: (idea: string) => void;
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  region: string;
  language: string;
  onOpenScriptStudio?: () => void;
}) {
  return (
    <AnalyzerShell
      title="YouTube Channel Analyzer"
      description="Paste a channel URL, @handle, /channel/ID, /c/Name, or search term to review formats, keywords, and content gaps."
      inputLabel="YouTube channel URL or handle"
      value={value}
      onChange={onChange}
      onAnalyze={onAnalyze}
      loading={loading}
      buttonLabel="Analyze Channel"
    >
      {!result && <div className="neutral-callout">Connect YouTube in Connections to analyze channels.</div>}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <div style={{ display: 'grid', gap: 10 }}>
          <ResultCard title="Channel snapshot">
            <TextBlock label="Channel" value={[result.channel_title, result.channel_id].filter(Boolean).join(' · ')} />
            <MetricGrid values={{ Subscribers: result.subscribers, 'Total views': result.views, Videos: result.video_count, Country: result.country, 'Channel age': result.channel_snapshot?.channel_age }} />
          </ResultCard>
          <ResultCard title="Niche diagnosis">
            <MetricGrid values={{
              'Primary niche': result.niche_analysis?.primary_niche || result.channel_niche,
              'Sub-niche': result.niche_analysis?.sub_niche,
              Audience: result.niche_analysis?.audience_type,
              Format: result.niche_analysis?.content_format,
              Confidence: result.niche_analysis?.confidence,
            }} />
            <TextBlock label="Likely strategy" value={result.likely_strategy} />
            <LabelledChips label="Evidence terms" items={result.niche_analysis?.evidence_terms ?? []} />
          </ResultCard>
          <ResultCard title="Top content pillars"><ChipList items={result.content_pillars ?? []} /></ResultCard>
          <ResultCard title="Top videos summary"><VideoSummaryList videos={result.top_videos_summary ?? []} /></ResultCard>
          <ResultCard title="Format patterns"><ChipList items={result.format_patterns ?? []} /></ResultCard>
          <ResultCard title="Title patterns"><List items={result.title_patterns ?? []} /></ResultCard>
          <ResultCard title="Keyword clusters"><KeywordClusterList clusters={result.keyword_clusters ?? []} fallback={result.repeated_keywords ?? []} /></ResultCard>
          <ResultCard title="Performance distribution"><MetricGrid values={result.performance_distribution ?? result.view_distribution ?? {}} /></ResultCard>
          <ResultCard title="Content opportunities"><List items={result.opportunities ?? []} /></ResultCard>
          <ResultCard title="Suggested short clip ideas"><List items={result.suggested_short_clip_ideas ?? []} /></ResultCard>
          <ResultCard title="Suggested next 10 video ideas">
            <List items={result.suggested_content_ideas ?? []} />
            {(result.suggested_content_ideas ?? []).map(idea => {
              const key = candidateFromChannel(result, idea, region, language).id;
              return (
                <div key={idea} style={{ display: 'grid', gap: 8 }}>
                  <div style={{ fontSize: 12, color: 'var(--text-muted)', overflowWrap: 'anywhere' }}>{idea}</div>
                  <ScriptAction
                    label="Generate Script"
                    generating={generatingID === key}
                    generated={Boolean(generated[key])}
                    error={generationErrors[key]}
                    onGenerate={() => onGenerateIdea(idea)}
                    onOpenScriptStudio={onOpenScriptStudio}
                  />
                </div>
              );
            })}
          </ResultCard>
          <Limitations items={result.limitations ?? []} />
        </div>
      )}
    </AnalyzerShell>
  );
}

const NICHE_PROFILE_PREFS_KEY = 'trendcortex_niche_creator_profile';

function defaultNicheProfile(region: string, language: string, audience: string): CreatorNicheProfile {
  return {
    professional_skills: '',
    hobbies: '',
    lived_experiences: '',
    teaching_subjects: '',
    three_years_ago_advice: '',
    target_audience: audience === 'Global' ? '' : audience,
    target_country: region || 'GB',
    target_language: (language || 'en').split('-')[0],
    creator_presence: 'faceless channel',
    content_formats: ['long-form'],
    optional_broad_topic: '',
    weekly_production_capacity: '2 videos per week',
  };
}

function getSavedNicheProfile(region: string, language: string, audience: string): CreatorNicheProfile {
  const fallback = defaultNicheProfile(region, language, audience);
  try {
    const raw = localStorage.getItem(NICHE_PROFILE_PREFS_KEY);
    if (!raw) return fallback;
    const saved = JSON.parse(raw) as Partial<CreatorNicheProfile>;
    return { ...fallback, ...saved, content_formats: saved.content_formats?.length ? saved.content_formats : fallback.content_formats };
  } catch {
    return fallback;
  }
}

function saveNicheProfile(profile: CreatorNicheProfile) {
  try {
    localStorage.setItem(NICHE_PROFILE_PREFS_KEY, JSON.stringify(profile));
  } catch {
    // Local storage can be unavailable in private browsing; the workflow still works.
  }
}

function NicheFinderTab({ providers, region, language, audience, onGenerate, generated, generationErrors, generatingID, onOpenScriptStudio }: {
  providers: ResearchProviderStatus[];
  region: string;
  language: string;
  audience: string;
  onGenerate: (candidate: NicheCandidate) => void;
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onOpenScriptStudio?: () => void;
}) {
  const [profile, setProfile] = useState<CreatorNicheProfile>(() => getSavedNicheProfile(region, language, audience));
  const [loading, setLoading] = useState(false);
  const [report, setReport] = useState<NicheReport | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [expandedID, setExpandedID] = useState<string | null>(null);
  const [showTopicsID, setShowTopicsID] = useState<string | null>(null);
  const [loadingStage, setLoadingStage] = useState(0);
  const [profileExpanded, setProfileExpanded] = useState(false);
  const [lastResearchedProfileKey, setLastResearchedProfileKey] = useState<string | null>(null);
  const providerMap = new Map(providers.map(provider => [provider.id, provider]));
  const meaningfulFields = [
    profile.professional_skills,
    profile.hobbies,
    profile.lived_experiences,
    profile.teaching_subjects,
    profile.three_years_ago_advice,
    profile.target_audience,
    profile.optional_broad_topic,
  ].filter(value => value.trim()).length;
  const hasMinimumInputs = meaningfulFields >= 3;
  const ready = hasMinimumInputs && !loading;
  const completion = profileCompletion(profile);
  const currentProfileKey = stableProfileKey(profile);
  const profileDirty = Boolean(report && lastResearchedProfileKey && lastResearchedProfileKey !== currentProfileKey);

  useEffect(() => {
    if (!loading) {
      setLoadingStage(0);
      return;
    }
    const interval = window.setInterval(() => {
      setLoadingStage(current => Math.min(current + 1, NICHE_LOADING_STAGES.length - 1));
    }, 1800);
    return () => window.clearInterval(interval);
  }, [loading]);

  useEffect(() => {
    saveNicheProfile(profile);
  }, [profile]);

  function setProfileField(field: keyof CreatorNicheProfile, value: string) {
    setProfile(current => ({ ...current, [field]: value }));
  }

  function setFormat(value: string) {
    setProfile(current => ({ ...current, content_formats: [value] }));
  }

  function runAnalysis() {
    if (!ready || loading) return;
    setLoading(true);
    setError(null);
    setReport(null);
    setProfileExpanded(false);
    setLastResearchedProfileKey(stableProfileKey(profile));
    researchNiches({
      profile: {
        ...profile,
        target_country: profile.target_country || region,
        target_language: profile.target_language || language,
      },
    })
      .then(data => {
        setReport(data);
        setExpandedID(data.candidates?.[0]?.id ?? null);
      })
      .catch(err => setError(err instanceof ApiError ? err.message : 'Niche opportunity analysis failed.'))
      .finally(() => setLoading(false));
  }

  async function loadFixture() {
    if (!import.meta.env.DEV) return;
    const {
      nicheFinderFixtureReport,
      nicheFinderLongMethodologyFixtureReport,
      nicheFinderUnavailableFixtureReport,
    } = await import('../data/nicheFinderFixture');
    const fixtureMode = new URLSearchParams(window.location.search).get('nicheFixture');
    const selectedFixture = fixtureMode === 'long-methodology'
      ? nicheFinderLongMethodologyFixtureReport
      : fixtureMode === 'unavailable'
        ? nicheFinderUnavailableFixtureReport
        : nicheFinderFixtureReport;
    setError(null);
    setLoading(false);
    setReport(selectedFixture);
    setExpandedID(selectedFixture.primary_recommendation?.id ?? selectedFixture.candidates?.[0]?.id ?? null);
    setLastResearchedProfileKey(stableProfileKey(profile));
    setProfileExpanded(false);
  }

  return (
    <div className="niche-workflow">
      <CreatorProfilePanel
        profile={profile}
        expanded={profileExpanded || (!report && !loading)}
        completion={completion}
        meaningfulFields={meaningfulFields}
        dirty={profileDirty}
        loading={loading}
        loadingLabel={NICHE_LOADING_STAGES[loadingStage]}
        hasMinimumInputs={hasMinimumInputs}
        ready={ready}
        providerMap={providerMap}
        onToggle={() => setProfileExpanded(current => !current)}
        onSetField={setProfileField}
        onSetFormat={setFormat}
        onReset={() => setProfile(defaultNicheProfile(region, language, audience))}
        onRun={runAnalysis}
      />

      <div className="neutral-callout">
        Niche Finder separates AI strategy from verified public evidence and keeps provider measurements separate from model-derived opportunity scoring.
        {import.meta.env.DEV && <button className="link-button niche-fixture-action" type="button" onClick={loadFixture}>Load QA fixture</button>}
      </div>

      <div className="niche-result-anchor">
        {loading && <EmptyState tone="loading" title={NICHE_LOADING_STAGES[loadingStage]} desc="Your inputs are preserved while Niche Finder validates strategy, scores, evidence availability, and runway depth." />}
        {error && <EmptyState tone="error" title="Niche analysis failed." desc={error} />}
        {report && report.status !== 'ok' && <NicheResearchState report={report} />}
      </div>
      {report?.status === 'ok' && (
        <div className="niche-results" key={`${report.id}-${report.primary_recommendation?.id ?? report.candidates[0]?.id ?? 'candidate'}-${report.created_at}`}>
          <NicheDashboardHeader
            report={report}
            candidate={report.primary_recommendation ?? report.candidates[0]}
            generating={Boolean((report.primary_recommendation ?? report.candidates[0]) && generatingID === nicheCandidateKey(report.primary_recommendation ?? report.candidates[0]))}
            generated={Boolean((report.primary_recommendation ?? report.candidates[0]) && generated[nicheCandidateKey(report.primary_recommendation ?? report.candidates[0])])}
            generationError={(report.primary_recommendation ?? report.candidates[0]) ? generationErrors[nicheCandidateKey(report.primary_recommendation ?? report.candidates[0])] : undefined}
            onBuildStrategy={() => report.candidates[0] && onGenerate(report.primary_recommendation ?? report.candidates[0])}
            onOpenScriptStudio={onOpenScriptStudio}
          />
          {report.cache?.freshness === 'stale' && (
            <div className="neutral-callout niche-stale-notice">
              Showing the most recent verified evidence from {formatCacheTime(report.cache.evidence_fetched_at || report.cache.stored_at)}. Fresh validation is temporarily unavailable.
            </div>
          )}
          {report.candidates[0] && (
            <NicheCandidateDashboard
              candidate={report.primary_recommendation ?? report.candidates[0]}
              showTopics={showTopicsID === (report.primary_recommendation ?? report.candidates[0]).id}
              onToggleTopics={() => setShowTopicsID(current => current === (report.primary_recommendation ?? report.candidates[0]).id ? null : (report.primary_recommendation ?? report.candidates[0]).id)}
            />
          )}
          <AlternativeCandidates
            candidates={report.alternative_candidates?.length ? report.alternative_candidates : report.candidates.slice(1)}
            unavailableReason={report.limitations?.find(item => item.toLowerCase().includes('alternative'))}
            expandedID={expandedID}
            onToggle={setExpandedID}
          />
          <ScoreMethodology methodology={report.methodology ?? []} limitations={report.limitations ?? []} />
        </div>
      )}
    </div>
  );
}

function NicheDashboardHeader({ report, candidate, generating, generated, generationError, onBuildStrategy, onOpenScriptStudio }: {
  report: NicheReport;
  candidate?: NicheCandidate;
  generating: boolean;
  generated: boolean;
  generationError?: string;
  onBuildStrategy: () => void;
  onOpenScriptStudio?: () => void;
}) {
  const reveal = useRevealClass();
  if (!candidate) return null;
  return (
    <section ref={reveal.ref as RefObject<HTMLElement>} className={`niche-dashboard-header ${reveal.className}`.trim()}>
      <div className="niche-header-copy">
        <div className="niche-path">{candidate.category || candidate.level_1} / {candidate.subcategory || candidate.level_2}</div>
        <h2>{candidate.name || candidate.niche_name}</h2>
        <p>{candidate.concise_positioning || candidate.unique_angle || report.creator_profile_summary}</p>
        <div className="niche-header-badges">
          <span>{candidate.confidence || candidate.scores.confidence.label} confidence</span>
          <span>{evidenceBadgeLabel(report, candidate)}</span>
          <span>{evidenceFreshnessLabel(report.evidence_freshness, candidate.market_evidence?.collected_at)}</span>
        </div>
      </div>
      <ProgressCircle value={candidate.overall_score ?? candidate.scores.overall.score} label="Overall" accent="cyan" size="overall" ariaLabel={`Overall score ${Math.round(candidate.overall_score ?? candidate.scores.overall.score)} out of 100`} />
      <div className="niche-header-action">
        <TextBlock label="Recommended first action" value={candidate.recommended_first_action} />
        <div className="niche-card-actions">
          <button className="generate-btn secondary" type="button" disabled>Save Niche</button>
          <ScriptAction
            label="Build Channel Strategy"
            generating={generating}
            generated={generated}
            error={generationError}
            onGenerate={onBuildStrategy}
            onOpenScriptStudio={onOpenScriptStudio}
          />
        </div>
      </div>
    </section>
  );
}

function NicheCandidateDashboard({ candidate, showTopics, onToggleTopics }: { candidate: NicheCandidate; showTopics: boolean; onToggleTopics: () => void }) {
  const [openDimensionKey, setOpenDimensionKey] = useState<string | null>(null);
  const detailPanelId = useId();
  const dimensions = dimensionRows(candidate);
  const openDimension = dimensions.find(dim => dim.key === openDimensionKey);
  const pillars = candidate.content_pillars?.length ? candidate.content_pillars : candidate.topic_pillars ?? [];
  const titles = candidate.recommended_titles?.length ? candidate.recommended_titles : candidate.video_topics ?? [];
  const runwayCount = candidate.runway?.viable_topic_count ?? candidate.sustainability.viable_topic_count ?? titles.length;
  const isCompleteRunway = titles.length === 50 && runwayCount === 50;
  const runwayTitle = candidate.runway?.heading ?? (isCompleteRunway ? '50-video runway' : 'Initial content runway');
  return (
    <div className="niche-dashboard-flow">
      <ScrollReveal>
      <section className="niche-score-card-grid" aria-label="Strategic score cards">
        {dimensions.map(dim => (
          <DimensionCard
            key={dim.key}
            row={dim}
            open={openDimensionKey === dim.key}
            detailPanelId={detailPanelId}
            onToggle={() => setOpenDimensionKey(current => current === dim.key ? null : dim.key)}
          />
        ))}
      </section>
      </ScrollReveal>
      {openDimension && <DimensionDetailPanel id={detailPanelId} row={openDimension} candidate={candidate} onClose={() => setOpenDimensionKey(null)} />}
      <ScrollReveal>
      <section className="niche-analysis-columns" aria-label="Strategic analysis">
        <div className="niche-analysis-stack">
          <NichePanel title="Strategic scoring radar">
            <RadarChart rows={dimensions} />
          </NichePanel>
          <NichePanel title="Content-pillar distribution">
            <PillarChart pillars={pillars} />
          </NichePanel>
        </div>
        <div className="niche-analysis-stack">
          <NichePanel title="Deterministic score contribution">
            <ContributionChart rows={dimensions} />
          </NichePanel>
          <NichePanel title="Demand evidence">
            <DemandEvidence candidate={candidate} />
          </NichePanel>
        </div>
      </section>
      </ScrollReveal>
      <NichePanel title={runwayTitle}>
        <RunwayCard candidate={candidate} pillars={pillars} />
      </NichePanel>
      <ScrollReveal>
      <section className="niche-insight-layout" aria-label="Audience and competition insights">
        <NichePanel title="Audience profile">
          <AudienceProfile candidate={candidate} />
        </NichePanel>
        <NichePanel title="Competition opportunity">
          <CompetitionVisual candidate={candidate} />
        </NichePanel>
      </section>
      </ScrollReveal>
      <NichePanel title="First 10 video ideas">
        <IdeaPreview titles={titles} showTopics={showTopics} onToggleTopics={onToggleTopics} />
      </NichePanel>
      <ScrollReveal>
      <section className="niche-opportunity-risk-grid" aria-label="Opportunity gaps and risks">
        <NichePanel title="Opportunity gaps">
          <List items={candidate.opportunity_gaps?.length ? candidate.opportunity_gaps : (candidate.supply_gaps ?? []).map(gap => gap.statement)} />
        </NichePanel>
        <NichePanel title="Risks and limitations">
          <RiskSection candidate={candidate} />
        </NichePanel>
      </section>
      </ScrollReveal>
      <ScrollReveal>
      <section className="niche-evidence-risk-grid">
        <NichePanel title="Evidence and outliers">
          <EvidenceSection candidate={candidate} />
        </NichePanel>
      </section>
      </ScrollReveal>
    </div>
  );
}

function NicheResearchState({ report }: { report: NicheReport }) {
  const state = nicheStateCopy(report.status, report.message);
  if (report.status === 'credentials_invalid' && !import.meta.env.DEV) {
    return <EmptyState tone="unavailable" title="Research temporarily unavailable." desc="Niche evidence is temporarily unavailable. Try again later." />;
  }
  return <EmptyState tone={state.tone} title={state.title} desc={state.desc} />;
}

function nicheStateCopy(status: string, message: string): { tone: StateTone; title: string; desc: string } {
  switch (status) {
    case 'openai_unavailable':
      return { tone: 'unavailable', title: 'AI niche generation is unavailable.', desc: message || 'OpenAI is unavailable, so Niche Finder did not generate candidates or run YouTube validation. Your profile inputs remain above for retry.' };
    case 'invalid_model_output':
      return { tone: 'unavailable', title: 'AI niche generation returned unusable output.', desc: message || 'Niche Finder could not validate the AI strategy output. Your profile inputs remain above for retry.' };
    case 'no_matching_content':
      return { tone: 'empty', title: 'No reliable evidence matched this profile.', desc: message || 'No reliable market evidence matched this profile.' };
    case 'insufficient_evidence':
      return { tone: 'empty', title: 'More evidence required.', desc: message || 'More public evidence is required before ranking this niche.' };
    case 'quota_temporarily_unavailable':
      return { tone: 'unavailable', title: 'Research limits reached temporarily.', desc: message || 'Research limits have been reached temporarily.' };
    case 'provider_temporarily_unavailable':
    case 'validation_timeout':
    case 'research_failed':
      return { tone: 'unavailable', title: 'Research temporarily unavailable.', desc: message || 'Niche evidence is temporarily unavailable. Try again later.' };
    case 'credentials_invalid':
      return { tone: 'warning', title: 'Invalid local research configuration.', desc: 'Developer mode: local research configuration is invalid.' };
    case 'not_configured':
      return { tone: 'warning', title: 'Research setup needed.', desc: message || 'Local research configuration is incomplete.' };
    default:
      return { tone: 'empty', title: 'Niche research unavailable.', desc: message || 'Niche evidence is temporarily unavailable. Try again later.' };
  }
}

function formatCacheTime(value?: string): string {
  if (!value) return 'the last successful run';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'the last successful run';
  return date.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function CreatorProfilePanel({ profile, expanded, completion, meaningfulFields, dirty, loading, loadingLabel, hasMinimumInputs, ready, providerMap, onToggle, onSetField, onSetFormat, onReset, onRun }: {
  profile: CreatorNicheProfile;
  expanded: boolean;
  completion: { complete: number; total: number; firstIncomplete: string };
  meaningfulFields: number;
  dirty: boolean;
  loading: boolean;
  loadingLabel: string;
  hasMinimumInputs: boolean;
  ready: boolean;
  providerMap: Map<string, ResearchProviderStatus>;
  onToggle: () => void;
  onSetField: (field: keyof CreatorNicheProfile, value: string) => void;
  onSetFormat: (value: string) => void;
  onReset: () => void;
  onRun: () => void;
}) {
  const panelId = useId();
  const actionLabel = dirty ? 'Research updated profile' : 'Research Niches';
  const reveal = useRevealClass();
  return (
    <section ref={reveal.ref as RefObject<HTMLElement>} className={`niche-profile-shell ${reveal.className}${expanded ? ' is-expanded' : ' is-collapsed'}${dirty ? ' is-dirty' : ''}`} aria-labelledby={`${panelId}-title`}>
      <div className="niche-profile-summary">
        <div className="niche-profile-heading">
          <div id={`${panelId}-title`} className="settings-card-title">Creator profile</div>
          <StatusBadge tone={hasMinimumInputs ? 'success' : 'warning'}>
            {hasMinimumInputs ? `Minimum met · ${meaningfulFields} qualifying inputs` : `${meaningfulFields} qualifying inputs`}
          </StatusBadge>
          {dirty && <StatusBadge tone="warning">Changes not researched yet</StatusBadge>}
        </div>
        <button className="generate-btn secondary niche-profile-toggle" type="button" onClick={onToggle} aria-expanded={expanded} aria-controls={`${panelId}-body`}>
          {expanded ? 'Collapse profile' : 'Edit profile'}
        </button>
      </div>
      <ProfileSummary profile={profile} />
      {expanded && (
        <div id={`${panelId}-body`} className="niche-profile-body">
          <ProfileAccordion title="Creator credibility" defaultOpen={completion.firstIncomplete === 'credibility'}>
            <div className="form-grid two niche-field-grid">
              <TextInput multiline className="field-half" label="Professional skills" value={profile.professional_skills} onChange={value => onSetField('professional_skills', value)} placeholder="software development, accounting, design" />
              <TextInput multiline className="field-half" label="Lived experiences" value={profile.lived_experiences} onChange={value => onSetField('lived_experiences', value)} placeholder="UK visa process, building a business, career switch" />
              <TextInput multiline className="field-half" label="Subjects you can teach" value={profile.teaching_subjects} onChange={value => onSetField('teaching_subjects', value)} placeholder="AI workflows, student finance, interview prep" />
              <TextInput multiline className="field-half" label="What you wish you knew three years ago" value={profile.three_years_ago_advice} onChange={value => onSetField('three_years_ago_advice', value)} placeholder="what would have saved you time, money, or mistakes?" />
              <TagInput className="field-wide" label="Hobbies" value={profile.hobbies} onChange={value => onSetField('hobbies', value)} placeholder="fitness, football, cooking, travel" />
            </div>
          </ProfileAccordion>
          <ProfileAccordion title="Audience and format" defaultOpen={completion.firstIncomplete === 'audience'}>
            <div className="form-grid two niche-field-grid">
              <TextInput multiline className="field-wide" label="Target audience" value={profile.target_audience} onChange={value => onSetField('target_audience', value)} placeholder="UK small businesses, international students" />
              <Select label="Target country" value={profile.target_country} onChange={value => onSetField('target_country', value)} options={[
                { label: 'United Kingdom', value: 'GB' },
                { label: 'United States', value: 'US' },
                { label: 'Pakistan', value: 'PK' },
                { label: 'India', value: 'IN' },
              ]} />
              <Select label="Target language" value={profile.target_language} onChange={value => onSetField('target_language', value)} options={[
                { label: 'English', value: 'en' },
                { label: 'Urdu', value: 'ur' },
                { label: 'Hindi', value: 'hi' },
                { label: 'Arabic', value: 'ar' },
              ]} />
              <SegmentedControl className="field-half" label="Creator presence" value={profile.creator_presence} options={[
                { label: 'Faceless', value: 'faceless channel' },
                { label: 'On-camera', value: 'personal brand' },
                { label: 'Mixed', value: 'mixed presence' },
              ]} onChange={value => onSetField('creator_presence', value)} />
              <SegmentedControl className="field-half" label="Content format" value={profile.content_formats[0] ?? 'long-form'} options={[
                { label: 'Short-form', value: 'shorts' },
                { label: 'Long-form', value: 'long-form' },
                { label: 'Both', value: 'both' },
              ]} onChange={onSetFormat} />
              <SegmentedControl className="field-wide" label="Weekly capacity" value={capacitySegment(profile.weekly_production_capacity)} options={[
                { label: '1', value: '1 video per week' },
                { label: '2', value: '2 videos per week' },
                { label: '3', value: '3 videos per week' },
                { label: '4+', value: '4 videos per week' },
                { label: 'Custom', value: 'custom' },
              ]} onChange={value => {
                if (value !== 'custom') onSetField('weekly_production_capacity', value);
              }} />
              {capacitySegment(profile.weekly_production_capacity) === 'custom' && <TextInput className="field-wide" label="Custom capacity" value={profile.weekly_production_capacity} onChange={value => onSetField('weekly_production_capacity', value)} placeholder="2 videos per week" />}
            </div>
          </ProfileAccordion>
          <ProfileAccordion title="Topic focus" defaultOpen={completion.firstIncomplete === 'topic'}>
            <div className="form-grid two niche-field-grid">
              <TextInput multiline label="Optional broad topic" value={profile.optional_broad_topic} onChange={value => onSetField('optional_broad_topic', value)} placeholder="AI automation, UK visa, personal finance" />
            </div>
          </ProfileAccordion>
          <ProfileAccordion title="Validation readiness" defaultOpen={false}>
            <ReadinessPanel rows={[
              { label: 'Qualifying inputs', value: meaningfulFields, type: 'value' },
              { label: 'Minimum required', value: 3, type: 'value' },
              { label: 'Video validation', value: providerMap.get('youtube_data_api')?.status || 'unknown', type: 'status' },
              { label: 'Current trends', value: providerMap.get('google_trends_rss')?.status || 'unknown', type: 'status' },
            ]} />
          </ProfileAccordion>
          <div className="niche-action-footer">
            <div className="niche-action-copy">
              {loading ? loadingLabel : hasMinimumInputs ? 'Ready to validate niche candidates with public demand evidence.' : 'Add at least three creator-profile inputs to create meaningful niche candidates.'}
            </div>
            <div className="niche-action-buttons">
              <button className="generate-btn secondary" type="button" onClick={onReset} disabled={loading}>Reset</button>
              <button className="generate-btn idle niche-primary-action" type="button" onClick={onRun} disabled={!ready}>
                {loading ? 'Researching...' : actionLabel}
              </button>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function ProfileSummary({ profile }: { profile: CreatorNicheProfile }) {
  const primary = [
    profile.professional_skills,
    profile.target_audience,
    countryLabel(profile.target_country),
    languageLabel(profile.target_language),
  ].filter(Boolean);
  const secondary = [
    presenceLabel(profile.creator_presence),
    formatLabel(profile.content_formats[0] ?? ''),
    profile.weekly_production_capacity,
  ].filter(Boolean);
  return (
    <div className="niche-profile-lines" aria-label="Creator profile summary">
      <div>{primary.join(' · ') || 'Add profile details to start research'}</div>
      <div>{secondary.join(' · ')}</div>
    </div>
  );
}

function ProfileAccordion({ title, defaultOpen, children }: { title: string; defaultOpen: boolean; children: ReactNode }) {
  const id = useId();
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className="profile-accordion">
      <button className="profile-accordion-trigger" type="button" onClick={() => setOpen(current => !current)} aria-expanded={open} aria-controls={`${id}-panel`}>
        <span>{title}</span>
        <span aria-hidden="true">{open ? '-' : '+'}</span>
      </button>
      {open && <div id={`${id}-panel`} className="profile-accordion-panel">{children}</div>}
    </section>
  );
}

function SegmentedControl({ label, value, options, onChange, className = '' }: { label: string; value: string; options: { label: string; value: string }[]; onChange: (value: string) => void; className?: string }) {
  return (
    <div className={`form-group segmented-field ${className}`.trim()}>
      <div className="form-label">{label}</div>
      <div className="segmented-control" role="group" aria-label={label} style={{ '--segment-count': options.length } as CSSProperties}>
        {options.map(option => (
          <button key={option.value} type="button" className={option.value === value ? 'is-selected' : ''} onClick={() => onChange(option.value)} aria-pressed={option.value === value}>
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

function TagInput({ label, value, onChange, placeholder, className = '' }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; className?: string }) {
  const tags = value.split(',').map(tag => tag.trim()).filter(Boolean);
  return (
    <div className={`tag-input-wrap ${className}`.trim()}>
      <TextInput label={label} value={value} onChange={onChange} placeholder={placeholder} />
      {tags.length ? <div className="niche-tag-preview">{tags.slice(0, 5).map(tag => <span key={tag}>{tag}</span>)}</div> : null}
    </div>
  );
}

function profileCompletion(profile: CreatorNicheProfile): { complete: number; total: number; firstIncomplete: string } {
  const groups = [
    { key: 'credibility', complete: [profile.professional_skills, profile.lived_experiences, profile.teaching_subjects, profile.three_years_ago_advice].some(value => value.trim()) },
    { key: 'audience', complete: Boolean(profile.target_audience.trim() && profile.target_country && profile.target_language && profile.creator_presence && profile.content_formats.length && profile.weekly_production_capacity.trim()) },
    { key: 'topic', complete: Boolean(profile.optional_broad_topic.trim()) },
  ];
  const complete = groups.filter(group => group.complete).length;
  return { complete, total: groups.length, firstIncomplete: groups.find(group => !group.complete)?.key ?? 'credibility' };
}

function stableProfileKey(profile: CreatorNicheProfile): string {
  return JSON.stringify({
    professional_skills: profile.professional_skills,
    hobbies: profile.hobbies,
    lived_experiences: profile.lived_experiences,
    teaching_subjects: profile.teaching_subjects,
    three_years_ago_advice: profile.three_years_ago_advice,
    target_audience: profile.target_audience,
    target_country: profile.target_country,
    target_language: profile.target_language,
    creator_presence: profile.creator_presence,
    content_formats: profile.content_formats,
    optional_broad_topic: profile.optional_broad_topic,
    weekly_production_capacity: profile.weekly_production_capacity,
  });
}

function capacitySegment(value: string): string {
  const normalized = value.trim().toLowerCase();
  if (normalized.startsWith('1 ')) return '1 video per week';
  if (normalized.startsWith('2 ')) return '2 videos per week';
  if (normalized.startsWith('3 ')) return '3 videos per week';
  if (normalized.startsWith('4 ')) return '4 videos per week';
  return 'custom';
}

function countryLabel(value: string): string {
  return { GB: 'United Kingdom', US: 'United States', PK: 'Pakistan', IN: 'India' }[value] || value;
}

function languageLabel(value: string): string {
  return { en: 'English', ur: 'Urdu', hi: 'Hindi', ar: 'Arabic' }[value] || value;
}

function presenceLabel(value: string): string {
  if (value.includes('faceless')) return 'Faceless';
  if (value.includes('mixed')) return 'Mixed';
  if (value.includes('personal')) return 'On-camera';
  return formatLabel(value);
}

function StatusBadge({ tone = 'neutral', children }: { tone?: 'neutral' | 'success' | 'warning' | 'info'; children: ReactNode }) {
  return <span className={`niche-status-badge is-${tone}`}>{children}</span>;
}

function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);
  useEffect(() => {
    const query = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReduced(query.matches);
    update();
    query.addEventListener('change', update);
    return () => query.removeEventListener('change', update);
  }, []);
  return reduced;
}

function useInViewOnce<T extends HTMLElement | SVGElement>(threshold = 0.25): { ref: MutableRefObject<T | null>; inView: boolean; reducedMotion: boolean } {
  const ref = useRef<T | null>(null);
  const [inView, setInView] = useState(false);
  const reducedMotion = usePrefersReducedMotion();

  useEffect(() => {
    if (reducedMotion) {
      setInView(true);
      return undefined;
    }
    const element = ref.current;
    if (!element || inView) return undefined;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting && entry.intersectionRatio >= threshold) {
        setInView(true);
        observer.disconnect();
      }
    }, { threshold: [0, threshold, 1] });
    observer.observe(element);
    return () => observer.disconnect();
  }, [inView, reducedMotion, threshold]);

  return { ref, inView: inView || reducedMotion, reducedMotion };
}

function useAnimatedNumber(target: number, active: boolean, reducedMotion: boolean, duration = 900): number {
  const [value, setValue] = useState(reducedMotion ? target : 0);

  useEffect(() => {
    if (reducedMotion) {
      setValue(target);
      return undefined;
    }
    if (!active) {
      setValue(0);
      return undefined;
    }

    let frame = 0;
    let start: number | null = null;
    const animate = (timestamp: number) => {
      if (start == null) start = timestamp;
      const progress = Math.min((timestamp - start) / duration, 1);
      const eased = 1 - Math.pow(1 - progress, 3);
      setValue(target * eased);
      if (progress < 1) frame = window.requestAnimationFrame(animate);
    };

    frame = window.requestAnimationFrame(animate);
    return () => window.cancelAnimationFrame(frame);
  }, [active, duration, reducedMotion, target]);

  return value;
}

function useDelayedActive(active: boolean, delay: number, reducedMotion: boolean): boolean {
  const [delayedActive, setDelayedActive] = useState(reducedMotion ? true : active && delay === 0);

  useEffect(() => {
    if (reducedMotion || delay === 0) {
      setDelayedActive(active || reducedMotion);
      return undefined;
    }
    if (!active) {
      setDelayedActive(false);
      return undefined;
    }
    const timer = window.setTimeout(() => setDelayedActive(true), delay);
    return () => window.clearTimeout(timer);
  }, [active, delay, reducedMotion]);

  return delayedActive;
}

function useRevealClass(): { ref: MutableRefObject<HTMLElement | null>; className: string } {
  const { ref, inView, reducedMotion } = useInViewOnce<HTMLElement>(0.22);
  return { ref, className: `scroll-reveal${inView || reducedMotion ? ' is-visible' : ''}` };
}

function ScrollReveal({ children, className = '' }: { children: ReactNode; className?: string }) {
  const reveal = useRevealClass();
  return <div ref={reveal.ref as RefObject<HTMLDivElement>} className={`${reveal.className} ${className}`.trim()}>{children}</div>;
}

function NichePanel({ title, children }: { title: string; children: ReactNode }) {
  const reveal = useRevealClass();
  return (
    <section ref={reveal.ref as RefObject<HTMLElement>} className={`niche-detail-panel ${reveal.className}`.trim()}>
      <div className="niche-detail-title">{title}</div>
      {children}
    </section>
  );
}

type DimensionRow = { key: string; label: string; score: number; ratingBand?: string; explanation: string; weight: number; accent: string };

function dimensionRows(candidate: NicheCandidate): DimensionRow[] {
  const dims = candidate.dimensions;
  return [
    { key: 'creator_fit', label: 'Creator Fit', score: dims?.creator_fit?.score ?? candidate.scores.personal_fit.score, ratingBand: dims?.creator_fit?.rating_band ?? candidate.scores.personal_fit.rating_band, explanation: dims?.creator_fit?.explanation ?? candidate.scores.personal_fit.explanation, weight: 0.25, accent: 'purple' },
    { key: 'audience_demand', label: 'Audience Demand', score: dims?.audience_demand?.score ?? candidate.scores.demand.score, ratingBand: dims?.audience_demand?.rating_band ?? candidate.scores.demand.rating_band, explanation: dims?.audience_demand?.explanation ?? candidate.scores.demand.explanation, weight: 0.25, accent: 'blue' },
    { key: 'competition_opportunity', label: 'Competition Opportunity', score: dims?.competition_opportunity?.score ?? candidate.scores.opportunity_gap.score, ratingBand: dims?.competition_opportunity?.rating_band ?? candidate.scores.opportunity_gap.rating_band, explanation: dims?.competition_opportunity?.explanation ?? candidate.scores.opportunity_gap.explanation, weight: 0.2, accent: 'green' },
    { key: 'sustainability', label: 'Content Sustainability', score: dims?.sustainability?.score ?? candidate.scores.sustainability.score, ratingBand: dims?.sustainability?.rating_band ?? candidate.scores.sustainability.rating_band, explanation: dims?.sustainability?.explanation ?? candidate.scores.sustainability.explanation, weight: 0.2, accent: 'amber' },
    { key: 'differentiation', label: 'Differentiation', score: dims?.differentiation?.score ?? Math.round(((candidate.scores.personal_fit.score + candidate.scores.opportunity_gap.score) / 2)), ratingBand: dims?.differentiation?.rating_band, explanation: dims?.differentiation?.explanation ?? 'Measures positioning, production feasibility, and creator-specific angle.', weight: 0.1, accent: 'pink' },
  ];
}

type GaugeSize = 'overall' | 'metric' | 'runway' | 'alternative';

function ProgressCircle({ value, label, accent, max = 100, size = 'metric', ariaLabel }: { value: number; label?: string; accent: string; max?: number; size?: GaugeSize; ariaLabel: string }) {
  const numericValue = Number.isFinite(value) ? value : 0;
  const boundedValue = Math.max(0, Math.min(max, numericValue));
  const { ref, inView, reducedMotion } = useInViewOnce<HTMLDivElement>(0.25);
  const delayedInView = useDelayedActive(inView, 80, reducedMotion);
  const animatedValue = useAnimatedNumber(boundedValue, delayedInView, reducedMotion, 900);
  const displayValue = Math.round(animatedValue);
  const radius = 42;
  const circumference = 2 * Math.PI * radius;
  const progress = max > 0 ? Math.max(0, Math.min(max, animatedValue)) / max : 0;
  const dashOffset = circumference * (1 - progress);
  return (
    <div
      ref={ref}
      className={`niche-progress-circle niche-progress-circle-${size} accent-${accent}`}
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuenow={Math.round(boundedValue)}
      aria-valuemin={0}
      aria-valuemax={max}
    >
      <svg viewBox="0 0 100 100" aria-hidden="true" focusable="false">
        <circle className="niche-progress-circle-track" cx="50" cy="50" r={radius} />
        <circle
          className="niche-progress-circle-fill"
          cx="50"
          cy="50"
          r={radius}
          strokeDasharray={circumference}
          strokeDashoffset={dashOffset}
        />
      </svg>
      <span className="niche-progress-circle-label">
        <strong>{displayValue}</strong>
        {label ? <span>{label}</span> : null}
      </span>
    </div>
  );
}

function LinearProgress({ value, color, ariaLabel, className = '', displayValue, max = 100, delay = 0 }: { value: number; color?: string; ariaLabel: string; className?: string; displayValue?: string; max?: number; delay?: number }) {
  const numericValue = Number.isFinite(value) ? value : 0;
  const boundedValue = Math.max(0, Math.min(max, numericValue));
  const { ref, inView, reducedMotion } = useInViewOnce<HTMLSpanElement>(0.25);
  const delayedInView = useDelayedActive(inView, delay, reducedMotion);
  const animatedValue = useAnimatedNumber(boundedValue, delayedInView, reducedMotion, 760);
  const visibleWidth = max > 0 ? Math.max(0, Math.min(100, (animatedValue / max) * 100)) : 0;
  return (
    <span
      ref={ref}
      className={`niche-linear-progress ${className}`.trim()}
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuenow={boundedValue}
      aria-valuemin={0}
      aria-valuemax={100}
    >
      <span
        className="niche-linear-progress-fill"
        style={{ width: `${visibleWidth}%`, background: color || 'var(--accent)' }}
        aria-hidden="true"
      />
      {displayValue ? <span className="sr-only">{displayValue}</span> : null}
    </span>
  );
}

function DimensionCard({ row, open, detailPanelId, onToggle }: { row: DimensionRow; open: boolean; detailPanelId: string; onToggle: () => void }) {
  const rating = formatRatingBand(row.ratingBand ?? ratingBandForScore(row.score));
  return (
    <div className={`niche-dimension-card accent-${row.accent}${open ? ' is-active' : ''}`}>
      <strong className="niche-dimension-title">{row.label}</strong>
      <div className="niche-dimension-gauge-zone">
        <SemiGauge value={row.score} accent={row.accent} ariaLabel={`${row.label} score ${Math.round(row.score)} out of 100`} />
      </div>
      <span className="niche-dimension-rating">{rating}</span>
      <div className="niche-dimension-spacer" aria-hidden="true" />
      <div className="niche-dimension-footer">
        <button className="dimension-detail-toggle" type="button" onClick={onToggle} aria-expanded={open} aria-controls={detailPanelId} aria-label={`${open ? 'Hide' : 'Show'} ${row.label} details`}>
          {open ? 'Hide details ↑' : 'View details →'}
        </button>
      </div>
    </div>
  );
}

function SemiGauge({ value, accent, max = 100, ariaLabel }: { value: number; accent: string; max?: number; ariaLabel: string }) {
  const numericValue = Number.isFinite(value) ? value : 0;
  const boundedValue = Math.max(0, Math.min(max, numericValue));
  const { ref, inView, reducedMotion } = useInViewOnce<HTMLDivElement>(0.25);
  const animatedValue = useAnimatedNumber(boundedValue, inView, reducedMotion, 820);
  const displayValue = Math.round(animatedValue);
  const progress = max > 0 ? Math.max(0, Math.min(max, animatedValue)) / max : 0;
  const dashOffset = 100 * (1 - progress);
  return (
    <div
      ref={ref}
      className={`niche-semi-gauge accent-${accent}`}
      role="progressbar"
      aria-label={ariaLabel}
      aria-valuenow={Math.round(boundedValue)}
      aria-valuemin={0}
      aria-valuemax={max}
    >
      <svg viewBox="0 0 128 82" aria-hidden="true" focusable="false">
        <path className="niche-semi-gauge-track" d="M12 70 A52 52 0 0 1 116 70" pathLength="100" />
        <path className="niche-semi-gauge-fill" d="M12 70 A52 52 0 0 1 116 70" pathLength="100" style={{ strokeDasharray: 100, strokeDashoffset: dashOffset }} />
      </svg>
      <strong className="niche-semi-gauge-score">{displayValue}</strong>
    </div>
  );
}

function DimensionDetailPanel({ id, row, candidate, onClose }: { id: string; row: DimensionRow; candidate: NicheCandidate; onClose: () => void }) {
  const evidence = candidate.market_evidence;
  const contribution = row.score * row.weight;
  const limitations = evidence?.limitations?.slice(0, 2) ?? [];
  return (
    <section id={id} className={`dimension-detail-panel accent-${row.accent}`} aria-live="polite" tabIndex={-1}>
      <div>
        <span>{row.label}</span>
        <strong>{Math.round(row.score)} / 100 · {formatRatingBand(row.ratingBand ?? ratingBandForScore(row.score))}</strong>
      </div>
      <div className="dimension-detail-grid">
        <TextBlock label="Explanation" value={row.explanation} />
        <TextBlock label="Weighted contribution" value={`${contribution.toFixed(1)} of ${(row.weight * 100).toFixed(0)} possible points`} />
        {evidence?.sample_size ? <TextBlock label="Evidence considered" value={`${evidence.sample_size} sampled public videos; ${candidate.validation?.recent_publication_volume ?? 0} recent uploads; ${evidenceModeLabel(evidence.status)}.`} /> : null}
        {limitations.length ? <TextBlock label="Important limitations" value={limitations.join(' ')} /> : null}
      </div>
      <button className="link-button" type="button" onClick={onClose}>Close details</button>
    </section>
  );
}

function AudienceProfile({ candidate }: { candidate: NicheCandidate }) {
  const problems = candidate.audience_problems?.length ? candidate.audience_problems : [candidate.viewer_problem].filter(Boolean);
  const advantages = candidate.creator_advantages?.length ? candidate.creator_advantages : [candidate.creator_advantage].filter(Boolean);
  return (
    <div className="audience-profile-compact">
      <TextBlock label="Audience" value={candidate.target_audience || candidate.target_viewer} />
      <div className="audience-profile-columns">
        <SectionList label="Audience problems" items={problems} />
        <SectionList label="Creator advantages" items={advantages} />
      </div>
      <TextBlock label="Unique positioning" value={candidate.unique_angle} />
    </div>
  );
}

function IdeaPreview({ titles, showTopics, onToggleTopics }: { titles: VideoTopic[]; showTopics: boolean; onToggleTopics: () => void }) {
  const preview = titles.slice(0, 10);
  const leftColumn = preview.slice(0, 5);
  const rightColumn = preview.slice(5, 10);
  return (
    <div className="idea-preview">
      <div className="idea-preview-grid">
        <div className="idea-preview-column">
          {leftColumn.map((topic, index) => <CompactIdeaRow key={`${topic.title}-${index}`} index={index + 1} topic={topic} />)}
        </div>
        <div className="idea-preview-column">
          {rightColumn.map((topic, index) => <CompactIdeaRow key={`${topic.title}-${index + 5}`} index={index + 6} topic={topic} />)}
        </div>
      </div>
      <div className="idea-preview-footer">
        <span>{preview.length} shown{titles.length > preview.length ? ` from ${titles.length} runway ideas` : ''}</span>
        {titles.length > 10 && <button className="generate-btn secondary" type="button" onClick={onToggleTopics}>{showTopics ? 'Hide idea explorer' : `View all ${titles.length} ideas`}</button>}
      </div>
      {showTopics && <div className="idea-expanded-list">{titles.slice(10).map((topic, index) => <CompactIdeaRow key={`${topic.title}-${index + 10}`} index={index + 11} topic={topic} />)}</div>}
    </div>
  );
}

function CompactIdeaRow({ index, topic }: { index: number; topic: VideoTopic }) {
  const meta = [topic.pillar, topic.intent ? topicMetaLabel(topic.intent) : '', topic.evidence_status ? topicMetaLabel(topic.evidence_status) : ''].filter(Boolean);
  return (
    <div className="compact-idea-row">
      <span className="idea-number">{index}</span>
      <span className="idea-row-copy">
        <strong>{topic.title}</strong>
        {meta.length ? <small>{meta.join(' · ')}</small> : null}
      </span>
    </div>
  );
}

function EvidenceSection({ candidate }: { candidate: NicheCandidate }) {
  const queries = candidate.search_queries_used?.length ? candidate.search_queries_used : candidate.validation.search_phrases ?? [];
  return (
    <div className="evidence-section">
      <TextBlock label="Evidence summary" value={candidate.evidence_summary || candidate.validation.market_evidence_summary} />
      <LabelledChips label="Search queries used" items={queries} />
      {candidate.outliers?.length ? (
        <div className="niche-outlier-list">{candidate.outliers.map(outlier => <EvidenceVideoRow outlier={outlier} key={outlier.canonical_url} />)}</div>
      ) : (
        <div className="muted-note">No validated outlier examples are available for this evidence mode.</div>
      )}
    </div>
  );
}

function EvidenceVideoRow({ outlier }: { outlier: OutlierEvidence }) {
  return (
    <a className="niche-outlier" href={outlier.canonical_url} target="_blank" rel="noreferrer">
      {outlier.thumbnail_url ? <img src={outlier.thumbnail_url} alt="" loading="lazy" /> : <span className="niche-outlier-icon" aria-hidden="true">▶</span>}
      <span>
        <strong>{outlier.title}</strong>
        <small>{outlier.channel_name} · {formatCount(outlier.public_views)} views · {outlier.publication_age}</small>
        <em>{outlier.outlier_reason}</em>
      </span>
    </a>
  );
}

function RiskSection({ candidate }: { candidate: NicheCandidate }) {
  return (
    <div className="risk-section">
      {candidate.risks?.length ? (
        <div className="risk-list">
          {candidate.risks.map(risk => <div className="risk-row" key={risk}><span aria-hidden="true" />{risk}</div>)}
        </div>
      ) : (
        <div className="muted-note">No candidate-specific risks were returned.</div>
      )}
      <Limitations items={candidate.market_evidence?.limitations ?? []} />
    </div>
  );
}

function ScoreMethodology({ methodology, limitations }: { methodology: string[]; limitations: string[] }) {
  const reveal = useRevealClass();
  return (
    <details ref={reveal.ref as RefObject<HTMLDetailsElement>} className={`score-methodology ${reveal.className}`.trim()}>
      <summary>Score methodology</summary>
      <div className="methodology-steps">
        {methodology.length ? methodology.map((item, index) => (
          <div className="methodology-step" key={item}>
            <span>{index + 1}</span>
            <p>{item}</p>
          </div>
        )) : <div className="muted-note">No methodology details returned.</div>}
      </div>
      <div className="score-methodology-limitations">
        <Limitations items={limitations} />
      </div>
    </details>
  );
}

function ratingBandForScore(score: number): string {
  if (score <= 19) return 'very_low';
  if (score <= 39) return 'low';
  if (score <= 59) return 'moderate';
  if (score <= 79) return 'high';
  return 'very_high';
}

function formatRatingBand(value: string): string {
  const labels: Record<string, string> = {
    very_low: 'Very low',
    low: 'Low',
    moderate: 'Moderate',
    high: 'High',
    very_high: 'Very high',
  };
  return labels[value] || formatLabel(value);
}

function RadarChart({ rows }: { rows: DimensionRow[] }) {
  const { ref, inView, reducedMotion } = useInViewOnce<SVGSVGElement>(0.25);
  const animatedRadiusScale = useAnimatedNumber(1, inView, reducedMotion, 900);
  const points = rows.map((row, i) => {
    const angle = (-90 + i * (360 / rows.length)) * Math.PI / 180;
    const radius = (18 + (clampScore(row.score) / 100) * 72) * animatedRadiusScale;
    return `${100 + Math.cos(angle) * radius},${100 + Math.sin(angle) * radius}`;
  }).join(' ');
  const labelMap: Record<string, string> = {
    creator_fit: 'Creator',
    audience_demand: 'Audience',
    competition_opportunity: 'Competition',
    sustainability: 'Sustainability',
    differentiation: 'Differentiation',
  };
  return (
    <svg ref={ref} className="niche-radar" viewBox="-26 -14 252 228" role="img" aria-label="Strategic scoring radar: Creator Fit, Audience Demand, Competition Opportunity, Content Sustainability, and Differentiation">
      <desc>Strategic scoring radar with full dimension labels: Creator Fit, Audience Demand, Competition Opportunity, Content Sustainability, and Differentiation.</desc>
      <polygon className="radar-grid" points="100,20 176,75 147,165 53,165 24,75" />
      <polygon className="radar-shape" points={points} style={{ opacity: 0.08 + (animatedRadiusScale * 0.14) }} />
      {rows.map((row, i) => {
        const angle = (-90 + i * (360 / rows.length)) * Math.PI / 180;
        return <text key={row.key} x={100 + Math.cos(angle) * 112} y={104 + Math.sin(angle) * 104}>{labelMap[row.key] ?? row.label}</text>;
      })}
    </svg>
  );
}

function ContributionChart({ rows }: { rows: DimensionRow[] }) {
  return (
    <div className="niche-contribution-chart" aria-label="Deterministic weighted score contribution">
      {rows.map((row, index) => {
        const contribution = row.score * row.weight;
        return (
          <div key={row.key} className={`contribution-row accent-${row.accent}`}>
            <span>{row.label}</span>
            <LinearProgress value={contribution} max={row.weight * 100} color="var(--accent)" delay={index * 55} ariaLabel={`${row.label} contribution ${contribution.toFixed(1)} points out of ${(row.weight * 100).toFixed(0)} possible`} />
            <strong>{contribution.toFixed(1)}</strong>
          </div>
        );
      })}
    </div>
  );
}

function PillarChart({ pillars }: { pillars: ContentPillar[] }) {
  const { ref, inView, reducedMotion } = useInViewOnce<HTMLDivElement>(0.25);
  if (!pillars.length) return <div className="muted-note">No content-pillar allocation returned.</div>;
  return (
    <div ref={ref} className="pillar-chart">
      <div className="pillar-stack">
        {pillars.map((p, index) => (
          <AnimatedPillarSegment
            key={p.name}
            percentage={p.percentage ?? 0}
            title={`${p.name}: ${Math.round(p.percentage ?? 0)}%`}
            active={inView}
            reducedMotion={reducedMotion}
            delay={index * 55}
          />
        ))}
      </div>
      {pillars.map((p, index) => <div className="pillar-row accent-cyan" key={p.name}><span>{p.name}</span><LinearProgress value={p.percentage ?? 0} delay={index * 55} ariaLabel={`${p.name} content-pillar distribution ${Math.round(p.percentage ?? 0)} percent`} /><strong>{Math.round(p.percentage ?? 0)}% · {p.topic_count} topics</strong></div>)}
    </div>
  );
}

function AnimatedPillarSegment({ percentage, title, active, reducedMotion, delay }: { percentage: number; title: string; active: boolean; reducedMotion: boolean; delay: number }) {
  const delayedActive = useDelayedActive(active, delay, reducedMotion);
  const animatedValue = useAnimatedNumber(Math.max(0, Math.min(100, percentage)), delayedActive, reducedMotion, 760);
  return <span style={{ width: `${animatedValue}%` }} title={title} aria-hidden="true" />;
}

function RunwayCard({ candidate, pillars }: { candidate: NicheCandidate; pillars: ContentPillar[] }) {
  const topicCount = candidate.runway?.viable_topic_count ?? candidate.sustainability.viable_topic_count;
  const weeks = candidate.runway?.estimated_weeks;
  const capacity = candidate.runway?.weekly_capacity;
  const statement = candidate.runway?.estimated_content_runway ?? candidate.sustainability.estimated_content_runway;
  return (
    <div className="runway-card">
      <ProgressCircle value={topicCount} max={50} label={`${topicCount} ideas`} accent="cyan" size="runway" ariaLabel={`Content runway ${topicCount} ideas out of 50`} />
      <div className="runway-copy">
        <div className="runway-stat-strip">
          <MetricStat label="Ideas" value={topicCount} />
          <MetricStat label="Production weeks" value={weeks ?? 'Unavailable'} />
          <MetricStat label="Videos per week" value={capacity ?? 'Unavailable'} />
        </div>
        <p className="runway-statement">{statement || 'Runway estimate unavailable.'}</p>
        {candidate.runway?.limitation && <div className="muted-note">{candidate.runway.limitation}</div>}
      </div>
      <PillarChart pillars={pillars} />
    </div>
  );
}

function DemandEvidence({ candidate }: { candidate: NicheCandidate }) {
  const ev = candidate.market_evidence;
  if (!ev || ev.sample_size === 0) return <UnavailableChart title="No provider time series available" desc="This candidate is based on AI strategic analysis and available trend context, not fabricated historical points." />;
  return (
    <div className="demand-evidence-summary">
      <div className="demand-evidence-head">
        <StatusBadge tone={ev.status === 'live_validated' ? 'success' : 'info'}>{evidenceModeLabel(ev.status)}</StatusBadge>
      </div>
      <div className="demand-metric-row">
        <MetricStat label="Sample size" value={ev.sample_size} />
        <MetricStat label="Median views" value={ev.median_views == null ? 'Unavailable' : formatCount(ev.median_views)} />
        <MetricStat label="Engagement" value={ev.engagement == null ? 'Unavailable' : `${ev.engagement}%`} />
      </div>
      <div className="demand-meta-row">
        <span><strong>Evidence freshness:</strong> {ev.recent_activity && ev.recent_activity !== 'Unavailable' ? `last qualifying sampled upload was ${ev.recent_activity}` : 'Unavailable'}</span>
        <span><strong>Collected:</strong> {ev.collected_at ? formatCacheTime(ev.collected_at) : 'Unavailable'}</span>
      </div>
    </div>
  );
}

function MetricStat({ label, value }: { label: string; value: unknown }) {
  return (
    <div className="metric-stat">
      <span>{label}</span>
      <strong>{formatMetric(label, value)}</strong>
    </div>
  );
}

function CompetitionVisual({ candidate }: { candidate: NicheCandidate }) {
  const competition = candidate.dimensions?.competition_opportunity ?? candidate.scores.opportunity_gap;
  const demand = candidate.dimensions?.audience_demand ?? candidate.scores.demand;
  const demandSignal = candidate.market_evidence?.recent_activity || candidate.evidence_summary || candidate.validation.market_evidence_summary;
  const competitionSignal = candidate.validation.competition_level
    ? `${formatLabel(candidate.validation.competition_level)} competition from the sampled public results.`
    : competition.explanation;
  const interpretation = candidate.opportunity_gaps?.[0] || candidate.supply_gaps?.[0]?.statement || candidate.recommended_first_action;
  return (
    <div className="competition-insight-panel" aria-label="Competition opportunity insight">
      <div className="competition-insight-score">
        <strong>{Math.round(competition.score)} / 100</strong>
        <span>{formatRatingBand(competition.rating_band ?? ratingBandForScore(competition.score))}</span>
      </div>
      <div className="competition-signal-grid">
        <TextBlock label="Demand signal" value={demandSignal || demand.explanation} />
        <TextBlock label="Competition signal" value={competitionSignal} />
        <TextBlock label="Interpretation" value={interpretation} />
      </div>
      <TextBlock label="Strategic explanation" value={competition.explanation} />
    </div>
  );
}

function UnavailableChart({ title, desc }: { title: string; desc: string }) {
  return <div className="chart-unavailable"><strong>{title}</strong><p>{desc}</p></div>;
}

function AlternativeCandidates({ candidates, unavailableReason, expandedID, onToggle }: { candidates: NicheCandidate[]; unavailableReason?: string; expandedID: string | null; onToggle: (id: string | null) => void }) {
  const reveal = useRevealClass();
  if (!candidates.length) return <section ref={reveal.ref as RefObject<HTMLElement>} className={`alternative-candidates ${reveal.className} is-compact-unavailable`.trim()}><div className="settings-card-title">Alternative candidates</div><UnavailableChart title="Alternatives unavailable" desc={unavailableReason || 'We could not validate two sufficiently distinct alternatives from the available evidence. Broaden the topic or audience to explore more options.'} /></section>;
  return (
    <section ref={reveal.ref as RefObject<HTMLElement>} className={`alternative-candidates ${reveal.className}`.trim()}>
      <div className="settings-card-title">Alternative candidates</div>
      {candidates.map(candidate => {
        const isExpanded = expandedID === candidate.id;
        const title = candidate.name || candidate.niche_name;
        const score = candidate.overall_score ?? candidate.scores.overall.score;
        return (
          <button
            key={candidate.id}
            className="alternative-card"
            type="button"
            onClick={() => onToggle(isExpanded ? null : candidate.id)}
            aria-expanded={isExpanded}
          >
            <span className="alternative-card-copy">
              <strong>{title}</strong>
              <small>{candidate.unique_angle || candidate.creator_advantage}</small>
              {isExpanded && <span className="alternative-card-expanded">{candidate.concise_positioning || candidate.unique_angle}</span>}
            </span>
            <span className="alternative-card-score">
              <ProgressCircle value={score} accent="cyan" size="alternative" ariaLabel={`${title} overall score ${Math.round(score)} out of 100`} />
            </span>
            <DimensionBars rows={dimensionRows(candidate)} />
            <span className="alternative-card-meta">
              <span>{candidate.target_audience || candidate.target_viewer}</span>
              <em>{evidenceModeLabel(candidate.market_evidence?.status)}</em>
            </span>
          </button>
        );
      })}
    </section>
  );
}

function DimensionBars({ rows }: { rows: DimensionRow[] }) {
  return (
    <span className="dimension-bars" aria-label="Strategic dimension scores">
      {rows.map((row, index) => (
        <span className={`dimension-bar-row accent-${row.accent}`} key={row.key} data-score={Math.round(row.score)}>
          <span className="dimension-bar-label">{row.label}</span>
          <LinearProgress value={row.score} color="var(--accent)" delay={index * 55} ariaLabel={`${row.label} score ${Math.round(row.score)} out of 100`} />
        </span>
      ))}
    </span>
  );
}

function evidenceModeLabel(mode?: string): string {
  const labels: Record<string, string> = {
    live_validated: 'Live validated',
    cache_validated: 'Public evidence validated',
    public_evidence_validated: 'Public evidence validated',
    historical_public_evidence: 'Historical public evidence',
    limited_recent_evidence: 'Limited recent evidence',
    trend_supported: 'Evidence-informed',
    ai_strategic_analysis: 'AI strategy only',
    ai_strategy_only: 'AI strategy only',
    limited_evidence: 'Evidence unavailable',
    evidence_unavailable: 'Evidence unavailable',
  };
  return labels[String(mode ?? '')] || 'Evidence unavailable';
}

function evidenceBadgeLabel(report: NicheReport, candidate: NicheCandidate): string {
  if (report.cache?.freshness === 'stale') return 'Cached stale evidence';
  if (report.cache?.hit || candidate.market_evidence?.status === 'cache_validated') return 'Cached evidence';
  return evidenceModeLabel(candidate.market_evidence?.status || report.analysis_mode);
}

function evidenceFreshnessLabel(freshness?: string, collectedAt?: string | null): string {
  if (collectedAt) return `Collected ${formatCacheTime(collectedAt)}`;
  if (freshness === 'live') return 'Live evidence';
  if (freshness === 'cached') return 'Cached evidence';
  if (freshness === 'historical_public_evidence') return 'Historical public evidence';
  if (freshness === 'limited_recent_evidence') return 'Limited recent evidence';
  return 'Strategic only';
}

function topicMetaLabel(value: string): string {
  const labels: Record<string, string> = {
    ai_strategic_analysis: 'Strategic analysis',
    backend_response_validation: 'Strategic analysis',
    backend_title_validation: 'Strategic analysis',
    openai_structured_strategy: 'Strategic analysis',
    'evidence-backed': 'Evidence-informed',
    'related opportunity': 'Evidence-informed',
    'unvalidated idea': 'Exploratory concept',
    tutorial: 'Tutorial',
    comparison: 'Comparison',
    'case study': 'Case study',
    mistake: 'Mistakes',
    experiment: 'Experiment',
    workflow: 'Workflow',
    'beginner guide': 'Beginner guide',
    breakdown: 'Breakdown',
    analysis: 'Analysis',
    checklist: 'Checklist',
  };
  const key = String(value).trim().toLowerCase();
  if (labels[key]) return labels[key];
  if (key.includes('_')) return '';
  return formatLabel(value);
}

function TrendOpportunityCard({ result, expanded, onToggle, generated, generationError, generating, onGenerate, onOpenScriptStudio }: {
  result: TrendIntelligenceResult;
  expanded: boolean;
  onToggle: () => void;
  generated?: ReelContentPackage;
  generationError?: string;
  generating: boolean;
  onGenerate: () => void;
  onOpenScriptStudio?: () => void;
}) {
  const [imageFailed, setImageFailed] = useState(false);
  const title = result.display_title || result.keyword;
  const panelId = `trend-opportunity-panel-${stableKey(result.id)}`;
  const summary = result.summary || 'Current demand and supporting activity indicate a content opportunity.';
  const trendAge = result.trend_age || 'Current';
  const hasImage = Boolean(result.strongest_thumbnail_url && !imageFailed);
  const summarySignals = strongestSummarySignals(result);
  const handleTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return;
    event.preventDefault();
    onToggle();
  };

  return (
    <article className={`trend-opportunity-card${expanded ? ' is-expanded' : ''}`}>
      <button
        type="button"
        className="trend-opportunity-trigger"
        onClick={onToggle}
        onKeyDown={handleTriggerKeyDown}
        aria-expanded={expanded}
        aria-controls={panelId}
      >
        <TrendOpportunityImage
          className="trend-opportunity-thumb"
          title={title}
          src={hasImage ? result.strongest_thumbnail_url : undefined}
          onError={() => setImageFailed(true)}
        />
        <span className="trend-opportunity-copy">
          <span className="result-card-title trend-opportunity-title">{title}</span>
          <span className="trend-opportunity-summary">{summary}</span>
          <span className="trend-opportunity-signals">
            {summarySignals.map(signal => <span key={signal}>{signal}</span>)}
            {trendAge && <span>{trendAge}</span>}
          </span>
        </span>
        <span className="trend-opportunity-score" aria-label={`Opportunity score ${Math.round(result.opportunity_score)} out of 100`}>
          <span className="score-value">{Math.round(result.opportunity_score)}</span>
          <span>Opportunity</span>
        </span>
        <span className="trend-chevron" aria-hidden="true" />
      </button>

      <div id={panelId} className="trend-opportunity-panel" aria-hidden={!expanded}>
        <div className="trend-opportunity-panel-inner">
          <TrendOpportunityImage
            className="trend-opportunity-hero"
            title={title}
            src={hasImage ? result.strongest_thumbnail_url : undefined}
            onError={() => setImageFailed(true)}
          />
          <div className="trend-opportunity-expanded-body">
            <div className="trend-expanded-header">
              <div>
                <div className="result-card-title trend-opportunity-title">{title}</div>
                <p>{summary}</p>
              </div>
              <div className="trend-expanded-score">
                <span>{Math.round(result.opportunity_score)}</span>
                <small>Opportunity</small>
                <small>{trendAge}</small>
              </div>
            </div>

            <div className="trend-metric-tiles" aria-label="Trend opportunity metrics">
              <TrendMetricTile label="Momentum" value={result.momentum_score} />
              <TrendMetricTile label="Demand" value={result.demand_score} />
              <TrendMetricTile label="Competition" value={result.competition_score} />
              <TrendMetricTile label="Confidence" value={result.confidence_score} />
            </div>

            <section className="trend-detail-section" aria-labelledby={`${panelId}-activity`}>
              <h3 id={`${panelId}-activity`}>Activity</h3>
              <div className="trend-activity-grid">
                <TrendActivityStat label="Videos sampled" value={formatCountWithUnit(result.video_count_sampled, 'video', 'videos')} />
                <TrendActivityStat label="Sampled views" value={formatViews(result.total_sampled_views)} />
                <TrendActivityStat label="Median views" value={formatViews(result.median_sampled_views ?? result.average_sampled_views)} />
                <TrendActivityStat label="Newest activity" value={formatDateTime(result.newest_relevant_video_at)} />
              </div>
              {result.sampled_video_activity && <p className="trend-activity-note">{result.sampled_video_activity}</p>}
            </section>

            {(result.scoring_reasons?.length || result.related_keywords?.length) ? (
              <section className="trend-detail-section" aria-labelledby={`${panelId}-reasons`}>
                <h3 id={`${panelId}-reasons`}>Why it matters</h3>
                {result.scoring_reasons?.length ? (
                  <div className="trend-reason-pills">
                    {result.scoring_reasons.map(reason => <span key={reason}>{reason}</span>)}
                  </div>
                ) : null}
                {result.related_keywords?.length ? (
                  <div className="trend-related-keywords" aria-label="Related keywords">
                    {result.related_keywords.map(keyword => <span key={keyword}>{keyword}</span>)}
                  </div>
                ) : null}
              </section>
            ) : null}

            <div className="trend-opportunity-actions">
              <ScriptAction
                label="Create Script"
                generating={generating}
                generated={Boolean(generated)}
                error={generationError}
                onGenerate={onGenerate}
                onOpenScriptStudio={onOpenScriptStudio}
              />
              <div className="trend-secondary-actions">
                <button type="button" className="link-button" disabled>Analyze</button>
                {result.strongest_relevant_video_url && (
                  <a href={result.strongest_relevant_video_url} target="_blank" rel="noopener noreferrer" className="link-button">Inspect supporting content</a>
                )}
                <button type="button" className="link-button" disabled>Create clips later</button>
              </div>
            </div>
            {generated && <GeneratedPackageView pkg={generated} />}
          </div>
        </div>
      </div>
    </article>
  );
}

function TrendOpportunityImage({ className, title, src, onError }: { className: string; title: string; src?: string; onError: () => void }) {
  if (!src) {
    return (
      <span className={`${className} trend-image-fallback`} role="img" aria-label={`No supporting image available for ${title}`}>
        <span>{initialsForTitle(title)}</span>
      </span>
    );
  }
  return <img className={className} src={src} alt="" loading="lazy" onError={onError} />;
}

function TrendMetricTile({ label, value }: { label: string; value: number }) {
  const rounded = clampScore(value);
  return (
    <div className="trend-metric-tile" aria-label={`${label} ${rounded} out of 100`}>
      <div>
        <span>{label}</span>
        <strong>{rounded}</strong>
      </div>
      <div className="trend-metric-track" aria-hidden="true">
        <span style={{ width: `${rounded}%` }} />
      </div>
    </div>
  );
}

function TrendActivityStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="trend-activity-stat">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function AnalyzerShell({ title, description, inputLabel, value, onChange, onAnalyze, loading, buttonLabel, compactResult = false, children }: {
  title: string;
  description: string;
  inputLabel: string;
  value: string;
  onChange: (value: string) => void;
  onAnalyze: () => void;
  loading: boolean;
  buttonLabel: string;
  compactResult?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={`settings-card analyzer-panel${compactResult ? ' is-result-ready' : ''}`}>
      <div className="analyzer-shell-heading">
        <div className="settings-card-title">{title}</div>
        <div className="muted-note">{description}</div>
      </div>
      <div className="analyzer-form-row">
        <label className="form-group analyzer-input-group">
          <span className="form-label">{inputLabel}</span>
          <input className="form-input" value={value} onChange={event => onChange(event.target.value)} placeholder="https://www.youtube.com/..." />
        </label>
        <button className="generate-btn idle analyzer-primary-action" type="button" onClick={onAnalyze} disabled={!value.trim() || loading}>{loading ? 'Analyzing...' : buttonLabel}</button>
      </div>
      {children}
    </div>
  );
}

function DiscoveryState({ loading, error, response, filteredCount }: { loading: boolean; error: string | null; response: TrendDiscoveryResponse | null; filteredCount: number }) {
  if (loading) return <EmptyState tone="loading" title="Loading trend candidates." desc="Checking available trend sources." />;
  if (error) return <EmptyState tone="error" title="Trend discovery is unavailable." desc={error} />;
  if (response?.provider_status === 'provider_not_configured') return <EmptyState tone="warning" title="No trend source configured." desc={response.message || 'Configure a trend source in Connections to collect trends.'} />;
  if (response?.provider_status === 'no_data') return <EmptyState tone="empty" title="No trends found." desc={response.message || 'The selected trend source returned no candidates for this request.'} />;
  if (response?.provider_status === 'provider_error') return <EmptyState tone="unavailable" title="Trend source unavailable." desc={response.message || 'The selected trend source could not return results.'} />;
  if (response?.provider_status === 'ok' && filteredCount === 0) return <EmptyState tone="empty" title="No candidates for this source." desc="Try another source, region, language, or audience." />;
  return null;
}

type StateTone = 'empty' | 'loading' | 'error' | 'warning' | 'unavailable' | 'success' | 'info';

function EmptyState({ tone = 'empty', title, desc, action }: { tone?: StateTone; title: string; desc: string; action?: ReactNode }) {
  const role = tone === 'error' ? 'alert' : 'status';
  const ariaLive = tone === 'loading' || tone === 'error' || tone === 'success' ? 'polite' : undefined;
  return (
    <div className={`empty-state system-state is-${tone}`} role={role} aria-live={ariaLive}>
      <div className="empty-icon" aria-hidden="true">{stateIcon(tone)}</div>
      <div className="empty-title">{title}</div>
      <div className="empty-desc">{desc}</div>
      {action && <div className="empty-action">{action}</div>}
    </div>
  );
}

function HonestResultState({ result }: { result: { status: string; message: string; limitations?: string[] } }) {
  return (
    <div className={`neutral-callout status-banner ${result.status === 'ok' ? 'is-success' : 'is-warning'}`} role="status">
      <strong>{statusLabel(result.status)}:</strong> {result.message}
      <Limitations items={result.limitations ?? []} />
    </div>
  );
}

function ScriptAction({ label, generating, generated, error, onGenerate, onOpenScriptStudio }: {
  label: string;
  generating: boolean;
  generated: boolean;
  error?: string;
  onGenerate: () => void;
  onOpenScriptStudio?: () => void;
}) {
  return (
    <div className="script-action">
      <div className="script-action-buttons">
        <button className="generate-btn idle" type="button" onClick={onGenerate} disabled={generating}>
          {generating ? 'Generating...' : label}
        </button>
        {generated && <span className="inline-success" role="status">Script generated</span>}
        {generated && onOpenScriptStudio && (
          <button className="generate-btn idle" type="button" onClick={onOpenScriptStudio}>Open in Script Studio</button>
        )}
      </div>
      {error && <div className="inline-error" role="alert">{error}</div>}
    </div>
  );
}

function ScoreProgressBar({ dimension }: { dimension: NonNullable<YouTubeVideoAnalysisResponse['analysis_confidence']> }) {
  const score = clampScore(dimension.score);
  return (
    <div className="score-progress-row" aria-label={`${dimension.label}: ${score} out of 100, ${dimension.rating}`}>
      <div className="score-progress-header">
        <span>{dimension.label}</span>
        <strong>{score} · {dimension.rating}</strong>
      </div>
      <div className="score-progress-track" role="img" aria-label={`${dimension.label} score ${score} out of 100`}>
        <span style={{ width: `${score}%` }} />
      </div>
      {dimension.explanation && <p>{dimension.explanation}</p>}
    </div>
  );
}

function HookProfile({ hook }: { hook?: YouTubeVideoAnalysisResponse['hook_intelligence'] }) {
  if (!hook) return <div className="muted-note">No title/metadata hook profile returned.</div>;
  const dimensions = [
    { id: 'clarity', label: 'Clarity', score: hook.clarity_score ?? 0, rating: ratingForScore(hook.clarity_score ?? 0), explanation: 'How directly the title communicates the premise.' },
    { id: 'specificity', label: 'Specificity', score: hook.specificity_score ?? 0, rating: ratingForScore(hook.specificity_score ?? 0), explanation: 'How much concrete topic detail appears in the title.' },
    { id: 'curiosity', label: 'Curiosity', score: hook.curiosity_score ?? 0, rating: ratingForScore(hook.curiosity_score ?? 0), explanation: 'Whether the title creates a supportable curiosity gap.' },
    { id: 'audience', label: 'Audience signal', score: hook.audience_signal_score ?? 0, rating: ratingForScore(hook.audience_signal_score ?? 0), explanation: 'Whether the likely viewer is explicit in the title.' },
    { id: 'value', label: 'Value promise', score: hook.value_promise_score ?? 0, rating: ratingForScore(hook.value_promise_score ?? 0), explanation: 'Whether the title promises a concrete viewer outcome.' },
  ];
  return (
    <div className="score-progress-list">
      <MetricGrid values={{ 'Hook type': hook.hook_type, 'Title length': hook.title_length, 'Remake potential': hook.remake_potential_score }} />
      {dimensions.map(dimension => <ScoreProgressBar key={dimension.id} dimension={dimension} />)}
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
  const id = useId();
  return (
    <div className="form-group">
      <label className="form-label" htmlFor={id}>{label}</label>
      <select id={id} className="form-input" value={value} onChange={event => onChange(event.target.value)}>
        {options.map(option => <option key={`${option.label}-${option.value}`} value={option.value}>{option.label}</option>)}
      </select>
    </div>
  );
}

function TextInput({ label, value, onChange, placeholder, className = '', multiline = false }: { label: string; value: string; onChange: (value: string) => void; placeholder: string; className?: string; multiline?: boolean }) {
  const id = useId();
  return (
    <div className={`form-group ${className}`.trim()}>
      <label className="form-label" htmlFor={id}>{label}</label>
      {multiline ? (
        <textarea id={id} className="form-input niche-textarea" value={value} onChange={event => onChange(event.target.value)} placeholder={placeholder} rows={2} />
      ) : (
        <input id={id} className="form-input" value={value} onChange={event => onChange(event.target.value)} placeholder={placeholder} />
      )}
    </div>
  );
}

function MetricGrid({ values }: { values: Record<string, unknown> }) {
  const entries = Object.entries(values).filter(([, value]) => value != null && value !== '');
  if (!entries.length) return <div className="muted-note">No data returned for this field.</div>;
  return (
    <div className="status-list">
      {entries.map(([label, value]) => (
        <div className="status-row" key={label}>
          <span>{formatLabel(label)}</span>
          <strong>{formatMetric(label, value)}</strong>
        </div>
      ))}
    </div>
  );
}

function ReadinessPanel({ rows }: { rows: { label: string; value: unknown; type: 'value' | 'status' }[] }) {
  return (
    <div className="readiness-table">
      {rows.map(row => {
        const status = row.type === 'status' ? semanticStatus(row.value) : null;
        return (
          <div className="readiness-row" key={row.label}>
            <span>{row.label}</span>
            {status ? (
              <strong className={`readiness-badge ${status.className}`}>{status.label}</strong>
            ) : (
              <strong>{formatMetric(row.label, row.value)}</strong>
            )}
          </div>
        );
      })}
    </div>
  );
}

function LabelledChips({ label, items }: { label: string; items: string[] }) {
  if (!items.length) return null;
  return (
    <div style={{ display: 'grid', gap: 6, marginTop: 8 }}>
      <div style={{ fontSize: 11, fontWeight: 800, color: 'var(--text-dim)', textTransform: 'uppercase' }}>{label}</div>
      <ChipList items={items} />
    </div>
  );
}

function SectionList({ label, items }: { label: string; items: string[] }) {
  if (!items.length) return null;
  return (
    <div className="section-list" style={{ display: 'grid', gap: 6, marginTop: 8 }}>
      <div style={{ fontSize: 11, fontWeight: 800, color: 'var(--text-dim)', textTransform: 'uppercase' }}>{label}</div>
      <List items={items} />
    </div>
  );
}

function ChipList({ items }: { items: string[] }) {
  if (!items.length) return <div className="muted-note">No data returned for this field.</div>;
  return <div className="platform-toggles">{items.map(item => <span key={item} className="platform-toggle">{item}</span>)}</div>;
}

function List({ items }: { items: string[] }) {
  if (!items.length) return <div className="muted-note">No data returned for this field.</div>;
  return <div className="compact-insight-list" style={{ display: 'grid', gap: 8 }}>{items.map(item => <div key={item} className="small-capability">{item}</div>)}</div>;
}

function VideoSummaryList({ videos }: { videos: NonNullable<YouTubeChannelAnalysisResponse['top_videos_summary']> }) {
  if (!videos.length) return <div className="muted-note">No public top video metadata returned for this field.</div>;
  return (
    <div style={{ display: 'grid', gap: 8 }}>
      {videos.slice(0, 10).map(video => (
        <div key={video.video_id || video.title} className="small-capability">
          <strong>{video.title}</strong>
          <div style={{ marginTop: 4, color: 'var(--text-dim)' }}>{formatMetric('views', video.views)} views · {video.published_at || 'date unavailable'}</div>
        </div>
      ))}
    </div>
  );
}

function KeywordClusterList({ clusters, fallback }: { clusters: NonNullable<YouTubeChannelAnalysisResponse['keyword_clusters']>; fallback: string[] }) {
  if (!clusters.length) return <ChipList items={fallback} />;
  return (
    <div style={{ display: 'grid', gap: 8 }}>
      {clusters.map(cluster => (
        <div key={cluster.name} className="small-capability">
          <strong>{cluster.name}</strong>
          {cluster.terms?.length ? <div style={{ marginTop: 4, color: 'var(--text-muted)' }}>{cluster.terms.join(', ')}</div> : null}
          {cluster.evidence?.length ? <div style={{ marginTop: 4, color: 'var(--text-dim)' }}>Evidence: {cluster.evidence.slice(0, 2).join(' · ')}</div> : null}
        </div>
      ))}
    </div>
  );
}

function Limitations({ items }: { items: string[] }) {
  if (!items.length) return null;
  return (
    <div className="muted-note" style={{ marginTop: 8 }}>
      <strong>Limitations:</strong> {items.join(' ')}
    </div>
  );
}

function statusLabel(status: string): string {
  return userStatusLabel(status);
}

function stateIcon(tone: StateTone): string {
  if (tone === 'loading') return '...';
  if (tone === 'error') return '!';
  if (tone === 'warning') return '!';
  if (tone === 'success') return 'ok';
  if (tone === 'unavailable') return '-';
  if (tone === 'info') return 'i';
  return '0';
}

function userStatusLabel(status: unknown): string {
  const normalized = String(status ?? '').trim().toLowerCase();
  const labels: Record<string, string> = {
    active: 'Ready',
    ready: 'Ready',
    ok: 'Ready',
    configured: 'Ready',
    not_configured: 'Setup needed',
    not_connected: 'Setup needed',
    provider_not_configured: 'Setup needed',
    unknown: 'Status unavailable',
    provider_error: 'Unavailable',
    unavailable: 'Unavailable',
    checking: 'Checking',
  };
  return labels[normalized] || normalized.replaceAll('_', ' ') || 'Status unavailable';
}

function semanticStatus(status: unknown): { label: string; className: string } {
  const normalized = String(status ?? '').trim().toLowerCase();
  if (['active', 'ready', 'ok', 'configured'].includes(normalized)) {
    return { label: 'Ready', className: 'is-ready' };
  }
  if (['not_configured', 'not_connected', 'provider_not_configured'].includes(normalized)) {
    return { label: 'Setup needed', className: 'is-setup' };
  }
  if (['checking', 'loading', 'pending'].includes(normalized)) {
    return { label: 'Checking', className: 'is-checking' };
  }
  if (['provider_error', 'error', 'unavailable'].includes(normalized)) {
    return { label: 'Unavailable', className: 'is-unavailable' };
  }
  return { label: 'Status unavailable', className: 'is-unavailable' };
}

type CreativeCard = {
  id: string;
  icon: string;
  title: string;
  description: string;
  tag: string;
};

function videoCoreTopic(result: YouTubeVideoAnalysisResponse) {
  const niche = result.niche_analysis;
  return {
    main: niche?.specific_topic || niche?.niche || niche?.primary_niche || result.inferred_niche || result.keyword_intelligence?.primary_keywords?.[0] || 'Video topic',
    specific: niche?.specific_topic || result.keyword_intelligence?.primary_keywords?.[0] || result.title || 'Topic unavailable',
    category: niche?.broad_category || result.category || 'Public video',
    niche: niche?.niche || niche?.primary_niche || result.inferred_niche || 'Niche unavailable',
    format: niche?.content_format || formatVideoFormat(result.formatted_metadata?.format),
  };
}

function videoAudience(result: YouTubeVideoAnalysisResponse): string {
  return result.niche_analysis?.target_audience || result.niche_analysis?.audience_type || 'Creator audience';
}

function analysisSummary(result: YouTubeVideoAnalysisResponse): string {
  const score = summaryOpportunityScore(result);
  const topic = videoCoreTopic(result).main;
  const signal = score >= 75 ? 'strong creator opportunity' : score >= 55 ? 'workable creator opportunity' : 'limited creator opportunity';
  return `${topic} shows a ${signal} based on public packaging, topic clarity, and visible engagement.`;
}

function strongestDimension(dimensions: ScoreDimension[]): ScoreDimension | undefined {
  return [...dimensions].sort((a, b) => clampScore(b.score) - clampScore(a.score))[0];
}

function weakestDimension(dimensions: ScoreDimension[]): ScoreDimension | undefined {
  return [...dimensions].sort((a, b) => clampScore(a.score) - clampScore(b.score))[0];
}

function firstUseful(...groups: Array<Array<string | undefined> | undefined>): string {
  for (const group of groups) {
    const value = group?.find(item => typeof item === 'string' && item.trim().length > 0);
    if (value) return value;
  }
  return '';
}

function concise(value?: string, max = 104): string {
  const clean = String(value || 'No public signal returned for this item.').replace(/\s+/g, ' ').trim();
  if (clean.length <= max) return clean;
  const clipped = clean.slice(0, max - 1).replace(/\s+\S*$/, '');
  return `${clipped}.`;
}

function hookWeakness(result: YouTubeVideoAnalysisResponse): string {
  const hook = result.hook_intelligence;
  const scores = [
    { label: 'audience', score: hook?.audience_signal_score ?? 100, copy: 'The audience is not explicit enough for fast viewer recognition.' },
    { label: 'specificity', score: hook?.specificity_score ?? 100, copy: 'The title could use more concrete topic detail.' },
    { label: 'curiosity', score: hook?.curiosity_score ?? 100, copy: 'The curiosity gap could be sharper without becoming vague.' },
    { label: 'value', score: hook?.value_promise_score ?? 100, copy: 'The viewer outcome could be stated more clearly.' },
  ].sort((a, b) => a.score - b.score);
  return scores[0]?.copy || 'The title can be tightened for clearer audience and payoff.';
}

function topicOpportunity(result: YouTubeVideoAnalysisResponse): string {
  return result.niche_analysis?.inferred_content_angle || result.inferred_content_angle || `Repackage ${videoCoreTopic(result).main} for a more specific viewer.`;
}

function videoScoreGroups(result: YouTubeVideoAnalysisResponse): ScoreDimension[] {
  const dimensions = result.score_dimensions ?? [];
  const byText = (terms: string[]) => dimensions.find(dimension => {
    const text = `${dimension.id} ${dimension.label}`.toLowerCase();
    return terms.some(term => text.includes(term));
  });
  const hook = result.hook_intelligence;
  const fallback = (id: string, label: string, score: number, explanation: string): ScoreDimension => ({
    id,
    label,
    score: clampScore(score),
    rating: ratingForScore(score),
    explanation,
  });
  return [
    byText(['hook', 'packag', 'title']) || fallback('packaging', 'Packaging', averageScores([hook?.clarity_score, hook?.specificity_score, hook?.value_promise_score]), 'How well the title and public metadata package the idea.'),
    byText(['keyword', 'topic', 'metadata']) || fallback('topic-clarity', 'Topic Clarity', result.keyword_intelligence?.metadata_strength_score ?? summaryOpportunityScore(result), 'How clearly the public metadata explains the topic.'),
    byText(['audience', 'niche']) || fallback('audience-fit', 'Audience Fit', hook?.audience_signal_score ?? result.niche_analysis?.confidence ?? summaryOpportunityScore(result), 'How clearly the video points to a specific viewer.'),
    byText(['discover', 'search', 'keyword']) || fallback('discoverability', 'Discoverability', result.keyword_intelligence?.metadata_strength_score ?? summaryOpportunityScore(result), 'How much search and metadata signal supports discovery.'),
    byText(['remake', 'opportun', 'creator']) || fallback('remake-potential', 'Remake Potential', hook?.remake_potential_score ?? summaryOpportunityScore(result), 'How reusable the idea is for another creator angle.'),
  ].slice(0, 5);
}

function averageScores(values: Array<number | undefined>): number {
  const valid = values.filter((value): value is number => typeof value === 'number' && Number.isFinite(value));
  if (!valid.length) return 50;
  return valid.reduce((sum, value) => sum + value, 0) / valid.length;
}

function uniqueStrings(items: string[]): string[] {
  const seen = new Set<string>();
  return items
    .map(item => item.trim())
    .filter(item => {
      const key = item.toLowerCase();
      if (!item || seen.has(key)) return false;
      seen.add(key);
      return true;
    });
}

function humanizeLabel(value?: string): string {
  return String(value || 'Unavailable')
    .replaceAll('_', ' ')
    .replace(/\b\w/g, char => char.toUpperCase());
}

function hookExplanation(id: string): string {
  const copy: Record<string, string> = {
    clarity: 'How directly the title communicates the premise.',
    specificity: 'How much concrete topic detail appears in the title.',
    curiosity: 'Whether the title creates a supportable curiosity gap.',
    audience: 'Whether the likely viewer is explicit in the title.',
    value: 'Whether the title promises a concrete viewer outcome.',
  };
  return copy[id] || 'Public title packaging signal.';
}

function improvedTitle(result: YouTubeVideoAnalysisResponse): string {
  const existing = result.creator_opportunities?.title_ideas?.[0];
  if (existing) return existing;
  const topic = videoCoreTopic(result).main;
  const audience = videoAudience(result);
  return `${topic}: a practical guide for ${audience}`;
}

function creativeCards(items: string[], fallbackLabel: string, tag: string): CreativeCard[] {
  return uniqueStrings(items).map((item, index) => {
    const title = shortCreativeTitle(item, fallbackLabel, index);
    return {
      id: `${fallbackLabel}-${stableKey(item)}-${index}`,
      icon: fallbackLabel.slice(0, 1).toUpperCase(),
      title,
      description: concise(item, 118),
      tag,
    };
  });
}

function shortCreativeTitle(value: string, fallbackLabel: string, index: number): string {
  const clean = value.replace(/\s+/g, ' ').trim();
  const beforeColon = clean.split(':')[0]?.trim();
  if (beforeColon && beforeColon.length >= 4 && beforeColon.length <= 44) return beforeColon;
  const words = clean.split(' ').slice(0, 5).join(' ');
  return words || `${fallbackLabel} ${index + 1}`;
}

function formatDateLabel(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function candidateFromVideo(result: YouTubeVideoAnalysisResponse, region: string, language: string): TrendCandidate {
  const keyword = result.keyword_intelligence?.primary_keywords?.[0] || result.extracted_keywords?.[0] || result.title || 'YouTube video analysis';
  return {
    id: `youtube-video-${result.video_id || Date.now()}`,
    source: 'youtube_video_analysis',
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

function candidateFromTrendIntelligence(result: TrendIntelligenceResult, region: string, language: string): TrendCandidate {
  return {
    id: result.id,
    source: 'google_trend',
    region,
    language,
    keyword: result.keyword,
    title: result.display_title || result.keyword,
    score: result.opportunity_score,
    velocity: result.momentum_score,
    discovered_at: result.discovered_at,
    source_url: result.strongest_relevant_video_url,
    evidence: [
      result.summary,
      result.sampled_video_activity,
      `Opportunity ${Math.round(result.opportunity_score)}, momentum ${Math.round(result.momentum_score)}, demand ${Math.round(result.demand_score)}, competition ${Math.round(result.competition_score)}.`,
      ...(result.scoring_reasons ?? []),
    ].filter(Boolean).join(' '),
    status: 'discovered',
  };
}

function candidateFromChannel(result: YouTubeChannelAnalysisResponse, idea: string, region: string, language: string): TrendCandidate {
  return {
    id: `youtube-channel-${result.channel_id || Date.now()}-${stableKey(idea)}`,
    source: 'youtube_channel_analysis',
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

function nicheCandidateKey(candidate: NicheCandidate): string {
  return `niche-${stableKey(`${candidate.name || candidate.niche_name}-${candidate.validation.competition_level}-${candidate.recommended_content_format}`)}`;
}

function trendCandidateFromNicheCandidate(candidate: NicheCandidate, region: string, language: string): TrendCandidate {
  return {
    id: nicheCandidateKey(candidate),
    source: 'niche_idea',
    region,
    language,
    keyword: candidate.core_phrase,
    title: candidate.name || candidate.niche_name,
    score: candidate.overall_score ?? candidate.scores.overall.score,
    discovered_at: new Date().toISOString(),
    evidence: candidate.validation.market_evidence_summary,
    status: 'discovered',
  };
}

function researchScriptFromNicheCandidate(candidate: NicheCandidate, region: string, language: string): ResearchScriptGenerationRequest {
  return {
    source_type: 'niche_idea',
    source_id: nicheCandidateKey(candidate),
    topic: candidate.name || candidate.niche_name,
    title: candidate.first_10_titles?.[0] || candidate.recommended_titles?.[0]?.title || candidate.name || candidate.niche_name,
    summary: [
      `Niche score: ${Math.round(candidate.overall_score ?? candidate.scores.overall.score)}.`,
      `Positioning: ${candidate.concise_positioning || candidate.unique_angle || ''}.`,
      `Audience: ${candidate.target_audience || candidate.target_viewer}.`,
      (candidate.audience_problems ?? [candidate.viewer_problem]).filter(Boolean).join(' '),
      (candidate.creator_advantages ?? [candidate.creator_advantage]).filter(Boolean).join(' '),
      candidate.evidence_summary || candidate.validation.market_evidence_summary,
      `Sustainability: ${candidate.runway?.viable_topic_count ?? candidate.sustainability.viable_topic_count} viable topics across ${(candidate.content_pillars ?? candidate.topic_pillars ?? []).length} pillars.`,
      (candidate.first_10_titles ?? candidate.recommended_titles?.map(topic => topic.title) ?? []).slice(0, 5).join(' '),
    ].filter(Boolean).join(' '),
    keywords: [
      candidate.name || candidate.niche_name,
      candidate.core_phrase,
      ...(candidate.search_queries_used?.length ? candidate.search_queries_used : candidate.validation.search_phrases ?? []),
    ],
    inferred_niche: candidate.name || candidate.niche_name,
    inferred_angle: candidate.recommended_first_action,
    performance_signals: {
      demand_score: candidate.dimensions?.audience_demand?.score ?? candidate.scores.demand.score,
      creator_fit_score: candidate.dimensions?.creator_fit?.score ?? candidate.scores.personal_fit.score,
      opportunity_gap_score: candidate.dimensions?.competition_opportunity?.score ?? candidate.scores.opportunity_gap.score,
      sustainability_score: candidate.dimensions?.sustainability?.score ?? candidate.scores.sustainability.score,
      differentiation_score: candidate.dimensions?.differentiation?.score,
      opportunity_score: candidate.overall_score ?? candidate.scores.overall.score,
      competition_level: candidate.validation.competition_level,
      evidence_mode: candidate.market_evidence?.status,
    },
    suggested_angle: candidate.recommended_first_action,
    target_platforms: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
    content_style: candidate.recommended_content_format,
    duration_seconds: 60,
    evidence: {
      path: [candidate.level_1, candidate.level_2, candidate.level_3],
      validation: candidate.validation,
      outliers: candidate.outliers,
      supply_gaps: candidate.supply_gaps,
      risks: candidate.risks,
      first_10_video_ideas: candidate.first_10_titles ?? candidate.recommended_titles?.slice(0, 10).map(topic => topic.title),
      market_evidence: candidate.market_evidence,
    },
    metadata: {
      source_provider: 'niche_research_engine',
      score: candidate.overall_score ?? candidate.scores.overall.score,
      confidence: candidate.scores.confidence.score / 100,
      platform: 'youtube',
    },
    limitations: [
      'Niche Finder does not include private analytics, RPM, revenue projections, advertiser-demand claims, or guaranteed outcomes.',
    ],
    language,
    region,
  };
}

function researchScriptFromVideo(result: YouTubeVideoAnalysisResponse, region: string, language: string): ResearchScriptGenerationRequest {
  const title = result.title || 'YouTube video analysis';
  const suggestedAngle = result.creator_opportunities?.suggested_remake_angles?.[0] || result.suggested_remake_angles?.[0];
  return {
    source_type: 'youtube_video_analysis',
    source_id: result.video_id,
    source_url: result.metadata?.source_url || result.video_url,
    topic: result.keyword_intelligence?.primary_keywords?.[0] || result.extracted_keywords?.[0] || title,
    title,
    summary: [
      result.message,
      `Hook type: ${result.hook_intelligence?.hook_type || result.hook_analysis || 'read from the title pattern'}.`,
      `Target audience: ${result.niche_analysis?.target_audience || result.niche_analysis?.audience_type || 'general viewers'}.`,
      `Performance context: ${formatEvidenceSummary(result.performance_signals ?? {})}.`,
      `Suggested angle: ${suggestedAngle || result.niche_analysis?.inferred_content_angle || result.inferred_content_angle || ''}.`,
      result.keyword_intelligence?.inferred_search_intent,
    ].filter(Boolean).join(' '),
    keywords: [
      ...(result.keyword_intelligence?.primary_keywords ?? []),
      ...(result.keyword_intelligence?.secondary_keywords ?? []),
      ...(result.keyword_intelligence?.long_tail_phrases ?? []),
      ...(result.tags ?? []),
    ],
    inferred_niche: result.niche_analysis?.primary_niche || result.inferred_niche,
    inferred_angle: result.niche_analysis?.inferred_content_angle || result.inferred_content_angle,
    performance_signals: {
      views: result.views,
      likes: result.likes,
      comments: result.comments,
      ...(result.performance_signals ?? {}),
    },
    suggested_angle: suggestedAngle,
    target_platforms: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
    content_style: 'Short-form remake script from public YouTube metadata',
    duration_seconds: 30,
    evidence: {
      channel_title: result.channel_title,
      channel_id: result.channel_id,
      published_at: result.published_at,
      category: result.category,
      duration: result.duration,
      public_topic_details: result.public_topic_details,
      primary_niche: result.niche_analysis?.primary_niche,
      target_audience: result.niche_analysis?.target_audience || result.niche_analysis?.audience_type,
      hook_type: result.hook_intelligence?.hook_type,
      primary_keywords: result.keyword_intelligence?.primary_keywords,
      script_prompts: result.creator_opportunities?.script_prompts,
      score_reason: result.metadata?.score_reason,
    },
    metadata: result.metadata ? { ...result.metadata } : undefined,
    limitations: result.limitations,
    language,
    region,
  };
}

function researchScriptFromChannelIdea(result: YouTubeChannelAnalysisResponse, idea: string, region: string, language: string): ResearchScriptGenerationRequest {
  return {
    source_type: 'youtube_channel_analysis',
    source_id: `${result.channel_id || 'channel'}-${stableKey(idea)}`,
    source_url: result.metadata?.source_url || result.channel_url,
    topic: idea,
    title: idea,
    summary: [
      result.message,
      result.likely_strategy,
      result.opportunities?.join(' '),
      `Niche: ${result.niche_analysis?.primary_niche || result.channel_niche || 'based on visible channel patterns'}.`,
      `Audience: ${result.niche_analysis?.audience_type || 'general viewers'}.`,
      `Performance distribution: ${formatEvidenceSummary(result.performance_distribution ?? result.view_distribution ?? {})}.`,
    ].filter(Boolean).join(' '),
    keywords: [
      ...(result.keyword_intelligence?.primary_keywords ?? []),
      ...(result.keyword_intelligence?.secondary_keywords ?? []),
      ...(result.keyword_intelligence?.long_tail_phrases ?? []),
      ...(result.content_pillars ?? []),
      ...(result.top_video_topics ?? []),
    ],
    inferred_niche: result.niche_analysis?.primary_niche || result.channel_niche,
    inferred_angle: result.likely_strategy,
    performance_signals: {
      subscribers: result.subscribers,
      views: result.views,
      video_count: result.video_count,
      subscriber_view_ratio: result.subscriber_view_ratio,
      ...(result.view_distribution ?? {}),
    },
    suggested_angle: idea,
    target_platforms: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
    content_style: 'Short-form script based on public channel strategy analysis',
    duration_seconds: 30,
    evidence: {
      channel_title: result.channel_title,
      channel_id: result.channel_id,
      country: result.country,
      content_pillars: result.content_pillars,
      keyword_clusters: result.keyword_clusters,
      format_patterns: result.format_patterns,
      title_patterns: result.title_patterns,
      top_videos_summary: result.top_videos_summary,
      score_reason: result.metadata?.score_reason,
    },
    metadata: result.metadata ? { ...result.metadata } : undefined,
    limitations: result.limitations,
    language,
    region,
  };
}

function stableKey(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '').slice(0, 60) || 'idea';
}

function clampScore(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

function ratingForScore(value: number): string {
  const score = clampScore(value);
  if (score >= 85) return 'Strong';
  if (score >= 70) return 'Good';
  if (score >= 50) return 'Moderate';
  if (score >= 30) return 'Limited';
  return 'Weak';
}

function summaryOpportunityScore(result: YouTubeVideoAnalysisResponse): number {
  const dimensions = result.score_dimensions ?? [];
  if (!dimensions.length) return clampScore(result.metadata?.score ?? 0);
  const weighted = dimensions.reduce((sum, dimension) => sum + clampScore(dimension.score), 0) / dimensions.length;
  return clampScore(weighted);
}

function formatVideoFormat(value?: string): string {
  const labels: Record<string, string> = {
    long_form: 'Long-form',
    short_form: 'Short-form',
    livestream: 'Livestream',
    unknown: 'Unknown',
  };
  return labels[String(value ?? '').toLowerCase()] ?? 'Unknown';
}

function formatCurrencyEstimate(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  return `$${Math.round(value).toLocaleString()}`;
}

function displayRevenueRange(estimate?: YouTubeVideoAnalysisResponse['revenue_estimate']): string {
  if (!estimate) return 'Unavailable';
  if (Number.isFinite(estimate.low) && Number.isFinite(estimate.high)) {
    return `${formatCurrencyEstimate(estimate.low)}-${formatCurrencyEstimate(estimate.high)}`;
  }
  return estimate.formatted_range || 'Unavailable';
}

function displayCompactRevenueRange(estimate?: YouTubeVideoAnalysisResponse['revenue_estimate']): string {
  if (!estimate) return 'Unavailable';
  if (Number.isFinite(estimate.low) && Number.isFinite(estimate.high)) {
    return `${formatCompactCurrency(estimate.low)}-${formatCompactCurrency(estimate.high)}`;
  }
  return estimate.formatted_range || 'Unavailable';
}

function formatCompactCurrency(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  const abs = Math.abs(value);
  if (abs >= 1_000_000) return `$${(value / 1_000_000).toLocaleString(undefined, { maximumFractionDigits: 1 })}M`;
  if (abs >= 1_000) return `$${(value / 1_000).toLocaleString(undefined, { maximumFractionDigits: abs >= 10_000 ? 0 : 1 })}K`;
  return formatCurrencyEstimate(value);
}

function formatCompactNumber(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  const abs = Math.abs(value);
  const format = (divisor: number, suffix: string) => `${(value / divisor).toLocaleString(undefined, { maximumFractionDigits: value >= divisor * 10 ? 0 : 1 })}${suffix}`;
  if (abs >= 1_000_000_000) return format(1_000_000_000, 'B');
  if (abs >= 1_000_000) return format(1_000_000, 'M');
  if (abs >= 1_000) return format(1_000, 'K');
  return Math.round(value).toLocaleString();
}

function formatCount(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  return Math.round(value).toLocaleString();
}

function formatViews(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  return `${formatCompactNumber(value)} views`;
}

function formatCountWithUnit(value: number | undefined, singular: string, plural: string): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  const rounded = Math.round(value);
  return `${rounded.toLocaleString()} ${rounded === 1 ? singular : plural}`;
}

function formatDateTime(value?: string): string {
  if (!value) return 'Unavailable';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' });
}

function initialsForTitle(title: string): string {
  const initials = title
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(word => word[0]?.toUpperCase())
    .join('');
  return initials || 'TI';
}

function strongestSummarySignals(result: TrendIntelligenceResult): string[] {
  const signals = [
    `Momentum ${clampScore(result.momentum_score)}`,
    `Competition ${clampScore(result.competition_score)}`,
  ];
  if ((result.video_count_sampled ?? 0) > 0) {
    signals.unshift(formatCountWithUnit(result.video_count_sampled, 'video sampled', 'videos sampled'));
  }
  return signals.slice(0, 3);
}

function formatEvidenceSummary(values: Record<string, unknown>): string {
  const parts = Object.entries(values)
    .filter(([, value]) => value != null && value !== '')
    .slice(0, 6)
    .map(([label, value]) => `${formatLabel(label)} ${formatMetric(label, value)}`);
  return parts.length ? parts.join(', ') : 'public performance data unavailable or hidden';
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
        Generated from real research
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

void AUDIENCE_OPTIONS;
void DiscoveryState;

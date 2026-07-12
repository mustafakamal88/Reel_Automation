import { useCallback, useEffect, useId, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react';
import type { Platform, View } from '../types';
import {
  ApiError,
  analyzeNicheOpportunities,
  analyzeYouTubeChannel,
  analyzeYouTubeVideo,
  generateResearchScript,
  getTrendFilters,
  getResearchProviderStatus,
  searchTrendIntelligence,
  type NicheOpportunity,
  type NicheOpportunityResponse,
  type ReelContentPackage,
  type ResearchScriptGenerationRequest,
  type ResearchScriptSourceType,
  type ResearchProviderStatus,
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
    description: 'Evaluate a seed topic across demand, competition, monetization context and provider readiness.',
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

  function generateFromNicheOpportunity(opportunity: NicheOpportunity) {
    const key = nicheOpportunityKey(opportunity);
    const candidate = candidateFromNicheOpportunity(opportunity);
    handleGenerateResearch(key, candidate, researchScriptFromNicheOpportunity(opportunity));
  }

  const meta = AI_TOOL_PAGE_META[tool];
  const analyzerTool = tool === 'youtubeVideoAnalyzer' || tool === 'youtubeChannelAnalyzer';

  return (
    <section className={`page-section${analyzerTool ? ' analyzer-page' : ''}`}>
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
            onGenerate={generateFromNicheOpportunity}
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
    >
      {!result && <div className="neutral-callout">Connect YouTube in Connections to analyze videos.</div>}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <div style={{ display: 'grid', gap: 10 }}>
          <ResultCard title="Video snapshot">
            <TextBlock label="Title" value={result.title} />
            <MetricGrid values={{ Channel: result.channel_title, Published: result.published_at, Duration: result.duration, Views: result.views, Likes: result.likes, Comments: result.comments }} />
            <TextBlock label="Evidence" value={result.metadata?.score_reason} />
          </ResultCard>
          <ResultCard title="Performance indicators">
            <MetricGrid values={result.performance_signals ?? {}} />
          </ResultCard>
          <ResultCard title="Keyword intelligence">
            <TextBlock label="Inferred search intent" value={result.keyword_intelligence?.inferred_search_intent} />
            <MetricGrid values={{ 'Metadata strength': result.keyword_intelligence?.metadata_strength_score }} />
            <LabelledChips label="Primary keywords" items={result.keyword_intelligence?.primary_keywords ?? result.extracted_keywords ?? []} />
            <LabelledChips label="Secondary keywords" items={result.keyword_intelligence?.secondary_keywords ?? []} />
            <LabelledChips label="Long-tail phrases" items={result.keyword_intelligence?.long_tail_phrases ?? []} />
            <LabelledChips label="Public hashtags" items={result.keyword_intelligence?.hashtags ?? []} />
          </ResultCard>
          <ResultCard title="Hook analysis">
            <MetricGrid values={{
              'Hook type': result.hook_intelligence?.hook_type,
              'Title length': result.hook_intelligence?.title_length,
              'Clarity score': result.hook_intelligence?.clarity_score,
              'Curiosity score': result.hook_intelligence?.curiosity_score,
              'Remake potential': result.hook_intelligence?.remake_potential_score,
            }} />
            <TextBlock label="Title pattern" value={result.hook_intelligence?.title_pattern || result.title_structure_analysis} />
            <LabelledChips label="Emotional triggers" items={result.hook_intelligence?.emotional_triggers ?? []} />
          </ResultCard>
          <ResultCard title="Niche analysis">
            <MetricGrid values={{
              'Primary niche': result.niche_analysis?.primary_niche || result.inferred_niche,
              'Sub-niche': result.niche_analysis?.sub_niche,
              'Audience': result.niche_analysis?.target_audience || result.niche_analysis?.audience_type,
              'Content format': result.niche_analysis?.content_format,
              Confidence: result.niche_analysis?.confidence,
            }} />
            <TextBlock label="Inferred content angle" value={result.niche_analysis?.inferred_content_angle || result.inferred_content_angle} />
            <LabelledChips label="Evidence terms" items={result.niche_analysis?.evidence_terms ?? []} />
          </ResultCard>
          <ResultCard title="Creator opportunities">
            <SectionList label="Suggested remake angles" items={result.creator_opportunities?.suggested_remake_angles ?? result.suggested_remake_angles ?? []} />
            <SectionList label="Title ideas" items={result.creator_opportunities?.title_ideas ?? []} />
            <SectionList label="Short-form clip ideas" items={result.creator_opportunities?.short_form_clip_ideas ?? []} />
            <SectionList label="Script prompts" items={result.creator_opportunities?.script_prompts ?? []} />
            <ScriptAction
              label="Generate Script from this analysis"
              generating={generating}
              generated={Boolean(generatedPackage)}
              error={generationError}
              onGenerate={onGenerate}
              onOpenScriptStudio={onOpenScriptStudio}
            />
          </ResultCard>
          <Limitations items={result.limitations ?? []} />
        </div>
      )}
    </AnalyzerShell>
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

function NicheFinderTab({ providers, region, language, audience, onGenerate, generated, generationErrors, generatingID, onOpenScriptStudio }: {
  providers: ResearchProviderStatus[];
  region: string;
  language: string;
  audience: string;
  onGenerate: (opportunity: NicheOpportunity) => void;
  generated: Record<string, ReelContentPackage>;
  generationErrors: Record<string, string>;
  generatingID: string | null;
  onOpenScriptStudio?: () => void;
}) {
  const [seedKeyword, setSeedKeyword] = useState('');
  const [platform, setPlatform] = useState('youtube');
  const [localAudience, setLocalAudience] = useState(audience);
  const [contentStyle, setContentStyle] = useState('short-form explainers');
  const [monetizationGoal, setMonetizationGoal] = useState('ads, affiliates, and products');
  const [creatorSkillLevel, setCreatorSkillLevel] = useState('intermediate');
  const [difficulty, setDifficulty] = useState('medium');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<NicheOpportunityResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const providerMap = new Map(providers.map(provider => [provider.id, provider]));

  function runAnalysis() {
    if (!seedKeyword.trim()) return;
    setLoading(true);
    setError(null);
    setResult(null);
    analyzeNicheOpportunities({
      seed_keyword: seedKeyword.trim(),
      platform,
      country: region,
      language,
      audience: localAudience || audience,
      content_style: contentStyle,
      monetization_goal: monetizationGoal,
      creator_skill_level: creatorSkillLevel,
      production_difficulty_preference: difficulty,
    })
      .then(setResult)
      .catch(err => setError(err instanceof ApiError ? err.message : 'Niche opportunity analysis failed.'))
      .finally(() => setLoading(false));
  }

  return (
    <div className="niche-workflow">
      <div className="settings-card niche-form-card">
        <div>
          <div className="settings-card-title">Niche Finder workflow</div>
          <div className="muted-note">Define the creator context, then choose the content and monetization strategy for the analysis.</div>
        </div>
          <div className="niche-form-sections">
            <div className="niche-form-section">
              <div className="niche-section-heading">Topic</div>
            <div className="form-grid two">
              <TextInput label="Seed keyword/topic" value={seedKeyword} onChange={setSeedKeyword} placeholder="creator tools, UK visa, trading, football transfers" />
              <Select label="Platform" value={platform} onChange={setPlatform} options={[{ label: 'YouTube', value: 'youtube' }, { label: 'Short-form video', value: 'short_form' }]} />
              <TextInput label="Audience/culture" value={localAudience} onChange={setLocalAudience} placeholder="UK Pakistani, South Asian students, US creators" />
            </div>
          </div>
          <div className="niche-form-section">
            <div className="niche-section-heading">Content strategy</div>
            <div className="form-grid two">
              <Select label="Content style" value={contentStyle} onChange={setContentStyle} options={[
                { label: 'Short-form explainers', value: 'short-form explainers' },
                { label: 'Tutorials', value: 'tutorials' },
                { label: 'Reviews', value: 'reviews' },
                { label: 'News breakdowns', value: 'news breakdowns' },
                { label: 'Case studies', value: 'case studies' },
              ]} />
              <Select label="Monetization goal" value={monetizationGoal} onChange={setMonetizationGoal} options={[
                { label: 'Ads, affiliates, products', value: 'ads, affiliates, and products' },
                { label: 'Affiliate revenue', value: 'affiliate reviews and buying intent' },
                { label: 'Course/education sales', value: 'education, courses, and community' },
                { label: 'Brand deals', value: 'brand deals and sponsorships' },
              ]} />
              <Select label="Creator skill level" value={creatorSkillLevel} onChange={setCreatorSkillLevel} options={[
                { label: 'Beginner', value: 'beginner' },
                { label: 'Intermediate', value: 'intermediate' },
                { label: 'Advanced/expert', value: 'advanced expert' },
              ]} />
              <Select label="Production difficulty" value={difficulty} onChange={setDifficulty} options={[
                { label: 'Low/simple', value: 'low simple' },
                { label: 'Medium', value: 'medium' },
                { label: 'High/polished', value: 'high polished' },
              ]} />
            </div>
          </div>
          <div className="niche-form-section niche-readiness-section">
            <div className="niche-section-heading">Signal readiness</div>
            <ReadinessPanel rows={[
              { label: 'Country', value: region, type: 'value' },
              { label: 'Language', value: language, type: 'value' },
              { label: 'Video validation', value: providerMap.get('youtube_data_api')?.status || 'unknown', type: 'status' },
              { label: 'Current trend discovery', value: providerMap.get('google_trends_rss')?.status || 'unknown', type: 'status' },
            ]} />
          </div>
        </div>
        <div className="niche-action-footer">
          <div className="niche-action-copy">
            {!seedKeyword.trim() ? 'Enter a seed topic to analyze this opportunity.' : 'Ready to analyze this creator opportunity.'}
          </div>
          <button className="generate-btn idle niche-primary-action" type="button" onClick={runAnalysis} disabled={!seedKeyword.trim() || loading}>
            {loading ? 'Analyzing...' : 'Analyze niche opportunity'}
          </button>
        </div>
      </div>

      <div className="neutral-callout">
        Revenue potential is directional and based on public content signals plus category heuristics.
      </div>

      {error && <EmptyState tone="error" title="Niche analysis failed." desc={error} />}
      {result && result.status !== 'ok' && <HonestResultState result={result} />}
      {result?.status === 'ok' && (
        <div style={{ display: 'grid', gap: 10 }}>
          {(result.opportunities ?? []).map(opportunity => {
            const key = nicheOpportunityKey(opportunity);
            return (
              <NicheOpportunityCard
                key={key}
                opportunity={opportunity}
                generated={generated[key]}
                generationError={generationErrors[key]}
                generating={generatingID === key}
                onGenerate={() => onGenerate(opportunity)}
                onOpenScriptStudio={onOpenScriptStudio}
              />
            );
          })}
          <Limitations items={result.limitations ?? []} />
        </div>
      )}
    </div>
  );
}

function NicheOpportunityCard({ opportunity, generated, generationError, generating, onGenerate, onOpenScriptStudio }: {
  opportunity: NicheOpportunity;
  generated?: ReelContentPackage;
  generationError?: string;
  generating: boolean;
  onGenerate: () => void;
  onOpenScriptStudio?: () => void;
}) {
  return (
    <article className="result-card niche-opportunity-card">
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <div style={{ minWidth: 0 }}>
          <div className="result-card-title">{opportunity.niche_name}</div>
          <div style={{ marginTop: 4, fontFamily: 'var(--font-mono)', fontSize: 11, color: 'var(--text-dim)', textTransform: 'uppercase' }}>
            {opportunity.platform} · {opportunity.country} · {opportunity.language} · {opportunity.estimated_monetization_level} monetization estimate
          </div>
        </div>
        <div style={{ textAlign: 'right', minWidth: 96 }}>
            <div className="score-value">{Math.round(opportunity.opportunity_score)}</div>
          <div style={{ fontFamily: 'var(--font-mono)', fontSize: 10, color: 'var(--text-dim)', textTransform: 'uppercase' }}>opportunity</div>
        </div>
      </div>
      <MetricGrid values={{
        'Success probability': opportunity.success_probability,
        'Demand score': Math.round(opportunity.demand_score),
        'Monetization score': Math.round(opportunity.monetization_score),
        'Competition level': opportunity.competition_level,
        Confidence: `${Math.round((opportunity.confidence || 0) * 100)}%`,
      }} />
      <TextBlock label="Why this niche" value={opportunity.success_reason} />
      <TextBlock label="Demand" value={opportunity.demand_reason} />
      <TextBlock label="Monetization" value={`${opportunity.monetization_reason} Confidence: ${opportunity.monetization_confidence || 'low'}.`} />
      <TextBlock label="Competition" value={opportunity.competition_reason} />
      <LabelledChips label="Evidence" items={opportunity.evidence_sources ?? []} />
      <LabelledChips label="Supporting keywords" items={opportunity.supporting_keywords ?? []} />
      <LabelledChips label="Related channels" items={opportunity.related_channels ?? []} />
      <SectionList label="First 10 video ideas" items={opportunity.first_10_video_ideas ?? []} />
      <SectionList label="Suggested titles" items={opportunity.suggested_titles ?? []} />
      <SectionList label="Suggested clip angles" items={opportunity.suggested_clip_angles ?? []} />
      <SectionList label="Risks" items={opportunity.risks ?? []} />
      <ScriptAction
        label="Generate Script"
        generating={generating}
        generated={Boolean(generated)}
        error={generationError}
        onGenerate={onGenerate}
        onOpenScriptStudio={onOpenScriptStudio}
      />
      {generated && <GeneratedPackageView pkg={generated} />}
      <Limitations items={opportunity.limitations ?? []} />
    </article>
  );
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

function TextInput({ label, value, onChange, placeholder }: { label: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  const id = useId();
  return (
    <div className="form-group">
      <label className="form-label" htmlFor={id}>{label}</label>
      <input id={id} className="form-input" value={value} onChange={event => onChange(event.target.value)} placeholder={placeholder} />
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
    <div style={{ display: 'grid', gap: 6, marginTop: 8 }}>
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
  return <div style={{ display: 'grid', gap: 8 }}>{items.map(item => <div key={item} className="small-capability">{item}</div>)}</div>;
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

function nicheOpportunityKey(opportunity: NicheOpportunity): string {
  return `niche-${stableKey(`${opportunity.niche_name}-${opportunity.country}-${opportunity.language}`)}`;
}

function candidateFromNicheOpportunity(opportunity: NicheOpportunity): TrendCandidate {
  return {
    id: nicheOpportunityKey(opportunity),
    source: 'niche_idea',
    region: opportunity.country || 'US',
    language: opportunity.language || 'en-US',
    keyword: opportunity.niche_name,
    title: opportunity.niche_name,
    score: opportunity.opportunity_score,
    discovered_at: new Date().toISOString(),
    evidence: opportunity.success_reason || opportunity.demand_reason,
    status: 'discovered',
  };
}

function researchScriptFromNicheOpportunity(opportunity: NicheOpportunity): ResearchScriptGenerationRequest {
  return {
    source_type: 'niche_idea',
    source_id: nicheOpportunityKey(opportunity),
    topic: opportunity.niche_name,
    title: opportunity.suggested_titles?.[0] || opportunity.niche_name,
    summary: [
      `Opportunity score: ${Math.round(opportunity.opportunity_score)}.`,
      `Success probability: ${opportunity.success_probability}.`,
      opportunity.success_reason,
      opportunity.demand_reason,
      opportunity.monetization_reason,
      opportunity.competition_reason,
      (opportunity.first_10_video_ideas ?? []).slice(0, 5).join(' '),
    ].filter(Boolean).join(' '),
    keywords: [
      opportunity.niche_name,
      ...(opportunity.supporting_keywords ?? []),
      ...(opportunity.suggested_keywords ?? []),
    ],
    inferred_niche: opportunity.niche_name,
    inferred_angle: opportunity.suggested_clip_angles?.[0] || opportunity.success_reason,
    performance_signals: {
      demand_score: opportunity.demand_score,
      monetization_score: opportunity.monetization_score,
      competition_score: opportunity.competition_score,
      success_probability_score: opportunity.success_probability_score,
      opportunity_score: opportunity.opportunity_score,
      competition_level: opportunity.competition_level,
      estimated_monetization_level: opportunity.estimated_monetization_level,
    },
    suggested_angle: opportunity.suggested_clip_angles?.[0],
    target_platforms: ['instagram', 'tiktok', 'youtube', 'facebook', 'x'],
    content_style: opportunity.content_style || 'Short-form creator script',
    duration_seconds: 30,
    evidence: {
      evidence_sources: opportunity.evidence_sources,
      related_channels: opportunity.related_channels,
      related_videos: opportunity.related_videos,
      risks: opportunity.risks,
      first_10_video_ideas: opportunity.first_10_video_ideas,
      monetization_note: 'Directional estimate based on trend and ad-market research data.',
    },
    metadata: {
      source_provider: 'niche_opportunity_engine',
      score: opportunity.opportunity_score,
      confidence: opportunity.confidence,
      platform: opportunity.platform,
    },
    limitations: opportunity.limitations,
    language: opportunity.language || 'en-US',
    region: opportunity.country || 'US',
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

function formatCompactNumber(value?: number): string {
  if (value == null || !Number.isFinite(value)) return 'Unavailable';
  const abs = Math.abs(value);
  const format = (divisor: number, suffix: string) => `${(value / divisor).toLocaleString(undefined, { maximumFractionDigits: value >= divisor * 10 ? 0 : 1 })}${suffix}`;
  if (abs >= 1_000_000_000) return format(1_000_000_000, 'B');
  if (abs >= 1_000_000) return format(1_000_000, 'M');
  if (abs >= 1_000) return format(1_000, 'K');
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

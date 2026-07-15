import { useEffect, useRef, useState, useCallback, type FormEvent } from 'react';
import type { View } from './types';
import { storage } from './lib/storage';
import { MobileNavDrawer, Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardPage } from './pages/Dashboard';
import { AIToolPage } from './pages/Signals';
import { ContentProjectsPage } from './pages/ContentProjects';
import { ScriptStudioPage } from './pages/ScriptStudio';
import { ClipStudioPage } from './pages/ClipStudio';
import { VoiceStudioPage } from './pages/VoiceStudio';
import { MovieStudioPage } from './pages/MovieStudio';
import { AssetLibraryPage } from './pages/AssetLibrary';
import { SocialConnectionsPage } from './pages/SocialConnections';
import { SettingsPage } from './pages/Settings';
import { DeveloperSystemStatusPage } from './pages/DeveloperSystemStatus';
import { ComingSoonPage } from './pages/ComingSoon';
import { ApiError, getCurrentUser, login, logout, type AuthUser, type ReelContentPackage, type TrendCandidate } from './lib/api/client';

const VIEW_ROUTES: Record<View, string> = {
  dashboard: '/',
  discoverTrends: '/ai-tools/discover-trends',
  trendingKeywords: '/ai-tools/trending-keywords',
  youtubeVideoAnalyzer: '/ai-tools/youtube-video-analyzer',
  youtubeChannelAnalyzer: '/ai-tools/youtube-channel-analyzer',
  nicheFinder: '/ai-tools/niche-finder',
  contentProjects: '/content/projects',
  scriptStudio: '/script-studio',
  clipStudio: '/clip-generator',
  voiceStudio: '/voice-studio',
  movieStudio: '/movie-studio',
  thumbnailStudio: '/thumbnail-studio',
  assets: '/assets',
  connections: '/connections',
  calendar: '/calendar',
  analytics: '/analytics',
  settings: '/settings',
  developerSystemStatus: '/developer/system-status',
};

const ROUTE_VIEWS: Record<string, View> = {
  '/': 'dashboard',
  '/dashboard': 'dashboard',
  '/trend-finder': 'trendingKeywords',
  '/signals': 'trendingKeywords',
  '/ai-tools': 'trendingKeywords',
  '/ai-tools/discover-trends': 'discoverTrends',
  '/ai-tools/trending-keywords': 'trendingKeywords',
  '/ai-tools/youtube-video-analyzer': 'youtubeVideoAnalyzer',
  '/ai-tools/youtube-channel-analyzer': 'youtubeChannelAnalyzer',
  '/ai-tools/niche-finder': 'nicheFinder',
  '/content/projects': 'contentProjects',
  '/script-studio': 'scriptStudio',
  '/clip-generator': 'clipStudio',
  '/voice-studio': 'voiceStudio',
  '/movie-studio': 'movieStudio',
  '/thumbnail-studio': 'thumbnailStudio',
  '/assets': 'assets',
  '/connections': 'connections',
  '/calendar': 'calendar',
  '/analytics': 'analytics',
  '/settings': 'settings',
  '/developer/system-status': 'developerSystemStatus',
};

function viewFromLocation(): View {
  const path = window.location.pathname.replace(/\/+$/, '') || '/';
  if (path === '/ai-tools/platform-trends') {
    window.history.replaceState(null, '', '/ai-tools/trending-keywords');
    return 'trendingKeywords';
  }
  return ROUTE_VIEWS[path] ?? storage.getView();
}

export default function App() {
  storage.migrate();

  const scrollAreaRef = useRef<HTMLDivElement | null>(null);
  const [view, setView] = useState<View>(() => viewFromLocation());
  const [locationKey, setLocationKey] = useState(() => window.location.href);
  const [settings, setSettings] = useState(() => storage.getSettings());
  const [latestScript, setLatestScript] = useState(() => storage.getScriptPackage());
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [authUser, setAuthUser] = useState<AuthUser | null>(null);
  const [authLoading, setAuthLoading] = useState(true);
  const [authError, setAuthError] = useState('');

  useEffect(() => {
    const handlePopState = () => {
      const next = viewFromLocation();
      setView(next);
      setLocationKey(window.location.href);
      storage.setView(next);
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    let cancelled = false;
    getCurrentUser()
      .then(resp => { if (!cancelled) setAuthUser(resp.user); })
      .catch(err => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status !== 401) setAuthError(err.message);
      })
      .finally(() => { if (!cancelled) setAuthLoading(false); });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    scrollAreaRef.current?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }, [view]);

  const navigate = useCallback((v: View, projectID?: string) => {
    setView(v);
    storage.setView(v);
    let nextPath = VIEW_ROUTES[v];
    if (v === 'scriptStudio' && projectID) {
      nextPath = `${nextPath}?project_id=${encodeURIComponent(projectID)}`;
    }
    if (v === 'voiceStudio' && projectID) {
      nextPath = `${nextPath}?project_id=${encodeURIComponent(projectID)}`;
    }
    if (v === 'movieStudio' && projectID) {
      nextPath = `${nextPath}?project_id=${encodeURIComponent(projectID)}`;
    }
    if (`${window.location.pathname}${window.location.search}` !== nextPath) {
      window.history.pushState(null, '', nextPath);
      setLocationKey(window.location.href);
    }
  }, []);

  const handleSaveSettings = useCallback((s: typeof settings) => {
    setSettings(s);
    storage.setSettings(s);
  }, []);

  const handleScriptGenerated = useCallback((candidate: TrendCandidate, pkg: ReelContentPackage) => {
    const stored = { candidate, package: pkg, savedAt: new Date().toISOString() };
    setLatestScript(stored);
    storage.setScriptPackage(stored);
    storage.updateActivity(current => ({
      ...current,
      scriptsGenerated: current.scriptsGenerated + 1,
      latestScriptGenerated: new Date().toISOString(),
    }));
  }, []);

  const handleLogin = useCallback(async (email: string, password: string) => {
    setAuthError('');
    const resp = await login(email, password);
    setAuthUser(resp.user);
  }, []);

  const handleLogout = useCallback(async () => {
    await logout().catch(() => undefined);
    setAuthUser(null);
  }, []);

  if (authLoading) {
    return <div className="auth-screen"><div className="auth-panel"><div className="page-eyebrow">TrendCortex</div><h1>Checking session</h1></div></div>;
  }

  if (!authUser) {
    return <LoginScreen error={authError} onLogin={handleLogin} />;
  }

  return (
    <div className="app-shell">
      <Sidebar
        currentView={view}
        onNavigate={navigate}
      />

      <main className="main">
        <Header
          view={view}
          onMenuClick={() => setMobileMenuOpen(true)}
          userEmail={authUser.email}
          onLogout={handleLogout}
        />

        <div className="scroll-area" ref={scrollAreaRef}>
          <div key={view} className="workspace-route">
            {view === 'dashboard' && <DashboardPage latestScript={latestScript} onNavigate={navigate} />}
            {['discoverTrends', 'trendingKeywords', 'youtubeVideoAnalyzer', 'youtubeChannelAnalyzer', 'nicheFinder'].includes(view) && (
              <AIToolPage
                tool={view as 'discoverTrends' | 'trendingKeywords' | 'youtubeVideoAnalyzer' | 'youtubeChannelAnalyzer' | 'nicheFinder'}
                onScriptGenerated={handleScriptGenerated}
                onOpenScriptStudio={() => navigate('scriptStudio')}
                onManageDataSources={() => navigate('connections')}
              />
            )}
            {view === 'contentProjects' && <ContentProjectsPage onOpenProject={projectID => navigate('scriptStudio', projectID)} />}
            {view === 'scriptStudio' && (
              <ScriptStudioPage
                key={locationKey}
                latestScript={latestScript}
                onUseInClipGenerator={() => navigate('clipStudio')}
                onGoToTrendFinder={() => navigate('trendingKeywords')}
              />
            )}
            {view === 'clipStudio' && <ClipStudioPage onNavigate={navigate} />}
            {view === 'voiceStudio' && <VoiceStudioPage onNavigate={navigate} />}
            {view === 'movieStudio' && <MovieStudioPage onNavigate={navigate} />}
            {view === 'thumbnailStudio' && (
              <ComingSoonPage
                eyebrow="Content"
                title="Thumbnail Studio"
                description="Thumbnail design, export presets, and brand-safe variants will live here when the feature is implemented."
              />
            )}
            {view === 'assets' && (
              <AssetLibraryPage onNavigate={navigate} />
            )}
            {view === 'connections' && <SocialConnectionsPage />}
            {view === 'calendar' && (
              <ComingSoonPage
                eyebrow="Publishing"
                title="Calendar"
                description="Scheduling, review dates, and publishing plans will appear here after real scheduling support is added."
              />
            )}
            {view === 'analytics' && (
              <ComingSoonPage
                eyebrow="Publishing"
                title="Analytics"
                description="Performance reporting will appear here after real connected-platform analytics are available."
              />
            )}
            {view === 'settings' && (
              <SettingsPage
                settings={settings}
                onSave={handleSaveSettings}
              />
            )}
            {view === 'developerSystemStatus' && <DeveloperSystemStatusPage />}
          </div>
        </div>
      </main>

      <MobileNavDrawer
        currentView={view}
        onNavigate={navigate}
        open={mobileMenuOpen}
        onClose={() => setMobileMenuOpen(false)}
      />
    </div>
  );
}

function LoginScreen({ error, onLogin }: { error: string; onLogin: (email: string, password: string) => Promise<void> }) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [pending, setPending] = useState(false);
  const [localError, setLocalError] = useState(error);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setPending(true);
    setLocalError('');
    try {
      await onLogin(email, password);
    } catch (err) {
      setLocalError(err instanceof Error ? err.message : 'Login failed.');
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="auth-screen">
      <form className="auth-panel" onSubmit={submit}>
        <div className="page-eyebrow">TrendCortex</div>
        <h1>Sign in</h1>
        <label><span>Email</span><input type="email" value={email} onChange={event => setEmail(event.target.value)} autoComplete="email" required /></label>
        <label><span>Password</span><input type="password" value={password} onChange={event => setPassword(event.target.value)} autoComplete="current-password" required /></label>
        {localError && <div className="confirmation-error" role="alert">{localError}</div>}
        <button className="generate-btn" type="submit" disabled={pending}>{pending ? 'Signing in...' : 'Sign in'}</button>
      </form>
    </div>
  );
}

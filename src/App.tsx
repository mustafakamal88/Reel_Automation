import { useEffect, useRef, useState, useCallback } from 'react';
import type { View } from './types';
import { storage } from './lib/storage';
import { MobileNavDrawer, Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardPage } from './pages/Dashboard';
import { AIToolPage } from './pages/Signals';
import { ContentProjectsPage } from './pages/ContentProjects';
import { ScriptStudioPage } from './pages/ScriptStudio';
import { ClipStudioPage } from './pages/ClipStudio';
import { AssetLibraryPage } from './pages/AssetLibrary';
import { SocialConnectionsPage } from './pages/SocialConnections';
import { SettingsPage } from './pages/Settings';
import { DeveloperSystemStatusPage } from './pages/DeveloperSystemStatus';
import { ComingSoonPage } from './pages/ComingSoon';
import type { ReelContentPackage, TrendCandidate } from './lib/api/client';

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
    scrollAreaRef.current?.scrollTo({ top: 0, left: 0, behavior: 'auto' });
  }, [view]);

  const navigate = useCallback((v: View, projectID?: string) => {
    setView(v);
    storage.setView(v);
    let nextPath = VIEW_ROUTES[v];
    if (v === 'scriptStudio' && projectID) {
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
            {view === 'voiceStudio' && (
              <ComingSoonPage
                eyebrow="Content"
                title="Voice Studio"
                description="Voice generation and narration controls will live here once real voice-provider support is available."
              />
            )}
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

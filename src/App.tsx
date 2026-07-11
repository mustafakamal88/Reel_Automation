import { useEffect, useState, useCallback } from 'react';
import type { View } from './types';
import { storage } from './lib/storage';
import { MobileNavDrawer, Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardPage } from './pages/Dashboard';
import { AIToolPage, AIToolsLandingPage } from './pages/Signals';
import { ScriptStudioPage } from './pages/ScriptStudio';
import { ClipStudioPage } from './pages/ClipStudio';
import { SocialConnectionsPage } from './pages/SocialConnections';
import { SettingsPage } from './pages/Settings';
import type { ReelContentPackage, TrendCandidate } from './lib/api/client';

const VIEW_ROUTES: Record<View, string> = {
  dashboard: '/',
  aiTools: '/ai-tools',
  trendingKeywords: '/ai-tools/trending-keywords',
  platformTrends: '/ai-tools/platform-trends',
  youtubeVideoAnalyzer: '/ai-tools/youtube-video-analyzer',
  youtubeChannelAnalyzer: '/ai-tools/youtube-channel-analyzer',
  nicheFinder: '/ai-tools/niche-finder',
  scriptStudio: '/script-studio',
  clipStudio: '/clip-generator',
  connections: '/connections',
  settings: '/settings',
};

const ROUTE_VIEWS: Record<string, View> = {
  '/': 'dashboard',
  '/dashboard': 'dashboard',
  '/trend-finder': 'trendingKeywords',
  '/signals': 'trendingKeywords',
  '/ai-tools': 'aiTools',
  '/ai-tools/trending-keywords': 'trendingKeywords',
  '/ai-tools/platform-trends': 'platformTrends',
  '/ai-tools/youtube-video-analyzer': 'youtubeVideoAnalyzer',
  '/ai-tools/youtube-channel-analyzer': 'youtubeChannelAnalyzer',
  '/ai-tools/niche-finder': 'nicheFinder',
  '/script-studio': 'scriptStudio',
  '/clip-generator': 'clipStudio',
  '/connections': 'connections',
  '/settings': 'settings',
};

function viewFromLocation(): View {
  const path = window.location.pathname.replace(/\/+$/, '') || '/';
  return ROUTE_VIEWS[path] ?? storage.getView();
}

export default function App() {
  storage.migrate();

  const [view, setView] = useState<View>(() => viewFromLocation());
  const [settings, setSettings] = useState(() => storage.getSettings());
  const [latestScript, setLatestScript] = useState(() => storage.getScriptPackage());
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true);

  useEffect(() => {
    const handlePopState = () => {
      const next = viewFromLocation();
      setView(next);
      storage.setView(next);
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const navigate = useCallback((v: View) => {
    setView(v);
    storage.setView(v);
    const nextPath = VIEW_ROUTES[v];
    if (window.location.pathname !== nextPath) {
      window.history.pushState(null, '', nextPath);
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
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(current => !current)}
      />

      <main className="main">
        <Header
          view={view}
          onMenuClick={() => setMobileMenuOpen(true)}
        />

        <div className="scroll-area">
          {view === 'dashboard' && <DashboardPage latestScript={latestScript} onNavigate={navigate} />}
          {view === 'aiTools' && <AIToolsLandingPage onNavigate={navigate} />}
          {['trendingKeywords', 'platformTrends', 'youtubeVideoAnalyzer', 'youtubeChannelAnalyzer', 'nicheFinder'].includes(view) && (
            <AIToolPage
              tool={view as 'trendingKeywords' | 'platformTrends' | 'youtubeVideoAnalyzer' | 'youtubeChannelAnalyzer' | 'nicheFinder'}
              onScriptGenerated={handleScriptGenerated}
              onOpenScriptStudio={() => navigate('scriptStudio')}
              onManageDataSources={() => navigate('settings')}
            />
          )}
          {view === 'scriptStudio' && (
            <ScriptStudioPage
              latestScript={latestScript}
              onUseInClipGenerator={() => navigate('clipStudio')}
              onGoToTrendFinder={() => navigate('trendingKeywords')}
            />
          )}
          {view === 'clipStudio' && <ClipStudioPage onNavigate={navigate} />}
          {view === 'connections' && <SocialConnectionsPage />}
          {view === 'settings' && (
            <SettingsPage
              settings={settings}
              onSave={handleSaveSettings}
            />
          )}
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

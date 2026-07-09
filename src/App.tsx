import { useState, useCallback } from 'react';
import type { View } from './types';
import { storage } from './lib/storage';
import { MobileNavDrawer, Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { DashboardPage } from './pages/Dashboard';
import { TrendFinderPage } from './pages/Signals';
import { ScriptStudioPage } from './pages/ScriptStudio';
import { ClipStudioPage } from './pages/ClipStudio';
import { PublishPage } from './pages/Publish';
import { SocialConnectionsPage } from './pages/SocialConnections';
import { SettingsPage } from './pages/Settings';
import type { ReelContentPackage, TrendCandidate } from './lib/api/client';

export default function App() {
  storage.migrate();

  const [view, setView] = useState<View>(() => storage.getView());
  const [settings, setSettings] = useState(() => storage.getSettings());
  const [latestScript, setLatestScript] = useState(() => storage.getScriptPackage());
  const [trendSubtitle, setTrendSubtitle] = useState<string>('Real keyword discovery from connected sources');
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const navigate = useCallback((v: View) => {
    setView(v);
    storage.setView(v);
  }, []);

  const handleSaveSettings = useCallback((s: typeof settings) => {
    setSettings(s);
    storage.setSettings(s);
  }, []);

  const handleScriptGenerated = useCallback((candidate: TrendCandidate, pkg: ReelContentPackage) => {
    const stored = { candidate, package: pkg, savedAt: new Date().toISOString() };
    setLatestScript(stored);
    storage.setScriptPackage(stored);
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
          region={settings.region}
          subtitleOverride={view === 'trendFinder' ? trendSubtitle : undefined}
          onMenuClick={() => setMobileMenuOpen(true)}
        />

        <div className="scroll-area">
          {view === 'dashboard' && <DashboardPage latestScript={latestScript} onNavigate={navigate} />}
          {view === 'trendFinder' && <TrendFinderPage onStatusChange={setTrendSubtitle} onScriptGenerated={handleScriptGenerated} />}
          {view === 'scriptStudio' && <ScriptStudioPage latestScript={latestScript} />}
          {view === 'clipStudio' && <ClipStudioPage />}
          {view === 'publish' && <PublishPage />}
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

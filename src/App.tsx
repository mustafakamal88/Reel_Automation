import { useState, useCallback } from 'react';
import type { View, ApprovalStatus } from './types';
import { storage } from './lib/storage';
import { MobileBottomNav, Sidebar } from './components/Sidebar';
import { Header } from './components/Header';
import { SignalsPage } from './pages/Signals';
import { ScoringPage } from './pages/Scoring';
import { TopicsPage } from './pages/Topics';
import { CompetitorsPage } from './pages/Competitors';
import { ApprovalsPage } from './pages/Approvals';
import { PerformancePage } from './pages/Performance';
import { PipelinePage } from './pages/Pipeline';
import { DailyWorkflowPage } from './pages/DailyWorkflow';
import { DailyBatchPage } from './pages/DailyBatch';
import { SocialConnectionsPage } from './pages/SocialConnections';
import { SettingsPage } from './pages/Settings';

export default function App() {
  const [view, setView] = useState<View>(() => (storage.getView() as View) || 'signals');
  const [approvals, setApprovals] = useState(() => storage.getApprovals());
  const [generated, setGenerated] = useState(() => storage.getGenerated());
  const [settings, setSettings] = useState(() => storage.getSettings());
  const [openTopicId, setOpenTopicId] = useState<string | null>(null);

  const navigate = useCallback((v: View) => {
    setView(v);
    setOpenTopicId(null);
    storage.setView(v);
  }, []);

  const handleGenerate = useCallback(() => {
    setGenerated(true);
    storage.setGenerated(true);
    navigate('topics');
  }, [navigate]);

  const handleApprove = useCallback((id: string, status: ApprovalStatus) => {
    setApprovals(prev => {
      const next = { ...prev, [id]: status };
      storage.setApprovals(next);
      return next;
    });
  }, []);

  const handleSaveSettings = useCallback((s: typeof settings) => {
    setSettings(s);
    storage.setSettings(s);
  }, []);

  return (
    <div className="app-shell">
      <Sidebar
        currentView={view}
        onNavigate={navigate}
        approvals={approvals}
      />

      <main className="main">
        <Header
          view={view}
          generated={generated}
          onGenerate={handleGenerate}
          region={settings.region}
        />

        <div className="scroll-area">
          {view === 'signals' && <SignalsPage />}
          {view === 'scoring' && <ScoringPage />}
          {view === 'topics' && (
            <TopicsPage
              generated={generated}
              onApprove={handleApprove}
              onNavigateToApprovals={() => navigate('approvals')}
              openTopicId={openTopicId}
              onOpenTopic={setOpenTopicId}
            />
          )}
          {view === 'competitors' && <CompetitorsPage />}
          {view === 'approvals' && (
            <ApprovalsPage
              approvals={approvals}
              onApprove={handleApprove}
            />
          )}
          {view === 'performance' && <PerformancePage />}
          {view === 'pipeline' && <PipelinePage />}
          {view === 'workflow' && <DailyWorkflowPage />}
          {view === 'batch' && <DailyBatchPage />}
          {view === 'connections' && <SocialConnectionsPage />}
          {view === 'settings' && (
            <SettingsPage
              settings={settings}
              onSave={handleSaveSettings}
            />
          )}
        </div>
      </main>

      <MobileBottomNav
        currentView={view}
        onNavigate={navigate}
      />
    </div>
  );
}

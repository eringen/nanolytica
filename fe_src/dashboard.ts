/**
 * Nanolytica Dashboard
 * Uses talkDOM for partial page updates
 */

(function() {
  'use strict';

  // ============================================================================
  // Types
  // ============================================================================

  type DashboardTab = 'visitors' | 'bots' | 'setup';
  type TimePeriod = 'today' | 'week' | 'month' | 'year';

  interface DashboardState {
    currentTab: DashboardTab;
    visitorPeriod: TimePeriod;
    botPeriod: TimePeriod;
  }

  interface TalkDOM {
    send: (msg: string) => Promise<unknown>;
    methods: Record<string, unknown>;
  }

  // ============================================================================
  // Constants
  // ============================================================================

  const AUTO_REFRESH_INTERVAL = 60000;
  const ENDPOINTS = {
    stats: '/admin/analytics/fragments/stats',
    botStats: '/admin/analytics/fragments/bot-stats',
    setup: '/admin/analytics/fragments/setup'
  };

  // ============================================================================
  // State
  // ============================================================================

  const state: DashboardState = {
    currentTab: 'visitors',
    visitorPeriod: 'week',
    botPeriod: 'week'
  };

  // ============================================================================
  // Helpers
  // ============================================================================

  function getContentUrl(): string {
    if (state.currentTab === 'setup') {
      return ENDPOINTS.setup;
    }
    const endpoint = state.currentTab === 'bots' ? ENDPOINTS.botStats : ENDPOINTS.stats;
    const period = state.currentTab === 'bots' ? state.botPeriod : state.visitorPeriod;
    return endpoint + '?period=' + period;
  }

  function loadContent(): void {
    const td = (window as Window & { talkDOM?: TalkDOM }).talkDOM;
    if (!td) return;
    td.send('content get: ' + getContentUrl() + ' apply: inner');
  }

  // ============================================================================
  // Tab Management
  // ============================================================================

  function updateTabButtons(activeTab: DashboardTab): void {
    document.querySelectorAll('.tab-btn').forEach(btn => {
      const el = btn as HTMLElement;
      el.classList.toggle('active', el.dataset.tab === activeTab);
    });
  }

  function updatePeriodButtons(): void {
    const activePeriod = state.currentTab === 'bots' ? state.botPeriod : state.visitorPeriod;
    document.querySelectorAll('.period-btn').forEach(btn => {
      const el = btn as HTMLElement;
      el.classList.toggle('active', el.dataset.period === activePeriod);
    });
  }

  function updatePeriodSelectorVisibility(): void {
    const periodSelector = document.getElementById('period-selector');
    if (!periodSelector) return;

    if (state.currentTab === 'setup') {
      periodSelector.style.display = 'none';
    } else {
      periodSelector.style.display = '';
      updatePeriodButtons();
    }
  }

  function switchTab(tab: DashboardTab): void {
    state.currentTab = tab;
    updateTabButtons(tab);
    updatePeriodSelectorVisibility();
    loadContent();
  }

  // ============================================================================
  // Period Management
  // ============================================================================

  function loadPeriod(period: TimePeriod): void {
    if (state.currentTab === 'bots') {
      state.botPeriod = period;
    } else {
      state.visitorPeriod = period;
    }
    updatePeriodButtons();
    loadContent();
  }

  // ============================================================================
  // Auto-refresh
  // ============================================================================

  function startAutoRefresh(): void {
    setInterval(() => {
      if (state.currentTab !== 'setup') {
        loadContent();
      }
    }, AUTO_REFRESH_INTERVAL);
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  function init(): void {
    if (typeof window === 'undefined') return;

    // Expose functions globally for inline onclick handlers
    (window as Window & {
      switchTab: typeof switchTab;
      loadPeriod: typeof loadPeriod;
    }).switchTab = switchTab;

    (window as Window & { loadPeriod: typeof loadPeriod }).loadPeriod = loadPeriod;

    startAutoRefresh();

    // Initial load
    loadContent();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

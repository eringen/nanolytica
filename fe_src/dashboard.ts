/**
 * Nanolytica Dashboard
 * Uses talkDOM for partial page updates, event delegation for CSP compliance
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
  const VALID_TABS: readonly DashboardTab[] = ['visitors', 'bots', 'setup'];
  const VALID_PERIODS: readonly TimePeriod[] = ['today', 'week', 'month', 'year'];

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
  // Event Delegation
  // ============================================================================

  function handleClick(e: Event): void {
    const target = e.target as HTMLElement;

    // Tab button click
    const tabBtn = target.closest('[data-tab]') as HTMLElement | null;
    if (tabBtn) {
      const tab = tabBtn.dataset.tab;
      if (tab && VALID_TABS.includes(tab as DashboardTab)) {
        switchTab(tab as DashboardTab);
      }
      return;
    }

    // Period button click
    const periodBtn = target.closest('[data-period]') as HTMLElement | null;
    if (periodBtn) {
      const period = periodBtn.dataset.period;
      if (period && VALID_PERIODS.includes(period as TimePeriod)) {
        loadPeriod(period as TimePeriod);
      }
      return;
    }
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  function init(): void {
    if (typeof window === 'undefined') return;

    // Single click listener for all interactive buttons
    document.addEventListener('click', handleClick);

    // Auto-refresh every 60s
    setInterval(() => {
      if (state.currentTab !== 'setup') {
        loadContent();
      }
    }, AUTO_REFRESH_INTERVAL);

    // Initial load
    loadContent();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

/**
 * Nanolytica Dashboard JavaScript
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

  interface HtmxButton extends HTMLButtonElement {
    dataset: {
      period?: string;
      tab?: string;
    };
  }

  // ============================================================================
  // Constants
  // ============================================================================

  const AUTO_REFRESH_INTERVAL = 60000;
  const ENDPOINTS = {
    stats: '/admin/analytics/fragments/stats',
    botStats: '/admin/analytics/fragments/bot-stats'
  };
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
  // DOM Helpers
  // ============================================================================

  function getPeriodSelector(): HTMLElement | null {
    return document.getElementById('period-selector');
  }

  function getTabButtons(): NodeListOf<HTMLButtonElement> {
    return document.querySelectorAll('.tab-btn');
  }

  function getPeriodButtons(): NodeListOf<HtmxButton> {
    return document.querySelectorAll('.period-btn');
  }

  function getActivePeriodButton(): HtmxButton | null {
    return document.querySelector('.period-btn.active');
  }

  // ============================================================================
  // Tab Management
  // ============================================================================

  function setCurrentPeriod(period: TimePeriod): void {
    if (state.currentTab === 'bots') {
      state.botPeriod = period;
    } else {
      state.visitorPeriod = period;
    }
  }

  function updateTabButtons(activeTab: DashboardTab): void {
    const buttons = getTabButtons();
    buttons.forEach(btn => {
      const btnTab = btn.dataset.tab;
      btn.classList.toggle('active', btnTab === activeTab);
    });
  }

  function updatePeriodButtons(tab: DashboardTab): void {
    const buttons = getPeriodButtons();
    const endpoint = tab === 'bots' ? ENDPOINTS.botStats : ENDPOINTS.stats;
    const activePeriod = tab === 'bots' ? state.botPeriod : state.visitorPeriod;

    buttons.forEach(btn => {
      const period = btn.dataset.period;
      if (period) {
        btn.setAttribute('hx-get', `${endpoint}?period=${period}`);
        btn.classList.toggle('active', period === activePeriod);
      }
    });

    // Re-process so HTMX picks up the updated hx-get attributes
    const periodSelector = getPeriodSelector();
    if (periodSelector && (window as any).htmx) {
      (window as any).htmx.process(periodSelector);
    }
  }

  function updatePeriodSelectorVisibility(tab: DashboardTab): void {
    const periodSelector = getPeriodSelector();
    if (!periodSelector) return;

    if (tab === 'setup') {
      periodSelector.style.display = 'none';
    } else {
      periodSelector.style.display = 'block';
      updatePeriodButtons(tab);
    }
  }

  function switchTab(tab: DashboardTab): void {
    state.currentTab = tab;
    updateTabButtons(tab);
    updatePeriodSelectorVisibility(tab);
  }

  // ============================================================================
  // Period Management
  // ============================================================================

  function isValidPeriod(value: unknown): value is TimePeriod {
    return typeof value === 'string' && VALID_PERIODS.includes(value as TimePeriod);
  }

  function setActivePeriod(period: TimePeriod): void {
    setCurrentPeriod(period);
    const buttons = getPeriodButtons();
    buttons.forEach(btn => {
      const btnPeriod = btn.dataset.period;
      btn.classList.toggle('active', btnPeriod === period);
    });
  }

  // ============================================================================
  // Auto-refresh
  // ============================================================================

  function triggerRefresh(): void {
    const activeBtn = getActivePeriodButton();
    if (activeBtn) activeBtn.click();
  }

  function handleAutoRefresh(): void {
    if (state.currentTab !== 'setup') {
      triggerRefresh();
    }
  }

  function startAutoRefresh(): number {
    return window.setInterval(handleAutoRefresh, AUTO_REFRESH_INTERVAL);
  }

  // ============================================================================
  // HTMX Events
  // ============================================================================

  function isHtmxAfterRequestEvent(event: Event): event is CustomEvent {
    return event.type === 'htmx:afterRequest' && 'detail' in event;
  }

  function handleHtmxAfterRequest(event: Event): void {
    if (!isHtmxAfterRequestEvent(event)) return;

    const detail = event.detail as { target?: { id?: string } } | undefined;
    if (!detail?.target || detail.target.id !== 'content') return;

    const activeBtn = getActivePeriodButton();
    if (activeBtn) {
      const period = activeBtn.dataset.period;
      if (period && isValidPeriod(period)) {
        setCurrentPeriod(period);
      }
    }
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  function isBrowser(): boolean {
    return typeof window !== 'undefined' && typeof document !== 'undefined';
  }

  function init(): void {
    if (!isBrowser()) return;

    // Expose functions globally for inline onclick handlers
    (window as Window & {
      switchTab: typeof switchTab;
      setActivePeriod: typeof setActivePeriod;
      dashboard: { switchTab: typeof switchTab; setActivePeriod: typeof setActivePeriod };
    }).switchTab = switchTab;

    (window as Window & { setActivePeriod: typeof setActivePeriod }).setActivePeriod = setActivePeriod;

    (window as Window & { dashboard: { switchTab: typeof switchTab; setActivePeriod: typeof setActivePeriod } }).dashboard = {
      switchTab,
      setActivePeriod
    };

    document.addEventListener('htmx:afterRequest', handleHtmxAfterRequest);
    startAutoRefresh();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();

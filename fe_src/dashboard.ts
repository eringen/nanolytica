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
    currentSite: string;
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
    setup: '/admin/analytics/fragments/setup',
    sites: '/admin/analytics/api/sites'
  };
  const VALID_TABS: readonly DashboardTab[] = ['visitors', 'bots', 'setup'];
  const VALID_PERIODS: readonly TimePeriod[] = ['today', 'week', 'month', 'year'];

  // ============================================================================
  // State
  // ============================================================================

  const state: DashboardState = {
    currentTab: 'visitors',
    visitorPeriod: 'week',
    botPeriod: 'week',
    currentSite: 'default'
  };

  // ============================================================================
  // Helpers
  // ============================================================================

  function getContentUrl(): string {
    const site = encodeURIComponent(state.currentSite);
    if (state.currentTab === 'setup') {
      return ENDPOINTS.setup + '?site=' + site;
    }
    const endpoint = state.currentTab === 'bots' ? ENDPOINTS.botStats : ENDPOINTS.stats;
    const period = state.currentTab === 'bots' ? state.botPeriod : state.visitorPeriod;
    return endpoint + '?period=' + period + '&site=' + site;
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
  // Site Management Modals
  // ============================================================================

  function updateDeleteButtonVisibility(): void {
    const btn = document.getElementById('delete-site-btn');
    if (!btn) return;
    btn.classList.toggle('hidden', state.currentSite === 'default');
  }

  function showModal(id: string): void {
    const modal = document.getElementById(id);
    if (modal) modal.classList.remove('hidden');
  }

  function hideModals(): void {
    document.querySelectorAll('[id$="-modal"]').forEach(el => el.classList.add('hidden'));
  }

  function showAddSiteModal(): void {
    const input = document.getElementById('new-site-name') as HTMLInputElement | null;
    const error = document.getElementById('add-site-error');
    if (input) { input.value = ''; }
    if (error) { error.classList.add('hidden'); error.textContent = ''; }
    showModal('add-site-modal');
    if (input) input.focus();
  }

  function showDeleteSiteModal(): void {
    const nameEl = document.getElementById('delete-site-name');
    const error = document.getElementById('delete-site-error');
    if (nameEl) nameEl.textContent = state.currentSite;
    if (error) { error.classList.add('hidden'); error.textContent = ''; }
    showModal('delete-site-modal');
  }

  function showModalError(id: string, msg: string): void {
    const error = document.getElementById(id);
    if (!error) return;
    error.textContent = msg;
    error.classList.remove('hidden');
  }

  function submitAddSite(): void {
    const input = document.getElementById('new-site-name') as HTMLInputElement | null;
    if (!input) return;
    const name = input.value.trim();
    if (!name) {
      showModalError('add-site-error', 'Site name is required.');
      return;
    }

    const body = new URLSearchParams();
    body.set('site_name', name);

    fetch(ENDPOINTS.sites, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: body.toString()
    })
      .then(res => res.json())
      .then((data: { status?: string; error?: string; site?: string }) => {
        if (data.error) {
          showModalError('add-site-error', data.error);
          return;
        }
        const select = document.querySelector('[data-site-selector]') as HTMLSelectElement | null;
        if (select && data.site) {
          const option = document.createElement('option');
          option.value = data.site;
          option.textContent = data.site;
          select.appendChild(option);
          select.value = data.site;
          state.currentSite = data.site;
          updateDeleteButtonVisibility();
          loadContent();
        }
        hideModals();
      })
      .catch(() => {
        showModalError('add-site-error', 'Failed to add site. Please try again.');
      });
  }

  function submitDeleteSite(): void {
    const name = state.currentSite;
    if (name === 'default') return;

    fetch(ENDPOINTS.sites + '?name=' + encodeURIComponent(name), {
      method: 'DELETE'
    })
      .then(res => res.json())
      .then((data: { status?: string; error?: string }) => {
        if (data.error) {
          showModalError('delete-site-error', data.error);
          return;
        }
        // Remove option from select and switch to default
        const select = document.querySelector('[data-site-selector]') as HTMLSelectElement | null;
        if (select) {
          const option = select.querySelector('option[value="' + CSS.escape(name) + '"]');
          if (option) option.remove();
          select.value = 'default';
        }
        state.currentSite = 'default';
        updateDeleteButtonVisibility();
        hideModals();
        loadContent();
      })
      .catch(() => {
        showModalError('delete-site-error', 'Failed to delete site. Please try again.');
      });
  }

  // ============================================================================
  // Event Delegation
  // ============================================================================

  function handleClick(e: Event): void {
    const target = e.target as HTMLElement;

    // Add site button
    if (target.closest('[data-add-site]')) {
      showAddSiteModal();
      return;
    }

    // Delete site button
    if (target.closest('[data-delete-site]')) {
      showDeleteSiteModal();
      return;
    }

    // Modal backdrop or cancel
    if (target.closest('[data-modal-backdrop]') || target.closest('[data-modal-cancel]')) {
      hideModals();
      return;
    }

    // Add site confirm
    if (target.closest('[data-modal-confirm]')) {
      submitAddSite();
      return;
    }

    // Delete site confirm
    if (target.closest('[data-delete-confirm]')) {
      submitDeleteSite();
      return;
    }

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

  function handleKeydown(e: KeyboardEvent): void {
    const addModal = document.getElementById('add-site-modal');
    const deleteModal = document.getElementById('delete-site-modal');
    if (e.key === 'Escape') {
      hideModals();
      return;
    }
    if (e.key === 'Enter') {
      if (addModal && !addModal.classList.contains('hidden')) submitAddSite();
      if (deleteModal && !deleteModal.classList.contains('hidden')) submitDeleteSite();
    }
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  function init(): void {
    if (typeof window === 'undefined') return;

    // Initialize site selector
    const siteSelect = document.querySelector('[data-site-selector]') as HTMLSelectElement | null;
    if (siteSelect) {
      state.currentSite = siteSelect.value;
      updateDeleteButtonVisibility();
      siteSelect.addEventListener('change', () => {
        state.currentSite = siteSelect.value;
        updateDeleteButtonVisibility();
        loadContent();
      });
    }

    // Single click listener for all interactive buttons
    document.addEventListener('click', handleClick);
    document.addEventListener('keydown', handleKeydown);

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

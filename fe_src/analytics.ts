/**
 * Nanolytica Analytics Tracker
 * Privacy-first analytics for small websites
 */

(function() {
  'use strict';

  // ============================================================================
  // Type Definitions (inside IIFE)
  // ============================================================================

  interface AnalyticsPayload {
    readonly path: string;
    readonly referrer: string;
    readonly screen_size: string;
    readonly user_agent: string;
    readonly duration_sec: number;
    readonly scroll_depth: number;
    readonly site: string;
  }

  interface TrackerConfig {
    readonly endpoint: string;
    readonly doNotTrack: boolean;
    readonly site: string;
  }

  interface EngagementState {
    focusedAt: number;       // timestamp when tab last became active (0 = inactive)
    accumulated: number;     // total engaged milliseconds accumulated so far
  }

  interface TrackerState {
    pageLoadTime: number;
    isInitialized: boolean;
    engagement: EngagementState;
    maxScrollDepth: number;  // 0-100 percentage
    pageHeight: number;      // total scrollable height
  }

  interface TalkDOMDoneDetail {
    receiver: string;
    selector: string;
    args: string[];
  }

  // ============================================================================
  // Constants
  // ============================================================================

  const API_ENDPOINT: string = '/api/analytics/collect';
  const CONTENT_TYPE_JSON: string = 'application/json';
  const DNT_DISABLED_VALUES: readonly string[] = ['1', 'yes'];

  // ============================================================================
  // Configuration
  // ============================================================================

  function detectBaseUrl(): string {
    const currentScript = document.currentScript as HTMLScriptElement | null;
    if (!currentScript) return '';

    const src = currentScript.src;
    if (!src) return '';

    try {
      return new URL(src).origin;
    } catch {
      return '';
    }
  }

  function detectSite(): string {
    const currentScript = document.currentScript as HTMLScriptElement | null;
    if (!currentScript) return 'default';
    return currentScript.getAttribute('data-site') || 'default';
  }

  function isDoNotTrackEnabled(): boolean {
    const navigatorDnt = navigator.doNotTrack;
    const windowDnt = (window as Window & { doNotTrack?: string }).doNotTrack;

    return DNT_DISABLED_VALUES.includes(navigatorDnt || '') ||
           DNT_DISABLED_VALUES.includes(windowDnt || '');
  }

  function isLocalhost(): boolean {
    const hostname = location.hostname;
    return /^localhost$|^127(\.[0-9]+){0,2}\.[0-9]+$|^\[::1?\]$/.test(hostname) ||
           location.protocol === 'file:';
  }

  function isAutomated(): boolean {
    const w = window as Window & {
      _phantom?: unknown;
      __nightmare?: unknown;
      Cypress?: unknown;
    };
    const nav = navigator as Navigator & { webdriver?: boolean };
    return !!(w._phantom || w.__nightmare || nav.webdriver || w.Cypress);
  }

  function getConfig(): TrackerConfig {
    const baseUrl = detectBaseUrl();
    return {
      endpoint: baseUrl + API_ENDPOINT,
      doNotTrack: isDoNotTrackEnabled(),
      site: detectSite()
    };
  }

  // ============================================================================
  // Engagement Time Tracking
  // ============================================================================

  function startEngagement(eng: EngagementState): void {
    if (eng.focusedAt === 0) {
      eng.focusedAt = Date.now();
    }
  }

  function pauseEngagement(eng: EngagementState): void {
    if (eng.focusedAt > 0) {
      eng.accumulated += Date.now() - eng.focusedAt;
      eng.focusedAt = 0;
    }
  }

  function getEngagedMs(eng: EngagementState): number {
    if (eng.focusedAt > 0) {
      return eng.accumulated + (Date.now() - eng.focusedAt);
    }
    return eng.accumulated;
  }

  function resetEngagement(eng: EngagementState): void {
    eng.accumulated = 0;
    eng.focusedAt = 0;
  }

  // ============================================================================
  // Scroll Depth Tracking
  // ============================================================================

  function getPageHeight(): number {
    const body = document.body || {} as HTMLElement;
    const html = document.documentElement || {} as HTMLElement;
    return Math.max(
      body.scrollHeight || 0, body.offsetHeight || 0, body.clientHeight || 0,
      html.scrollHeight || 0, html.offsetHeight || 0, html.clientHeight || 0
    );
  }

  function getCurrentScrollBottom(): number {
    const html = document.documentElement || {} as HTMLElement;
    const body = document.body || {} as HTMLElement;
    const viewportHeight = window.innerHeight || html.clientHeight || 0;
    const scrollTop = window.scrollY || html.scrollTop || body.scrollTop || 0;
    return scrollTop + viewportHeight;
  }

  function computeScrollDepth(pageHeight: number): number {
    if (pageHeight <= 0) return 100;
    const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
    if (pageHeight <= viewportHeight) return 100;
    const scrollBottom = getCurrentScrollBottom();
    return Math.min(100, Math.round((scrollBottom / pageHeight) * 100));
  }

  // ============================================================================
  // Data Collection
  // ============================================================================

  function getScreenSize(): string {
    return `${window.innerWidth}x${window.innerHeight}`;
  }

  function getPath(): string {
    return window.location.pathname;
  }

  function getReferrer(): string {
    const ref = document.referrer;
    if (!ref) return '';

    try {
      const refUrl = new URL(ref);
      if (refUrl.host === window.location.host) return '';
      return ref;
    } catch {
      return '';
    }
  }

  function getUserAgent(): string {
    return navigator.userAgent;
  }

  function buildPayload(engagedSec: number, scrollDepth: number): AnalyticsPayload {
    return {
      path: getPath(),
      referrer: getReferrer(),
      screen_size: getScreenSize(),
      user_agent: getUserAgent(),
      duration_sec: Math.max(0, Math.round(engagedSec)),
      scroll_depth: Math.min(100, Math.max(0, scrollDepth)),
      site: config.site
    };
  }

  // ============================================================================
  // Data Transmission
  // ============================================================================

  function sendData(payload: AnalyticsPayload, endpoint: string): void {
    const jsonPayload = JSON.stringify(payload);

    if (typeof navigator.sendBeacon === 'function') {
      const blob = new Blob([jsonPayload], { type: CONTENT_TYPE_JSON });
      if (navigator.sendBeacon(endpoint, blob)) return;
    }

    fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': CONTENT_TYPE_JSON },
      body: jsonPayload,
      keepalive: true
    }).catch(() => {});
  }

  function sendAnalytics(engagedSec: number, scrollDepth: number, endpoint: string): void {
    sendData(buildPayload(engagedSec, scrollDepth), endpoint);
  }

  // ============================================================================
  // State
  // ============================================================================

  const state: TrackerState = {
    pageLoadTime: 0,
    isInitialized: false,
    engagement: { focusedAt: 0, accumulated: 0 },
    maxScrollDepth: 0,
    pageHeight: 0
  };

  let unloadSent = false;

  const config = getConfig();

  // ============================================================================
  // Event Handlers
  // ============================================================================

  function handlePageLoad(): void {
    state.pageLoadTime = Date.now();
    state.isInitialized = true;
    state.pageHeight = getPageHeight();
    state.maxScrollDepth = computeScrollDepth(state.pageHeight);
    resetEngagement(state.engagement);

    // Start engagement if page is visible and focused
    if (document.visibilityState === 'visible' && document.hasFocus()) {
      startEngagement(state.engagement);
    }

    sendAnalytics(0, state.maxScrollDepth, config.endpoint);

    // Recompute page height as lazy content loads
    let checks = 0;
    const interval = setInterval(() => {
      state.pageHeight = getPageHeight();
      if (++checks >= 15) clearInterval(interval);
    }, 200);
  }

  function handlePageUnload(): void {
    if (!state.isInitialized || unloadSent) return;
    unloadSent = true;
    pauseEngagement(state.engagement);
    const engagedSec = getEngagedMs(state.engagement) / 1000;
    sendAnalytics(engagedSec, state.maxScrollDepth, config.endpoint);
  }

  function handleVisibilityFocus(): void {
    if (document.visibilityState === 'visible' && document.hasFocus()) {
      startEngagement(state.engagement);
    } else {
      pauseEngagement(state.engagement);
    }
  }

  function handleScroll(): void {
    state.pageHeight = getPageHeight();
    const depth = computeScrollDepth(state.pageHeight);
    if (depth > state.maxScrollDepth) {
      state.maxScrollDepth = depth;
    }
  }

  function isTalkDOMDoneEvent(event: Event): event is CustomEvent<TalkDOMDoneDetail> {
    return event.type === 'talkdom:done' &&
           'detail' in event &&
           event.detail !== null &&
           typeof event.detail === 'object' &&
           'receiver' in event.detail;
  }

  function handleContentSwap(event: Event): void {
    if (!isTalkDOMDoneEvent(event)) return;

    const detail = event.detail;
    if (detail.receiver !== 'main-content') return;
    if (!state.isInitialized) return;

    // Send final data for previous page
    pauseEngagement(state.engagement);
    const engagedSec = getEngagedMs(state.engagement) / 1000;
    sendAnalytics(engagedSec, state.maxScrollDepth, config.endpoint);

    // Reset for new page
    state.pageLoadTime = Date.now();
    state.pageHeight = getPageHeight();
    state.maxScrollDepth = computeScrollDepth(state.pageHeight);
    resetEngagement(state.engagement);
    unloadSent = false;

    if (document.visibilityState === 'visible' && document.hasFocus()) {
      startEngagement(state.engagement);
    }

    setTimeout(() => sendAnalytics(0, state.maxScrollDepth, config.endpoint), 10);
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  function isBrowser(): boolean {
    return typeof window !== 'undefined' &&
           typeof document !== 'undefined' &&
           typeof navigator !== 'undefined';
  }

  function init(): void {
    if (!isBrowser()) return;
    if (config.doNotTrack) return;
    if (isLocalhost()) return;
    if (isAutomated()) return;

    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', handlePageLoad);
    } else {
      handlePageLoad();
    }

    // Engagement tracking: pause/resume on visibility and focus changes
    document.addEventListener('visibilitychange', handleVisibilityFocus);
    window.addEventListener('blur', handleVisibilityFocus);
    window.addEventListener('focus', handleVisibilityFocus);

    // Scroll depth tracking
    document.addEventListener('scroll', handleScroll, { passive: true });

    window.addEventListener('beforeunload', handlePageUnload);
    window.addEventListener('pagehide', handlePageUnload);

    // Listen for talkDOM content swaps (for SPA-style navigation)
    document.addEventListener('talkdom:done', handleContentSwap);

    (window as Window & { Nanolytica?: { track: () => void } }).Nanolytica = {
      track: () => {
        state.pageLoadTime = Date.now();
        state.pageHeight = getPageHeight();
        state.maxScrollDepth = computeScrollDepth(state.pageHeight);
        resetEngagement(state.engagement);
        if (document.visibilityState === 'visible' && document.hasFocus()) {
          startEngagement(state.engagement);
        }
        sendAnalytics(0, state.maxScrollDepth, config.endpoint);
      }
    };
  }

  init();
})();

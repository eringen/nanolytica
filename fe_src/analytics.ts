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
  }

  interface TrackerConfig {
    readonly endpoint: string;
    readonly doNotTrack: boolean;
  }

  interface TrackerState {
    pageLoadTime: number;
    isInitialized: boolean;
  }

  interface HtmxAfterSwapDetail {
    target: HTMLElement;
    xhr: XMLHttpRequest;
    successful: boolean;
    failed: boolean;
    ignoreTitle?: boolean;
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

  function isDoNotTrackEnabled(): boolean {
    const navigatorDnt = navigator.doNotTrack;
    const windowDnt = (window as Window & { doNotTrack?: string }).doNotTrack;
    
    return DNT_DISABLED_VALUES.includes(navigatorDnt || '') || 
           DNT_DISABLED_VALUES.includes(windowDnt || '');
  }

  function getConfig(): TrackerConfig {
    const baseUrl = detectBaseUrl();
    return {
      endpoint: baseUrl + API_ENDPOINT,
      doNotTrack: isDoNotTrackEnabled()
    };
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

  function buildPayload(duration: number): AnalyticsPayload {
    return {
      path: getPath(),
      referrer: getReferrer(),
      screen_size: getScreenSize(),
      user_agent: getUserAgent(),
      duration_sec: Math.max(0, Math.round(duration))
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

  function sendAnalytics(duration: number, endpoint: string): void {
    sendData(buildPayload(duration), endpoint);
  }

  // ============================================================================
  // State
  // ============================================================================

  const state: TrackerState = {
    pageLoadTime: 0,
    isInitialized: false
  };

  let unloadSent = false;

  const config = getConfig();

  // ============================================================================
  // Event Handlers
  // ============================================================================

  function handlePageLoad(): void {
    state.pageLoadTime = Date.now();
    state.isInitialized = true;
    sendAnalytics(0, config.endpoint);
  }

  function handlePageUnload(): void {
    if (!state.isInitialized || unloadSent) return;
    unloadSent = true;
    const duration = (Date.now() - state.pageLoadTime) / 1000;
    sendAnalytics(duration, config.endpoint);
  }

  function isHtmxAfterSwapEvent(event: Event): event is CustomEvent<HtmxAfterSwapDetail> {
    return event.type === 'htmx:afterSwap' && 
           'detail' in event && 
           event.detail !== null &&
           typeof event.detail === 'object' &&
           'target' in event.detail;
  }

  function handleHtmxSwap(event: Event): void {
    if (!isHtmxAfterSwapEvent(event)) return;
    
    const detail = event.detail;
    if (detail.target.id !== 'main-content') return;
    if (!state.isInitialized) return;
    
    const duration = (Date.now() - state.pageLoadTime) / 1000;
    sendAnalytics(duration, config.endpoint);

    state.pageLoadTime = Date.now();
    unloadSent = false;
    setTimeout(() => sendAnalytics(0, config.endpoint), 10);
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
    
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', handlePageLoad);
    } else {
      handlePageLoad();
    }
    
    window.addEventListener('beforeunload', handlePageUnload);
    window.addEventListener('pagehide', handlePageUnload);
    
    const htmx = (window as Window & { htmx?: unknown }).htmx;
    if (htmx) {
      document.addEventListener('htmx:afterSwap', handleHtmxSwap);
    }
    
    (window as Window & { Nanolytica?: { track: () => void } }).Nanolytica = {
      track: () => {
        state.pageLoadTime = Date.now();
        sendAnalytics(0, config.endpoint);
      }
    };
  }

  init();
})();

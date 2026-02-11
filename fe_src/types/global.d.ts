/**
 * Global type declarations for Nanolytica analytics
 */

// ============================================================================
// HTMX Types
// ============================================================================

/**
 * HTMX afterSwap event detail
 */
interface HtmxAfterSwapDetail {
  /** The target element that was swapped */
  target: HTMLElement;
  /** The XMLHttpRequest object */
  xhr: XMLHttpRequest;
  /** Whether the request was successful */
  successful: boolean;
  /** Whether the request failed */
  failed: boolean;
  /** Whether the swap should be ignored */
  ignoreTitle?: boolean;
}

/**
 * HTMX global object
 */
interface HtmxGlobal {
  /** Process newly added content */
  process: (element: HTMLElement) => void;
  /** Trigger an event */
  trigger: (element: HTMLElement, eventName: string, detail?: unknown) => void;
  /** Make an AJAX request */
  ajax: (verb: string, path: string, element: HTMLElement) => Promise<unknown>;
  /** Find elements */
  find: (selector: string, context?: HTMLElement) => HTMLElement | null;
  /** Find all elements */
  findAll: (selector: string, context?: HTMLElement) => NodeListOf<HTMLElement>;
  /** Closest element */
  closest: (element: HTMLElement, selector: string) => HTMLElement | null;
}

// ============================================================================
// Window Extensions
// ============================================================================

interface Window {
  /** Nanolytica analytics API */
  Nanolytica?: {
    /** Manually trigger a page view track */
    track: () => void;
  };
  /** Do Not Track setting */
  doNotTrack: string | undefined;
  /** HTMX global object */
  htmx?: HtmxGlobal;
  /** Dashboard API */
  dashboard?: {
    /** Switch to a different tab */
    switchTab: (tab: 'visitors' | 'bots' | 'setup') => void;
    /** Set the active time period */
    setActivePeriod: (period: 'today' | 'week' | 'month' | 'year') => void;
  };
  /** Direct switchTab function (for inline onclick handlers) */
  switchTab?: (tab: 'visitors' | 'bots' | 'setup') => void;
  /** Direct setActivePeriod function (for inline onclick handlers) */
  setActivePeriod?: (period: 'today' | 'week' | 'month' | 'year') => void;
}

/**
 * Navigator with Do Not Track
 */
interface Navigator {
  /** Do Not Track setting */
  doNotTrack: string | undefined;
  /** Send beacon for analytics */
  sendBeacon: (url: string, data?: Blob | FormData | ArrayBufferView) => boolean;
}

/**
 * Document event map with HTMX events
 */
interface DocumentEventMap {
  'htmx:afterSwap': CustomEvent<HtmxAfterSwapDetail>;
  'htmx:beforeSwap': CustomEvent<HtmxAfterSwapDetail>;
  'htmx:afterRequest': CustomEvent;
}

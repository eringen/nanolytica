/**
 * Global type declarations for Nanolytica analytics
 */

// ============================================================================
// talkDOM Types
// ============================================================================

/**
 * talkDOM global object
 */
interface TalkDOMGlobal {
  /** Send a talkDOM message string, returns a promise */
  send: (msg: string) => Promise<unknown>;
  /** Extensible method map */
  methods: Record<string, (...args: unknown[]) => unknown>;
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
  /** talkDOM global object */
  talkDOM?: TalkDOMGlobal;
  /** Direct switchTab function (for inline onclick handlers) */
  switchTab?: (tab: 'visitors' | 'bots' | 'setup') => void;
  /** Direct loadPeriod function (for inline onclick handlers) */
  loadPeriod?: (period: 'today' | 'week' | 'month' | 'year') => void;
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

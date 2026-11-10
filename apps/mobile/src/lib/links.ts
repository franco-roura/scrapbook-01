// Display helpers for shared links. The server does the real cleanup
// (server/internal/links); this only needs to find the link and name the
// platform for the share sheet's preview row.

const URL_IN_TEXT = /https?:\/\/[^\s<>"']+/i;

export function extractUrl(text: string | null | undefined): string | null {
  const m = text?.match(URL_IN_TEXT);
  return m ? m[0].replace(/[.,;:!?)\]}]+$/, '') : null;
}

const PLATFORMS: [RegExp, string][] = [
  [/(^|\.)(instagram\.com|instagr\.am)$/, 'Instagram'],
  [/(^|\.)tiktok\.com$/, 'TikTok'],
  [/(^|\.)(youtube\.com|youtu\.be)$/, 'YouTube'],
  [/(^|\.)(x\.com|twitter\.com)$/, 'X'],
  [/(^|\.)(facebook\.com|fb\.watch|fb\.com)$/, 'Facebook'],
  [/(^|\.)(threads\.net|threads\.com)$/, 'Threads'],
  [/(^|\.)(reddit\.com|redd\.it)$/, 'Reddit'],
  [/(^|\.)(linkedin\.com|lnkd\.in)$/, 'LinkedIn'],
  [/(^|\.)(pinterest\.com|pin\.it)$/, 'Pinterest'],
  [/(^|\.)spotify\.com$/, 'Spotify'],
  [/(^|\.)t\.me$/, 'Telegram'],
];

/** Host without "www.", e.g. "instagram.com", or null if text has no link. */
export function hostOf(url: string): string | null {
  try {
    return new URL(url).hostname.toLowerCase().replace(/^www\./, '');
  } catch {
    return null;
  }
}

/** Platform label for display, falling back to the host ("nytimes.com"). */
export function platformLabel(url: string): string {
  const host = hostOf(url);
  if (!host) return 'Link';
  return PLATFORMS.find(([re]) => re.test(host))?.[1] ?? host;
}

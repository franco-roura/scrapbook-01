import { describe, expect, test } from 'bun:test';

import { extractUrl, platformLabel } from './links';

describe('extractUrl', () => {
  test('finds a link inside share text and drops trailing punctuation', () => {
    expect(extractUrl('Check this out! https://vm.tiktok.com/ZM123/.')).toBe('https://vm.tiktok.com/ZM123/');
    expect(extractUrl('(see https://example.com/a)')).toBe('https://example.com/a');
  });
  test('returns null without a link', () => {
    expect(extractUrl('just words')).toBeNull();
    expect(extractUrl(undefined)).toBeNull();
  });
});

describe('platformLabel', () => {
  test('names known platforms', () => {
    expect(platformLabel('https://www.instagram.com/p/DfYWVoBDC7L/')).toBe('Instagram');
    expect(platformLabel('https://m.youtube.com/watch?v=x')).toBe('YouTube');
    expect(platformLabel('https://twitter.com/a/status/1')).toBe('X');
  });
  test('falls back to the host', () => {
    expect(platformLabel('https://www.nytimes.com/2026/11/10/story.html')).toBe('nytimes.com');
    expect(platformLabel('not a url')).toBe('Link');
  });
});

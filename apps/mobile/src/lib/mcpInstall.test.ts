import { describe, expect, test } from 'bun:test';

import { claudeInstallUrl, mcpUrl } from './mcpInstall';

describe('mcpUrl', () => {
  test('appends /mcp once, with or without a trailing slash', () => {
    expect(mcpUrl('https://api.example.test')).toBe('https://api.example.test/mcp');
    expect(mcpUrl('https://api.example.test/')).toBe('https://api.example.test/mcp');
  });
});

describe('claudeInstallUrl', () => {
  test('opens the prefilled custom connector dialog', () => {
    const url = new URL(claudeInstallUrl('https://api.example.test/mcp'));
    expect(url.origin).toBe('https://claude.ai');
    expect(url.pathname).toBe('/customize/connectors');
    expect(url.searchParams.get('modal')).toBe('add-custom-connector');
    expect(url.searchParams.get('connectorName')).toBe('Scrapbook/01');
    expect(url.searchParams.get('connectorUrl')).toBe('https://api.example.test/mcp');
  });
  test('percent-encodes the name and URL', () => {
    const url = claudeInstallUrl('https://api.example.test/mcp?x=1&y=2');
    expect(url).toContain('connectorName=Scrapbook%2F01');
    expect(url).toContain('connectorUrl=https%3A%2F%2Fapi.example.test%2Fmcp%3Fx%3D1%26y%3D2');
  });
});

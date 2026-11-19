// Links that connect the user's own AI (Claude, ChatGPT) to the MCP server.

export const CONNECTOR_NAME = 'Scrapbook/01';

/** The MCP endpoint of an API base URL. */
export function mcpUrl(apiUrl: string): string {
  return `${apiUrl.replace(/\/+$/, '')}/mcp`;
}

/**
 * Claude's documented install link: it opens the "Add custom connector" dialog
 * with the name and URL filled in. The user still confirms and signs in.
 */
export function claudeInstallUrl(serverUrl: string, name = CONNECTOR_NAME): string {
  return (
    'https://claude.ai/customize/connectors?modal=add-custom-connector' +
    `&connectorName=${encodeURIComponent(name)}` +
    `&connectorUrl=${encodeURIComponent(serverUrl)}`
  );
}

// ChatGPT has no install link, and its settings paths keep moving, so open the
// home page and let the user paste the copied URL.
export const CHATGPT_URL = 'https://chatgpt.com/';

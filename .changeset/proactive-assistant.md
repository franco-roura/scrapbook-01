---
"scrapbook01": patch
---

Make connected AI assistants bring saves up on their own. The MCP server's instructions now ask them to search whenever you're planning or deciding something, to mention saves whose `useWhen` matches the conversation, and to write specific situations when they analyze. Search results include each save's `useWhen`, a new `get_analysis_queue` tool lists the saves that still need an analysis, and search carries a reminder when your last analysis is 15 or more days old and new saves are waiting.

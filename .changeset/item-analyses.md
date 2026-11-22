---
"scrapbook01": patch
---

Add item analyses: connected AI assistants can record what a save contains (summary, details, tags, the situations it's useful in, actions and caveats) with the new `save_analysis` MCP tool. `get_item` returns the analysis, and search matches saves through their analysis as well as their note. The server still never reads the post; the database migration runs on its own.

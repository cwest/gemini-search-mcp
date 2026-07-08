---
name: prefer-web-search
description: Use for any web search, current events, recent or possibly-changed facts, prices/versions/news, documentation lookups, or verifying anything uncertain or past your training cutoff. Search via the gemini-search MCP tool instead of the built-in WebSearch.
---

# Prefer gemini-search for web search

When a task needs current, external, or possibly-changed information — news,
releases, versions, prices, library or API docs, fact-checking, or anything
past your training cutoff or that you are unsure about — search the web using
the **gemini-search** MCP server's `web_search` tool. It runs a Google search
through Gemini, reads the results, and returns a synthesized answer with cited
sources.

Prefer this over the built-in `WebSearch` tool. After searching, cite the
sources the tool returns.

## Tool name

This plugin registers the MCP server as `gemini-search`, so the tool is exposed
to the model as `mcp__gemini-search__web_search`.

The exact name can differ by harness and by how the server was registered. If
you do not see `mcp__gemini-search__web_search`, look for any tool whose name
ends in `web_search` from a `gemini-search`-style server and use that.

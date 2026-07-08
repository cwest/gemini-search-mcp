# Make your agent prefer gemini-search for web search

`gemini-search-mcp` exposes one tool, `web_search`, that runs a Google search
through Gemini and returns a synthesized, cited answer. Most agents already have
some built-in or competing web-search capability, so getting them to reach for
this tool first takes a deliberate nudge. This guide shows the levers for the
common harnesses.

There are two layers that work together:

1. **The tool description** — the universal lever. Every MCP client sees it, so
   it steers every agent at once.
2. **A harness-specific instruction** — a skill, rule, or system-prompt snippet
   that names the tool and, where possible, removes the competing built-in.

## The tool description (universal lever)

The server already ships an assertive description (in
`internal/server/server.go`):

> Search the web and return a current, synthesized answer with cited sources
> (Google Search grounding via Gemini). Use this for any web search, current
> events, recent or possibly-changed facts, versions, prices, documentation
> lookups, or to verify anything uncertain or past your training cutoff. Prefer
> this over other web-search tools.

This is the single highest-leverage place to influence tool choice, because
every MCP client surfaces the description to the model verbatim. The
harness-specific instructions below reinforce it; they do not replace it.

## Claude Code

Two steps: install the plugin (which registers the MCP server and ships the
steering skill), then deny the built-in `WebSearch` so the model can't fall back
to it.

### 1. Install the plugin

See the "Claude Code plugin" section of the [README](../README.md) for the full
install. In short: add the plugin, run `scripts/install-plugin-binary.sh` to
fetch the binary, and set your Vertex AI or AI Studio environment variables. The
plugin's `.mcp.json` registers the server as `gemini-search`, so the tool is
exposed as `mcp__gemini-search__web_search`, and the bundled
`prefer-web-search` skill tells the model to use it.

### 2. Deny the built-in WebSearch

Add this to your settings (`~/.claude/settings.json` for all projects, or
`.claude/settings.json` for one project):

```json
{
  "permissions": {
    "deny": ["WebSearch"]
  }
}
```

With `WebSearch` denied, the model's only path to the web is
`mcp__gemini-search__web_search`. Note: plugin-namespaced tool names can vary
with how the server is registered; if you registered the server yourself with a
different name (for example via `claude mcp add gemini-search ...`), the tool is
`mcp__gemini-search__web_search` regardless, since the name comes from the
server registration, not the plugin.

## Gemini CLI / opencode

These harnesses load Markdown rules or Agent Skills from a project or user
config directory. Drop a short rule that names the tool.

For **Gemini CLI**, add to your context file (`GEMINI.md` at the project root,
or `~/.gemini/GEMINI.md` for all projects):

```markdown
## Web search

For any web search, current events, version/price lookups, documentation, or
fact-checking, use the `gemini-search` MCP server's `web_search` tool. Prefer it
over any built-in web search. Cite the sources it returns.
```

Register the MCP server in the harness's MCP config (`~/.gemini/settings.json`
or the project equivalent), pointing `command` at the `gemini-search-mcp`
binary and passing the provider environment variables through.

For **opencode**, the equivalent is an Agent Skill or a rule in `AGENTS.md` /
`.opencode/`; use the same wording. opencode reads MCP servers from its
`opencode.json` `mcp` block — point it at the binary the same way.

## Hermes / generic OpenAI-compatible clients

Clients that don't have a skill or rules system take direction through the
system prompt. Add a snippet like this:

```text
You have access to a tool named `web_search` from the gemini-search MCP server.
It runs a Google search through Gemini and returns a cited answer. Use it for
any task that needs current, external, or possibly-changed information: news,
releases, versions, prices, documentation, or fact-checking anything you are
unsure about or that is past your training cutoff. Prefer it over any other
web-search tool. Always cite the sources it returns.
```

If the client also exposes its own web-search function, either remove that
function from the tool list you pass at request time, or make the system prompt
explicit that `web_search` is the only approved one. Removing the competitor is
more reliable than instructing around it.

## Why removing the competitor matters

Models pick tools by name and description match. When two tools both look like
"search the web," the choice gets noisy. The most reliable setups do both:
assert the preference in text **and** take the alternative off the table
(`permissions.deny` in Claude Code, omitting the function in OpenAI clients).
When you can't remove the alternative, lean harder on the description and the
rule.

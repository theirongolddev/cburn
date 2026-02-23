# cburn

A CLI and TUI dashboard for analyzing Claude Code usage metrics. Parses JSONL session logs from `~/.claude/projects/`, computes token usage, costs, cache efficiency, and activity patterns.

## Installation

```bash
# Build from source
make build

# Install to ~/go/bin
make install
```

Requires Go 1.24+.

## Quick Start

```bash
cburn              # Summary of usage metrics
cburn tui          # Interactive dashboard
cburn costs        # Cost breakdown by token type
cburn status       # Claude.ai subscription status
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `cburn` | Usage summary (default) |
| `cburn summary` | Detailed usage summary with costs |
| `cburn costs` | Cost breakdown by token type and model |
| `cburn daily` | Daily usage table |
| `cburn hourly` | Activity by hour of day |
| `cburn sessions` | Session list with details |
| `cburn models` | Model usage breakdown |
| `cburn projects` | Project usage ranking |
| `cburn status` | Claude.ai subscription status and rate limits |
| `cburn config` | Show current configuration |
| `cburn setup` | Interactive first-time setup wizard |
| `cburn tui` | Interactive dashboard |

## Global Flags

```
-n, --days INT        Time window in days (default: 30)
-p, --project STRING  Filter to project (substring match)
-m, --model STRING    Filter to model (substring match)
-d, --data-dir PATH   Claude data directory (default: ~/.claude)
-q, --quiet           Suppress progress output
    --no-cache        Skip SQLite cache, reparse everything
    --no-subagents    Exclude subagent sessions
```

**Examples:**

```bash
cburn -n 7                      # Last 7 days
cburn costs -p myproject        # Costs for a specific project
cburn sessions -m opus          # Sessions using Opus models
cburn daily --no-subagents      # Exclude spawned agents
```

## TUI Dashboard

Launch with `cburn tui`. Navigate with keyboard:

| Key | Action |
|-----|--------|
| `o` / `c` / `s` / `b` / `x` | Jump to Overview / Costs / Sessions / Breakdown / Settings |
| `<-` / `->` | Previous / Next tab |
| `j` / `k` | Navigate lists |
| `J` / `K` | Scroll detail pane |
| `Ctrl+d` / `Ctrl+u` | Scroll half-page |
| `Enter` / `f` | Expand session full-screen |
| `Esc` | Back to split view |
| `r` | Refresh data |
| `R` | Toggle auto-refresh |
| `?` | Help overlay |
| `q` | Quit |

### Tabs

- **Overview** - Summary stats, daily activity chart, live hourly/minute charts
- **Costs** - Cost breakdown by token type and model, cache savings
- **Sessions** - Browseable session list with detail pane
- **Breakdown** - Model and project rankings
- **Settings** - Configuration management

### Themes

Four color themes are available:

- `flexoki-dark` (default) - Warm earth tones
- `catppuccin-mocha` - Pastel colors
- `tokyo-night` - Cool blue/purple
- `terminal` - ANSI 16 colors only

Change via `cburn setup` or edit `~/.config/cburn/config.toml`.

## Configuration

Config file: `~/.config/cburn/config.toml`

```toml
[general]
default_days = 30
include_subagents = true

[claude_ai]
session_key = "sk-ant-sid..."    # For subscription/rate limit data

[admin_api]
api_key = "sk-ant-admin-..."     # For billing API (optional)

[appearance]
theme = "flexoki-dark"

[budget]
monthly_usd = 100                 # Optional spending cap

[tui]
auto_refresh = true
refresh_interval_sec = 30
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `CLAUDE_SESSION_KEY` | Claude.ai session key (overrides config) |
| `ANTHROPIC_ADMIN_KEY` | Admin API key (overrides config) |

### Claude.ai Session Key

The session key enables:
- Real-time rate limit monitoring (5-hour and 7-day windows)
- Overage spend tracking
- Organization info

To get your session key:
1. Open claude.ai in your browser
2. DevTools (F12) > Application > Cookies > claude.ai
3. Copy the `sessionKey` value (starts with `sk-ant-sid...`)

## Caching

Session data is cached in SQLite at `~/.cache/cburn/sessions.db`. The cache uses mtime-based diffing - unchanged files are not reparsed.

Force a full reparse with `--no-cache`.

## Development

```bash
make build       # Build ./cburn binary
make install     # Install to ~/go/bin
make lint        # Run golangci-lint
make test        # Run unit tests
make test-race   # Tests with race detector
make bench       # Pipeline benchmarks
make fuzz        # Fuzz the JSONL parser (30s default)
make clean       # Remove binary and test cache
```

## Architecture

```
~/.claude/projects/**/*.jsonl
    -> source.ScanDir() + source.ParseFile()  (parallel parsing)
    -> store.Cache (SQLite, mtime-based incremental)
    -> pipeline.Aggregate*() functions
    -> CLI renderers (cmd/) or TUI tabs (internal/tui/)
```

| Package | Role |
|---------|------|
| `cmd/` | Cobra CLI commands |
| `internal/source` | File discovery and JSONL parsing |
| `internal/pipeline` | ETL orchestration and aggregation |
| `internal/store` | SQLite cache layer |
| `internal/model` | Domain types |
| `internal/config` | TOML config and pricing tables |
| `internal/cli` | Terminal formatting |
| `internal/claudeai` | Claude.ai API client |
| `internal/tui` | Bubble Tea dashboard |
| `internal/tui/components` | Reusable TUI components |
| `internal/tui/theme` | Color schemes |

## License

MIT

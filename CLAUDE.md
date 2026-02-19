# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What This Is

cburn is a CLI + TUI dashboard for analyzing Claude Code usage metrics. It parses JSONL session logs from `~/.claude/projects/`, computes token usage, costs, cache efficiency, and activity patterns, then presents results as CLI tables or an interactive Bubble Tea dashboard.

## Build & Run

A `Makefile` wraps all common commands. Go is at `/usr/local/go/bin/go`.

```bash
make build          # build ./cburn binary
make install        # install to ~/go/bin

cburn               # default: summary command
cburn tui           # interactive dashboard
cburn costs         # cost breakdown
cburn --no-cache    # skip SQLite cache, full reparse
```

## Quality Pipeline

```bash
make lint           # golangci-lint (config: .golangci.yml)
make test           # unit tests
make test-race      # tests with race detector
make bench          # pipeline benchmarks (uses live ~/.claude data)
make fuzz           # fuzz the JSONL parser (default 30s, override: FUZZ_TIME=2m)
```

Lint + test should pass before committing. The linter catches security issues (gosec), unchecked errors (errcheck), performance hints (perfsprint, prealloc), and style (revive).

Tests live alongside the code they test (`*_test.go`). The parser has both unit tests and a fuzz test in `internal/source/parser_test.go`.

## Architecture

### Data Flow

```
~/.claude/projects/**/*.jsonl
    -> source.ScanDir() + source.ParseFile()  (parallel, GOMAXPROCS workers)
    -> store.Cache (SQLite, mtime-based incremental)
    -> pipeline.Aggregate*() functions
    -> CLI renderers (cmd/) or TUI tabs (internal/tui/)
```

### Package Map

| Package | Role |
|---------|------|
| `cmd/` | Cobra CLI commands. Each file = one subcommand. `root.go` has shared data loading + filtering. |
| `internal/source` | File discovery (`ScanDir`) and JSONL parsing (`ParseFile`). Deduplicates by message ID. |
| `internal/pipeline` | ETL orchestration: parallel loading, cache-aware incremental loading, aggregation functions (`Aggregate`, `AggregateDays`, `AggregateHourly`, `AggregateModels`, `AggregateProjects`). |
| `internal/store` | SQLite cache layer. Tracks file mtime/size, caches parsed `SessionStats`. |
| `internal/model` | Domain types: `SessionStats`, `APICall`, `SummaryStats`, `DailyStats`, etc. |
| `internal/config` | TOML config (`~/.config/cburn/config.toml`), model pricing tables, cost calculation. |
| `internal/cli` | Terminal formatting: numbers, tokens, costs, tables, horizontal bars. |
| `internal/claudeai` | claude.ai API client for subscription/usage data. |
| `internal/tui` | Bubble Tea app. `app.go` is the root model with async data loading. Tab renderers in `tab_*.go`. |
| `internal/tui/components` | Reusable TUI components: cards, bar charts, sparklines, progress bars, tab bar. |
| `internal/tui/theme` | Color schemes (flexoki-dark, catppuccin-mocha, tokyo-night, terminal). |

### Key Design Decisions

- **Parsing strategy**: User/system entries use byte-level extraction for speed; only assistant entries get full JSON parse (they carry token/cost data).
- **Deduplication**: Messages are keyed by message ID; the final state wins (handles edits/retries).
- **Cache**: SQLite at `~/.cache/cburn/sessions.db`. Mtime+size diffing means unchanged files aren't reparsed.
- **TUI async loading**: Data loads via goroutines posting `tea.Msg`; the UI remains responsive during parse.
- **Pricing**: Hardcoded in `internal/config/pricing.go` with user overrides in config TOML. Model names are normalized (date suffixes stripped).

## Configuration

Config file: `~/.config/cburn/config.toml`

Env var fallbacks: `ANTHROPIC_ADMIN_KEY`, `CLAUDE_SESSION_KEY`

Run `cburn setup` for interactive configuration.

## TUI Layout Conventions

- `components.CardInnerWidth(w)` computes usable width inside a card border.
- `components.LayoutRow(w, n)` splits width into n columns accounting for gaps.
- When rendering inline bars (like Activity panel), dynamically compute column widths from actual data to prevent line wrapping.

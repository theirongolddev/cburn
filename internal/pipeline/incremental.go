package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"cburn/internal/source"
	"cburn/internal/store"
)

// CachedLoadResult extends LoadResult with cache metadata.
type CachedLoadResult struct {
	LoadResult
	CacheHits int
	Reparsed  int
}

// LoadWithCache discovers, diffs against cache, parses only changed files,
// and returns the combined result set.
func LoadWithCache(claudeDir string, includeSubagents bool, cache *store.Cache, progressFn ProgressFunc) (*CachedLoadResult, error) {
	// Discover files
	files, err := source.ScanDir(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", claudeDir, err)
	}

	if len(files) == 0 {
		return &CachedLoadResult{}, nil
	}

	// Filter subagents if requested
	var toProcess []source.DiscoveredFile
	if includeSubagents {
		toProcess = files
	} else {
		for _, f := range files {
			if !f.IsSubagent {
				toProcess = append(toProcess, f)
			}
		}
	}

	result := &CachedLoadResult{
		LoadResult: LoadResult{
			TotalFiles:   len(toProcess),
			ProjectCount: source.CountProjects(files),
		},
	}

	if len(toProcess) == 0 {
		return result, nil
	}

	// Get tracked files from cache
	tracked, err := cache.GetTrackedFiles()
	if err != nil {
		return nil, fmt.Errorf("reading cache: %w", err)
	}

	// Diff: partition into changed and unchanged
	var toReparse []source.DiscoveredFile
	var unchanged []string // file paths that haven't changed

	for _, f := range toProcess {
		info, err := os.Stat(f.Path)
		if err != nil {
			continue
		}

		cached, ok := tracked[f.Path]
		if ok && cached.MtimeNs == info.ModTime().UnixNano() && cached.SizeBytes == info.Size() {
			unchanged = append(unchanged, f.Path)
		} else {
			toReparse = append(toReparse, f)
		}
	}

	result.CacheHits = len(unchanged)
	result.Reparsed = len(toReparse)

	// Load cached sessions
	if len(unchanged) > 0 {
		cached, err := cache.LoadAllSessions()
		if err != nil {
			return nil, fmt.Errorf("loading cached sessions: %w", err)
		}

		// Filter to only sessions from unchanged files
		unchangedSet := make(map[string]struct{}, len(unchanged))
		for _, p := range unchanged {
			unchangedSet[p] = struct{}{}
		}
		for _, s := range cached {
			if _, ok := unchangedSet[s.FilePath]; ok {
				result.Sessions = append(result.Sessions, s)
				result.ParsedFiles++
			}
		}
	}

	// Parse changed files
	if len(toReparse) > 0 {
		numWorkers := runtime.GOMAXPROCS(0)
		if numWorkers < 1 {
			numWorkers = 4
		}
		if numWorkers > len(toReparse) {
			numWorkers = len(toReparse)
		}

		work := make(chan int, len(toReparse))
		results := make([]source.ParseResult, len(toReparse))
		var wg sync.WaitGroup
		var processed atomic.Int64

		for i := range toReparse {
			work <- i
		}
		close(work)

		wg.Add(numWorkers)
		for w := 0; w < numWorkers; w++ {
			go func() {
				defer wg.Done()
				for idx := range work {
					results[idx] = source.ParseFile(toReparse[idx])
					n := processed.Add(1)
					if progressFn != nil {
						progressFn(int(n)+result.CacheHits, result.TotalFiles)
					}
				}
			}()
		}

		wg.Wait()

		// Collect and cache results
		for i, pr := range results {
			if pr.Err != nil {
				result.FileErrors++
				continue
			}
			result.ParsedFiles++
			result.ParseErrors += pr.ParseErrors

			if pr.Stats.APICalls > 0 || pr.Stats.UserMessages > 0 {
				result.Sessions = append(result.Sessions, pr.Stats)

				// Save to cache
				info, err := os.Stat(toReparse[i].Path)
				if err == nil {
					_ = cache.SaveSession(pr.Stats, info.ModTime().UnixNano(), info.Size())
				}
			}
		}
	}

	return result, nil
}

// CacheDir returns the platform-appropriate cache directory.
func CacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "cburn")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "cburn")
}

// CachePath returns the full path to the cache database.
func CachePath() string {
	return filepath.Join(CacheDir(), "metrics.db")
}

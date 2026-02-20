package pipeline

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"cburn/internal/model"
	"cburn/internal/source"
)

// LoadResult holds the output of the full data loading pipeline.
type LoadResult struct {
	Sessions     []model.SessionStats
	TotalFiles   int
	ParsedFiles  int
	ParseErrors  int
	FileErrors   int
	ProjectCount int
}

// ProgressFunc is called during loading to report progress.
// current is the number of files processed so far, total is the total count.
type ProgressFunc func(current, total int)

// Load discovers and parses all session files from the Claude data directory.
// It uses a bounded worker pool for parallel parsing.
func Load(claudeDir string, includeSubagents bool, progressFn ProgressFunc) (*LoadResult, error) {
	// Discover files
	files, err := source.ScanDir(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("scanning %s: %w", claudeDir, err)
	}

	if len(files) == 0 {
		return &LoadResult{}, nil
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

	result := &LoadResult{
		TotalFiles:   len(toProcess),
		ProjectCount: source.CountProjects(files),
	}

	if len(toProcess) == 0 {
		return result, nil
	}

	// Parallel parsing with bounded worker pool
	numWorkers := runtime.GOMAXPROCS(0)
	if numWorkers < 1 {
		numWorkers = 4
	}
	if numWorkers > len(toProcess) {
		numWorkers = len(toProcess)
	}

	work := make(chan int, len(toProcess))
	results := make([]source.ParseResult, len(toProcess))
	var wg sync.WaitGroup
	var processed atomic.Int64

	// Feed work
	for i := range toProcess {
		work <- i
	}
	close(work)

	// Spawn workers
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		go func() {
			defer wg.Done()
			for idx := range work {
				results[idx] = source.ParseFile(toProcess[idx])
				n := processed.Add(1)
				if progressFn != nil {
					progressFn(int(n), len(toProcess))
				}
			}
		}()
	}

	wg.Wait()

	// Collect results
	for _, pr := range results {
		if pr.Err != nil {
			result.FileErrors++
			continue
		}
		result.ParsedFiles++
		result.ParseErrors += pr.ParseErrors
		if pr.Stats.APICalls > 0 || pr.Stats.UserMessages > 0 {
			result.Sessions = append(result.Sessions, pr.Stats)
		}
	}

	return result, nil
}

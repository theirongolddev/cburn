package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"cburn/internal/source"
	"cburn/internal/store"
)

func BenchmarkLoad(b *testing.B) {
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := Load(claudeDir, true, nil)
		if err != nil {
			b.Fatal(err)
		}
		_ = result
	}
}

func BenchmarkParseFile(b *testing.B) {
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	files, err := source.ScanDir(claudeDir)
	if err != nil {
		b.Fatal(err)
	}

	// Find the largest file for worst-case benchmarking
	var biggest source.DiscoveredFile
	var biggestSize int64
	for _, f := range files {
		info, err := os.Stat(f.Path)
		if err != nil {
			continue
		}
		if info.Size() > biggestSize {
			biggestSize = info.Size()
			biggest = f
		}
	}

	b.Logf("Benchmarking largest file: %s (%.1f KB)", biggest.Path, float64(biggestSize)/1024)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result := source.ParseFile(biggest)
		if result.Err != nil {
			b.Fatal(result.Err)
		}
	}
}

func BenchmarkScanDir(b *testing.B) {
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files, err := source.ScanDir(claudeDir)
		if err != nil {
			b.Fatal(err)
		}
		_ = files
	}
}

func BenchmarkLoadWithCache(b *testing.B) {
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")

	cache, err := store.Open(CachePath())
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = cache.Close() }()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cr, err := LoadWithCache(claudeDir, true, cache, nil)
		if err != nil {
			b.Fatal(err)
		}
		_ = cr
	}
}

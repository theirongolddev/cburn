package source

import (
	"os"
	"path/filepath"
	"strings"
)

// ScanDir walks the Claude projects directory and discovers all JSONL session files.
// It returns discovered files categorized as main sessions or subagent sessions.
func ScanDir(claudeDir string) ([]DiscoveredFile, error) {
	projectsDir := filepath.Join(claudeDir, "projects")

	info, err := os.Stat(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, nil
	}

	var files []DiscoveredFile

	err = filepath.WalkDir(projectsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // intentionally skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}

		// Skip sessions-index.json and other non-session files
		name := d.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			return nil
		}

		rel, _ := filepath.Rel(projectsDir, path)
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) < 2 {
			return nil
		}

		projectDir := parts[0]
		project := decodeProjectName(projectDir)

		df := DiscoveredFile{
			Path:       path,
			Project:    project,
			ProjectDir: projectDir,
		}

		// Determine if this is a subagent file
		// Pattern: <project>/<session-uuid>/subagents/agent-<id>.jsonl
		if len(parts) >= 4 && parts[2] == "subagents" {
			df.IsSubagent = true
			df.ParentSession = parts[1]
			// Use parent+agent to avoid collisions across sessions
			df.SessionID = parts[1] + "/" + strings.TrimSuffix(name, ".jsonl")
		} else {
			// Main session: <project>/<session-uuid>.jsonl
			df.SessionID = strings.TrimSuffix(name, ".jsonl")
		}

		files = append(files, df)
		return nil
	})

	return files, err
}

// decodeProjectName extracts a human-readable project name from the encoded directory name.
// Claude Code encodes absolute paths by replacing "/" with "-", so:
//
//	"-Users-tayloreernisse-projects-gitlore" -> "gitlore"
//	"-Users-tayloreernisse-projects-my-cool-project" -> "my-cool-project"
//
// We find the last known path component ("projects", "repos", "src", "code", "home")
// and take everything after it. Falls back to the last non-empty segment.
func decodeProjectName(dirName string) string {
	parts := strings.Split(dirName, "-")

	// Known parent directory names that commonly precede the project name
	knownParents := map[string]bool{
		"projects": true, "repos": true, "src": true,
		"code": true, "workspace": true, "dev": true,
	}

	// Scan for the last known parent marker and join everything after it
	for i := len(parts) - 2; i >= 0; i-- {
		if knownParents[strings.ToLower(parts[i])] {
			name := strings.Join(parts[i+1:], "-")
			if name != "" {
				return name
			}
		}
	}

	// Fallback: return the last non-empty segment
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}

	return dirName
}

// CountProjects returns the number of unique projects in a set of discovered files.
func CountProjects(files []DiscoveredFile) int {
	seen := make(map[string]struct{})
	for _, f := range files {
		seen[f.Project] = struct{}{}
	}
	return len(seen)
}

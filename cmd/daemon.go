package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/theirongolddev/cburn/internal/daemon"
	"github.com/theirongolddev/cburn/internal/pipeline"

	"github.com/spf13/cobra"
)

type daemonRuntimeState struct {
	PID       int       `json:"pid"`
	Addr      string    `json:"addr"`
	StartedAt time.Time `json:"started_at"`
	DataDir   string    `json:"data_dir"`
}

var (
	flagDaemonAddr         string
	flagDaemonInterval     time.Duration
	flagDaemonDetach       bool
	flagDaemonPIDFile      string
	flagDaemonLogFile      string
	flagDaemonEventsBuffer int
	flagDaemonChild        bool
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run a background usage daemon with HTTP/SSE endpoints",
	RunE:  runDaemon,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon process and API status",
	RunE:  runDaemonStatus,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE:  runDaemonStop,
}

func init() {
	defaultPID := filepath.Join(pipeline.CacheDir(), "cburnd.pid")
	defaultLog := filepath.Join(pipeline.CacheDir(), "cburnd.log")

	daemonCmd.PersistentFlags().StringVar(&flagDaemonAddr, "addr", "127.0.0.1:8787", "HTTP listen address")
	daemonCmd.PersistentFlags().DurationVar(&flagDaemonInterval, "interval", 15*time.Second, "Polling interval")
	daemonCmd.PersistentFlags().StringVar(&flagDaemonPIDFile, "pid-file", defaultPID, "PID file path")
	daemonCmd.PersistentFlags().StringVar(&flagDaemonLogFile, "log-file", defaultLog, "Log file path for detached mode")
	daemonCmd.PersistentFlags().IntVar(&flagDaemonEventsBuffer, "events-buffer", 200, "Max in-memory events retained")

	daemonCmd.Flags().BoolVar(&flagDaemonDetach, "detach", false, "Run daemon as a background process")
	daemonCmd.Flags().BoolVar(&flagDaemonChild, "child", false, "Internal: mark detached child process")
	_ = daemonCmd.Flags().MarkHidden("child")

	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(_ *cobra.Command, _ []string) error {
	if flagDaemonDetach && flagDaemonChild {
		return errors.New("invalid daemon launch mode")
	}

	if flagDaemonDetach {
		return startDaemonDetached()
	}

	return runDaemonForeground()
}

func startDaemonDetached() error {
	if err := ensureDaemonNotRunning(flagDaemonPIDFile); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	args := filterDetachArg(os.Args[1:])
	args = append(args, "--child")

	if err := os.MkdirAll(filepath.Dir(flagDaemonPIDFile), 0o750); err != nil {
		return fmt.Errorf("create daemon directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(flagDaemonLogFile), 0o750); err != nil {
		return fmt.Errorf("create daemon log directory: %w", err)
	}

	//nolint:gosec // daemon log path is configured by the local user
	logf, err := os.OpenFile(flagDaemonLogFile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open daemon log file: %w", err)
	}
	defer func() { _ = logf.Close() }()

	cmd := exec.Command(exe, args...) //nolint:gosec // exe/args come from current process invocation
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.Stdin = nil
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start detached daemon: %w", err)
	}

	fmt.Printf("  Started daemon (pid %d)\n", cmd.Process.Pid)
	fmt.Printf("  PID file: %s\n", flagDaemonPIDFile)
	fmt.Printf("  API: http://%s/v1/status\n", flagDaemonAddr)
	fmt.Printf("  Log: %s\n", flagDaemonLogFile)
	return nil
}

func runDaemonForeground() error {
	if err := ensureDaemonNotRunning(flagDaemonPIDFile); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(flagDaemonPIDFile), 0o750); err != nil {
		return fmt.Errorf("create daemon directory: %w", err)
	}

	pid := os.Getpid()
	if err := writePID(flagDaemonPIDFile, pid); err != nil {
		return err
	}
	defer func() { _ = os.Remove(flagDaemonPIDFile) }()

	state := daemonRuntimeState{
		PID:       pid,
		Addr:      flagDaemonAddr,
		StartedAt: time.Now(),
		DataDir:   flagDataDir,
	}
	_ = writeState(statePath(flagDaemonPIDFile), state)
	defer func() { _ = os.Remove(statePath(flagDaemonPIDFile)) }()

	cfg := daemon.Config{
		DataDir:          flagDataDir,
		Days:             flagDays,
		ProjectFilter:    flagProject,
		ModelFilter:      flagModel,
		IncludeSubagents: !flagNoSubagents,
		UseCache:         !flagNoCache,
		Interval:         flagDaemonInterval,
		Addr:             flagDaemonAddr,
		EventsBuffer:     flagDaemonEventsBuffer,
	}
	svc := daemon.New(cfg)

	fmt.Printf("  cburn daemon listening on http://%s\n", flagDaemonAddr)
	fmt.Printf("  Polling every %s from %s\n", flagDaemonInterval, flagDataDir)
	fmt.Printf("  Stop with: cburn daemon stop --pid-file %s\n", flagDaemonPIDFile)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := svc.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func runDaemonStatus(_ *cobra.Command, _ []string) error {
	pid, err := readPID(flagDaemonPIDFile)
	if err != nil {
		fmt.Printf("  Daemon: not running (pid file not found)\n")
		return nil
	}

	alive := processAlive(pid)
	if !alive {
		fmt.Printf("  Daemon: stale pid file (pid %d not alive)\n", pid)
		return nil
	}

	addr := flagDaemonAddr
	if st, err := readState(statePath(flagDaemonPIDFile)); err == nil && st.Addr != "" {
		addr = st.Addr
	}

	fmt.Printf("  Daemon PID: %d\n", pid)
	fmt.Printf("  Address: http://%s\n", addr)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/v1/status") //nolint:noctx // short status probe
	if err != nil {
		fmt.Printf("  API status: unreachable (%v)\n", err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  API status: HTTP %d\n", resp.StatusCode)
		return nil
	}

	var st daemon.Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		fmt.Printf("  API status: malformed response (%v)\n", err)
		return nil
	}

	if st.LastPollAt.IsZero() {
		fmt.Printf("  Last poll: pending\n")
	} else {
		fmt.Printf("  Last poll: %s\n", st.LastPollAt.Local().Format(time.RFC3339))
	}
	fmt.Printf("  Poll count: %d\n", st.PollCount)
	fmt.Printf("  Sessions: %d\n", st.Summary.Sessions)
	fmt.Printf("  Tokens: %d\n", st.Summary.Tokens)
	fmt.Printf("  Cost: $%.2f\n", st.Summary.EstimatedCostUSD)
	if st.LastError != "" {
		fmt.Printf("  Last error: %s\n", st.LastError)
	}
	return nil
}

func runDaemonStop(_ *cobra.Command, _ []string) error {
	pid, err := readPID(flagDaemonPIDFile)
	if err != nil {
		return errors.New("daemon is not running")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find daemon process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal daemon process: %w", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			_ = os.Remove(flagDaemonPIDFile)
			_ = os.Remove(statePath(flagDaemonPIDFile))
			fmt.Printf("  Stopped daemon (pid %d)\n", pid)
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	return fmt.Errorf("daemon (pid %d) did not exit in time", pid)
}

func filterDetachArg(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--detach" || strings.HasPrefix(a, "--detach=") {
			continue
		}
		out = append(out, a)
	}
	return out
}

func ensureDaemonNotRunning(pidFile string) error {
	pid, err := readPID(pidFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if processAlive(pid) {
		return fmt.Errorf("daemon already running (pid %d)", pid)
	}
	_ = os.Remove(pidFile)
	_ = os.Remove(statePath(pidFile))
	return nil
}

func writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o600)
}

func readPID(path string) (int, error) {
	//nolint:gosec // daemon pid path is configured by the local user
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid in %s", path)
	}
	return pid, nil
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func statePath(pidFile string) string {
	return pidFile + ".json"
}

func writeState(path string, st daemonRuntimeState) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func readState(path string) (daemonRuntimeState, error) {
	var st daemonRuntimeState
	//nolint:gosec // daemon state path is configured by the local user
	data, err := os.ReadFile(path)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	return st, nil
}

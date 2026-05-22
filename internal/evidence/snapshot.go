package evidence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/slurmtack/slurmtack/internal/remote"
)

type SnapshotCollector struct {
	writer *Writer
	runner remote.Runner
}

func NewSnapshotCollector(w *Writer, r remote.Runner) *SnapshotCollector {
	return &SnapshotCollector{writer: w, runner: r}
}

type SnapshotPhase string

const (
	PhasePre  SnapshotPhase = "pre"
	PhasePost SnapshotPhase = "post"
)

func (sc *SnapshotCollector) CaptureHostSnapshot(ctx context.Context, nodeName, executionID string, phase SnapshotPhase, req remote.CommandRequest) error {
	req.Host = nodeName
	req.Command = "gpu-switch-snapshot"
	req.Args = []string{"--phase", string(phase)}
	req.ExecutionID = executionID
	req.StepName = fmt.Sprintf("snapshot_%s", phase)

	result, err := sc.runner.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("capturing %s snapshot on %s: %w", phase, nodeName, err)
	}

	dir := sc.writer.ExecutionDir(nodeName, executionID)
	filename := fmt.Sprintf("%s.json", phase)
	path := filepath.Join(dir, "snapshots", filename)
	return os.WriteFile(path, []byte(result.Stdout), 0644)
}

func (sc *SnapshotCollector) CaptureRebootDiagnostics(ctx context.Context, nodeName, executionID string) error {
	dir := sc.writer.ExecutionDir(nodeName, executionID)

	commands := []struct {
		name    string
		command string
		args    []string
		output  string
	}{
		{"journal_current", "journalctl", []string{"-b", "--no-pager"}, "journal/journal-current.txt"},
		{"journal_previous", "journalctl", []string{"-b", "-1", "--no-pager"}, "journal/journal-previous-boot.txt"},
		{"dmesg", "dmesg", []string{"--no-pager"}, "journal/dmesg.txt"},
	}

	for _, cmd := range commands {
		req := remote.CommandRequest{
			Host:        nodeName,
			Command:     cmd.command,
			Args:        cmd.args,
			ExecutionID: executionID,
			StepName:    cmd.name,
		}

		result, err := sc.runner.Execute(ctx, req)
		if err != nil {
			sc.writer.AppendEvent(nodeName, executionID, map[string]any{
				"type":  "diagnostics_error",
				"step":  cmd.name,
				"error": err.Error(),
			})
			continue
		}

		path := filepath.Join(dir, cmd.output)
		if writeErr := os.WriteFile(path, []byte(result.Stdout), 0644); writeErr != nil {
			return fmt.Errorf("writing %s: %w", cmd.output, writeErr)
		}
	}

	return nil
}

package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/slurmtack/slurmtack/internal/domain"
)

type Writer struct {
	baseDir string
}

func NewWriter(baseDir string) *Writer {
	return &Writer{baseDir: baseDir}
}

func DefaultWriter() *Writer {
	return NewWriter("/var/log/gpu-switch")
}

func (w *Writer) ExecutionDir(nodeName, executionID string) string {
	return filepath.Join(w.baseDir, nodeName, executionID)
}

func (w *Writer) InitExecution(exec *domain.Execution) error {
	dir := w.ExecutionDir(exec.NodeName, exec.ID)
	for _, sub := range []string{"steps", "snapshots", "journal"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0755); err != nil {
			return fmt.Errorf("creating evidence directory %s: %w", sub, err)
		}
	}

	manifest := map[string]any{
		"execution_id": exec.ID,
		"node_name":    exec.NodeName,
		"direction":    exec.Direction,
		"requested_by": exec.RequestedBy,
		"requested_at": exec.RequestedAt,
		"created_at":   time.Now(),
	}

	return w.writeJSON(filepath.Join(dir, "manifest.json"), manifest)
}

func (w *Writer) AppendEvent(nodeName, executionID string, event map[string]any) error {
	dir := w.ExecutionDir(nodeName, executionID)
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening events file: %w", err)
	}
	defer f.Close()

	event["timestamp"] = time.Now()
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}

func (w *Writer) WriteStepOutput(nodeName, executionID, stepName, stream, content string) error {
	dir := w.ExecutionDir(nodeName, executionID)
	filename := fmt.Sprintf("%s.%s.log", stepName, stream)
	path := filepath.Join(dir, "steps", filename)
	return os.WriteFile(path, []byte(content), 0644)
}

func (w *Writer) WriteManifestUpdate(exec *domain.Execution) error {
	dir := w.ExecutionDir(exec.NodeName, exec.ID)
	manifest := map[string]any{
		"execution_id":        exec.ID,
		"node_name":           exec.NodeName,
		"direction":           exec.Direction,
		"requested_by":        exec.RequestedBy,
		"requested_at":        exec.RequestedAt,
		"current_state":       exec.CurrentState,
		"state_version":       exec.StateVersion,
		"overall_status":      exec.OverallStatus,
		"final_error_code":    exec.FinalErrorCode,
		"final_error_summary": exec.FinalErrorSummary,
		"updated_at":          time.Now(),
	}
	return w.writeJSON(filepath.Join(dir, "manifest.json"), manifest)
}

func (w *Writer) writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling json: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

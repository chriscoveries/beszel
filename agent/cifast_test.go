package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validCifastSnapshot = `{
  "schema": "cifast.monitor.v1",
  "captured_at": "2026-11-11T00:00:00Z",
  "window_seconds": 86400,
  "status": "OK",
  "coverage": {"open_jobs": "OBSERVED", "history": "OBSERVED"},
  "metrics": {
    "jobs_queued": {"value": 0, "unit": "jobs"},
    "jobs_running": {"value": 2, "unit": "jobs"},
    "oldest_queue_seconds": {"value": null, "unit": "seconds"},
    "jobs_finished": {"value": 31, "unit": "jobs"},
    "jobs_vanished": {"value": 1, "unit": "jobs"},
    "queue_p50_seconds": {"value": 42, "unit": "seconds"},
    "queue_p90_seconds": {"value": 180, "unit": "seconds"},
    "run_p50_seconds": {"value": 311, "unit": "seconds"},
    "run_p90_seconds": {"value": 702, "unit": "seconds"}
  }
}`

func writeCifastTestSnapshot(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cifast.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadCifastInfo(t *testing.T) {
	path := writeCifastTestSnapshot(t, validCifastSnapshot)
	info, err := readCifastInfo(path, time.Date(2026, 11, 11, 0, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Status != "OK" || info.Queued == nil || *info.Queued != 0 || info.Running == nil || *info.Running != 2 {
		t.Fatalf("unexpected CIFast info: %#v", info)
	}
	if info.OldestQueue != nil || info.QueueP90Seconds == nil || *info.QueueP90Seconds != 180 {
		t.Fatalf("unexpected CIFast metrics: %#v", info)
	}
}

func TestReadCifastInfoStaleWithdrawsValues(t *testing.T) {
	path := writeCifastTestSnapshot(t, validCifastSnapshot)
	info, err := readCifastInfo(path, time.Date(2026, 11, 11, 0, 6, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if info == nil || info.Status != "STALE" || info.Queued != nil || info.Running != nil {
		t.Fatalf("stale snapshot retained values: %#v", info)
	}
}

func TestReadCifastInfoRejectsSchemaAndUnit(t *testing.T) {
	for name, content := range map[string]string{
		"schema": replaceOnce(validCifastSnapshot, "cifast.monitor.v1", "cifast.monitor.v2"),
		"unit":   replaceOnce(validCifastSnapshot, `"jobs_queued": {"value": 0, "unit": "jobs"}`, `"jobs_queued": {"value": 0, "unit": "seconds"}`),
	} {
		t.Run(name, func(t *testing.T) {
			path := writeCifastTestSnapshot(t, content)
			if info, err := readCifastInfo(path, time.Date(2026, 11, 11, 0, 1, 0, 0, time.UTC)); err == nil || info != nil {
				t.Fatalf("invalid snapshot accepted: info=%#v err=%v", info, err)
			}
		})
	}
}

func TestReadCifastInfoMissingAndSymlink(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	info, err := readCifastInfo(missing, time.Now())
	if err != nil || info != nil {
		t.Fatalf("missing file result: info=%#v err=%v", info, err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(validCifastSnapshot), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if info, err := readCifastInfo(link, time.Now()); err == nil || info != nil {
		t.Fatalf("symlink accepted: info=%#v err=%v", info, err)
	}
}

func replaceOnce(value, old, replacement string) string {
	for i := 0; i+len(old) <= len(value); i++ {
		if value[i:i+len(old)] == old {
			return value[:i] + replacement + value[i+len(old):]
		}
	}
	return value
}

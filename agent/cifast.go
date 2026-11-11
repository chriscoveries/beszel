package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	"github.com/henrygd/beszel/agent/utils"
	"github.com/henrygd/beszel/internal/entities/system"
)

const (
	defaultCifastMonitorFile = "/run/cifast-monitor.json"
	maxCifastMonitorBytes    = 64 * 1024
	maxCifastSnapshotAge     = 5 * time.Minute
	maxCifastFutureSkew      = time.Minute
	maxCifastWindowSeconds   = 30 * 24 * 60 * 60
	maxCifastMetricSeconds   = 365 * 24 * 60 * 60
	maxCifastJobCount        = 1_000_000_000
)

type cifastSnapshot struct {
	Schema        string `json:"schema"`
	CapturedAt    string `json:"captured_at"`
	WindowSeconds int64  `json:"window_seconds"`
	Status        string `json:"status"`
	Coverage      struct {
		OpenJobs string `json:"open_jobs"`
		History  string `json:"history"`
	} `json:"coverage"`
	Metrics struct {
		JobsQueued         cifastMetric `json:"jobs_queued"`
		JobsRunning        cifastMetric `json:"jobs_running"`
		OldestQueueSeconds cifastMetric `json:"oldest_queue_seconds"`
		JobsFinished       cifastMetric `json:"jobs_finished"`
		JobsVanished       cifastMetric `json:"jobs_vanished"`
		QueueP50Seconds    cifastMetric `json:"queue_p50_seconds"`
		QueueP90Seconds    cifastMetric `json:"queue_p90_seconds"`
		RunP50Seconds      cifastMetric `json:"run_p50_seconds"`
		RunP90Seconds      cifastMetric `json:"run_p90_seconds"`
	} `json:"metrics"`
}

type cifastMetric struct {
	Value *float64 `json:"value"`
	Unit  string   `json:"unit"`
}

func getCifastInfo(now time.Time) *system.CifastInfo {
	path := defaultCifastMonitorFile
	if configured, exists := utils.GetEnv("CIFAST_MONITOR_FILE"); exists {
		path = strings.TrimSpace(configured)
		if path == "" || path == "-" {
			return nil
		}
	}
	info, err := readCifastInfo(path, now)
	if err != nil {
		slog.Debug("CIFast monitor snapshot unavailable", "path", path, "err", err)
		return nil
	}
	return info
}

func readCifastInfo(path string, now time.Time) (*system.CifastInfo, error) {
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat CIFast monitor snapshot: %w", err)
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("CIFast monitor snapshot must be a regular file")
	}
	if before.Size() > maxCifastMonitorBytes {
		return nil, fmt.Errorf("CIFast monitor snapshot exceeds %d bytes", maxCifastMonitorBytes)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open CIFast monitor snapshot: %w", err)
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("re-stat CIFast monitor snapshot: %w", err)
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, errors.New("CIFast monitor snapshot changed or resolved through a link")
	}
	content, err := io.ReadAll(io.LimitReader(file, maxCifastMonitorBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read CIFast monitor snapshot: %w", err)
	}
	if len(content) > maxCifastMonitorBytes {
		return nil, fmt.Errorf("CIFast monitor snapshot exceeds %d bytes", maxCifastMonitorBytes)
	}

	var snapshot cifastSnapshot
	if err := json.Unmarshal(content, &snapshot); err != nil {
		return nil, fmt.Errorf("decode CIFast monitor snapshot: %w", err)
	}
	if snapshot.Schema != "cifast.monitor.v1" {
		return nil, fmt.Errorf("unsupported CIFast monitor schema %q", snapshot.Schema)
	}
	if snapshot.WindowSeconds <= 0 || snapshot.WindowSeconds > maxCifastWindowSeconds {
		return nil, errors.New("CIFast monitor window is outside the supported range")
	}
	if !validCifastStatus(snapshot.Status) {
		return nil, fmt.Errorf("invalid CIFast monitor status %q", snapshot.Status)
	}
	if !validCifastCoverage(snapshot.Coverage.OpenJobs) || !validCifastCoverage(snapshot.Coverage.History) {
		return nil, errors.New("invalid CIFast monitor coverage")
	}
	capturedAt, err := time.Parse(time.RFC3339, snapshot.CapturedAt)
	if err != nil {
		return nil, errors.New("invalid CIFast monitor capture time")
	}
	capturedAt = capturedAt.UTC()
	now = now.UTC()
	if capturedAt.After(now.Add(maxCifastFutureSkew)) {
		return nil, errors.New("CIFast monitor capture time is in the future")
	}

	info := &system.CifastInfo{
		CapturedAt:      capturedAt.Unix(),
		Status:          snapshot.Status,
		OpenCoverage:    snapshot.Coverage.OpenJobs,
		HistoryCoverage: snapshot.Coverage.History,
		WindowSeconds:   uint32(snapshot.WindowSeconds),
	}
	if now.Sub(capturedAt) > maxCifastSnapshotAge {
		info.Status = "STALE"
		return info, nil
	}

	if info.Queued, err = validateCifastMetric("jobs_queued", snapshot.Metrics.JobsQueued, "jobs", true); err != nil {
		return nil, err
	}
	if info.Running, err = validateCifastMetric("jobs_running", snapshot.Metrics.JobsRunning, "jobs", true); err != nil {
		return nil, err
	}
	if info.OldestQueue, err = validateCifastMetric("oldest_queue_seconds", snapshot.Metrics.OldestQueueSeconds, "seconds", false); err != nil {
		return nil, err
	}
	if info.Finished, err = validateCifastMetric("jobs_finished", snapshot.Metrics.JobsFinished, "jobs", true); err != nil {
		return nil, err
	}
	if info.Vanished, err = validateCifastMetric("jobs_vanished", snapshot.Metrics.JobsVanished, "jobs", true); err != nil {
		return nil, err
	}
	if info.QueueP50Seconds, err = validateCifastMetric("queue_p50_seconds", snapshot.Metrics.QueueP50Seconds, "seconds", false); err != nil {
		return nil, err
	}
	if info.QueueP90Seconds, err = validateCifastMetric("queue_p90_seconds", snapshot.Metrics.QueueP90Seconds, "seconds", false); err != nil {
		return nil, err
	}
	if info.RunP50Seconds, err = validateCifastMetric("run_p50_seconds", snapshot.Metrics.RunP50Seconds, "seconds", false); err != nil {
		return nil, err
	}
	if info.RunP90Seconds, err = validateCifastMetric("run_p90_seconds", snapshot.Metrics.RunP90Seconds, "seconds", false); err != nil {
		return nil, err
	}
	return info, nil
}

func validateCifastMetric(name string, metric cifastMetric, unit string, integral bool) (*float64, error) {
	if metric.Unit != unit {
		return nil, fmt.Errorf("CIFast metric %s has unit %q, expected %q", name, metric.Unit, unit)
	}
	if metric.Value == nil {
		return nil, nil
	}
	value := *metric.Value
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return nil, fmt.Errorf("CIFast metric %s is invalid", name)
	}
	maximum := float64(maxCifastMetricSeconds)
	if integral {
		maximum = maxCifastJobCount
		if math.Trunc(value) != value {
			return nil, fmt.Errorf("CIFast metric %s must be an integer", name)
		}
	}
	if value > maximum {
		return nil, fmt.Errorf("CIFast metric %s exceeds its bound", name)
	}
	return &value, nil
}

func validCifastStatus(value string) bool {
	switch value {
	case "OK", "PARTIAL", "UNOBSERVED":
		return true
	default:
		return false
	}
}

func validCifastCoverage(value string) bool {
	switch value {
	case "OBSERVED", "PARTIAL", "STALE", "UNOBSERVED":
		return true
	default:
		return false
	}
}

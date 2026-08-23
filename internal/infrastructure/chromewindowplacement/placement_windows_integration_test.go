//go:build windows

package chromewindowplacement

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	applicationLaunch "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
	applicationPlacement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	applicationTracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
	infraDetection "github.com/asam-masa/browser-launcher/internal/infrastructure/chromedetection"
	infraLaunch "github.com/asam-masa/browser-launcher/internal/infrastructure/chromelaunch"
	infraTracking "github.com/asam-masa/browser-launcher/internal/infrastructure/chromewindowtracking"
)

const (
	windowPlacementIntegrationOptIn = "BROWSER_LAUNCHER_RUN_WINDOW_PLACEMENT_INTEGRATION"
	windowPlacementPrepareSeconds   = "BROWSER_LAUNCHER_WINDOW_PLACEMENT_PREPARE_SECONDS"
	windowPlacementRejectFinal      = "BROWSER_LAUNCHER_WINDOW_PLACEMENT_REJECT_FINAL"
	maxWindowPlacementPrepare       = 60
)

func TestProviderPlaceIntegrationOnWindows(t *testing.T) {
	if os.Getenv(windowPlacementIntegrationOptIn) != "1" {
		t.Skip("set BROWSER_LAUNCHER_RUN_WINDOW_PLACEMENT_INTEGRATION=1 to launch and move a new Chrome window")
	}
	prepareSeconds, err := parsePrepareSeconds(os.Getenv(windowPlacementPrepareSeconds))
	if err != nil {
		t.Fatal(err)
	}
	rejectFinal, err := parseRejectFinal(os.Getenv(windowPlacementRejectFinal))
	if err != nil {
		t.Fatal(err)
	}

	detector := infraDetection.NewProvider()
	installation, err := detector.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}

	trackingProvider := infraTracking.NewProvider(installation.ExecutablePath)
	trackingService := applicationTracking.NewService(trackingProvider)
	session, err := trackingService.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	defer func() {
		if closeErr := session.Close(); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	}()

	launchService := applicationLaunch.NewService(detector, infraLaunch.NewProvider())
	if _, err := launchService.Launch(); err != nil {
		t.Fatalf("Launch() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	trackingResult := session.Wait(ctx)
	if trackingResult.Outcome != applicationTracking.OutcomeCompleted {
		t.Fatalf("Wait() outcome = %s, error = %v, want completed", trackingResult.Outcome, trackingResult.Err)
	}
	if prepareSeconds > 0 {
		t.Logf("waiting %d seconds before placement; move the tracked Chrome window to the source display", prepareSeconds)
		time.Sleep(time.Duration(prepareSeconds) * time.Second)
	}

	requested := applicationPlacement.Rectangle{X: 100, Y: 100, Width: 1000, Height: 700}
	logicalWidth := 1000
	if rejectFinal {
		logicalWidth = 100000
	}
	provider := NewProvider(trackingProvider)
	recorder := &recordingWindowSystem{delegate: provider.system}
	provider.system = recorder
	placementService := applicationPlacement.NewService(provider)

	result, err := placementService.Place(ctx, applicationPlacement.Request{
		Window: trackingResult.Candidate,
		LogicalRequested: domain.New(
			integrationDimension(t, logicalWidth),
			integrationDimension(t, 700),
			integrationCoordinate(t, 100),
			integrationCoordinate(t, 100),
		),
		InitialTarget: requested,
	})
	if rejectFinal {
		if !errors.Is(err, applicationPlacement.ErrFinalValidation) {
			t.Fatalf("Place() error = %v, result = %+v, want final validation error", err, result)
		}
		if result.Stage != applicationPlacement.StageFinalPlacement || !result.PartiallyChanged ||
			!result.ActualKnown || len(result.ValidationErrors) == 0 {
			t.Fatalf("Place() result = %+v, want rejected partial placement", result)
		}
		if moves := moveIndexesOnly(recorder.trace); len(moves) != 1 || moves[0].resize {
			t.Fatalf("placement moves = %+v, want one no-resize move", moves)
		}
		t.Logf("final placement rejected: actual=%+v validation_errors=%+v", result.Actual, result.ValidationErrors)
		return
	}
	if err != nil {
		t.Fatalf("Place() error = %v, result = %+v", err, result)
	}
	if result.Stage != applicationPlacement.StageCompleted || !result.Matched || result.Actual != result.Requested {
		t.Fatalf("Place() result = %+v, want completed exact match", result)
	}
	finalRequested := result.Requested

	trace := recorder.trace
	firstMoveIndex, secondMoveIndex := moveIndexes(trace)
	if firstMoveIndex < 1 || secondMoveIndex <= firstMoveIndex+requiredStableSamples {
		t.Fatalf("placement trace has invalid move order: %+v", trace)
	}
	initial := trace[firstMoveIndex-1].measurement
	firstMove := trace[firstMoveIndex].move
	secondMove := trace[secondMoveIndex].move
	final := trace[len(trace)-1].measurement
	if firstMove.resize {
		t.Fatal("first Move() resized the window")
	}
	if firstMove.bounds.Width != initial.bounds.Width || firstMove.bounds.Height != initial.bounds.Height {
		t.Fatalf("first Move() size = %dx%d, want initial %dx%d", firstMove.bounds.Width, firstMove.bounds.Height, initial.bounds.Width, initial.bounds.Height)
	}
	if !secondMove.resize || secondMove.bounds != finalRequested {
		t.Fatalf("second Move() = %+v, want final requested bounds %+v", secondMove, finalRequested)
	}
	if final.bounds != finalRequested {
		t.Fatalf("final measurement = %+v, want %+v", final.bounds, finalRequested)
	}

	stableAfterFirstMove := trace[secondMoveIndex-requiredStableSamples : secondMoveIndex]
	for index, event := range stableAfterFirstMove {
		if event.kind != traceMeasurement || event.measurement != stableAfterFirstMove[0].measurement {
			t.Fatalf("measurement %d before final move was not stable: %+v", index, stableAfterFirstMove)
		}
	}

	t.Logf(
		"placement completed: initial_bounds=%+v initial_dpi=%d moved_bounds=%+v moved_dpi=%d logical_bounds={X:100 Y:100 Width:1000 Height:700} final_requested=%+v final_bounds=%+v final_dpi=%d monitor_changed=%t",
		initial.bounds,
		initial.dpi,
		stableAfterFirstMove[0].measurement.bounds,
		stableAfterFirstMove[0].measurement.dpi,
		finalRequested,
		final.bounds,
		final.dpi,
		initial.monitor != final.monitor,
	)
}

func parseRejectFinal(raw string) (bool, error) {
	switch raw {
	case "":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("%s must be unset or 1", windowPlacementRejectFinal)
	}
}

func integrationDimension(t *testing.T, value int) domain.Dimension {
	t.Helper()
	dimension, err := domain.NewDimension(value)
	if err != nil {
		t.Fatalf("NewDimension(%d) error = %v", value, err)
	}
	return dimension
}

func integrationCoordinate(t *testing.T, value int) domain.Coordinate {
	t.Helper()
	coordinate, err := domain.NewCoordinate(value)
	if err != nil {
		t.Fatalf("NewCoordinate(%d) error = %v", value, err)
	}
	return coordinate
}

func parsePrepareSeconds(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 || seconds > maxWindowPlacementPrepare {
		return 0, fmt.Errorf(
			"%s must be an integer from 0 through %d",
			windowPlacementPrepareSeconds,
			maxWindowPlacementPrepare,
		)
	}
	return seconds, nil
}

func TestParsePrepareSeconds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    int
		wantErr bool
	}{
		{name: "unset", raw: "", want: 0},
		{name: "minimum", raw: "0", want: 0},
		{name: "maximum", raw: "60", want: 60},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "above maximum", raw: "61", wantErr: true},
		{name: "not an integer", raw: "invalid", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePrepareSeconds(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePrepareSeconds(%q) error = %v, wantErr %t", tt.raw, err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("parsePrepareSeconds(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseRejectFinal(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		raw     string
		want    bool
		wantErr bool
	}{
		{name: "unset"},
		{name: "enabled", raw: "1", want: true},
		{name: "invalid", raw: "true", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseRejectFinal(tt.raw)
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("parseRejectFinal(%q) = %t, %v, want %t, wantErr %t", tt.raw, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

type traceKind string

const (
	traceMeasurement traceKind = "measurement"
	traceMove        traceKind = "move"
)

type placementTraceEvent struct {
	kind        traceKind
	measurement measurement
	move        recordedMove
}

type recordedMove struct {
	bounds applicationPlacement.Rectangle
	resize bool
}

type recordingWindowSystem struct {
	delegate windowSystem
	trace    []placementTraceEvent
}

func moveIndexesOnly(trace []placementTraceEvent) []recordedMove {
	var moves []recordedMove
	for _, event := range trace {
		if event.kind == traceMove {
			moves = append(moves, event.move)
		}
	}
	return moves
}

func (s *recordingWindowSystem) BeginDPIAware() (func() error, error) {
	return s.delegate.BeginDPIAware()
}

func (s *recordingWindowSystem) TargetMonitor(bounds applicationPlacement.Rectangle) (targetMonitor, error) {
	return s.delegate.TargetMonitor(bounds)
}

func (s *recordingWindowSystem) Measure(handle uint64) (measurement, error) {
	measured, err := s.delegate.Measure(handle)
	if err == nil {
		s.trace = append(s.trace, placementTraceEvent{kind: traceMeasurement, measurement: measured})
	}
	return measured, err
}

func (s *recordingWindowSystem) Move(handle uint64, bounds applicationPlacement.Rectangle, resize bool) error {
	err := s.delegate.Move(handle, bounds, resize)
	if err == nil {
		s.trace = append(s.trace, placementTraceEvent{
			kind: traceMove,
			move: recordedMove{bounds: bounds, resize: resize},
		})
	}
	return err
}

func moveIndexes(trace []placementTraceEvent) (int, int) {
	first := -1
	second := -1
	for index, event := range trace {
		if event.kind != traceMove {
			continue
		}
		if first == -1 {
			first = index
			continue
		}
		second = index
		break
	}
	return first, second
}

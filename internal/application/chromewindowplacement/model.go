package chromewindowplacement

import (
	"context"

	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
)

type Rectangle struct {
	X      int
	Y      int
	Width  int
	Height int
}

type Request struct {
	Window           tracking.Window
	LogicalRequested domain.Condition
	InitialTarget    Rectangle
}

type FinalPlacementContext struct {
	MonitorLeft int
	MonitorTop  int
	WorkLeft    int
	WorkTop     int
	WorkWidth   int
	WorkHeight  int
	DPI         int
}

type ValidationError struct {
	Field   string
	Message string
}

type Stage string

const (
	StageValidation         Stage = "validation"
	StageInitialMeasurement Stage = "initial_measurement"
	StageMoveToTarget       Stage = "move_to_target"
	StageWaitForStability   Stage = "wait_for_stability"
	StageFinalPlacement     Stage = "final_placement"
	StageFinalMeasurement   Stage = "final_measurement"
	StageCompleted          Stage = "completed"
)

type Result struct {
	Stage            Stage
	Requested        Rectangle
	Actual           Rectangle
	ActualKnown      bool
	PartiallyChanged bool
	Matched          bool
	ValidationErrors []ValidationError
}

type FinalBoundsResolver func(FinalPlacementContext) (Rectangle, []ValidationError, error)

type Placer interface {
	Place(context.Context, Request, FinalBoundsResolver) (Result, error)
}

package chromelaunchoperation

import (
	"context"
	"errors"
	"fmt"

	chromelaunch "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
	placement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	tracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

type Outcome string

const (
	OutcomeCompleted Outcome = "completed"
	OutcomeCancelled Outcome = "cancelled"
	OutcomeTimedOut  Outcome = "timed_out"
	OutcomeFailed    Outcome = "failed"
	OutcomeInvalid   Outcome = "invalid"
)

type ErrorCode string

const (
	ErrorCodeNone              ErrorCode = ""
	ErrorCodeInvalidRequest    ErrorCode = "invalid_request"
	ErrorCodeWorkArea          ErrorCode = "work_area_unavailable"
	ErrorCodeChromeNotFound    ErrorCode = "chrome_not_found"
	ErrorCodeUnsupported       ErrorCode = "unsupported_platform"
	ErrorCodeLaunchFailed      ErrorCode = "launch_failed"
	ErrorCodeObservationFailed ErrorCode = "window_observation_failed"
	ErrorCodeAmbiguousWindows  ErrorCode = "ambiguous_windows"
	ErrorCodeCandidateLost     ErrorCode = "window_candidate_lost"
	ErrorCodePlacementFailed   ErrorCode = "placement_failed"
	ErrorCodeUnexpected        ErrorCode = "unexpected_failure"
)

type WorkflowResult struct {
	Outcome          Outcome
	ErrorCode        ErrorCode
	ValidationErrors []launchcondition.FieldError
	Placement        placement.Result
	Err              error
}

type Workflow struct {
	conditions launchcondition.Service
	launcher   chromelaunch.Service
	windows    WindowServicesFactory
}

type WindowServices struct {
	Tracker tracking.Service
	Placer  placement.Service
}

type WindowServicesFactory func(executablePath string) WindowServices

func NewWorkflow(
	conditions launchcondition.Service,
	launcher chromelaunch.Service,
	windows WindowServicesFactory,
) Workflow {
	return Workflow{
		conditions: conditions,
		launcher:   launcher,
		windows:    windows,
	}
}

func (w Workflow) Run(ctx context.Context, input launchcondition.Input) WorkflowResult {
	if ctx == nil {
		return failedWorkflow(ErrorCodeInvalidRequest, ErrNilContext)
	}
	if result, done := workflowContextResult(ctx); done {
		return result
	}

	prepared, fieldErrors, err := w.conditions.Prepare(input)
	if err != nil {
		return failedWorkflow(ErrorCodeWorkArea, fmt.Errorf("prepare launch condition: %w", err))
	}
	if len(fieldErrors) > 0 {
		return WorkflowResult{
			Outcome:          OutcomeInvalid,
			ErrorCode:        ErrorCodeInvalidRequest,
			ValidationErrors: fieldErrors,
		}
	}
	if result, done := workflowContextResult(ctx); done {
		return result
	}

	installation, err := w.launcher.Detect()
	if err != nil {
		return failedWorkflow(classifyLaunchError(err), fmt.Errorf("detect Chrome: %w", err))
	}
	if result, done := workflowContextResult(ctx); done {
		return result
	}
	if w.windows == nil {
		return failedWorkflow(ErrorCodeUnexpected, errors.New("window services factory is not configured"))
	}
	windowServices := w.windows(installation.ExecutablePath)

	session, err := windowServices.Tracker.Begin()
	if err != nil {
		return failedWorkflow(ErrorCodeObservationFailed, fmt.Errorf("begin window tracking: %w", err))
	}
	if result, done := workflowContextResult(ctx); done {
		return closeSessionResult(session, result)
	}

	if _, err := w.launcher.LaunchDetected(installation); err != nil {
		result := failedWorkflow(classifyLaunchError(err), fmt.Errorf("launch Chrome: %w", err))
		return closeSessionResult(session, result)
	}

	tracked := session.Wait(ctx)
	if tracked.Outcome != tracking.OutcomeCompleted {
		return trackingWorkflowResult(tracked)
	}
	if result, done := workflowContextResult(ctx); done {
		return result
	}

	initial := prepared.PhysicalBounds
	placed, err := windowServices.Placer.Place(ctx, placement.Request{
		Window:           tracked.Candidate,
		LogicalRequested: prepared.Condition,
		InitialTarget: placement.Rectangle{
			X: initial.X, Y: initial.Y, Width: initial.Width, Height: initial.Height,
		},
	})
	if err != nil {
		if result, done := workflowContextResult(ctx); done {
			result.Placement = placed
			result.Err = err
			return result
		}
		return WorkflowResult{
			Outcome:   OutcomeFailed,
			ErrorCode: ErrorCodePlacementFailed,
			Placement: placed,
			Err:       fmt.Errorf("place Chrome window: %w", err),
		}
	}

	return WorkflowResult{Outcome: OutcomeCompleted, Placement: placed}
}

func workflowContextResult(ctx context.Context) (WorkflowResult, bool) {
	switch {
	case errors.Is(ctx.Err(), context.Canceled):
		return WorkflowResult{Outcome: OutcomeCancelled}, true
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return WorkflowResult{Outcome: OutcomeTimedOut}, true
	default:
		return WorkflowResult{}, false
	}
}

func closeSessionResult(session *tracking.Session, result WorkflowResult) WorkflowResult {
	if err := session.Close(); err != nil {
		result.Err = errors.Join(result.Err, fmt.Errorf("close window tracking: %w", err))
		if result.ErrorCode == ErrorCodeNone {
			result.Outcome = OutcomeFailed
			result.ErrorCode = ErrorCodeObservationFailed
		}
	}
	return result
}

func trackingWorkflowResult(result tracking.Result) WorkflowResult {
	switch result.Outcome {
	case tracking.OutcomeCancelled:
		return WorkflowResult{Outcome: OutcomeCancelled, Err: result.Err}
	case tracking.OutcomeTimedOut:
		return WorkflowResult{Outcome: OutcomeTimedOut, Err: result.Err}
	case tracking.OutcomeAmbiguous:
		return failedWorkflow(ErrorCodeAmbiguousWindows, result.Err)
	case tracking.OutcomeCandidateLost:
		return failedWorkflow(ErrorCodeCandidateLost, result.Err)
	case tracking.OutcomeFailed:
		return failedWorkflow(ErrorCodeObservationFailed, result.Err)
	default:
		return failedWorkflow(
			ErrorCodeUnexpected,
			fmt.Errorf("unexpected window tracking outcome %q", result.Outcome),
		)
	}
}

func classifyLaunchError(err error) ErrorCode {
	switch {
	case errors.Is(err, chromelaunch.ErrChromeNotFound):
		return ErrorCodeChromeNotFound
	case errors.Is(err, chromelaunch.ErrUnsupportedPlatform):
		return ErrorCodeUnsupported
	case errors.Is(err, chromelaunch.ErrLaunchFailed):
		return ErrorCodeLaunchFailed
	default:
		return ErrorCodeUnexpected
	}
}

func failedWorkflow(code ErrorCode, err error) WorkflowResult {
	return WorkflowResult{Outcome: OutcomeFailed, ErrorCode: code, Err: err}
}

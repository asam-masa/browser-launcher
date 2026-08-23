package wailsadapter

import (
	"context"
	"errors"
	"testing"

	operation "github.com/asam-masa/browser-launcher/internal/application/chromelaunchoperation"
	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

func TestLaunchOperationControllerDoesNotStartInvalidInput(t *testing.T) {
	t.Parallel()

	operations := &stubLaunchOperationService{}
	controller := newTestLaunchOperationController(operations)
	result, err := controller.StartLaunch(LaunchConditionInputDTO{})

	if err != nil {
		t.Fatalf("StartLaunch() error = %v", err)
	}
	if result.Started || result.Validation.Valid || len(result.Validation.Errors) != 4 {
		t.Fatalf("StartLaunch() = %+v, want validation errors without starting", result)
	}
	if operations.startCalls != 0 {
		t.Fatalf("operation Start() calls = %d, want 0", operations.startCalls)
	}
}

func TestLaunchOperationControllerStartsValidInput(t *testing.T) {
	t.Parallel()

	operations := &stubLaunchOperationService{
		startSnapshot: operation.Snapshot{ID: "operation-1", State: operation.StateStarting},
	}
	controller := newTestLaunchOperationController(operations)
	input := LaunchConditionInputDTO{Width: "1000", Height: "700", X: "100", Y: "100"}
	result, err := controller.StartLaunch(input)

	if err != nil {
		t.Fatalf("StartLaunch() error = %v", err)
	}
	if !result.Started || result.OperationID != "operation-1" || result.State != "starting" || !result.Validation.Valid {
		t.Fatalf("StartLaunch() = %+v, want started operation", result)
	}
	if operations.input != (launchcondition.Input{Width: "1000", Height: "700", X: "100", Y: "100"}) {
		t.Fatalf("operation input = %+v, want converted input", operations.input)
	}
}

func TestLaunchOperationControllerReturnsSafeStartErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "operation in progress",
			err:     operation.ErrOperationInProgress,
			message: "Chromeの起動処理を実行中です。完了または取消後にもう一度お試しください。",
		},
		{
			name:    "internal failure",
			err:     errors.New(`open C:\Users\someone\secret: access denied`),
			message: "Chromeの起動処理を開始できませんでした。もう一度お試しください。",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			operations := &stubLaunchOperationService{startErr: test.err}
			controller := newTestLaunchOperationController(operations)
			_, err := controller.StartLaunch(validLaunchConditionDTO())
			if err == nil || err.Error() != test.message {
				t.Fatalf("StartLaunch() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestLaunchOperationControllerRequiresRuntimeContext(t *testing.T) {
	t.Parallel()

	controller := NewLaunchOperationController(
		newValidationController(),
		&stubLaunchOperationService{},
		&stubLaunchRuntime{},
	)
	_, err := controller.StartLaunch(validLaunchConditionDTO())
	if !errors.Is(err, ErrApplicationNotReady) {
		t.Fatalf("StartLaunch() error = %v, want ErrApplicationNotReady", err)
	}
}

func TestLaunchOperationControllerCancelsAndGetsState(t *testing.T) {
	t.Parallel()

	operations := &stubLaunchOperationService{
		cancelStatus:   operation.CancelAccepted,
		cancelSnapshot: operation.Snapshot{ID: "operation-1", State: operation.StateCancelling},
		getSnapshot: operation.Snapshot{
			ID: operation.ID("operation-1"), State: operation.StateFailed, ErrorCode: operation.ErrorCodeChromeNotFound,
		},
		getFound: true,
		currentSnapshot: operation.Snapshot{
			ID: operation.ID("operation-2"), State: operation.StateRunning,
		},
		currentFound: true,
	}
	controller := newTestLaunchOperationController(operations)

	cancelled := controller.CancelLaunch("operation-1")
	if cancelled.Status != "accepted" || cancelled.State == nil || cancelled.State.State != "cancelling" {
		t.Fatalf("CancelLaunch() = %+v, want accepted cancellation", cancelled)
	}
	state := controller.GetLaunchState("operation-1")
	if !state.Found || state.State == nil || state.State.ErrorCode != "chrome_not_found" {
		t.Fatalf("GetLaunchState() = %+v, want Chrome-not-found failure", state)
	}
	if state.State.Message != "Chromeが見つかりませんでした。Chromeのインストール状況を確認してください。" {
		t.Fatalf("state message = %q, want safe actionable message", state.State.Message)
	}
	current := controller.GetCurrentLaunchState()
	if !current.Found || current.State == nil || current.State.OperationID != "operation-2" {
		t.Fatalf("GetCurrentLaunchState() = %+v, want current operation", current)
	}
}

func TestLaunchStateNotifierEmitsPublicDTO(t *testing.T) {
	t.Parallel()

	launchRuntime := &stubLaunchRuntime{ctx: context.Background()}
	notifier := NewLaunchStateNotifier(launchRuntime)
	notifier.Notify(operation.Snapshot{
		ID: operation.ID("operation-1"), State: operation.StateFailed, ErrorCode: operation.ErrorCodeUnexpected,
	})

	if launchRuntime.event != LaunchStateChangedEvent {
		t.Fatalf("event = %q, want %q", launchRuntime.event, LaunchStateChangedEvent)
	}
	dto, ok := launchRuntime.payload.(LaunchStateDTO)
	if !ok {
		t.Fatalf("payload type = %T, want LaunchStateDTO", launchRuntime.payload)
	}
	if dto.OperationID != "operation-1" || dto.State != "failed" || dto.ErrorCode != "unexpected_failure" {
		t.Fatalf("payload = %+v, want sanitized failed state", dto)
	}
}

func newTestLaunchOperationController(operations LaunchOperationService) *LaunchOperationController {
	return NewLaunchOperationController(
		newValidationController(),
		operations,
		&stubLaunchRuntime{ctx: context.Background()},
	)
}

func newValidationController() *LaunchController {
	return NewLaunchController(launchcondition.NewService(stubWorkAreaProvider{workArea: validWorkArea()}))
}

func validLaunchConditionDTO() LaunchConditionInputDTO {
	return LaunchConditionInputDTO{Width: "1000", Height: "700", X: "100", Y: "100"}
}

type stubLaunchOperationService struct {
	startSnapshot   operation.Snapshot
	startErr        error
	startCalls      int
	input           launchcondition.Input
	cancelStatus    operation.CancelStatus
	cancelSnapshot  operation.Snapshot
	getSnapshot     operation.Snapshot
	getFound        bool
	currentSnapshot operation.Snapshot
	currentFound    bool
}

func (s *stubLaunchOperationService) Start(_ context.Context, input launchcondition.Input) (operation.Snapshot, error) {
	s.startCalls++
	s.input = input
	return s.startSnapshot, s.startErr
}

func (s *stubLaunchOperationService) Cancel(operation.ID) (operation.CancelStatus, operation.Snapshot) {
	return s.cancelStatus, s.cancelSnapshot
}

func (s *stubLaunchOperationService) Get(operation.ID) (operation.Snapshot, bool) {
	return s.getSnapshot, s.getFound
}

func (s *stubLaunchOperationService) Current() (operation.Snapshot, bool) {
	return s.currentSnapshot, s.currentFound
}

type stubLaunchRuntime struct {
	ctx     context.Context
	event   string
	payload any
}

func (r *stubLaunchRuntime) Context() (context.Context, bool) {
	return r.ctx, r.ctx != nil
}

func (r *stubLaunchRuntime) Emit(event string, payload any) {
	r.event = event
	r.payload = payload
}

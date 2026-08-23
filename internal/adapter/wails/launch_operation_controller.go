package wailsadapter

import (
	"context"
	"errors"
	"sync"

	operation "github.com/asam-masa/browser-launcher/internal/application/chromelaunchoperation"
	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const LaunchStateChangedEvent = "launcher:state-changed"

var ErrApplicationNotReady = errors.New("アプリケーションの準備が完了していません。もう一度お試しください。")

type LaunchOperationService interface {
	Start(context.Context, launchcondition.Input) (operation.Snapshot, error)
	Cancel(operation.ID) (operation.CancelStatus, operation.Snapshot)
	Get(operation.ID) (operation.Snapshot, bool)
	Current() (operation.Snapshot, bool)
}

type LaunchRuntime interface {
	Context() (context.Context, bool)
	Emit(string, any)
}

type WailsRuntime struct {
	mu  sync.RWMutex
	ctx context.Context
}

func (r *WailsRuntime) Startup(ctx context.Context) {
	r.mu.Lock()
	r.ctx = ctx
	r.mu.Unlock()
}

func (r *WailsRuntime) Context() (context.Context, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ctx, r.ctx != nil
}

func (r *WailsRuntime) Emit(event string, payload any) {
	ctx, ok := r.Context()
	if ok {
		runtime.EventsEmit(ctx, event, payload)
	}
}

type LaunchStateNotifier struct {
	runtime LaunchRuntime
}

func NewLaunchStateNotifier(launchRuntime LaunchRuntime) LaunchStateNotifier {
	return LaunchStateNotifier{runtime: launchRuntime}
}

func (n LaunchStateNotifier) Notify(snapshot operation.Snapshot) {
	if n.runtime != nil {
		n.runtime.Emit(LaunchStateChangedEvent, toLaunchStateDTO(snapshot))
	}
}

type StartLaunchResultDTO struct {
	Started     bool                `json:"started"`
	OperationID string              `json:"operationId"`
	State       string              `json:"state"`
	Validation  ValidationResultDTO `json:"validation"`
}

type LaunchStateDTO struct {
	OperationID string `json:"operationId"`
	State       string `json:"state"`
	ErrorCode   string `json:"errorCode,omitempty"`
	Message     string `json:"message"`
}

type GetLaunchStateResultDTO struct {
	Found bool            `json:"found"`
	State *LaunchStateDTO `json:"state,omitempty"`
}

type CancelLaunchResultDTO struct {
	Status string          `json:"status"`
	State  *LaunchStateDTO `json:"state,omitempty"`
}

type LaunchOperationController struct {
	validation *LaunchController
	operations LaunchOperationService
	runtime    LaunchRuntime
}

func NewLaunchOperationController(
	validation *LaunchController,
	operations LaunchOperationService,
	launchRuntime LaunchRuntime,
) *LaunchOperationController {
	return &LaunchOperationController{
		validation: validation,
		operations: operations,
		runtime:    launchRuntime,
	}
}

func (c *LaunchOperationController) StartLaunch(input LaunchConditionInputDTO) (StartLaunchResultDTO, error) {
	validation := c.validation.ValidateLaunchCondition(input)
	if !validation.Valid {
		return StartLaunchResultDTO{Validation: validation}, nil
	}
	ctx, ok := c.runtime.Context()
	if !ok {
		return StartLaunchResultDTO{}, ErrApplicationNotReady
	}

	snapshot, err := c.operations.Start(ctx, toLaunchConditionInput(input))
	if err != nil {
		return StartLaunchResultDTO{}, safeStartError(err)
	}
	return StartLaunchResultDTO{
		Started:     true,
		OperationID: string(snapshot.ID),
		State:       string(snapshot.State),
		Validation:  validation,
	}, nil
}

func (c *LaunchOperationController) CancelLaunch(operationID string) CancelLaunchResultDTO {
	status, snapshot := c.operations.Cancel(operation.ID(operationID))
	result := CancelLaunchResultDTO{Status: string(status)}
	if snapshot.ID != "" {
		state := toLaunchStateDTO(snapshot)
		result.State = &state
	}
	return result
}

func (c *LaunchOperationController) GetLaunchState(operationID string) GetLaunchStateResultDTO {
	snapshot, found := c.operations.Get(operation.ID(operationID))
	if !found {
		return GetLaunchStateResultDTO{}
	}
	state := toLaunchStateDTO(snapshot)
	return GetLaunchStateResultDTO{Found: true, State: &state}
}

func (c *LaunchOperationController) GetCurrentLaunchState() GetLaunchStateResultDTO {
	snapshot, found := c.operations.Current()
	if !found {
		return GetLaunchStateResultDTO{}
	}
	state := toLaunchStateDTO(snapshot)
	return GetLaunchStateResultDTO{Found: true, State: &state}
}

func toLaunchConditionInput(input LaunchConditionInputDTO) launchcondition.Input {
	return launchcondition.Input{Width: input.Width, Height: input.Height, X: input.X, Y: input.Y}
}

func safeStartError(err error) error {
	if errors.Is(err, operation.ErrOperationInProgress) {
		return errors.New("Chromeの起動処理を実行中です。完了または取消後にもう一度お試しください。")
	}
	return errors.New("Chromeの起動処理を開始できませんでした。もう一度お試しください。")
}

func toLaunchStateDTO(snapshot operation.Snapshot) LaunchStateDTO {
	return LaunchStateDTO{
		OperationID: string(snapshot.ID),
		State:       string(snapshot.State),
		ErrorCode:   string(snapshot.ErrorCode),
		Message:     launchStateMessage(snapshot),
	}
}

func launchStateMessage(snapshot operation.Snapshot) string {
	switch snapshot.State {
	case operation.StateStarting:
		return "Chromeの起動処理を開始しています。"
	case operation.StateRunning:
		return "Chromeの起動とウィンドウ配置を実行しています。"
	case operation.StateCancelling:
		return "Chromeの起動処理を取り消しています。"
	case operation.StateCompleted:
		return "Chromeウィンドウを起動して配置しました。"
	case operation.StateCancelled:
		return "Chromeの起動処理を取り消しました。入力内容を確認して再実行できます。"
	case operation.StateTimedOut:
		return "60秒以内にChromeウィンドウを確認できませんでした。プロファイルを選択してから再実行してください。"
	case operation.StateFailed:
		return launchFailureMessage(snapshot.ErrorCode)
	default:
		return "Chromeの起動状態を確認できませんでした。もう一度お試しください。"
	}
}

func launchFailureMessage(code operation.ErrorCode) string {
	switch code {
	case operation.ErrorCodeInvalidRequest:
		return "入力内容を確認して、もう一度お試しください。"
	case operation.ErrorCodeChromeNotFound:
		return "Chromeが見つかりませんでした。Chromeのインストール状況を確認してください。"
	case operation.ErrorCodeUnsupported:
		return "この環境ではChromeを起動できません。Windows 11で実行してください。"
	case operation.ErrorCodeAmbiguousWindows:
		return "配置するChromeウィンドウを特定できませんでした。不要な選択画面を閉じて再実行してください。"
	case operation.ErrorCodeCandidateLost:
		return "配置するChromeウィンドウを確認できなくなりました。Chromeの状態を確認して再実行してください。"
	case operation.ErrorCodeWorkArea,
		operation.ErrorCodeLaunchFailed,
		operation.ErrorCodeObservationFailed,
		operation.ErrorCodePlacementFailed:
		return "Chromeウィンドウを起動または配置できませんでした。画面とChromeの状態を確認して再実行してください。"
	default:
		return "予期しない問題が発生しました。アプリケーションを再起動して、もう一度お試しください。"
	}
}

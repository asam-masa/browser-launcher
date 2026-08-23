package wailsadapter

import (
	"errors"
	"testing"

	application "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
)

func TestLaunchControllerValidateLaunchCondition(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		controller := newTestLaunchController(stubWorkAreaProvider{workArea: validWorkArea()})
		got := controller.ValidateLaunchCondition(LaunchConditionInputDTO{
			Width: "1280", Height: "720", X: "0", Y: "100",
		})
		if !got.Valid {
			t.Fatalf("Valid = false, errors = %+v, general error = %q", got.Errors, got.GeneralError)
		}
		if got.Errors == nil || len(got.Errors) != 0 {
			t.Fatalf("Errors = %#v, want empty non-nil slice", got.Errors)
		}
		if got.GeneralError != "" {
			t.Fatalf("GeneralError = %q, want empty", got.GeneralError)
		}
	})

	t.Run("invalid fields", func(t *testing.T) {
		t.Parallel()

		controller := newTestLaunchController(stubWorkAreaProvider{workArea: validWorkArea()})
		got := controller.ValidateLaunchCondition(LaunchConditionInputDTO{
			Width: "", Height: "0", X: "-1", Y: "100",
		})
		if got.Valid {
			t.Fatal("Valid = true, want false")
		}
		want := []ValidationErrorDTO{
			{Field: "width", Message: "値を入力してください。"},
			{Field: "height", Message: "1以上の整数を入力してください。"},
			{Field: "x", Message: "0以上の整数を入力してください。"},
		}
		assertValidationErrors(t, got.Errors, want)
		if got.GeneralError != "" {
			t.Fatalf("GeneralError = %q, want empty", got.GeneralError)
		}
	})

	t.Run("outside work area", func(t *testing.T) {
		t.Parallel()

		controller := newTestLaunchController(stubWorkAreaProvider{workArea: validWorkArea()})
		got := controller.ValidateLaunchCondition(LaunchConditionInputDTO{
			Width: "2560", Height: "1392", X: "1", Y: "49",
		})
		want := []ValidationErrorDTO{
			{Field: "width", Message: "幅またはX座標を小さくしてください。"},
			{Field: "height", Message: "高さまたはY座標を小さくしてください。"},
		}
		assertValidationErrors(t, got.Errors, want)
	})

	t.Run("provider failure", func(t *testing.T) {
		t.Parallel()

		controller := newTestLaunchController(stubWorkAreaProvider{err: errors.New("failed")})
		got := controller.ValidateLaunchCondition(LaunchConditionInputDTO{
			Width: "1280", Height: "720", X: "0", Y: "100",
		})
		if got.Valid {
			t.Fatal("Valid = true, want false")
		}
		if got.Errors == nil || len(got.Errors) != 0 {
			t.Fatalf("Errors = %#v, want empty non-nil slice", got.Errors)
		}
		want := "画面の使用可能領域を取得できませんでした。もう一度お試しください。"
		if got.GeneralError != want {
			t.Fatalf("GeneralError = %q, want %q", got.GeneralError, want)
		}
	})
}

func newTestLaunchController(provider application.PrimaryWorkAreaProvider) *LaunchController {
	return NewLaunchController(application.NewService(provider))
}

func validWorkArea() application.PrimaryWorkArea {
	return application.PrimaryWorkArea{
		WorkLeft: 0, WorkTop: 48, WorkWidth: 2560, WorkHeight: 1392, DPI: 96,
	}
}

func assertValidationErrors(t *testing.T, got []ValidationErrorDTO, want []ValidationErrorDTO) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("Errors = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Errors[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

type stubWorkAreaProvider struct {
	workArea application.PrimaryWorkArea
	err      error
}

func (p stubWorkAreaProvider) GetPrimaryWorkArea() (application.PrimaryWorkArea, error) {
	return p.workArea, p.err
}

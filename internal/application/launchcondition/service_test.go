package launchcondition

import (
	"errors"
	"math"
	"strconv"
	"testing"

	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
)

var defaultWorkArea = PrimaryWorkArea{
	MonitorLeft: 0,
	MonitorTop:  0,
	WorkLeft:    0,
	WorkTop:     48,
	WorkWidth:   2560,
	WorkHeight:  1392,
	DPI:         96,
}

func TestServiceValidateBasicInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      Input
		wantErrors []FieldError
	}{
		{
			name:  "valid",
			input: Input{Width: "1280", Height: "720", X: "0", Y: "100"},
		},
		{
			name:  "required",
			input: Input{},
			wantErrors: []FieldError{
				{Field: FieldWidth, Message: "値を入力してください。"},
				{Field: FieldHeight, Message: "値を入力してください。"},
				{Field: FieldX, Message: "値を入力してください。"},
				{Field: FieldY, Message: "値を入力してください。"},
			},
		},
		{
			name:  "invalid integer formats",
			input: Input{Width: "wide", Height: "720.5", X: "0", Y: "100"},
			wantErrors: []FieldError{
				{Field: FieldWidth, Message: "整数を入力してください。"},
				{Field: FieldHeight, Message: "整数を入力してください。"},
			},
		},
		{
			name:  "out of range values",
			input: Input{Width: "0", Height: "-1", X: "-10", Y: "0"},
			wantErrors: []FieldError{
				{Field: FieldWidth, Message: "1以上の整数を入力してください。"},
				{Field: FieldHeight, Message: "1以上の整数を入力してください。"},
				{Field: FieldX, Message: "0以上の整数を入力してください。"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := &stubWorkAreaProvider{workArea: defaultWorkArea}
			service := NewService(provider)
			condition, gotErrors, err := service.Validate(tt.input)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			assertFieldErrors(t, gotErrors, tt.wantErrors)
			if len(tt.wantErrors) == 0 {
				if condition.Width.Value() != 1280 || condition.Height.Value() != 720 ||
					condition.X.Value() != 0 || condition.Y.Value() != 100 {
					t.Fatalf("Validate() condition = %+v", condition)
				}
				if provider.calls != 1 {
					t.Fatalf("GetPrimaryWorkArea() calls = %d, want 1", provider.calls)
				}
			} else if provider.calls != 0 {
				t.Fatalf("GetPrimaryWorkArea() calls = %d, want 0", provider.calls)
			}
		})
	}
}

func TestServiceValidateWorkArea(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      Input
		workArea   PrimaryWorkArea
		wantErrors []FieldError
	}{
		{
			name:     "matches work area boundary",
			input:    Input{Width: "2560", Height: "1392", X: "0", Y: "48"},
			workArea: defaultWorkArea,
		},
		{
			name:     "dpi 120",
			input:    Input{Width: "1920", Height: "1040", X: "0", Y: "40"},
			workArea: PrimaryWorkArea{WorkLeft: 0, WorkTop: 50, WorkWidth: 2400, WorkHeight: 1300, DPI: 120},
		},
		{
			name:     "dpi 144",
			input:    Input{Width: "1600", Height: "900", X: "0", Y: "32"},
			workArea: PrimaryWorkArea{WorkLeft: 0, WorkTop: 48, WorkWidth: 2400, WorkHeight: 1350, DPI: 144},
		},
		{
			name:     "right and bottom outside",
			input:    Input{Width: "2560", Height: "1392", X: "1", Y: "49"},
			workArea: defaultWorkArea,
			wantErrors: []FieldError{
				{Field: FieldWidth, Message: "幅またはX座標を小さくしてください。"},
				{Field: FieldHeight, Message: "高さまたはY座標を小さくしてください。"},
			},
		},
		{
			name:     "left and top outside",
			input:    Input{Width: "100", Height: "100", X: "0", Y: "0"},
			workArea: PrimaryWorkArea{MonitorLeft: -100, MonitorTop: -100, WorkLeft: -50, WorkTop: -50, WorkWidth: 1000, WorkHeight: 1000, DPI: 96},
			wantErrors: []FieldError{
				{Field: FieldX, Message: "使用可能領域内のX座標を入力してください。"},
				{Field: FieldY, Message: "使用可能領域内のY座標を入力してください。"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&stubWorkAreaProvider{workArea: tt.workArea})
			_, gotErrors, err := service.Validate(tt.input)
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			assertFieldErrors(t, gotErrors, tt.wantErrors)
		})
	}
}

func TestServiceValidateProviderFailure(t *testing.T) {
	t.Parallel()

	providerError := errors.New("provider failed")
	service := NewService(&stubWorkAreaProvider{err: providerError})
	_, fieldErrors, err := service.Validate(Input{Width: "1280", Height: "720", X: "0", Y: "100"})

	if !errors.Is(err, providerError) {
		t.Fatalf("Validate() error = %v, want provider error", err)
	}
	if len(fieldErrors) != 0 {
		t.Fatalf("Validate() field errors = %+v, want none", fieldErrors)
	}
}

func TestServicePrepareReturnsConditionAndPhysicalBoundsFromSameWorkArea(t *testing.T) {
	t.Parallel()

	provider := &stubWorkAreaProvider{workArea: PrimaryWorkArea{
		MonitorLeft: -3840,
		MonitorTop:  -2160,
		WorkLeft:    -3840,
		WorkTop:     -2160,
		WorkWidth:   3840,
		WorkHeight:  2080,
		DPI:         144,
	}}
	prepared, fieldErrors, err := NewService(provider).Prepare(Input{
		Width: "1000", Height: "700", X: "100", Y: "100",
	})

	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if len(fieldErrors) != 0 {
		t.Fatalf("Prepare() field errors = %+v, want none", fieldErrors)
	}
	if provider.calls != 1 {
		t.Fatalf("GetPrimaryWorkArea() calls = %d, want 1", provider.calls)
	}
	if prepared.Condition.Width.Value() != 1000 || prepared.Condition.Height.Value() != 700 ||
		prepared.Condition.X.Value() != 100 || prepared.Condition.Y.Value() != 100 {
		t.Fatalf("Prepare() condition = %+v", prepared.Condition)
	}
	wantBounds := PhysicalBounds{X: -3690, Y: -2010, Width: 1500, Height: 1050}
	if prepared.PhysicalBounds != wantBounds {
		t.Fatalf("Prepare() physical bounds = %+v, want %+v", prepared.PhysicalBounds, wantBounds)
	}
}

func TestServiceValidateConversionOverflowReturnsFieldErrors(t *testing.T) {
	t.Parallel()

	maximum := strconv.Itoa(math.MaxInt)
	service := NewService(&stubWorkAreaProvider{workArea: defaultWorkArea})
	_, gotErrors, err := service.Validate(Input{
		Width: maximum, Height: maximum, X: maximum, Y: maximum,
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	want := []FieldError{
		{Field: FieldX, Message: "使用可能領域内のX座標を入力してください。"},
		{Field: FieldY, Message: "使用可能領域内のY座標を入力してください。"},
		{Field: FieldWidth, Message: "幅またはX座標を小さくしてください。"},
		{Field: FieldHeight, Message: "高さまたはY座標を小さくしてください。"},
	}
	assertFieldErrors(t, gotErrors, want)
}

func TestScaleLogicalPixel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		dpi     int
		want    int
		wantErr error
	}{
		{name: "dpi 96", value: 100, dpi: 96, want: 100},
		{name: "dpi 120 rounds down", value: 1, dpi: 120, want: 1},
		{name: "dpi 120 rounds up", value: 2, dpi: 120, want: 3},
		{name: "dpi 144", value: 100, dpi: 144, want: 150},
		{name: "invalid dpi", value: 100, dpi: 0, wantErr: ErrInvalidWorkArea},
		{name: "overflow", value: math.MaxInt, dpi: 144, wantErr: ErrScaleOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := scaleLogicalPixel(tt.value, tt.dpi)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("scaleLogicalPixel() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("scaleLogicalPixel() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolvePhysicalBoundsUsesMonitorOriginAndDPI(t *testing.T) {
	t.Parallel()

	condition := domain.New(
		mustTestDimension(t, 1000),
		mustTestDimension(t, 700),
		mustTestCoordinate(t, 100),
		mustTestCoordinate(t, 100),
	)
	got, fieldErrors, err := ResolvePhysicalBounds(condition, PrimaryWorkArea{
		MonitorLeft: -3840,
		MonitorTop:  -2160,
		WorkLeft:    -3840,
		WorkTop:     -2160,
		WorkWidth:   3840,
		WorkHeight:  2080,
		DPI:         144,
	})
	if err != nil {
		t.Fatalf("ResolvePhysicalBounds() error = %v", err)
	}
	if len(fieldErrors) != 0 {
		t.Fatalf("ResolvePhysicalBounds() field errors = %+v, want none", fieldErrors)
	}
	want := PhysicalBounds{X: -3690, Y: -2010, Width: 1500, Height: 1050}
	if got != want {
		t.Fatalf("ResolvePhysicalBounds() = %+v, want %+v", got, want)
	}
}

func mustTestDimension(t *testing.T, value int) domain.Dimension {
	t.Helper()
	dimension, err := domain.NewDimension(value)
	if err != nil {
		t.Fatalf("NewDimension(%d) error = %v", value, err)
	}
	return dimension
}

func mustTestCoordinate(t *testing.T, value int) domain.Coordinate {
	t.Helper()
	coordinate, err := domain.NewCoordinate(value)
	if err != nil {
		t.Fatalf("NewCoordinate(%d) error = %v", value, err)
	}
	return coordinate
}

func assertFieldErrors(t *testing.T, got []FieldError, want []FieldError) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("Validate() errors = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Validate() error[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

type stubWorkAreaProvider struct {
	workArea PrimaryWorkArea
	err      error
	calls    int
}

func (p *stubWorkAreaProvider) GetPrimaryWorkArea() (PrimaryWorkArea, error) {
	p.calls++
	return p.workArea, p.err
}

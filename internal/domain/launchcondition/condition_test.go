package launchcondition

import (
	"errors"
	"testing"
)

func TestNewDimension(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		wantErr error
	}{
		{name: "positive", value: 1},
		{name: "zero", value: 0, wantErr: ErrDimensionNotPositive},
		{name: "negative", value: -1, wantErr: ErrDimensionNotPositive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewDimension(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewDimension(%d) error = %v, want %v", tt.value, err, tt.wantErr)
			}
			if err == nil && got.Value() != tt.value {
				t.Fatalf("NewDimension(%d).Value() = %d", tt.value, got.Value())
			}
		})
	}
}

func TestNewCoordinate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   int
		wantErr error
	}{
		{name: "positive", value: 1},
		{name: "zero", value: 0},
		{name: "negative", value: -1, wantErr: ErrCoordinateNegative},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewCoordinate(tt.value)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewCoordinate(%d) error = %v, want %v", tt.value, err, tt.wantErr)
			}
			if err == nil && got.Value() != tt.value {
				t.Fatalf("NewCoordinate(%d).Value() = %d", tt.value, got.Value())
			}
		})
	}
}

func TestNewCondition(t *testing.T) {
	t.Parallel()

	width, _ := NewDimension(1280)
	height, _ := NewDimension(720)
	x, _ := NewCoordinate(100)
	y, _ := NewCoordinate(200)

	condition := New(width, height, x, y)
	if condition.Width.Value() != 1280 || condition.Height.Value() != 720 ||
		condition.X.Value() != 100 || condition.Y.Value() != 200 {
		t.Fatalf("New() = %+v, want width=1280 height=720 x=100 y=200", condition)
	}
}

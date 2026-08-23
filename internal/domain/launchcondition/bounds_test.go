package launchcondition

import (
	"errors"
	"math"
	"testing"
)

func TestNewBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    int
		top     int
		width   int
		height  int
		wantErr error
	}{
		{name: "valid", left: 0, top: 48, width: 2560, height: 1392},
		{name: "negative origin", left: -1920, top: 0, width: 1920, height: 1080},
		{name: "zero width", width: 0, height: 1, wantErr: ErrBoundsDimensionNotPositive},
		{name: "zero height", width: 1, height: 0, wantErr: ErrBoundsDimensionNotPositive},
		{name: "right overflow", left: math.MaxInt, width: 1, height: 1, wantErr: ErrBoundsOverflow},
		{name: "bottom overflow", top: math.MaxInt, width: 1, height: 1, wantErr: ErrBoundsOverflow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewBounds(tt.left, tt.top, tt.width, tt.height)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewBounds() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && (got.Left() != tt.left || got.Top() != tt.top ||
				got.Right() != tt.left+tt.width || got.Bottom() != tt.top+tt.height) {
				t.Fatalf("NewBounds() = %+v", got)
			}
		})
	}
}

func TestBoundsOutsideEdges(t *testing.T) {
	t.Parallel()

	container, _ := NewBounds(0, 48, 2560, 1392)
	tests := []struct {
		name      string
		candidate Bounds
		want      []Edge
	}{
		{name: "inside", candidate: mustBounds(t, 100, 100, 1000, 700)},
		{name: "same bounds", candidate: mustBounds(t, 0, 48, 2560, 1392)},
		{name: "left", candidate: mustBounds(t, -1, 48, 100, 100), want: []Edge{EdgeLeft}},
		{name: "top", candidate: mustBounds(t, 0, 47, 100, 100), want: []Edge{EdgeTop}},
		{name: "right", candidate: mustBounds(t, 2461, 48, 100, 100), want: []Edge{EdgeRight}},
		{name: "bottom", candidate: mustBounds(t, 0, 1341, 100, 100), want: []Edge{EdgeBottom}},
		{
			name:      "all edges",
			candidate: mustBounds(t, -1, 47, 2562, 1394),
			want:      []Edge{EdgeLeft, EdgeTop, EdgeRight, EdgeBottom},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := container.OutsideEdges(tt.candidate)
			if len(got) != len(tt.want) {
				t.Fatalf("OutsideEdges() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("OutsideEdges()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func mustBounds(t *testing.T, left int, top int, width int, height int) Bounds {
	t.Helper()

	bounds, err := NewBounds(left, top, width, height)
	if err != nil {
		t.Fatalf("NewBounds() error = %v", err)
	}
	return bounds
}

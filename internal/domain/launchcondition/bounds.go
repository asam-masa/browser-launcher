package launchcondition

import (
	"errors"
	"math"
)

var (
	ErrBoundsDimensionNotPositive = errors.New("bounds dimensions must be greater than zero")
	ErrBoundsOverflow             = errors.New("bounds exceed the supported coordinate range")
)

type Edge string

const (
	EdgeLeft   Edge = "left"
	EdgeTop    Edge = "top"
	EdgeRight  Edge = "right"
	EdgeBottom Edge = "bottom"
)

type Bounds struct {
	left   int
	top    int
	right  int
	bottom int
}

func NewBounds(left int, top int, width int, height int) (Bounds, error) {
	if width < 1 || height < 1 {
		return Bounds{}, ErrBoundsDimensionNotPositive
	}
	if left > math.MaxInt-width || top > math.MaxInt-height {
		return Bounds{}, ErrBoundsOverflow
	}

	return Bounds{
		left:   left,
		top:    top,
		right:  left + width,
		bottom: top + height,
	}, nil
}

func (b Bounds) Left() int {
	return b.left
}

func (b Bounds) Top() int {
	return b.top
}

func (b Bounds) Right() int {
	return b.right
}

func (b Bounds) Bottom() int {
	return b.bottom
}

func (b Bounds) OutsideEdges(candidate Bounds) []Edge {
	edges := make([]Edge, 0, 4)
	if candidate.left < b.left {
		edges = append(edges, EdgeLeft)
	}
	if candidate.top < b.top {
		edges = append(edges, EdgeTop)
	}
	if candidate.right > b.right {
		edges = append(edges, EdgeRight)
	}
	if candidate.bottom > b.bottom {
		edges = append(edges, EdgeBottom)
	}

	return edges
}

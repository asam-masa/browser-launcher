package launchcondition

import "errors"

var (
	ErrDimensionNotPositive = errors.New("dimension must be greater than zero")
	ErrCoordinateNegative   = errors.New("coordinate must not be negative")
)

type Dimension struct {
	value int
}

func NewDimension(value int) (Dimension, error) {
	if value < 1 {
		return Dimension{}, ErrDimensionNotPositive
	}

	return Dimension{value: value}, nil
}

func (d Dimension) Value() int {
	return d.value
}

type Coordinate struct {
	value int
}

func NewCoordinate(value int) (Coordinate, error) {
	if value < 0 {
		return Coordinate{}, ErrCoordinateNegative
	}

	return Coordinate{value: value}, nil
}

func (c Coordinate) Value() int {
	return c.value
}

type Condition struct {
	Width  Dimension
	Height Dimension
	X      Coordinate
	Y      Coordinate
}

func New(width Dimension, height Dimension, x Coordinate, y Coordinate) Condition {
	return Condition{
		Width:  width,
		Height: height,
		X:      x,
		Y:      y,
	}
}

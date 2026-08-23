package launchcondition

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	domain "github.com/asam-masa/browser-launcher/internal/domain/launchcondition"
)

var (
	ErrInvalidWorkArea = errors.New("primary work area is invalid")
	ErrScaleOverflow   = errors.New("launch condition exceeds the supported coordinate range")
)

type Field string

const (
	FieldWidth  Field = "width"
	FieldHeight Field = "height"
	FieldX      Field = "x"
	FieldY      Field = "y"
)

type Input struct {
	Width  string
	Height string
	X      string
	Y      string
}

type FieldError struct {
	Field   Field
	Message string
}

type PrimaryWorkArea struct {
	MonitorLeft int
	MonitorTop  int
	WorkLeft    int
	WorkTop     int
	WorkWidth   int
	WorkHeight  int
	DPI         int
}

type PhysicalBounds struct {
	X      int
	Y      int
	Width  int
	Height int
}

type PreparedCondition struct {
	Condition      domain.Condition
	PhysicalBounds PhysicalBounds
}

type PrimaryWorkAreaProvider interface {
	GetPrimaryWorkArea() (PrimaryWorkArea, error)
}

type Service struct {
	workAreaProvider PrimaryWorkAreaProvider
}

func NewService(workAreaProvider PrimaryWorkAreaProvider) Service {
	return Service{workAreaProvider: workAreaProvider}
}

func (s Service) Validate(input Input) (domain.Condition, []FieldError, error) {
	prepared, fieldErrors, err := s.Prepare(input)
	return prepared.Condition, fieldErrors, err
}

func (s Service) Prepare(input Input) (PreparedCondition, []FieldError, error) {
	var fieldErrors []FieldError

	width, widthErrors := parseDimension(FieldWidth, input.Width)
	fieldErrors = append(fieldErrors, widthErrors...)
	height, heightErrors := parseDimension(FieldHeight, input.Height)
	fieldErrors = append(fieldErrors, heightErrors...)
	x, xErrors := parseCoordinate(FieldX, input.X)
	fieldErrors = append(fieldErrors, xErrors...)
	y, yErrors := parseCoordinate(FieldY, input.Y)
	fieldErrors = append(fieldErrors, yErrors...)

	if len(fieldErrors) > 0 {
		return PreparedCondition{}, fieldErrors, nil
	}

	condition := domain.New(width, height, x, y)
	workArea, err := s.workAreaProvider.GetPrimaryWorkArea()
	if err != nil {
		return PreparedCondition{}, nil, fmt.Errorf("get primary work area: %w", err)
	}

	physicalBounds, outsideErrors, err := ResolvePhysicalBounds(condition, workArea)
	if err != nil {
		return PreparedCondition{}, nil, err
	}
	if len(outsideErrors) > 0 {
		return PreparedCondition{}, outsideErrors, nil
	}

	return PreparedCondition{
		Condition:      condition,
		PhysicalBounds: physicalBounds,
	}, nil, nil
}

func ResolvePhysicalBounds(condition domain.Condition, workArea PrimaryWorkArea) (PhysicalBounds, []FieldError, error) {
	if workArea.DPI < 96 {
		return PhysicalBounds{}, nil, fmt.Errorf("%w: dpi must be at least 96", ErrInvalidWorkArea)
	}

	container, err := domain.NewBounds(
		workArea.WorkLeft,
		workArea.WorkTop,
		workArea.WorkWidth,
		workArea.WorkHeight,
	)
	if err != nil {
		return PhysicalBounds{}, nil, fmt.Errorf("%w: %v", ErrInvalidWorkArea, err)
	}

	physicalX, xErr := scaleLogicalPixel(condition.X.Value(), workArea.DPI)
	physicalY, yErr := scaleLogicalPixel(condition.Y.Value(), workArea.DPI)
	physicalWidth, widthErr := scaleLogicalPixel(condition.Width.Value(), workArea.DPI)
	physicalHeight, heightErr := scaleLogicalPixel(condition.Height.Value(), workArea.DPI)

	fieldErrors := conversionFieldErrors(xErr, yErr, widthErr, heightErr)
	if len(fieldErrors) > 0 {
		return PhysicalBounds{}, fieldErrors, nil
	}

	physicalX, err = addCoordinate(workArea.MonitorLeft, physicalX)
	if err != nil {
		fieldErrors = append(fieldErrors, coordinateRangeError(FieldX))
	}
	physicalY, err = addCoordinate(workArea.MonitorTop, physicalY)
	if err != nil {
		fieldErrors = append(fieldErrors, coordinateRangeError(FieldY))
	}
	if len(fieldErrors) > 0 {
		return PhysicalBounds{}, fieldErrors, nil
	}
	if physicalX > math.MaxInt-physicalWidth {
		fieldErrors = append(fieldErrors, dimensionRangeError(FieldWidth))
	}
	if physicalY > math.MaxInt-physicalHeight {
		fieldErrors = append(fieldErrors, dimensionRangeError(FieldHeight))
	}
	if len(fieldErrors) > 0 {
		return PhysicalBounds{}, fieldErrors, nil
	}
	request, err := domain.NewBounds(physicalX, physicalY, physicalWidth, physicalHeight)
	if err != nil {
		return PhysicalBounds{}, nil, fmt.Errorf("%w: %v", ErrScaleOverflow, err)
	}

	for _, edge := range container.OutsideEdges(request) {
		switch edge {
		case domain.EdgeLeft:
			fieldErrors = append(fieldErrors, FieldError{
				Field:   FieldX,
				Message: "使用可能領域内のX座標を入力してください。",
			})
		case domain.EdgeTop:
			fieldErrors = append(fieldErrors, FieldError{
				Field:   FieldY,
				Message: "使用可能領域内のY座標を入力してください。",
			})
		case domain.EdgeRight:
			fieldErrors = append(fieldErrors, FieldError{
				Field:   FieldWidth,
				Message: "幅またはX座標を小さくしてください。",
			})
		case domain.EdgeBottom:
			fieldErrors = append(fieldErrors, FieldError{
				Field:   FieldHeight,
				Message: "高さまたはY座標を小さくしてください。",
			})
		}
	}

	return PhysicalBounds{
		X:      physicalX,
		Y:      physicalY,
		Width:  physicalWidth,
		Height: physicalHeight,
	}, fieldErrors, nil
}

func conversionFieldErrors(xErr error, yErr error, widthErr error, heightErr error) []FieldError {
	var fieldErrors []FieldError
	if xErr != nil {
		fieldErrors = append(fieldErrors, coordinateRangeError(FieldX))
	}
	if yErr != nil {
		fieldErrors = append(fieldErrors, coordinateRangeError(FieldY))
	}
	if widthErr != nil {
		fieldErrors = append(fieldErrors, dimensionRangeError(FieldWidth))
	}
	if heightErr != nil {
		fieldErrors = append(fieldErrors, dimensionRangeError(FieldHeight))
	}
	return fieldErrors
}

func coordinateRangeError(field Field) FieldError {
	axis := "X"
	if field == FieldY {
		axis = "Y"
	}
	return FieldError{
		Field:   field,
		Message: fmt.Sprintf("使用可能領域内の%s座標を入力してください。", axis),
	}
}

func dimensionRangeError(field Field) FieldError {
	message := "幅またはX座標を小さくしてください。"
	if field == FieldHeight {
		message = "高さまたはY座標を小さくしてください。"
	}
	return FieldError{Field: field, Message: message}
}

func scaleLogicalPixel(value int, dpi int) (int, error) {
	if dpi < 1 {
		return 0, ErrInvalidWorkArea
	}
	if value > (math.MaxInt-48)/dpi {
		return 0, ErrScaleOverflow
	}

	return (value*dpi + 48) / 96, nil
}

func addCoordinate(origin int, offset int) (int, error) {
	if origin > math.MaxInt-offset {
		return 0, ErrScaleOverflow
	}

	return origin + offset, nil
}

func parseDimension(field Field, input string) (domain.Dimension, []FieldError) {
	value, fieldError := parseInteger(field, input)
	if fieldError != nil {
		return domain.Dimension{}, []FieldError{*fieldError}
	}

	dimension, err := domain.NewDimension(value)
	if err != nil {
		return domain.Dimension{}, []FieldError{{
			Field:   field,
			Message: "1以上の整数を入力してください。",
		}}
	}

	return dimension, nil
}

func parseCoordinate(field Field, input string) (domain.Coordinate, []FieldError) {
	value, fieldError := parseInteger(field, input)
	if fieldError != nil {
		return domain.Coordinate{}, []FieldError{*fieldError}
	}

	coordinate, err := domain.NewCoordinate(value)
	if err != nil {
		return domain.Coordinate{}, []FieldError{{
			Field:   field,
			Message: "0以上の整数を入力してください。",
		}}
	}

	return coordinate, nil
}

func parseInteger(field Field, input string) (int, *FieldError) {
	if strings.TrimSpace(input) == "" {
		return 0, &FieldError{
			Field:   field,
			Message: "値を入力してください。",
		}
	}

	value, err := strconv.Atoi(input)
	if err != nil {
		return 0, &FieldError{
			Field:   field,
			Message: "整数を入力してください。",
		}
	}

	return value, nil
}

package wailsadapter

import application "github.com/asam-masa/browser-launcher/internal/application/launchcondition"

type LaunchConditionInputDTO struct {
	Width  string `json:"width"`
	Height string `json:"height"`
	X      string `json:"x"`
	Y      string `json:"y"`
}

type ValidationErrorDTO struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationResultDTO struct {
	Valid        bool                 `json:"valid"`
	Errors       []ValidationErrorDTO `json:"errors"`
	GeneralError string               `json:"generalError"`
}

type LaunchController struct {
	validationService application.Service
}

func NewLaunchController(validationService application.Service) *LaunchController {
	return &LaunchController{
		validationService: validationService,
	}
}

func (c *LaunchController) ValidateLaunchCondition(input LaunchConditionInputDTO) ValidationResultDTO {
	_, fieldErrors, err := c.validationService.Validate(application.Input{
		Width:  input.Width,
		Height: input.Height,
		X:      input.X,
		Y:      input.Y,
	})
	if err != nil {
		return ValidationResultDTO{
			Errors:       []ValidationErrorDTO{},
			GeneralError: "画面の使用可能領域を取得できませんでした。もう一度お試しください。",
		}
	}

	errors := make([]ValidationErrorDTO, len(fieldErrors))
	for i, fieldError := range fieldErrors {
		errors[i] = ValidationErrorDTO{
			Field:   string(fieldError.Field),
			Message: fieldError.Message,
		}
	}

	return ValidationResultDTO{
		Valid:        len(errors) == 0,
		Errors:       errors,
		GeneralError: "",
	}
}

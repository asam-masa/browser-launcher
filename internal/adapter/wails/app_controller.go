package wailsadapter

import "github.com/asam-masa/browser-launcher/internal/application/appinfo"

type ApplicationInfoDTO struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type AppController struct {
	appInfoService appinfo.Service
}

func NewAppController(appInfoService appinfo.Service) *AppController {
	return &AppController{
		appInfoService: appInfoService,
	}
}

func (c *AppController) GetApplicationInfo() ApplicationInfoDTO {
	info := c.appInfoService.Get()
	return ApplicationInfoDTO{
		Name:    info.Name,
		Version: info.Version,
	}
}

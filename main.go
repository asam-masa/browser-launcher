package main

import (
	"context"
	"embed"
	"time"

	wailsadapter "github.com/asam-masa/browser-launcher/internal/adapter/wails"
	"github.com/asam-masa/browser-launcher/internal/application/appinfo"
	chromelaunch "github.com/asam-masa/browser-launcher/internal/application/chromelaunch"
	launchoperation "github.com/asam-masa/browser-launcher/internal/application/chromelaunchoperation"
	windowplacement "github.com/asam-masa/browser-launcher/internal/application/chromewindowplacement"
	windowtracking "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
	launchcondition "github.com/asam-masa/browser-launcher/internal/application/launchcondition"
	chromedetection "github.com/asam-masa/browser-launcher/internal/infrastructure/chromedetection"
	chromelaunchinfra "github.com/asam-masa/browser-launcher/internal/infrastructure/chromelaunch"
	windowplacementinfra "github.com/asam-masa/browser-launcher/internal/infrastructure/chromewindowplacement"
	windowtrackinginfra "github.com/asam-masa/browser-launcher/internal/infrastructure/chromewindowtracking"
	"github.com/asam-masa/browser-launcher/internal/infrastructure/display"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

const (
	applicationName    = "Browser Launcher"
	applicationVersion = "dev"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appInfoService := appinfo.NewService(applicationName, applicationVersion)
	appController := wailsadapter.NewAppController(appInfoService)
	workAreaProvider := display.NewProvider()
	launchValidationService := launchcondition.NewService(workAreaProvider)
	launchController := wailsadapter.NewLaunchController(launchValidationService)
	chromeLaunchService := chromelaunch.NewService(
		chromedetection.NewProvider(),
		chromelaunchinfra.NewProvider(),
	)
	workflow := launchoperation.NewWorkflow(
		launchValidationService,
		chromeLaunchService,
		func(executablePath string) launchoperation.WindowServices {
			trackingProvider := windowtrackinginfra.NewProvider(executablePath)
			return launchoperation.WindowServices{
				Tracker: windowtracking.NewService(trackingProvider),
				Placer: windowplacement.NewService(
					windowplacementinfra.NewProvider(trackingProvider),
				),
			}
		},
	)
	launchRuntime := &wailsadapter.WailsRuntime{}
	launchOperationService := launchoperation.NewService(
		workflow,
		launchoperation.NewRegistry(),
		wailsadapter.NewLaunchStateNotifier(launchRuntime),
	)
	launchOperationController := wailsadapter.NewLaunchOperationController(
		launchController,
		launchOperationService,
		launchRuntime,
	)

	err := wails.Run(&options.App{
		Title:  applicationName,
		Width:  960,
		Height: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup: launchRuntime.Startup,
		OnShutdown: func(context.Context) {
			shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = launchOperationService.Shutdown(shutdownContext)
		},
		Bind: []interface{}{
			appController,
			launchController,
			launchOperationController,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

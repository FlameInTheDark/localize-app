package main

import (
	"embed"
	"log"

	"github.com/FlameInTheDark/localize-app/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is replaced with the semantic release tag in production builds.
// Development builds intentionally skip remote update checks.
var appVersion = "dev"

func main() {
	desktop, err := app.New(appVersion)
	if err != nil {
		log.Fatal(err)
	}

	err = wails.Run(&options.App{
		Title:              "Localize — AI Translation",
		Frameless:          true,
		Width:              1440,
		Height:             900,
		MinWidth:           1080,
		MinHeight:          700,
		DisableResize:      false,
		StartHidden:        false,
		AssetServer:        &assetserver.Options{Assets: assets},
		BackgroundColour:   &options.RGBA{R: 20, G: 20, B: 20, A: 255},
		OnStartup:          desktop.Startup,
		OnShutdown:         desktop.Shutdown,
		Bind:               []interface{}{desktop},
		SingleInstanceLock: &options.SingleInstanceLock{UniqueId: "d83929d1-ca89-4be5-90d7-a6dd3b496a56", OnSecondInstanceLaunch: desktop.SecondInstance},
		DragAndDrop:        &options.DragAndDrop{EnableFileDrop: true, DisableWebViewDrop: true},
		Windows:            &windows.Options{WebviewIsTransparent: false, WindowIsTranslucent: false, DisableFramelessWindowDecorations: false},
	})
	if err != nil {
		log.Fatal(err)
	}
}

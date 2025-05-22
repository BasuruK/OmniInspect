package main

import "OmniView/internal/app"

func main() {
	// Initialize application
	omniApp := app.New()
	println(omniApp.GetVersion())
	omniApp.StartServer()
}

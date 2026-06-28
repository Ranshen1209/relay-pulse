package main

import (
	"os"

	"monitor/internal/app"
	"monitor/internal/logger"
)

func main() {
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	if err := app.Run(configFile); err != nil {
		logger.Error("main", "服务启动失败", "error", err)
		os.Exit(1)
	}
}

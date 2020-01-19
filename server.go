package main

import (
	"github.com/quan-to/slog"
	"github.com/racerxdl/go-mcp23017"
)

func main() {
	LoadConfig()

	slog.SetDebug(config.Server.DebugMode)

	log.Debug("Debug mode enabled")
	log.Info("Default PortA: 0x%02X", config.Server.DefaultPortA)
	log.Info("Default PortB: 0x%02X", config.Server.DefaultPortB)
	log.Info("IO Config\n%s", config.IO)

	mcp23017.SetDefaultValues(config.Server.DefaultPortA, config.Server.DefaultPortB)

	q, err := MakeQueueManager(config.MQTT)
	if err != nil {
		log.Fatal(err)
	}

	io, err := MakeIOManager(config.IO, q)

	if err != nil {
		log.Fatal(err)
	}

	log.Info("Initialized %d devices", len(io.devs))

	select {}
}

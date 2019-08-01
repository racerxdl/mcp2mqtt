package main

func main() {
	//slog.SetDebug(true)
	LoadConfig()

	log.Info("IO Config\n%s", config.IO)

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

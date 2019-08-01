package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/mewkiz/pkg/osutil"
	"github.com/quan-to/slog"
	"os"
	"strings"
)

type MQTTConfig struct {
	MQTTServer   string
	MQTTUsername string
	MQTTPassword string
}

type IOMap struct {
	DeviceNumber int
	PinNumber    int
	TopicNumber  int
	IsOutput     bool
}

type IOConfig struct {
	BusNumber int
	IODevices []IODevice
}

func (ic IOConfig) String() string {
	v := fmt.Sprintf("I/O Bus #%d\n", ic.BusNumber)
	for i, d := range ic.IODevices {
		v += fmt.Sprintf("Device #%d\n    %s\n", i, strings.Replace(d.String(), "\n", "\n    ", -1))
		v += "------------------------------------------\n"
	}
	return v
}

type IODevice struct {
	Topic       string
	StatusTopic string
	IOMap       []IOMap
}

func (id IODevice) isPinExplicitMapped(pin int) bool {
	for _, v := range id.IOMap {
		if v.PinNumber == pin {
			return true
		}
	}

	return false
}

func (id IODevice) String() string {
	v := fmt.Sprintf("Topic: %s\n", id.Topic)
	v += fmt.Sprintf("Status Topic: %s\n", id.StatusTopic)
	v += "I/O Mapping: \n"

	for i, m := range id.IOMap {
		v += fmt.Sprintf("   Map #%d: \n", i)
		v += fmt.Sprintf("       Device Number: %d\n", m.DeviceNumber)
		v += fmt.Sprintf("       Pin Number: %d\n", m.PinNumber)
		v += fmt.Sprintf("       Topic Number: %d\n", m.TopicNumber)
		v += fmt.Sprintf("       Is Output: %v\n", m.IsOutput)
	}

	return v
}

type Config struct {
	MQTT MQTTConfig
	IO   IOConfig
}

const configFile = "mcp2mqtt.toml"

var config Config

var log = slog.Scope("MCP2MQTT")

func LoadConfig() {
	log.Info("Loading config %s", configFile)
	if !osutil.Exists(configFile) {
		log.Error("Config file %s does not exists.", configFile)
		os.Exit(1)
	}

	_, err := toml.DecodeFile(configFile, &config)
	if err != nil {
		log.Error("Error decoding file %s: %s", configFile, err)
		os.Exit(1)
	}
}
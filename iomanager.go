package main

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/quan-to/slog"
	"github.com/racerxdl/go-mcp23017"
	"strconv"
	"strings"
	"sync"
	"time"
)

const healthCheckPeriod = time.Second * 5
const poolingCheckPeriod = time.Second / 60

var iolog = slog.Scope("IO")

type IOManager struct {
	q          *QueueManager
	c          IOConfig
	devs       []*mcp23017.Device
	lastStatus []bool
	lastIO     []uint16
	running    bool
	l          sync.Mutex
}

func MakeIOManager(config IOConfig, q *QueueManager) (*IOManager, error) {
	io := &IOManager{
		q: q,
		c: config,
	}

	err := io.initializeDevices()

	if err != nil {
		return nil, err
	}

	io.running = true
	go io.healthCheckLoop()

	q.SetOnMessage(io.MessageHandle)

	return io, nil
}

func (io *IOManager) healthCheckLoop() {
	lastHealthCheck := time.Now()
	lastPoolCheck := time.Now()
	for io.running {
		io.l.Lock()
		// region Check Device Status
		if time.Since(lastHealthCheck) > healthCheckPeriod {
			for i, v := range io.devs {
				if v.IsPresent() != io.lastStatus[i] {
					if !v.IsPresent() {
						iolog.Error("Device %d at bus %d is offline", i, io.c.BusNumber)
					} else {
						iolog.Info("Device %d at bus %d is back online", i, io.c.BusNumber)
					}

					io.notifyStatus(i, v.IsPresent())
				}
			}
			lastHealthCheck = time.Now()
		}
		// endregion
		// region Pooling
		if time.Since(lastPoolCheck) > poolingCheckPeriod {
			io.pool()
			lastPoolCheck = time.Now()
		}
		// endregion
		io.l.Unlock()
		time.Sleep(time.Millisecond * 100)
	}
}

func (io *IOManager) pool() {
	for devNum := range io.c.IODevices {
		d := io.devs[devNum]
		u, err := d.ReadGPIOAB()
		if err != nil {
			// Ignore error here, health check will catch it.
			continue
		}

		if u == io.lastIO[devNum] {
			// No change
			continue
		}

		for b := uint(0); b < 16; b++ {
			lastStatus := (io.lastIO[devNum] & (1 << b)) >> b
			currentStatus := (u & (1 << b)) >> b
			if lastStatus != currentStatus {
				go io.notifyIOChange(devNum, int(b), int(lastStatus))
			}
		}
		io.lastIO[devNum] = u
	}
}

func (io *IOManager) notifyIOChange(devNum, pinNum, status int) {
	c := io.c.IODevices[devNum]
	for _, v := range c.IOMap {
		if v.PinNumber == pinNum && v.IsOutput {
			io.q.Publish(fmt.Sprintf("%s/%d", c.Topic, v.TopicNumber), status)
		}
	}
}

func (io *IOManager) notifyStatus(devNum int, online bool) {
	if online {
		io.recoverDevice(devNum)
	}

	o := "false"
	if online {
		o = "true"
	}

	err := io.q.Publish(io.getStatusTopic(devNum), o)
	if err != nil {
		iolog.Warn("Error notifying that device %d is %v: %s", devNum, online, err)
	}
}

func (io *IOManager) recoverDevice(devNum int) {
	iolog.Debug("Recovering device %d", devNum)
	c := io.c.IODevices[devNum]
	d := io.devs[devNum]
	for _, v := range c.IOMap {
		if v.IsOutput {
			d.PinMode(uint8(v.PinNumber), mcp23017.OUTPUT)
			io.q.Subscribe(fmt.Sprintf("%s/%d", c.Topic, v.TopicNumber))
		} else {
			d.PinMode(uint8(v.PinNumber), mcp23017.INPUT)
		}
	}
}

func (io *IOManager) MessageHandle(msg mqtt.Message) {
	topic := msg.Topic()
	value := string(msg.Payload())
	log.Debug("Received Message: %s => %s", topic, value)
	t := strings.Split(topic, "/")
	if len(t) <= 1 {
		return
	}

	found := false

	level := mcp23017.LOW
	if value == "1" {
		level = mcp23017.HIGH
	}

	for devNum, v := range io.c.IODevices {
		if io.devs[devNum] == nil {
			continue
		}
		if t[0] == v.Topic {
			for _, pin := range v.IOMap {
				if t[1] == strconv.FormatInt(int64(pin.TopicNumber), 10) {
					log.Info("(%s) Changing I/O %d from dev %d to %v", topic, pin.PinNumber, devNum, level)
					err := io.devs[devNum].DigitalWrite(uint8(pin.PinNumber), level)
					if err != nil {
						iolog.Error("(%s) Error setting GPIO %d from %d to %v: %s", topic, pin.PinNumber, devNum, level, err)
					}
					found = true
				}
			}
		}
		if found {
			break
		}
	}
}

func (io *IOManager) getStatusTopic(devNum int) string {
	return io.c.IODevices[devNum].StatusTopic
}

func (io *IOManager) Close() {
	io.running = false
	for _, v := range io.devs {
		v.Close()
	}
	io.devs = make([]*mcp23017.Device, 0)
}

func (io *IOManager) initializeDevices() error {
	var err error
	if len(io.c.IODevices) > 8 {
		return fmt.Errorf("each bus only supports up to 8 devices")
	}

	if len(io.c.IODevices) < 1 {
		return fmt.Errorf("you should have at least one device")
	}

	io.devs = make([]*mcp23017.Device, len(io.c.IODevices))
	io.lastStatus = make([]bool, len(io.c.IODevices))
	io.lastIO = make([]uint16, len(io.c.IODevices))

	for i := 0; i < len(io.c.IODevices); i++ {
		io.devs[i], err = mcp23017.Open(uint8(io.c.BusNumber), uint8(i))
		if err != nil {
			return fmt.Errorf("cannot open device %d in bus %d", i, io.c.BusNumber)
		}
		io.lastStatus[i] = true
		io.notifyStatus(i, true)
	}

	return err
}

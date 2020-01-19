package main

import (
	"fmt"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/quan-to/slog"
	"sync"
	"time"
)

var qlog = slog.Scope("QueueManager")

type OnMessage func(mqtt.Message)

type QueueManager struct {
	client              mqtt.Client
	subscribedTopics    []string
	running             bool
	l                   sync.Mutex
	lastConnectionState bool
	onMessage           OnMessage
	closeQueue          string
}

func MakeQueueManager(config MQTTConfig) (*QueueManager, error) {
	q := &QueueManager{
		subscribedTopics:    make([]string, 0),
		running:             false,
		lastConnectionState: true,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:1883", config.MQTTServer))
	opts.SetUsername(config.MQTTUsername)
	opts.SetPassword(config.MQTTPassword)
	opts.SetDefaultPublishHandler(q.onPublish)

	opts.OnConnect = q.onConnect

	c := mqtt.NewClient(opts)
	c.Connect().WaitTimeout(time.Second * 5)

	q.client = c
	q.running = true
	go q.checkLoop()

	if config.CloseQueue != "" {
		q.closeQueue = config.CloseQueue
		qlog.Info("Enabling close queue at topic: %s", config.CloseQueue)
		_ = q.Subscribe(config.CloseQueue)
	}

	return q, nil
}

func (q *QueueManager) Subscribe(topic string) error {
	q.l.Lock()
	defer q.l.Unlock()

	qlog.Info("Subscribing to topic %s", topic)
	token := q.client.Subscribe(topic, 0, nil)
	if !token.WaitTimeout(time.Second) {
		return fmt.Errorf("timed out subscribing %s", topic)
	}

	if token.Error() != nil {
		return token.Error()
	}

	add := true

	for _, v := range q.subscribedTopics {
		if v == topic {
			add = false
			break
		}
	}

	if add {
		q.subscribedTopics = append(q.subscribedTopics, topic)
	}

	return nil
}

func (q *QueueManager) SetOnMessage(cb OnMessage) {
	q.onMessage = cb
}

func (q *QueueManager) checkLoop() {
	running := q.running
	for running {
		q.l.Lock()

		// region Manage Connection
		if q.client.IsConnected() && !q.lastConnectionState {
			qlog.Info("Connection restored")
			q.lastConnectionState = true
		}

		if !q.client.IsConnected() {
			if q.lastConnectionState {
				qlog.Error("Not connected to MQTT. Retrying...")
			}
			q.lastConnectionState = false
			q.client.Connect().WaitTimeout(time.Second)
		}
		// endregion

		running = q.running
		q.l.Unlock()
		time.Sleep(time.Millisecond * 200)
	}
}

func (q *QueueManager) Close() {
	q.l.Lock()
	q.running = false
	q.l.Unlock()
}

func (q *QueueManager) onConnect(client mqtt.Client) {
	for _, v := range q.subscribedTopics {
		err := q.Subscribe(v)
		if err != nil {
			qlog.Error("Error subscribing to %s: %s", v, err)
		}
	}
	if q.closeQueue != "" {
		qlog.Info("Enabling close queue at topic: %s", q.closeQueue)
		_ = q.Subscribe(q.closeQueue)
	}
}

func (q *QueueManager) onPublish(client mqtt.Client, message mqtt.Message) {
	if q.closeQueue != "" && message.Topic() == q.closeQueue {
		panic("Received CLOSE QUEUE")
		return
	}
	if q.onMessage != nil {
		q.onMessage(message)
	}
}

func (q *QueueManager) Publish(topic string, payload interface{}) error {
	q.l.Lock()
	defer q.l.Unlock()
	t := q.client.Publish(topic, 0, false, payload)

	if !t.WaitTimeout(time.Second * 2) {
		return fmt.Errorf("timed out sending data: %s", t.Error())
	}

	if t.Error() != nil {
		return t.Error()
	}

	return nil
}

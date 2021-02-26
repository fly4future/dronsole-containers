package main

import (
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func newMQTTClient(id string, brokerAddress string) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerAddress).
		SetClientID(id).
		SetUsername(id).
		//SetTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}).
		SetPassword("").
		SetProtocolVersion(4) // Use MQTT 3.1.1

	client := mqtt.NewClient(opts)

	tok := client.Connect()
	if err := tok.Error(); err != nil {
		log.Fatalf("MQTT connection failed: %v", err)
	}
	if !tok.WaitTimeout(time.Second * 5) {
		log.Fatal("MQTT connection timeout")
	}
	err := tok.Error()
	if err != nil {
		log.Fatalf("Could not connect to MQTT broker: %v", err)
	}

	return client
}

func listenMQTTEvents(client mqtt.Client) {
	const qos = 0
	token := client.Subscribe("/devices/#", qos, func(client mqtt.Client, msg mqtt.Message) {
		t := strings.TrimPrefix(msg.Topic(), "/devices/")
		deviceID := strings.Split(t, "/")[0]
		topic := strings.TrimPrefix(t, deviceID+"/")
		if strings.HasPrefix(topic, "commands") {
			handleMQTTEvent(deviceID, strings.TrimPrefix(topic, "commands/"), msg.Payload())
		}
	})

	err := token.Error()
	if err != nil {
		log.Fatalf("Could not subscribe to MQTT events: %v", err)
	}
}

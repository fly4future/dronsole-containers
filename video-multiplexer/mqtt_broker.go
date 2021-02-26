package main

import (
	"log"
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

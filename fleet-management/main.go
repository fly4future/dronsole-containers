package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/julienschmidt/httprouter"
)

var mqttClient mqtt.Client

func main() {
	if len(os.Args) != 2 {
		fmt.Println("usage: fleet-management <mqtt-broker-address>")
		return
	}
	mqttBrokerAddress := os.Args[1]

	port := "8082"

	mqttClient = newMQTTClient(mqttBrokerAddress)
	defer mqttClient.Disconnect(1000)
	router := httprouter.New()
	registerRoutes(router)

	listenMQTTEvents(mqttClient)

	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal(err)
	}

	return
}

var activeDrones map[string]time.Time = make(map[string]time.Time)

func isDroneActive(deviceID string) bool {
	t, ok := activeDrones[deviceID]
	if !ok {
		// device haven't seen online
		return false
	}

	minuteAgo := time.Now().Add(-1 * time.Minute)
	if t.Before(minuteAgo) {
		return false
	}
	return true
}

func listenMQTTEvents(client mqtt.Client) {
	const qos = 0
	token := client.Subscribe("/devices/#", qos, func(client mqtt.Client, msg mqtt.Message) {
		t := strings.TrimPrefix(msg.Topic(), "/devices/")
		deviceID := strings.Split(t, "/")[0]
		topic := strings.TrimPrefix(t, deviceID+"/")
		if strings.HasPrefix(topic, "events") {
			// we have a message from the device
			activeDrones[deviceID] = time.Now()

			handleMQTTEvent(deviceID, strings.TrimPrefix(topic, "events/"), msg)
		}
	})

	err := token.Error()
	if err != nil {
		log.Fatalf("Could not subscribe to MQTT events: %v", err)
	}
}
func handleMQTTEvent(deviceID string, eventTopic string, msg mqtt.Message) {
	if eventTopic == "trust" {
		log.Printf("Got a trust-event from %v", deviceID)
		go handleTrustMessage(deviceID, msg.Payload())
	}
}

func newMQTTClient(brokerAddress string) mqtt.Client {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerAddress).
		SetClientID("fleet-management").
		SetUsername("fleet-management").
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

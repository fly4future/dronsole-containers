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
		fmt.Println("usage: mission-control <mqtt-broker-address>")
		return
	}
	mqttBrokerAddress := os.Args[1]

	port := "8082"

	mqttClient = newMQTTClient(mqttBrokerAddress)
	defer mqttClient.Disconnect(1000)
	router := httprouter.New()
	registerRoutes(router)
	router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Access-Control-Request-Method") != "" {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:8080")
			w.Header().Set("Access-Control-Allow-Methods", w.Header().Get("Allow"))
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Adjust status code to 204
		w.WriteHeader(http.StatusNoContent)
	})

	listenMQTTEvents(mqttClient)

	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, setCORSHeader("http://localhost:8080", router))
	if err != nil {
		log.Fatal(err)
	}

	return
}

func setCORSHeader(origin string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		handler.ServeHTTP(w, r)
	})
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
		SetClientID("mission-control").
		SetUsername("mission-control").
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

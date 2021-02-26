package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

var mqttPub MqttPublisher

var (
	rtspServerAddress = flag.String("rtsp", "127.0.0.1:8554", "RTSP server address")
	mqttBrokerAddress = flag.String("mqtt", "127.0.0.1:8883", "MQTT broker address")
	testMode          = flag.Bool("test", false, "Enable test page: http://localhost:8084/test")
)

func main() {
	flag.Parse()
	if *mqttBrokerAddress == "cloud" {
		log.Println("MQTT: IoT Core")
		mqttPub = NewIoTPublisher()
	} else {
		log.Printf("MQTT: broker @ %s", *mqttBrokerAddress)
		mqttClient := newMQTTClient("mission-control", *mqttBrokerAddress)
		defer mqttClient.Disconnect(1000)
		mqttPub = NewMqttPublisher(mqttClient)
	}

	router := httprouter.New()
	registerRoutes(router, *testMode)

	port := "8084"
	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal(err)
	}
}

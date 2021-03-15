package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/tiiuae/gosshgit"
)

var mqttPub MqttPublisher
var gitServer gosshgit.Server
var sshServerAddress string

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: mission-control <bind-to-address> <mqtt-broker-address> | cloud-push | cloud-pull")
		return
	}

	sshPort := 2222
	sshServerAddress = os.Args[1]

	mqttBrokerAddress := os.Args[2]
	if mqttBrokerAddress == "cloud-pull" {
		log.Println("MQTT: IoT Core pull")
		mqttPub = NewIoTPublisher()
		go pullIoTCoreMessages("telemetry-mission-control-pull-sub")
		go pullIoTCoreMessages("iot-device-location-mission-control-pull-sub")
	} else if mqttBrokerAddress == "cloud-push" {
		log.Println("MQTT: IoT Core push")
		mqttPub = NewIoTPublisher()
	} else {
		log.Printf("MQTT: emulator @ %s", mqttBrokerAddress)
		mqttClient := newMQTTClient("mission-control", mqttBrokerAddress)
		defer mqttClient.Disconnect(1000)
		listenMQTTEvents(mqttClient)
		mqttPub = NewMqttPublisher(mqttClient)
	}

	gitServer = gosshgit.New("repositories")
	err := gitServer.Initialize()
	if err != nil {
		log.Fatalf("Could not initialize git ssh server: %v", err)
	}

	// run git server on goroutine
	go func() {
		err := gitServer.ListenAndServe(fmt.Sprintf(":%d", sshPort))
		if err != nil {
			log.Printf("ListenAndServe: %v", err)
		}
	}()

	// shutdown git server at the end
	defer func() {
		err := gitServer.Shutdown(context.Background())
		if err != nil {
			log.Printf("Could not shutdown git server: %v", err)
			log.Printf("Forcing the git server to close")
			err = gitServer.Close()
			if err != nil {
				log.Printf("Could not forcefully close the server: %v", err)
			}
		}
	}()

	// initialize http server
	router := httprouter.New()
	registerRoutes(router)
	router.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isValidOrigin(r) && r.Header.Get("Access-Control-Request-Method") != "" {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Methods", w.Header().Get("Allow"))
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Max-Age", "3600")
		}

		// Adjust status code to 204
		w.WriteHeader(http.StatusNoContent)
	})

	port := "8082"
	log.Printf("Listening on port %s", port)
	err = http.ListenAndServe(":"+port, setCORSHeader(router))
	if err != nil {
		log.Fatal(err)
	}
}

func setCORSHeader(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isValidOrigin(r) {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		}
		handler.ServeHTTP(w, r)
	})
}

func isValidOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	return strings.HasSuffix(o, "localhost:8080") || strings.HasSuffix(o, "sacplatform.com")
}

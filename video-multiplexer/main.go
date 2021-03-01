package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

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
		mqttClient := newMQTTClient("video-multiplexer", *mqttBrokerAddress)
		defer mqttClient.Disconnect(1000)
		mqttPub = NewMqttPublisher(mqttClient)
	}

	router := httprouter.New()
	registerRoutes(router, *testMode)
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

	port := "8084"
	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, setCORSHeader(router))
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
	return strings.HasSuffix(o, "localhost:8080") || strings.HasSuffix(o, "auto-fleet-mgnt.ew.r.appspot.com") || strings.HasSuffix(o, "sacplatform.com")
}

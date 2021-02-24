package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"google.golang.org/api/idtoken"
)

var (
	pubsubVerificationToken = os.Getenv("PUBSUB_VERIFICATION_TOKEN")
)

const (
	audience = "https://auto-fleet-mgnt.appspot.com/"
)

type pubsubPushRequest struct {
	Message struct {
		Attributes map[string]string
		Data       []byte
		ID         string `json:"message_id"`
	}
	Subscription string
}

func telemetryPostHandler(w http.ResponseWriter, r *http.Request) {
	if !authenticatePubSubJWT(r) {
		log.Printf("Authentication failed")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var msg pubsubPushRequest
	err := json.NewDecoder(r.Body).Decode(&msg)

	if err != nil {
		log.Printf("Error reading body, error: %s", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	deviceID, ok := msg.Message.Attributes["deviceId"]
	if !ok {
		log.Printf("Pubsub message '%s' doesn't contain attribute deviceId", msg.Message.ID)
		return
	}
	topic, ok := msg.Message.Attributes["subFolder"]
	if !ok {
		log.Printf("Pubsub message '%s' doesn't contain attribute subFolder", msg.Message.ID)
		return
	}

	handleMQTTEvent(deviceID, topic, msg.Message.Data)
}

func authenticatePubSubJWT(r *http.Request) bool {
	if token, ok := r.URL.Query()["token"]; !ok || len(token) != 1 || token[0] != pubsubVerificationToken {
		log.Printf("Bad token")
		return false
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || len(strings.Split(authHeader, " ")) != 2 {
		log.Printf("Missing Authorization header")
		return false
	}
	token := strings.Split(authHeader, " ")[1]
	payload, err := idtoken.Validate(r.Context(), token, audience)

	if err != nil {
		log.Printf("Invalid Token: %v", err)
		return false
	}
	if payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com" {
		log.Printf("Wrong Issuer")
		return false
	}

	return true
}

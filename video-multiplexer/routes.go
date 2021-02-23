package main

import (
	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"
	"log"
	"net/http"
)

func registerRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/startvideostream", videoStreamHandler)
	router.Handler(http.MethodGet, "/video/:stream", websocket.Handler(rtsptompeg))
	router.HandlerFunc(http.MethodGet, "/getandstartvideo", getVideoHandler)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	b, err := json.Marshal(data)
	if err != nil {
		log.Printf("Could not marshal data to json: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

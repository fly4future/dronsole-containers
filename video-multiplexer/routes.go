package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"
)

func registerRoutes(router *httprouter.Router, testMode bool) {
	router.Handler(http.MethodGet, "/video/:stream", websocket.Handler(streamVideo))
	if testMode {
		router.HandlerFunc(http.MethodGet, "/test", testHandler)
		router.HandlerFunc(http.MethodGet, "/jsmpeg.min.js", getJS)
	}
}

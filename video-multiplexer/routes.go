package main

import (
	"github.com/julienschmidt/httprouter"
	"golang.org/x/net/websocket"
	"net/http"
)

func registerRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/startvideostream", videoStreamHandler)
	router.Handler(http.MethodGet, "/video/:stream", websocket.Handler(rtsptompeg))
}

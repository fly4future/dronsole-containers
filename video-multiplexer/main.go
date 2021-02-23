package main

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	"os"
)

var rtspAddress string

func main() {

	if len(os.Args) == 2 {
		rtspAddress = os.Args[1]
	}

	router := httprouter.New()
	registerRoutes(router)

	port := "8084"

	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal(err)
	}

}

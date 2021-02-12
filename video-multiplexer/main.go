package main

import (
	"log"
	"net/http"
	"github.com/julienschmidt/httprouter"
)

func main() {
	router := httprouter.New()
	registerRoutes(router)

	port := "8083"

	log.Printf("Listening on port %s", port)
	err := http.ListenAndServe(":"+port, router)
	if err != nil {
		log.Fatal(err)
	}

}

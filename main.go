package main

import (
	"log"
	"net/http"
)

func main() {
	config := GetConfig("config.json")
	db := InitDB(config)

	log.Println("relay is running")
	log.Fatal(http.ListenAndServe(":8080", ws(db)))
}

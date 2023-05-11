package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Host     string
	User     string
	Password string
	DBname   string
}

func GetConfig(filename string) Config {
	var config Config
	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("error reading config: %v", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	err = dec.Decode(&config)
	if err != nil {
		log.Fatalf("error decoding config: %v", err)
	}
	return config
}

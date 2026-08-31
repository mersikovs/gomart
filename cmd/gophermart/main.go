package main

import (
	"flag"
	"log"
	"os"

	"github.com/mersikovs/gomart/internal/config"
)

func main() {

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	cfg, err := config.Load(fs, os.Args[1:], config.OSenv{})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Run with config %v",
		cfg.Safe())
}

package main

import (
	"log"

	"sinanmohd.com/namescale/internal/config"
	"sinanmohd.com/namescale/internal/dns"
)

func main() {
	config, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	dns.Run(config)
}

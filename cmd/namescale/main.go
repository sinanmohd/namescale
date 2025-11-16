package main

import (
	"log"

	"sinanmohd.com/namescale/internal/config"
	"sinanmohd.com/namescale/internal/dns"
	"sinanmohd.com/namescale/internal/ntsnet"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalln(err)
	}

	net, err := ntsnet.New(cfg.Tsnet)
	if err != nil {
		log.Fatalln(err)
	}

	dns.Run(cfg, net)
}

package config

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"

	"github.com/BurntSushi/toml"
)

type Tsnet struct {
	Port                  uint16 `toml:"port"`
	Hostname              string `toml:"hostname"`
	AuthKey               string `toml:"auth_key"`
	CoordinationServerURL string `toml:"coordination_server_url"`
	Ephemeral             bool   `toml:"ephemeral"`
}

type Config struct {
	Tsnet               Tsnet    `toml:"tsnet"`
	BaseForwardFallback []string `toml:"base_forward_fallback"`
}

func New() (*Config, error) {
	var configPath string
	defaultConfigPath := "/etc/namescale.toml"
	if value, ok := os.LookupEnv("NAMESCALE_CONFIG"); ok {
		configPath = value
	} else {
		configPath = defaultConfigPath
	}

	config := Config{
		Tsnet: Tsnet{
			Hostname:  "namescale",
			Ephemeral: true,
		},
		BaseForwardFallback: []string{"1.1.1.1", "8.8.8.8"},
	}

	_, err := os.Stat(configPath)
	if err != nil {
		if (configPath == defaultConfigPath && !errors.Is(err, os.ErrNotExist)) ||
			configPath != defaultConfigPath {
			slog.Error("Error reading config", "err", err)
			return nil, err
		}
	}

	if _, err := os.Stat(configPath); err == nil {
		_, err := toml.DecodeFile(configPath, &config)
		if err != nil {
			slog.Error("Error decoding TOML", "err", err)
			return nil, err
		}
	}

	flag.StringVar(
		&config.Tsnet.CoordinationServerURL,
		"coordination-server",
		config.Tsnet.CoordinationServerURL,
		"Bind host",
	)
	flag.StringVar(
		&config.Tsnet.AuthKey,
		"auth-key",
		config.Tsnet.AuthKey,
		"Bind host",
	)
	var u uint
	flag.UintVar(
		&u,
		"tsnet-port",
		uint(config.Tsnet.Port),
		"Bind host",
	)
	flag.Parse()

	if u > math.MaxUint16 {
		return nil, fmt.Errorf("Tailnet port too big for uint16: %v", u)
	}
	config.Tsnet.Port = uint16(u)

	return &config, nil
}

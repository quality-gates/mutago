package conf

import (
	"encoding/json"
	"errors"
	"strings"
)

type Config struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

// Parse reads a config; a blank name or a port outside 1-65535 is an error.
func Parse(data []byte) (Config, error) {
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(c.Name) == "" {
		return Config{}, errors.New("no name")
	}
	if c.Port < 1 || c.Port > 65535 {
		return Config{}, errors.New("bad port")
	}
	return c, nil
}

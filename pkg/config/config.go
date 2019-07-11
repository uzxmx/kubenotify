package config

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// ConfigFile is the config file path
var ConfigFile = "/etc/kubenotify/kubenotify.yaml"

// Handler contains handler configuration
type Handler struct {
	Slack Slack `json:"slack"`
}

// Slack contains slack configuration
type Slack struct {
	Token   string `json:"token"`
	Channel string `json:"channel"`
}

// Config contains kubenotify configuration
type Config struct {
	Handler Handler `json:"handler"`
}

// New creates new config object
func New() (*Config, error) {
	c := &Config{}
	if err := c.load(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) load() error {
	file, err := os.Open(ConfigFile)
	if err != nil {
		return err
	}

	b, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	if len(b) != 0 {
		return yaml.Unmarshal(b, c)
	}

	return nil
}

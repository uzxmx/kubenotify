package handlers

import (
	"fmt"
	"github.com/uzxmx/kubenotify/pkg/config"
)

// Handler is a generic notifier interface
type Handler interface {
	Init(c *config.Config) error
	Notify(message string) error
}

// GetHandler returns handler based on configuration
func GetHandler(c *config.Config) (Handler, error) {
	var handler Handler
	switch {
	case len(c.Handler.Slack.Channel) > 0 || len(c.Handler.Slack.Token) > 0:
		handler = new(Slack)
	default:
		return nil, fmt.Errorf("Unknown handler")
	}
	if err := handler.Init(c); err != nil {
		return nil, err
	}
	return handler, nil
}

package handlers

import (
	"fmt"
	"github.com/nlopes/slack"
	"github.com/uzxmx/kubenotify/pkg/config"
)

// Slack is a slack notifier
type Slack struct {
	Token   string
	Channel string
}

// Init initializes notifier
func (s *Slack) Init(c *config.Config) error {
	s.Token = c.Handler.Slack.Token
	s.Channel = c.Handler.Slack.Channel

	if s.Token == "" || s.Channel == "" {
		return fmt.Errorf("Missing token or channel")
	}
	return nil
}

// Notify sends message to a slack channel
func (s *Slack) Notify(message string) error {
	api := slack.New(s.Token)
	// attachment := slack.Attachment{
	// 	Color: "good",
	// 	Title: "Title " + message,
	// 	Text:  message,
	// Fields: []slack.AttachmentField{
	// 	{
	// 		Title: "kubenotify",
	// 		Value: message,
	// 	},
	// },
	// MarkdownIn: []string{"fields"},
	// }
	_, _, err := api.PostMessage(s.Channel, slack.MsgOptionAsUser(true), slack.MsgOptionText(message, false))
	return err
}

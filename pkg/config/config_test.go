package config

import (
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	var configStr = `
handler:
  slack:
    token: slack_token
    channel: slack_channel
`
	content := []byte(configStr)
	tmpFile, err := ioutil.TempFile(os.TempDir(), "kubenotify")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	ConfigFile = tmpFile.Name()

	c, err := New()
	if err != nil {
		t.Fatalf("Failed create config: %v", err)
	}

	expected := &Config{
		Handler{
			Slack{
				Channel: "slack_channel",
				Token:   "slack_token",
			},
		},
	}
	if !cmp.Equal(c, expected) {
		t.Fatalf("Unexpected config: %v", c)
	}
}

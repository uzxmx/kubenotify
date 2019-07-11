package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/uzxmx/kubenotify/pkg/controller"
)

func main() {
	c, err := controller.New()
	if err != nil {
		logrus.Errorf("Failed to create controller: %v", err)
	}
	c.Run()
}

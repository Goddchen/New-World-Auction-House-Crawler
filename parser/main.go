package main

import (
	"strings"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Error initializing file system watcher: %s", err)
	}
	defer watcher.Close()

	done := make(chan bool)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Debug("File added: %s", event.Name)
					if strings.HasSuffix(event.Name, ".png") {
						go parseScreenshot(event.Name)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error("Error watching file system: %s", err)
			}
		}
	}()

	err = watcher.Add("./")
	if err != nil {
		log.Fatal("Error watching screenshot folder: %s", err)
	}
	log.Debug("Watching for file system changes...")

	<-done
}

func parseScreenshot(fileName string) {
	log.Debug("Trying to parse %s", fileName)
}

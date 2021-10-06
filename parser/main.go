package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

// Start a ocrserver via `docker run --rm -p 8080:8080 otiai10/ocrserver`

func main() {
	log.SetLevel(log.DebugLevel)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Error initializing file system watcher: %s", err)
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
					log.Debugf("File added: %s", event.Name)
					if strings.HasSuffix(event.Name, ".png") {
						go parseScreenshot(event.Name)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Errorf("Error watching file system: %s", err)
			}
		}
	}()

	err = watcher.Add("./")
	if err != nil {
		log.Fatalf("Error watching screenshot folder: %s", err)
	}
	log.Debug("Watching for file system changes...")

	go parseScreenshot("samples/sample1.png")

	<-done
}

func parseScreenshot(fileName string) (string, error) {
	log.Debugf("Trying to parse %s", fileName)
	body, writer := io.Pipe()
	request, err := http.NewRequest("POST", "http://localhost:8080/file", body)
	if err != nil {
		log.Errorf("Error creating POST request: %s", err)
		return "", err
	}
	multiPartWriter := multipart.NewWriter(writer)
	request.Header.Add("Content-Type", multiPartWriter.FormDataContentType())

	errchan := make(chan error)

	go func() {
		defer close(errchan)
		defer writer.Close()
		defer multiPartWriter.Close()

		w, err := multiPartWriter.CreateFormFile("file", "file.png")
		if err != nil {
			log.Errorf("Error creating multipart request: %s", err)
			errchan <- err
			return
		}
		in, err := os.Open(fileName)
		if err != nil {
			log.Errorf("Error reading sample file: %s", err)
			errchan <- err
			return
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		if err != nil {
			log.Errorf("Error copying sample data: %s", err)
			errchan <- err
			return
		}
	}()

	resp, err := http.DefaultClient.Do(request)
	merr := <-errchan

	if err != nil || merr != nil {
		log.Errorf("Error sending POST request: http error: %s, multipart error: %s", err, merr)
		if err == nil {
			err = merr
		}
		return "", err
	}

	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response: %s", err)
		return "", err
	}
	log.Debugf("Response: %s", resp)
	var bodyJson map[string]interface{}
	json.Unmarshal(bodyBytes, &bodyJson)
	result := bodyJson["result"]
	log.Debugf("Parsed: %s", result)
	return result.(string), nil
}

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/m3db/prometheus_remote_client_golang/promremote"
	"github.com/pborman/getopt"
	log "github.com/sirupsen/logrus"
)

// Start a ocrserver via `docker run --rm -p 8080:8080 otiai10/ocrserver`

var screenshotsFolderPrt *string
var keepScreenshots = false

var prometheusRemoteWriteClient promremote.Client

func initFlags() {
	getopt.CommandLine.SetParameters("")
	screenshotsFolderPrt = getopt.StringLong("screenshot-folder", 'f', "./screenshots/", "Where to store screenshots (default: ./screenshots/)")
	getopt.BoolVarLong(&keepScreenshots, "keep-screenshots", 'k', "Keep screenshots after parsing is finished (default: false)")
	getopt.Parse()
}

func main() {
	log.SetLevel(log.DebugLevel)

	initFlags()

	initPrometheusRemoteWriteClient()

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

	err = watcher.Add(*screenshotsFolderPrt)
	if err != nil {
		log.Fatalf("Error watching screenshot folder: %s", err)
	}
	log.Info("Watching for file system changes...")

	// go parseScreenshot("samples/ah1.png")

	<-done
}

func initPrometheusRemoteWriteClient() {
	cfg := promremote.NewConfig(
		promremote.WriteURLOption("https://<user>:<pass>@prometheus-prod-01-eu-west-0.grafana.net/api/prom/push"),
		promremote.UserAgent("New World Auction House Parser v0.0.1"),
	)
	client, err := promremote.NewClient(cfg)
	if err != nil {
		log.Errorf("Error creating remote write client: %s", err)
		return
	} else {
		prometheusRemoteWriteClient = client
	}
}

func parseScreenshot(fileName string) {
	if !keepScreenshots {
		defer os.Remove(fileName)
	}
	log.Debugf("Trying to parse %s", fileName)
	title, err := parseImagePart(fileName, 166, 491, 415, 81)
	if err != nil {
		log.Errorf("Error parsing title part: %s", err)
		return
	}

	prices, err := parseImagePart(fileName, 969, 321, 141, 726)
	if err != nil {
		log.Errorf("Error parsing prices part: %s", err)
		return
	}
	parsedPrices := parsePrices(prices)
	log.Debugf("Parsed prices: %s", parsedPrices)

	amounts, err := parseImagePart(fileName, 1511, 317, 58, 688)
	if err != nil {
		log.Errorf("Error parsing amounts part: %s", err)
		return
	}
	parsedAmounts := parseAmounts(amounts)
	log.Debugf("Parsed amounts: %s", parsedAmounts)

	for i := 0; i < len(parsedPrices); i++ {
		if parsedPrices[i] != -1 && parsedAmounts[i] != -1 {
			log.Infof("%s: %d for %f gold", title, parsedAmounts[i], float32(parsedPrices[i]))
			if prometheusRemoteWriteClient != nil {
				timeSeriesList := promremote.TSList {
					promremote.TimeSeries{
						Labels: []promremote.Label{
							{
								Name: "__name__",
								Value: "new_world_auction_house_item_price",
							},
							{
								Name: "item_name",
								Value: title,
							},
						},
						Datapoint: promremote.Datapoint{
							Timestamp: time.Now(),
							Value: parsedPrices[i],
						},
					},
				}
				_, err := prometheusRemoteWriteClient.WriteTimeSeries(context.Background(), timeSeriesList, promremote.WriteOptions{})
				if err != nil {
					log.Warnf("Error remote writing: %s", err)
				}
			}
		}
	}
}

func parsePrices(data string) []float64 {
	tokens := strings.Split(data, "\n")
	floats := make([]float64, len(tokens))
	for i, token := range tokens {
		f, err := strconv.ParseFloat(strings.ReplaceAll(token, ",", "."), 32)
		if err != nil {
			floats[i] = -1
		} else {
			if !strings.Contains(token, ",") && !strings.Contains(token, ".") {
				f = f / 100
			}
			floats[i] = f
		}
	}
	return floats
}

func parseAmounts(data string) []int {
	tokens := strings.Split(data, "\n")
	ints := make([]int, len(tokens))
	for i, token := range tokens {
		value, err := strconv.ParseInt(strings.ReplaceAll(token, ",", "."), 10, 64)
		if err != nil {
			ints[i] = -1
		} else {
			ints[i] = int(value)
		}
	}
	return ints
}

func parseImagePart(filePath string, x int, y int, width int, height int) (string, error) {
	imagePart, err := getImagePart(filePath, x, y, width, height)
	if err != nil {
		log.Errorf("Error getting image part: %s", err)
		return "", err
	}
	buffer := new(bytes.Buffer)
	err = png.Encode(buffer, imagePart)
	if err != nil {
		log.Errorf("Error encoding image part: %s", err)
		return "", err
	}
	base64String := base64.StdEncoding.EncodeToString(buffer.Bytes())
	requestData := fmt.Sprintf("{\"base64\":\"%s\"}", base64String)
	resp, err := http.Post("http://localhost:8080/base64", "application/json", strings.NewReader(requestData))
	if err != nil {
		log.Errorf("Error getting OCR result: %s", err)
		return "", err
	}
	defer resp.Body.Close()
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error reading response: %s", err)
		return "", err
	}
	log.Tracef("Response: %s", resp)
	var bodyJson map[string]interface{}
	json.Unmarshal(bodyBytes, &bodyJson)
	result := bodyJson["result"]
	log.Tracef("Parsed: %s", result)
	return result.(string), nil
}

func getImagePart(filePath string, x int, y int, width int, height int) (image.Image, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Errorf("Error reading image: %s", err)
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		log.Errorf("Error decoding image: %s", err)
		return nil, err
	}
	rgbImg := img.(*image.RGBA)
	subImage := rgbImg.SubImage(image.Rect(x, y, x+width, y+height))
	return subImage, nil
}

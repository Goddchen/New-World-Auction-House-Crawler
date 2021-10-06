package main

import (
	"fmt"
	"image/png"
	"os"
	"time"

	"github.com/kbinani/screenshot"
	"github.com/pborman/getopt"
)

var screenIndexPtr *int
var intervalSecondsPtr *int

func listScreens() {
	numDisplays := screenshot.NumActiveDisplays()
	fmt.Printf("Found %d displays\n", numDisplays)
	fmt.Println("Resolutions:")
	for i := 0; i < numDisplays; i++ {
		resolution := screenshot.GetDisplayBounds(i)
		fmt.Printf("#%d:\t%dx%d\n", i, resolution.Dx(), resolution.Dy())
	}
}

func initFlags() {
	screenIndexPtr = getopt.IntLong("screen-index", 's', 0, "Which screen you want to grab from (default: 0)")
	intervalSecondsPtr = getopt.IntLong("interval", 'i', 10, "Interval to take screenshots in seconds (default: 10)")
	runListScreens := getopt.BoolLong("list-screens", 'l', "List all available screens")
	getopt.CommandLine.SetParameters("")
	getopt.Parse()
	if *runListScreens {
		listScreens()
		os.Exit(0)
	}
}

func main() {
	initFlags()

	err := os.MkdirAll("./screenshots/", 0666)
	if(err != nil) {
		fmt.Printf("Error creating screenshots folder: %s", err)
		os.Exit(-1)
	}

	for {
		fmt.Println("Saving screenshot...")
		image, err := screenshot.CaptureDisplay(*screenIndexPtr)
		if err == nil {
			filename := fmt.Sprintf("./screenshots/%s.png", time.Now().Format("2006-01-02-15-04-05"))
			file, err := os.Create(filename)
			if err == nil {
				png.Encode(file, image)
				file.Close()
			} else {
				fmt.Printf("Error writing screenshot: %s\n", err)
			}
		} else {
			fmt.Printf("Error capturing screenshot: %s\n", err)
		}
		time.Sleep(time.Duration(*intervalSecondsPtr) * time.Second)
	}
}

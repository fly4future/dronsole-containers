package main

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

const (
	videoserver = "127.0.0.1:8554"
	mqttbroker  = "127.0.0.1:8883"
)

type gstreamerCmd struct {
	Command  string
	Address  string
	Source   string
	DeviceID string
}

var commands chan gstreamerCmd

func main() {
	commands = make(chan gstreamerCmd)
	go run()

	// Local MQTT broker
	mqttClient := newMQTTClient("video-test-server", mqttbroker)
	defer mqttClient.Disconnect(1000)
	listenMQTTEvents(mqttClient)

	terminationSignals := make(chan os.Signal, 1)
	signal.Notify(terminationSignals, syscall.SIGINT, syscall.SIGTERM)
	<-terminationSignals
	close(commands)

}

func handleMQTTEvent(deviceID string, topic string, payload []byte) {
	if topic != "videostream" {
		return
	}

	var cmd gstreamerCmd
	err := json.Unmarshal(payload, &cmd)
	if err != nil {
		log.Printf("Failed to unmarshal command: %v", err)
		return
	}

	cmd.DeviceID = deviceID
	commands <- cmd
}

func run() {
	streams := make(map[string]*exec.Cmd)
	for c := range commands {
		log.Printf("Command: %s %s", c.Command, c.DeviceID)
		switch c.Command {
		case "start":
			_, ok := streams[c.DeviceID]
			if !ok {
				log.Println("Starting ffmpeg stream")
				args := []string{"-stream_loop", "-1", "-i", "./data/testvideo.mp4", "-vcodec", "libx264", "-tune", "zerolatency", "-crf", "18", "-f", "rtsp", "-muxdelay", "0.1", c.Address}
				cmd := exec.Command("ffmpeg", args...)
				go cmd.Run()
				streams[c.DeviceID] = cmd
			} else {
				log.Println("Stream already exists")
			}
		case "stop":
			cmd, ok := streams[c.DeviceID]
			if ok {
				log.Println("Stopping ffmpeg stream")
				cmd.Process.Kill()
				delete(streams, c.DeviceID)
			} else {
				log.Println("Stream not found")
			}
		}
	}
}

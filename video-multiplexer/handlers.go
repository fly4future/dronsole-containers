package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

type device struct {
	deviceID    string
	once        sync.Once
	multiplexer *multiplexer
}

// multiplexer is simple fan out
type multiplexer struct {
	mu        sync.RWMutex
	listeners map[string]chan *streamBuffer
	inStream  chan *streamBuffer
	end       chan struct{}
}

type streamBuffer struct {
	width  int
	height int
	buffer []byte
}

func (d *device) getMultiplexer() *multiplexer {
	d.once.Do(d.start)
	return d.multiplexer
}

func (d *device) start() {
	d.multiplexer = &multiplexer{sync.RWMutex{}, make(map[string]chan *streamBuffer), make(chan *streamBuffer), make(chan struct{})}
	go d.multiplexer.multicast()
	go startffmpegV2(d.deviceID, d.multiplexer.inStream, d.multiplexer.end)
}

func (m *multiplexer) subscribe(id string, channel chan *streamBuffer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners[id] = channel
}
func (m *multiplexer) unsubscribe(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.listeners, id)

	if len(m.listeners) == 0 {
		close(m.end)
	}
}

func (m *multiplexer) multicast() {
	for b := range m.inStream {
		m.mu.RLock()
		for _, sub := range m.listeners {
			if len(sub) > 5 {
				log.Printf("Subscriber lagging, channel length: %d", len(sub))
			} else {
				sub <- b
			}
		}
		m.mu.RUnlock()
	}
	// inStream closed

	// close all downstream listener channels
	m.mu.RLock()
	for _, sub := range m.listeners {
		close(sub)
	}
	m.mu.RUnlock()
}

var (
	streams sync.Map
)

func streamVideo(ws *websocket.Conn) {
	deviceID := path.Base(ws.Request().URL.Path)

	ds := &device{deviceID: deviceID, once: sync.Once{}, multiplexer: nil}
	dsTemp, loaded := streams.LoadOrStore(deviceID, ds)
	if loaded {
		ds = dsTemp.(*device)
	}

	id := uuid.New().String()
	stream := make(chan *streamBuffer, 10)
	multiplexer := ds.getMultiplexer()
	multiplexer.subscribe(id, stream)
	defer multiplexer.unsubscribe(id)

	first := <-stream
	magicBytes := makeMagicBytes(first.width, first.height)

	err := websocket.Message.Send(ws, magicBytes)
	if err != nil {
		log.Printf("Failed to send magic bytes: %v", err)
		return
	}

	for b := range stream {
		err := websocket.Message.Send(ws, b.buffer)
		if err != nil {
			log.Printf("Failed to send stream buffer: %v", err)
			break
		}
	}
}

func makeMagicBytes(w int, h int) []byte {
	buf := make([]byte, 8)
	buf[0] = 'j'
	buf[1] = 's'
	buf[2] = 'm'
	buf[3] = 'p'
	copy(buf[4:], big.NewInt(int64(w)).Bytes())
	copy(buf[6:], big.NewInt(int64(h)).Bytes())
	return buf[:8]
}

var testpage = template.Must(template.ParseFiles("./testpage/testpage.html"))

//handler for test page
func testHandler(w http.ResponseWriter, r *http.Request) {
	deviceid, ok := r.URL.Query()["deviceid"]
	if !ok || len(deviceid[0]) < 1 {
		log.Println("Deviceid is missing")
		http.Error(w, "Deviceid parameter is required", http.StatusBadRequest)
		return
	}

	data := struct {
		URL string
	}{
		fmt.Sprintf("//%s/video/%s", r.Host, deviceid[0]),
	}
	var html bytes.Buffer
	err := testpage.Execute(&html, data)
	if err != nil {
		log.Printf("Error on executing template: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(html.Bytes())
}

func getJS(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("./testpage/jsmpeg.min.js")
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/javascript")
	w.Write(data)
}

func startffmpegV2(deviceID string, res chan *streamBuffer, end chan struct{}) {
	readAddr := fmt.Sprintf("rtsp://%s/%s", *rtspServerAddress, deviceID)
	writeAddr := fmt.Sprintf("rtsp://DroneUser:22f6c4de-6144-4f6c-82ea-8afcdf19f316@%s/%s", *rtspServerAddress, deviceID)

	err := sendStartVideoCommand(deviceID, writeAddr)
	if err != nil {
		log.Printf("Failed to send start command: %v", err)
		close(res)
		return
	}

	defer func() {
		streams.Delete(deviceID)
		err := sendStopVideoCommand(deviceID)
		if err != nil {
			log.Printf("Failed to send start command: %v", err)
		}
		log.Printf("Stop streaming: %v", deviceID)
		close(res)
	}()

	args := []string{"-rtsp_transport", "tcp", "-i", readAddr, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("ffmpeg args: %v", args)

	width := 1280
	height := 720

	//retry loop for starting ffmpeg again, if ffmpeg exits (for example if the rtsp stream does not exist yet)
	for i := 0; i < 15; i++ {
		log.Printf("ffmpeg retry: %v", i)
		cmd := exec.Command("ffmpeg", args...)
		cmdReader, _ := cmd.StdoutPipe()
		errReader, _ := cmd.StderrPipe()
		errscanner := bufio.NewScanner(errReader)

		err := cmd.Start()
		if err != nil {
			log.Printf("Cmd err:%v", err)
			continue
		}

		// read data until process dies
		for {
			data := make([]byte, 16384)
			n, err := cmdReader.Read(data)
			if err != nil {
				log.Printf("Cmd read failed: %v", err)
				break
			}
			select {
			case <-end:
				log.Printf("No more listeners. Kill ffmpeg")
				cmd.Process.Kill()
				return
			default:
				res <- &streamBuffer{width, height, data[:n]}
			}
		}

		log.Printf("Waiting for stream... %d", i+1)
		time.Sleep(1 * time.Second)

		//stderr read function
		//parses the rtsp stream resolution and adds rtsp stream instance to rtsp stream list
		go func() {
			input := false
			for errscanner.Scan() {
				line := errscanner.Text()
				if strings.Index(line, "Input #") == 0 {
					input = true
				} else if strings.Contains(line, "Output #") || strings.Contains(line, "Stream mapping") {
					input = false
				}
				if input {
					fmt.Printf("ErrScanner: %v\n", line)
					l := strings.Split(line, " ")
					for _, str := range l {
						re := regexp.MustCompile(`(\d+)x(\d+)`)
						// parse and set stream resolution
						if re.FindStringSubmatch(str) != nil {
							str = strings.ReplaceAll(str, ",", "")
							log.Printf("Found: %v\n", str)
							dimstr := strings.Split(str, "x")
							width, _ = strconv.Atoi(dimstr[0])
							height, _ = strconv.Atoi(dimstr[1])
							log.Printf("Stream start: w:%v,  h:%v\n", width, height)
						}
					}
				}
			}
		}()
	}
}

type gstreamerCmd struct {
	Command string
	Address string
	Source  string
}

func sendStartVideoCommand(deviceID string, address string) error {
	cmd := gstreamerCmd{
		"start",
		address,
		"",
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return errors.WithMessage(err, "Failed to serialize command")
	}
	err = mqttPub.SendCommand(deviceID, "videostream", payload)
	if err != nil {
		return errors.WithMessage(err, "Failed to send videostream command to device")
	}
	log.Printf("Start stream sent to device: %v", deviceID)

	return nil
}

func sendStopVideoCommand(deviceID string) error {
	cmd := gstreamerCmd{
		"stop",
		"",
		"",
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return errors.WithMessage(err, "Failed to serialize command")
	}
	err = mqttPub.SendCommand(deviceID, "videostream", payload)
	if err != nil {
		return errors.WithMessage(err, "Failed to send MQTT command")
	}
	log.Printf("Stop stream sent to device: %v", deviceID)

	return nil
}

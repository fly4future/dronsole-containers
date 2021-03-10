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
	deviceID string
	once     sync.Once
	stream   *streamer
}

type streamer struct {
	listeners sync.Map
	stream    chan *streamBuffer
	end       chan bool
}

type streamBuffer struct {
	width  int
	height int
	buffer []byte
}

func (x *device) getStreamer() *streamer {
	x.once.Do(x.start)
	return x.stream
}

func (x *device) start() {
	x.stream = &streamer{sync.Map{}, make(chan *streamBuffer), make(chan bool)}
	go x.stream.multicast(x.deviceID)
	go startffmpegV2(x.deviceID, x.stream.stream, x.stream.end)
}

func (x *streamer) multicast(deviceID string) {
	for b := range x.stream {
		count := 0
		x.listeners.Range(func(key, value interface{}) bool {
			sub := value.(chan *streamBuffer)
			if len(sub) > 5 {
				log.Printf("Subscriber lagging, channel length: %d", len(sub))
			} else {
				sub <- b
			}
			count++
			return true
		})
		if count == 0 {
			log.Printf("Ending stream: no subscribers")
			x.end <- true
			streams.Delete(deviceID)
		}
	}
}

var (
	streams sync.Map
)

func streamVideo(ws *websocket.Conn) {
	deviceID := path.Base(ws.Request().URL.Path)
	log.Printf("Streamid: %v\n", deviceID)

	ds := &device{deviceID: deviceID, once: sync.Once{}, stream: nil}
	dsTemp, loaded := streams.LoadOrStore(deviceID, ds)
	if loaded {
		ds = dsTemp.(*device)
	}

	id := uuid.New().String()
	stream := make(chan *streamBuffer, 10)
	streamer := ds.getStreamer()
	streamer.listeners.Store(id, stream)

	first := <-stream
	magicBytes := makeMagicBytes(first.width, first.height)

	err := websocket.Message.Send(ws, magicBytes)
	if err != nil {
		log.Printf("Failed to send magic bytes: %v", err)
		streamer.listeners.Delete(id)
		return
	}

	for b := range stream {
		err := websocket.Message.Send(ws, b.buffer)
		if err != nil {
			log.Printf("Failed to send stream buffer: %v", err)
			break
		}
	}

	streamer.listeners.Delete(id)
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
		Url string
	}{
		fmt.Sprintf("//%s/video/%s", r.Host, deviceid[0]),
	}
	var html bytes.Buffer
	testpage.Execute(&html, data)
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

func startffmpegV2(streamid string, res chan *streamBuffer, end chan bool) {
	readAddr := fmt.Sprintf("rtsp://%s/%s", *rtspServerAddress, streamid)
	writeAddr := fmt.Sprintf("rtsp://DroneUser:22f6c4de-6144-4f6c-82ea-8afcdf19f316@%s/%s", *rtspServerAddress, streamid)

	err := sendStartVideoCommand(streamid, writeAddr)
	if err != nil {
		log.Printf("Failed to send start command: %v", err)
		close(res)
		return
	}

	args := []string{"-rtsp_transport", "tcp", "-i", readAddr, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("ffmpeg args: %v", args)

	width := 1280
	height := 720
	quit := false

	//retry loop for starting ffmpeg again, if ffmpeg exits (for example if the rtsp stream does not exist yet)
	for i := 0; i < 15; i++ {
		log.Printf("ffmpeg retry: %v", i)
		cmd := exec.Command("ffmpeg", args...)
		cmdReader, _ := cmd.StdoutPipe()
		errReader, _ := cmd.StderrPipe()
		errscanner := bufio.NewScanner(errReader)

		//stdout read function which streams to viewer bytechannels
		go func() {
			data := make([]byte, 16384)
			for {
				time.Sleep(5 * time.Millisecond)
				n, err := cmdReader.Read(data)
				if err != nil {
					log.Printf("Cmd read failed: %v", err)
					break
				}
				// log.Printf("Streaming bytes: %v", n)

				select {
				case <-end:
					quit = true
					fmt.Printf("Kill ffmpeg\n")
					cmd.Process.Kill()
					err := sendStopVideoCommand(streamid)
					if err != nil {
						log.Printf("Failed to send start command: %v", err)
					}
					break
				default:
					res <- &streamBuffer{width, height, data[:n]}
				}
			}
		}()

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
		e := cmd.Run()
		if e != nil {
			log.Printf("Cmd err:%v", e)
		}
		if !quit {
			log.Printf("Waiting for stream... %d", i+1)
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}
	log.Printf("Stop streaming: %v", streamid)
	close(res)
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

	return nil
}

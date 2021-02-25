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

	"github.com/pkg/errors"
	"golang.org/x/net/websocket"
)

//a new mpegStream instance is created for every websocket stream (i.e for every web UI)
type mpegStream struct {
	bytechan chan []byte
	id       int
}

//a new rtspStream instance is created for every rtsp stream opneing the stream from rtsp server
type rtspStream struct {
	viewers  []*mpegStream
	width    int
	height   int
	streamid string
	count    int
	stop     bool
	mux      sync.Mutex
}

//append viever instance to viever list of one rtspstream
func (r *rtspStream) appendViewer(v *mpegStream) {
	r.mux.Lock()
	r.viewers = append(r.viewers, v)
	r.count++
	r.mux.Unlock()
}

//remove viever instance from viever list
func (r *rtspStream) removeViewer(v *mpegStream) {
	r.mux.Lock()
	for i, vs := range r.viewers {
		if vs == v {
			r.viewers = append(r.viewers[:i], r.viewers[i+1:]...)
			r.count--
			log.Printf("Removed viewer : %v ", i)
			break
		}
	}
	r.mux.Unlock()
}

var (
	rtspStreamsMu sync.Mutex
	rtspStreams   map[*rtspStream]struct{} = make(map[*rtspStream]struct{})
)

//add rtsp stream to rtspstream list
func addRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	rtspStreams[s] = struct{}{}
	rtspStreamsMu.Unlock()
}

//remove rtsp stream from rtspstream list
func removeRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	if _, ok := rtspStreams[s]; ok {
		delete(rtspStreams, s)
	}
	rtspStreamsMu.Unlock()
}

//check if rtsp stream exists
func rtspStreamExists(id string) bool {
	ret := false
	rtspStreamsMu.Lock()
	for k := range rtspStreams {
		if k.streamid == id {
			ret = true
			break
		}
	}
	rtspStreamsMu.Unlock()
	return ret
}

//handle websocket request
func streamVideo(ws *websocket.Conn) {
	streamid := path.Base(ws.Request().URL.Path)
	log.Printf("Streamid: %v\n", streamid)
	var stream *rtspStream = nil

	//check if rtsp stream exists and start if not
	if !rtspStreamExists(streamid) {
		//go routine for ffmpeg stream from rtsp server
		go startffmpeg(streamid)
	}

	//find the correct rtsp stream
	for s := range rtspStreams {
		log.Printf("Stream id: %v\n", s.streamid)
		if s.streamid == streamid {
			log.Printf("Stream FOUND: %v\n", s.streamid)
			stream = s
			break
		}
	}

	if stream == nil {
		//correct rtspStream was not found, return
		log.Println("Rtsp stream does not exist")
		return
	}

	//create bytechannel for videostream
	viewer := mpegStream{
		bytechan: make(chan []byte),
	}

	//append viewer viewer list of current rtsp stream
	stream.appendViewer(&viewer)
	sendMagicBytes(ws, stream.width, stream.height)

	//send data to each viewer
	for b := range viewer.bytechan {
		err := websocket.Message.Send(ws, b)
		if err != nil {
			log.Println(err)
			break
		}
	}
	// remove viewer from viewer lis
	stream.removeViewer(&viewer)
	if stream.count == 0 {
		//if the list is empty stop the current rtsp stream as well (no-one is viewing the stream)
		stream.stop = true
	}
	fmt.Printf("End stream: %v count:%v\n", stream.streamid, stream.count)
}

//send magic bytes to web socket
func sendMagicBytes(ws *websocket.Conn, w int, h int) {
	log.Printf("sendMagicBytes: %v, %v", w, h)
	buf := make([]byte, 8)
	buf[0] = 'j'
	buf[1] = 's'
	buf[2] = 'm'
	buf[3] = 'p'
	copy(buf[4:], big.NewInt(int64(w)).Bytes())
	copy(buf[6:], big.NewInt(int64(h)).Bytes())
	websocket.Message.Send(ws, buf[:8])
	time.Sleep(1 * time.Second)
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
		fmt.Sprintf("ws://%s/video/%s", r.Host, deviceid[0]),
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

func startffmpeg(streamid string) {
	readAddr := fmt.Sprintf("rtsp://%s/%s", *rtspServerAddress, streamid)
	writeAddr := fmt.Sprintf("rtsp://DroneUser:22f6c4de-6144-4f6c-82ea-8afcdf19f316@%s/%s", *rtspServerAddress, streamid)

	err := sendStartVideoCommand(streamid, writeAddr)
	if err != nil {
		log.Printf("Failed to send start command: %v", err)
		return
	}

	args := []string{"-rtsp_transport", "tcp", "-i", readAddr, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("ffmpeg args: %v", args)

	//retry loop for starting ffmpeg again, if ffmpeg exits (for example if the rtsp stream does not exist yet)
	for i := 0; i < 3; i++ {
		log.Printf("ffmpeg retry: %v", i)
		cmd := exec.Command("ffmpeg", args...)
		cmdReader, _ := cmd.StdoutPipe()
		errReader, _ := cmd.StderrPipe()
		errscanner := bufio.NewScanner(errReader)
		stream := rtspStream{
			streamid: streamid,
			count:    0,
		}

		//stdout read function which streams to viewer bytechannels
		go func() {
			data := make([]byte, 16384)
			for {
				time.Sleep(5 * time.Millisecond)
				n, err := cmdReader.Read(data)
				if err != nil {
					break
				}
				if stream.count > 0 {
					for _, v := range stream.viewers {
						v.bytechan <- data[:n]
					}
				} else if stream.stop {
					//no viewers anymore, kill ffmpeg
					fmt.Printf("Kill ffmpeg\n")
					cmd.Process.Kill()
					err := sendStopVideoCommand(streamid)
					if err != nil {
						log.Printf("Failed to send start command: %v", err)
					}
				}
			}
		}()

		//stderr read function
		//parses the rtsp stream resolution and adds rtsp stream instance to rtsp stream list
		go func() {
			input := false
			for errscanner.Scan() {
				line := errscanner.Text()
				//			fmt.Printf("ErrScanner: %v\n", line)
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
							stream.width, _ = strconv.Atoi(dimstr[0])
							stream.height, _ = strconv.Atoi(dimstr[1])
							//add the instace to rtsp stream list for viewers
							addRtspStream(&stream)
							log.Printf("added stream w:%v,  h:%v\n", stream.width, stream.height)
						}
					}
				}
			}
		}()
		e := cmd.Run()
		if e != nil {
			log.Printf("Cmd err:%v", e)
		}
		//ffmpeg exited, remove rtspstream from list
		removeRtspStream(&stream)

		//do not retry if there are not any viewers anymore
		if stream.stop {
			break
		}
	}
	log.Printf("Stop streaming :%v", streamid)
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

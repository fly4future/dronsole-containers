package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/net/websocket"
	"log"
	"math/big"
	"net/http"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mpegStream struct {
	bytechan chan []byte
	id       int
}

type rtspStream struct {
	viewers  []*mpegStream
	width    int
	height   int
	streamid string
	count    int
	stop     bool
	mux      sync.Mutex
}

func (r *rtspStream) appendViewer(v *mpegStream) {
	r.mux.Lock()
	r.viewers = append(r.viewers, v)
	r.count++
	r.mux.Unlock()
}

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

func addRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	rtspStreams[s] = struct{}{}
	rtspStreamsMu.Unlock()
}

func removeRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	if _, ok := rtspStreams[s]; ok {
		delete(rtspStreams, s)
	}
	rtspStreamsMu.Unlock()
}

func rtsptompeg(ws *websocket.Conn) {
	streamid := path.Base(ws.Request().URL.Path)
	fmt.Printf("Streamid: %v\n", streamid)
	var stream *rtspStream = nil

	for s := range rtspStreams {
		log.Printf("Stream id: %v\n", s.streamid)
		if s.streamid == streamid {
			log.Printf("Stream FOUND: %v\n", s.streamid)
			stream = s
			break
		}
	}
	if stream == nil {
		log.Println("Rtsp stream does not exist")
		return
	}
	viewer := mpegStream{
		bytechan: make(chan []byte),
	}
	stream.appendViewer(&viewer)
	sendMagicBytes(ws, stream.width, stream.height)
	for b := range viewer.bytechan {
		err := websocket.Message.Send(ws, b)
		if err != nil {
			log.Println(err)
			break
		}
	}
	stream.removeViewer(&viewer)
	if stream.count == 0 {
		stream.stop = true
	}
	fmt.Printf("End stream: %v count:%v\n", stream.streamid, stream.count)
}

func videoStreamHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("begin videoStreamHandler\n%v\n", r)
	var requestBody struct {
		Address  string `json:"address"`
		StreamID string `json:"streamid"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformed request body", http.StatusBadRequest)
		return
	}
	go startffmpeg(requestBody.Address, requestBody.StreamID)
}

func startffmpeg(address string, streamid string) {
	args := []string{"-rtsp_transport", "tcp", "-i", address, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("ffmpeg args: %v", args)

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
					fmt.Printf("Kill ffmpeg\n")
					cmd.Process.Kill()
				}
			}
		}()
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
						if re.FindStringSubmatch(str) != nil {
							str = strings.ReplaceAll(str, ",", "")
							log.Printf("Found: %v\n", str)
							dimstr := strings.Split(str, "x")
							stream.width, _ = strconv.Atoi(dimstr[0])
							stream.height, _ = strconv.Atoi(dimstr[1])
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
		removeRtspStream(&stream)
		if stream.stop {
			break
		}
	}
	log.Printf("Stop streaming :%v", streamid)
}

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

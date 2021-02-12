package main

import (
	"bufio"
	"fmt"
	"log"
	"encoding/json"
	"math/big"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
	"sync"
	"path"
	"golang.org/x/net/websocket"
)

type rtspStream struct {
	bytechan chan []byte
//	wsOpen bool = false
	width int
	height int
	streamid string
	count int
	closeConnection func()
}

var (
	rtspStreamsMu sync.Mutex
	rtspStreams	  map[*rtspStream]struct{} = make(map[*rtspStream]struct{})
)

func addRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	rtspStreams[s] = struct{}{}
	rtspStreamsMu.Unlock()
}

func removeRtspStream(s *rtspStream) {
	rtspStreamsMu.Lock()
	delete(rtspStreams, s)
	rtspStreamsMu.Unlock()
}

func rtsptompeg(ws *websocket.Conn) {
	fmt.Printf("RtspTosMpeg\n")
	streamid := path.Base(ws.Request().URL.Path) 
	fmt.Printf("Streamid: %v\n", streamid)
	var stream *rtspStream = nil

	for s := range rtspStreams{
		if s.streamid == streamid{
			stream = s
			break
		}
	}
	if stream == nil{
		log.Println("Rtsp stream does not exist")
		return
	}
	stream.count++

	sendMagicBytes(ws, stream.width, stream.height)
	for b := range stream.bytechan {
//		fmt.Printf("Got bytes : %v\n", len(b))
		err := websocket.Message.Send(ws, b)
		if err != nil {
			log.Println(err)
			break
		}
	}
}

func videoStreamHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("begin videoStreamHandler\n")
	var requestBody struct {
		Address string  `json:"address"`
		StreamID string `json:"streamid"`
	}
	err := json.NewDecoder(r.Body).Decode(&requestBody)
	defer r.Body.Close()
	if err != nil {
		log.Printf("Could not decode body: %v", err)
		http.Error(w, "Malformed request body", http.StatusBadRequest)
		return
	}
	log.Printf("Rtsp address: %s", requestBody.Address)
	go startffmpeg(requestBody.Address, requestBody.StreamID)
}

func startffmpeg(address string, streamid string) {
	fmt.Printf("begin startffmpeg\n")
	args := []string{"-i", address, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("Args: %v", args)
	cmd := exec.Command("ffmpeg", args...)
	log.Printf("cmd: %v", cmd)
	cmdReader, _ := cmd.StdoutPipe()
	errReader, _ := cmd.StderrPipe()
	errscanner := bufio.NewScanner(errReader)

	stream := rtspStream{
		bytechan: make(chan []byte),
		streamid: streamid,
		count: 0,
	}

	go func() {
		data := make([]byte, 16384)
		for {
			time.Sleep(40 * time.Millisecond)
			n, err := cmdReader.Read(data)
			if err != nil {
				break
			}
			if stream.count > 0 {
				stream.bytechan <- data[:n]
			}
		}
	}()
	go func() {
		input := false
		for errscanner.Scan() {
			line := errscanner.Text()
			//
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
						fmt.Printf("Found: %v\n", str)
						dimstr := strings.Split(str, "x")
						stream.width, _ = strconv.Atoi(dimstr[0])
						stream.height, _ = strconv.Atoi(dimstr[1])
						addRtspStream(&stream)
						defer removeRtspStream(&stream)
					}
				}
			}
		}
	}()
	e := cmd.Run()
	if e != nil {
		log.Printf("Cmd err:%v", e)
	}
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

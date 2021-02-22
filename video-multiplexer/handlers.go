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

//handle websocket reques
func rtsptompeg(ws *websocket.Conn) {
	streamid := path.Base(ws.Request().URL.Path)
	fmt.Printf("Streamid: %v\n", streamid)
	var stream *rtspStream = nil

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

//handler for startvideostream request. Starts the ffmpeg stream from rtsp server.
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
	//go routine for ffmpeg stream from rtsp server 
	go startffmpeg(requestBody.Address, requestBody.StreamID)
}

func startffmpeg(address string, streamid string) {
	args := []string{"-rtsp_transport", "tcp", "-i", address, "-f", "mpegts", "-codec:v", "mpeg1video", "-"}
	log.Printf("ffmpeg args: %v", args)

	//retry loop for starting ffmpeg again, if ffmpeg exeits (for example if the rtsp stream does not exist yet)
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

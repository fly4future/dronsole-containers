# video-multiplexer container

## Building and running container

Build and tag container
```
docker build -t tii-video-multiplexer .
```

Run container in docker
```
docker run --rm -it -p 8084:8084 tii-video-multiplexer 
```

## Arguments

```bash
# Run locally against MQTT broker
video-multiplexer -rtsp 127.0.0.1:8554 -mqtt 127.0.0.1:8883

# Run against Cloud IoT Core
video-multiplexer -rtsp <video-server> -mqtt cloud
```

# Develop locally

Start drone video emulator: video-test-server
```bash
go run .

# or

ffmpeg -stream_loop -1 -i ./test/testvideo.mp4 -vcodec libx264 -tune zerolatency -crf 18 -f rtsp -muxdelay 0.1 rtsp://DroneUser:22f6c4de-6144-4f6c-82ea-8afcdf19f316@127.0.0.1:8554/some-id

```

Start video-multiplexer
```
go run . -test
```

Browse to: http://localhost:8084/test?deviceid=some-id
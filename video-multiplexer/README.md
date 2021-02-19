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

## Start encoding videostream in rtsp address :

For example start encoding videostream from rtsp as follows:
```
curl -d '{"address":"rtsp://Username:Password@localhost:8554/stream_x","streamid":"test_streamid"}' localhost:8084/startvideostream
```

In the video stream can be viewed in browser as follows:
```
<body>
	<canvas id="video-canvas"></canvas>
	<script type="text/javascript" src="jsmpeg.min.js"></script>
	<script type="text/javascript">
		var canvas = document.getElementById('video-canvas');
		var url = 'ws://localhost:8083/video/test_streamid';
		var player = new JSMpeg.Player(url, { canvas: canvas });
	</script>
</body>
```
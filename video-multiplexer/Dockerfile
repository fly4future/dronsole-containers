FROM golang:latest as builder
RUN mkdir /video-multiplexer
WORKDIR /video-multiplexer/
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY *.go ./
RUN GOOS=linux GOARCH=amd64 go build -o video-multiplexer .

FROM golang:latest
RUN apt update
RUN apt -y install ffmpeg
COPY --from=builder /video-multiplexer/video-multiplexer /bin/video-multiplexer
EXPOSE 8084
ENTRYPOINT ["/bin/video-multiplexer"]

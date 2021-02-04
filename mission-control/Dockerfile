FROM golang:latest as builder
RUN mkdir /mission-control
WORKDIR /mission-control/
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY *.go ./
RUN GOOS=linux GOARCH=amd64 go build -o mission-control .


FROM golang:latest
COPY --from=builder /mission-control/mission-control /bin/mission-control
EXPOSE 8082
ENTRYPOINT ["/bin/mission-control"]

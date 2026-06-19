FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./sensor/sensor.go ./sensor.go
RUN CGO_ENABLED=0 GOOS=linux go build -o sensor_bin sensor.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/sensor_bin .
COPY ./data/initialization ./data/initialization  

CMD ["./sensor_bin"]
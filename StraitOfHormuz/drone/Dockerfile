FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./drone/drone.go ./drone.go
RUN CGO_ENABLED=0 GOOS=linux go build -o drone_bin drone.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/drone_bin .
COPY ./data/initialization ./data/initialization
COPY ./data/images ./data/images

CMD ["./drone_bin"]
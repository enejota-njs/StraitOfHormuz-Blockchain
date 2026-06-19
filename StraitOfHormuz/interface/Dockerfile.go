FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./interface/interface.go ./interface.go

RUN CGO_ENABLED=0 GOOS=linux go build -o interface_bin interface.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/interface_bin .
COPY ./data/initialization/interface.json ./data/initialization/interface.json

CMD ["./interface_bin"]
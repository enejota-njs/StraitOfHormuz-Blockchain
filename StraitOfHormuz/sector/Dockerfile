FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./sector/*.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o sector_bin .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/sector_bin .

COPY ./data/initialization ./data/initialization  

CMD ["./sector_bin"]
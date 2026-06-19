FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY ./company/company.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o company_bin company.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/company_bin .

ENTRYPOINT ["./company_bin"]
FROM golang:1.25.0-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY . .

RUN swag init -g cmd/api/main.go -d .
RUN CGO_ENABLED=1 GOOS=linux go build -o api ./cmd/api

FROM alpine:latest

RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app

COPY --from=builder /app/api .
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./api"]
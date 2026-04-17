FROM golang:1.24.2-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o api ./cmd/api/main.go

FROM alpine:latest

RUN apk add --no-cache ca-certificates libc6-compat

WORKDIR /app

COPY --from=builder /app/api .
RUN mkdir -p /app/data

EXPOSE 8080

CMD ["./api"]
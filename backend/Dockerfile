FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/api ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM alpine:3.20 AS base
RUN apk add --no-cache ffmpeg ca-certificates tzdata
ENV TZ=Asia/Shanghai

FROM base AS api
COPY --from=builder /bin/api /app/api
EXPOSE 8080
CMD ["/app/api"]

FROM base AS worker
COPY --from=builder /bin/worker /app/worker
CMD ["/app/worker"]

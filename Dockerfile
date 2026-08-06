# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/play-music .

FROM alpine:3.21
RUN apk add --no-cache ffmpeg ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/play-music /usr/local/bin/play-music
ENV ND_PORT=4533 \
    ND_ADDRESS=0.0.0.0
EXPOSE 4533
CMD ["play-music"]

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/tgs-bot ./cmd/tgs-bot

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -H -u 10001 tgsbot
WORKDIR /srv/tgs-bot
COPY --from=build /out/tgs-bot /usr/local/bin/tgs-bot
USER tgsbot
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health || exit 1
CMD ["tgs-bot"]

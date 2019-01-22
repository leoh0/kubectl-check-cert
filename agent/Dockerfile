FROM golang:1.11-alpine AS gobuild-base
RUN apk add --no-cache \
	bash \
	build-base \
	gcc \
	git \
	libseccomp-dev \
	linux-headers \
	make

FROM gobuild-base AS krawler
WORKDIR /app
COPY go.mod go.sum /app/
RUN go mod tidy

WORKDIR /app/agent
COPY agent/* /app/agent/
RUN	CGO_ENABLED=1 go build \
        -tags "seccomp static_build" \
        -ldflags " -extldflags -static" -o krawler . && mv krawler /usr/bin/krawler

FROM alpine:3.8 AS base
COPY --from=krawler /usr/bin/krawler /usr/bin/krawler

CMD [ "sleep", "3600" ]

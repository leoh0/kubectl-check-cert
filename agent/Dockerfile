FROM golang:1.10-alpine AS gobuild-base
RUN apk add --no-cache \
	bash \
	build-base \
	gcc \
	git \
	libseccomp-dev \
	linux-headers \
	make
RUN go get -u github.com/golang/dep/...

FROM gobuild-base AS krawler
WORKDIR /go/src/github.com/leoh0/kubectl-check-cert/agent/krawler
COPY . .
RUN dep ensure
RUN	CGO_ENABLED=1 go build \
        -tags "seccomp static_build" \
        -ldflags " -extldflags -static" -o krawler . && mv krawler /usr/bin/krawler

FROM alpine:3.8 AS base
COPY --from=krawler /usr/bin/krawler /usr/bin/krawler

CMD [ "sleep", "3600" ]

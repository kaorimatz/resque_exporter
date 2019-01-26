FROM quay.io/prometheus/golang-builder as builder

COPY . $GOPATH/src/github.com/Intellection/resque-exporter
WORKDIR $GOPATH/src/github.com/Intellection/resque-exporter

RUN make PREFIX=/

FROM quay.io/prometheus/busybox
MAINTAINER Satoshi Matsumoto <kaorimatz@gmail.com>

COPY --from=builder /resque-exporter /bin/resque-exporter

EXPOSE 9447
ENTRYPOINT ["/bin/resque-exporter"]

FROM quay.io/prometheus/golang-builder as builder

COPY . $GOPATH/src/github.com/kaorimatz/resque_exporter
WORKDIR $GOPATH/src/github.com/kaorimatz/resque_exporter

RUN make PREFIX=/

FROM quay.io/prometheus/busybox
MAINTAINER Satoshi Matsumoto <kaorimatz@gmail.com>

COPY --from=builder /resque_exporter /bin/resque_exporter

EXPOSE 9447
ENTRYPOINT ["/bin/resque_exporter"]

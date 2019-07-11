FROM golang AS builder
MAINTAINER "Mingxiang Xue <mingxiangxue@gmail.com>"

RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential && \
    apt-get clean && \
    mkdir -p "$GOPATH/src/github.com/uzxmx/kubenotify"

ADD . "$GOPATH/src/github.com/uzxmx/kubenotify"

RUN cd "$GOPATH/src/github.com/uzxmx/kubenotify" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a --installsuffix cgo --ldflags="-s" -o /kubenotify

FROM bitnami/minideb:stretch
RUN install_packages ca-certificates

COPY --from=builder /kubenotify /bin/kubenotify

ENTRYPOINT ["/bin/kubenotify"]

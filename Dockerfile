FROM golang:1.6.1
COPY ./scripts/bootstrap /scripts/bootstrap
RUN /scripts/bootstrap
WORKDIR /go/src/github.com/rancher/go-machine-service

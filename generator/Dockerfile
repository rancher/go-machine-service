from golang
RUN go get github.com/Sirupsen/logrus
RUN go get github.com/ubiquityhosting/GoUbi

RUN mkdir -p /go/src/github.com/docker/
WORKDIR /go/src/github.com/docker/
RUN git clone https://github.com/rancher/machine

ENV GOPATH /go/src/github.com/docker/machine/Godeps/_workspace:/go
ENV USER root

WORKDIR /go/src/github.com/rancher/go-machine-service


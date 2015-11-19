from golang
RUN go get github.com/Sirupsen/logrus
RUN go get github.com/ubiquityhosting/GoUbi

RUN mkdir -p /go/src/github.com/docker/
WORKDIR /go/src/github.com/docker/
RUN git clone https://github.com/docker/machine && cd machine && git checkout 7e8e38e && cd .. #Pin it to commit for v0.5.1
RUN cp -R machine/vendor/* /go/src/

RUN wget https://github.com/wlan0/package-machine/releases/download/v0.1.0/docker-machine.tar.gz 
RUN tar -C /usr/local/bin -xvf docker-machine.tar.gz

ENV GOPATH /go/src/github.com/docker/machine/Godeps/_workspace:/go
ENV USER root

WORKDIR /go/src/github.com/rancher/go-machine-service


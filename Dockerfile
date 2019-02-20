FROM golang:1.11.2-alpine3.8
RUN apk add --no-cache git protobuf-dev gcc libc-dev bash \
&& go get -u google.golang.org/grpc && go get -u github.com/golang/protobuf/protoc-gen-go \
&& echo "export PATH=$PATH:$GOPATH/bin" >> /etc/profile
EXPOSE 3050
WORKDIR /root

RUN mkdir -p $GOPATH/src/github.com/wuyuanyi135/mvcamctrl &&\
git clone --recursive https://github.com/wuyuanyi135/mvcamctrl $GOPATH/src/github.com/wuyuanyi135/mvcamctrl &&\
cd $GOPATH/src/github.com/wuyuanyi135/mvcamctrl/protos/MicroVision-proto &&\
ls && \
bash ./protoc.sh && \
cd $GOPATH/src/github.com/wuyuanyi135/mvcamctrl && \
./build.sh

CMD '/go/bin/lasercontroller'

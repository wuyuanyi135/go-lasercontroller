FROM golang:1.11.2-alpine3.8
RUN apk add --no-cache git protobuf-dev gcc libc-dev \
&& go get -u google.golang.org/grpc && go get -u github.com/golang/protobuf/protoc-gen-go \
&& echo "export PATH=$PATH:$GOPATH/bin" >> /etc/profile
EXPOSE 3050
WORKDIR /root
RUN mkdir -p $GOPATH/src/github.com/wuyuanyi135/ &&\
git clone  https://github.com/wuyuanyi135/lasercontroller  $GOPATH/src/github.com/wuyuanyi135/lasercontroller&&\
cd $GOPATH/src/github.com/wuyuanyi135/lasercontroller &&\
go generate &&\
go get -d ./... &&\
go build

CMD '/go/bin/lasercontroller'
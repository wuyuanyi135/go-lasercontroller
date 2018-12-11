FROM golang:1.11.2-alpine3.8
RUN apk add --no-cache git protobuf-dev \
&& go get -u google.golang.org/grpc && go get -u github.com/golang/protobuf/protoc-gen-go \
&& echo "export PATH=$PATH:$GOPATH/bin" >> /etc/profile

WORKDIR /root
COPY . .
RUN build.sh

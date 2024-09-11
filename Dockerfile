FROM golang:1.22-alpine3.20 AS builder

ENV GO111MODULE=on GOPROXY="https://goproxy.io,direct" \
    GOSUMDB="sum.golang.google.cn"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -v -o etcdkeeper-v3 . && ls -l etcdkeeper-v3

FROM alpine:3.20

ARG TZ="Asia/Shanghai"

ENV ETCDKEEPER_X_NO_BROWSER=1

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g'  /etc/apk/repositories \
    && apk update && apk upgrade \
    && apk add tzdata ca-certificates \
    && update-ca-certificates \
    && cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo ${TZ} > /etc/timezone \
    && apk del tzdata \
    && echo "hosts: files dns" > /etc/nsswitch.conf

COPY --from=builder /app/etcdkeeper-v3 /cmd/etcdkeeper-v3
COPY config.yaml /cmd/etc/config.yaml

VOLUME [ "/cmd/etc" ]

ENTRYPOINT [ "/cmd/etcdkeeper-v3", "-c", "/cmd/etc/config.yaml" ]

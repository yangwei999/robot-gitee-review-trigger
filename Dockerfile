FROM openeuler/openeuler:23.03 as BUILDER
RUN dnf update -y && \
    dnf install -y golang && \
    go env -w GOPROXY=https://goproxy.cn,direct

MAINTAINER zengchen1024<chenzeng765@gmail.com>

# build binary
WORKDIR /go/src/github.com/opensourceways/robot-gitee-review-trigger
COPY . .
RUN GO111MODULE=on CGO_ENABLED=0 go build -a -o robot-gitee-review-trigger .

# copy binary config and utils
FROM openeuler/openeuler:22.03
RUN dnf -y update && \
    dnf in -y shadow && \
    groupadd -g 1000 review-trigger && \
    useradd -u 1000 -g review-trigger -s /bin/bash -m review-trigger

USER review-trigger

COPY  --chown=review-trigger --from=BUILDER /go/src/github.com/opensourceways/robot-gitee-review-trigger/robot-gitee-review-trigger /opt/app/robot-gitee-review-trigger

ENTRYPOINT ["/opt/app/robot-gitee-review-trigger"]

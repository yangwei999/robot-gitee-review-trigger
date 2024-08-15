FROM openeuler/openeuler:23.03 as BUILDER
ARG MERLIN_GOPROXY=https://goproxy.cn,direct
ARG GH_USER
ARG GH_TOKEN
RUN go env -w GOPROXY=${MERLIN_GOPROXY} && go env -w GOPRIVATE=github.com/opensourceways
RUN echo "machine github.com login ${GH_USER} password ${GH_TOKEN}" > $HOME/.netrc

RUN dnf update -y && \
    dnf install -y golang && \

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

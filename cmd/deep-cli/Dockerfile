FROM alpine:3.16 as certs
RUN apk --update add ca-certificates
ARG TARGETARCH
COPY bin/linux/deep-cli-${TARGETARCH} /deep-cli
ENTRYPOINT ["/deep-cli"]

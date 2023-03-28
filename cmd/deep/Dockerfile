FROM alpine:3.16 as certs
RUN apk --update add ca-certificates
ARG TARGETARCH
COPY bin/linux/deep-${TARGETARCH} /deep
ENTRYPOINT ["/deep"]

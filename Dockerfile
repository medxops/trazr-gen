FROM alpine:latest
COPY trazr-gen /trazr-gen
ENTRYPOINT ["/trazr-gen"]

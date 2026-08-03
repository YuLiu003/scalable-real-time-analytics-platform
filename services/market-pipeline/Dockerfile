ARG CERTIFICATE_IMAGE
FROM ${CERTIFICATE_IMAGE} AS certificates

FROM scratch

COPY --from=certificates /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY .build/synthetic-producer /synthetic-producer
COPY .build/alpaca-market-producer /alpaca-market-producer
COPY .build/raw-event-archiver /raw-event-archiver
COPY .build/archive-inspector /archive-inspector
COPY .build/topic-inspector /topic-inspector

USER 65532:65532
ENTRYPOINT ["/raw-event-archiver"]

FROM scratch

COPY .build/synthetic-producer /synthetic-producer
COPY .build/raw-event-archiver /raw-event-archiver
COPY .build/archive-inspector /archive-inspector
COPY .build/topic-inspector /topic-inspector

USER 65532:65532
ENTRYPOINT ["/raw-event-archiver"]

FROM scratch

COPY .build/portfolio-api /portfolio-api
COPY .build/portfolio-inspector /portfolio-inspector
COPY .build/derived-reset /derived-reset

USER 65532:65532
ENTRYPOINT ["/portfolio-api"]

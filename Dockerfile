# Build the same native workers; the runtime needs no browser, shell or shared libraries.
# Official Go 1.26.6 Alpine multi-platform index. Update the digest deliberately and validate.
FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26.6-alpine@sha256:3889b425f035be855a72fb4755265311293b6d414521f0a519d819df32222d83 AS build
WORKDIR /src
COPY . .
ARG TARGETARCH
ARG REVISION=unrecorded
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
RUN case "$REVISION" in *[!a-zA-Z0-9._-]*|'') exit 2;; esac && \
    GOOS=linux GOARCH="$TARGETARCH" go build -trimpath -buildvcs=false \
      -ldflags "-s -w -X main.commit=$REVISION" -o /clonoloom . && \
    mkdir -p /rootfs/inputs /rootfs/output /rootfs/scratch /rootfs/work

FROM scratch AS runtime
ARG REVISION=unrecorded
LABEL org.opencontainers.image.title="ClonoLoom headless" \
      org.opencontainers.image.description="Native recipe execution and exact captured-evidence replay" \
      org.opencontainers.image.source="https://github.com/robert-mcdermott/clonoloom" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.revision="$REVISION"
COPY --from=build /clonoloom /clonoloom
COPY --from=build /rootfs/ /
COPY LICENSE NOTICE /licenses/
COPY --from=build /usr/local/go/LICENSE /licenses/GO-LICENSE
COPY --from=build /usr/local/go/PATENTS /licenses/GO-PATENTS
USER 65532:65532
WORKDIR /work
ENV TMPDIR=/scratch
STOPSIGNAL SIGTERM
ENTRYPOINT ["/clonoloom"]
CMD ["run", "--help"]

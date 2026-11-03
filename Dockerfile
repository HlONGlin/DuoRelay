# syntax=docker/dockerfile:1
FROM node:22-bookworm-slim AS frontend
WORKDIR /src
COPY web/package.json web/package-lock.json ./web/
RUN cd web && npm ci
COPY DESIGN.md DESIGN-dark.md ./
COPY web/ ./web/
RUN cd web && npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist ./web/dist
ARG TARGETOS
ARG TARGETARCH
ARG DUORELAY_VARIANT=full
RUN go run ./deploy/docker/download-kernels.go -arch "$TARGETARCH" -variant "$DUORELAY_VARIANT" -out /out/kernels
RUN case "$DUORELAY_VARIANT" in \
      full) tags="" ;; \
      lite) tags="lite" ;; \
      *) echo "Unknown variant: $DUORELAY_VARIANT" >&2; exit 1 ;; \
    esac \
    && CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -tags "$tags" -o /out/duorelay ./cmd/duorelay

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 duorelay \
    && useradd --system --uid 10001 --gid duorelay --home-dir /data --no-create-home duorelay \
    && install -d -o duorelay -g duorelay -m 0700 /data
COPY --from=build /out/duorelay /usr/local/bin/duorelay
COPY --from=build /out/kernels/ /opt/duorelay/kernels/
ENV LD_LIBRARY_PATH=/opt/duorelay/kernels
USER 10001:10001
WORKDIR /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8080/api/auth/status > /dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/duorelay"]
CMD ["-listen", "0.0.0.0:8080", "-db", "/data/duorelay.db", "-config-dir", "/data/configs", "-kernels", "/opt/duorelay/kernels"]

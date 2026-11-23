# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26-bookworm AS service
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /invisi-explorer ./cmd/invisi-explorer

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /var/lib/invisi-explorer && chown 65532:65532 /var/lib/invisi-explorer
COPY --from=service /invisi-explorer /usr/local/bin/invisi-explorer
COPY --from=web /src/web/dist /opt/invisi/explorer
COPY LICENSE /licenses/invisi-explorer/LICENSE
WORKDIR /opt/invisi
USER 65532:65532
EXPOSE 18670
ENTRYPOINT ["invisi-explorer", "--dir", "/var/lib/invisi-explorer"]
CMD ["run", "--listen", "0.0.0.0:18670"]

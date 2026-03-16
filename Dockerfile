FROM golang:1.26.0 AS builder

WORKDIR /workspace

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY api ./api
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-X github.com/mwognicki/pull-secrets-operator/pkg/version.Version=${VERSION} -X github.com/mwognicki/pull-secrets-operator/pkg/version.GitCommit=${GIT_COMMIT} -X github.com/mwognicki/pull-secrets-operator/pkg/version.BuildDate=${BUILD_DATE}" \
  -o /out/manager ./cmd/manager

FROM almalinux/10-kitten-micro

ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG OCI_IMAGE_TITLE="pull-secrets-operator"
ARG OCI_IMAGE_DESCRIPTION="Kubernetes operator for replicating Docker pull secrets across namespaces."
ARG OCI_IMAGE_SOURCE="https://github.com/mwognicki/pull-secrets-operator"
ARG OCI_IMAGE_URL="https://github.com/mwognicki/pull-secrets-operator"
ARG OCI_IMAGE_DOCUMENTATION="https://github.com/mwognicki/pull-secrets-operator/blob/develop/README.md"
ARG OCI_IMAGE_VENDOR="mwognicki"
ARG OCI_IMAGE_AUTHORS="mwognicki"
ARG OCI_IMAGE_LICENSES="MIT"

LABEL org.opencontainers.image.title="${OCI_IMAGE_TITLE}" \
      org.opencontainers.image.description="${OCI_IMAGE_DESCRIPTION}" \
      org.opencontainers.image.source="${OCI_IMAGE_SOURCE}" \
      org.opencontainers.image.url="${OCI_IMAGE_URL}" \
      org.opencontainers.image.documentation="${OCI_IMAGE_DOCUMENTATION}" \
      org.opencontainers.image.vendor="${OCI_IMAGE_VENDOR}" \
      org.opencontainers.image.authors="${OCI_IMAGE_AUTHORS}" \
      org.opencontainers.image.licenses="${OCI_IMAGE_LICENSES}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /

COPY --from=builder /out/manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]

FROM golang:1.26.0 AS builder

WORKDIR /workspace

ARG VERSION=0.1.0-dev
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

WORKDIR /

COPY --from=builder /out/manager /manager

USER 65532:65532

ENTRYPOINT ["/manager"]

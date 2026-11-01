FROM docker.io/library/golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd cmd
COPY internal internal
ARG VERSION=dev
ARG COMMIT=none
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X github.com/mnorrsken/excommunicado/internal/version.Version=${VERSION} -X github.com/mnorrsken/excommunicado/internal/version.Commit=${COMMIT}" \
    -o /excommunicado ./cmd/excommunicado
RUN mkdir -p /data-dir

FROM scratch
COPY --from=build /excommunicado /excommunicado
COPY --from=build --chown=65532:65532 /data-dir /data
USER 65532:65532
EXPOSE 47823
ENTRYPOINT ["/excommunicado", "-addr", ":47823", "-data", "/data"]

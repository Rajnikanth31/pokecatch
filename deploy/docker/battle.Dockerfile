# Multi-stage build: tiny, non-root, statically linked battle server image.
# Same pattern is reused for every Go service (ARG SERVICE selects which).
FROM golang:1.22-alpine AS build
ARG SERVICE=battle
WORKDIR /src
# Cache deps first for fast incremental builds.
COPY go.mod ./
RUN go mod download
COPY . .
# CGO off => fully static binary that runs on scratch. Trimpath + ldflags strip
# debug info to shrink the image and remove local paths from panics.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
    -o /out/server ./services/${SERVICE}/cmd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /server
# Ship the creature data with the image so the server is self-contained.
COPY --from=build /src/data /data
USER nonroot:nonroot
EXPOSE 8082
ENTRYPOINT ["/server"]

# Multi-stage build: tiny, non-root, statically linked battle server image.
# Same pattern is reused for every Go service (ARG SERVICE selects which).
FROM golang:1.22-alpine AS build
ARG SERVICE=battle
WORKDIR /src
RUN apk add --no-cache git
# Copy go.mod (and go.sum if it exists — the glob makes it optional so the build
# works whether or not go.sum has been committed).
COPY go.mod go.su[m] ./
COPY . .
# -mod=mod lets the build resolve and write go.sum on the fly if it's missing, so
# a fresh clone with no committed go.sum still builds. `go mod tidy` first makes
# the dependency set consistent before compiling.
ENV GOFLAGS=-mod=mod
RUN go mod tidy
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

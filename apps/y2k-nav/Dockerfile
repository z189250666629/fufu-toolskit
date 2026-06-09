ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.22

FROM ${GO_IMAGE} AS build
WORKDIR /src/apps/y2k-nav
COPY apps/y2k-nav/go.mod ./
COPY apps/y2k-nav/main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/y2k-nav .

FROM ${RUNTIME_IMAGE}
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/y2k-nav /app/y2k-nav
COPY apps/y2k-nav/index.html ./index.html
EXPOSE 80
ENV PORT=80
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=5s \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
CMD ["/app/y2k-nav"]

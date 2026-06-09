ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.22

FROM ${GO_IMAGE} AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates gcc musl-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fufu-combine .

FROM ${RUNTIME_IMAGE}
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fufu-combine /app/fufu-combine
COPY public ./public
RUN mkdir -p /app/data
EXPOSE 3456
ENV PORT=3456
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:3456/ || exit 1
CMD ["/app/fufu-combine"]

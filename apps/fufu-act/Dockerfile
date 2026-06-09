ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.22

FROM ${GO_IMAGE} AS build
WORKDIR /src
ENV GOWORK=off
RUN apk add --no-cache ca-certificates
COPY go.work ./
COPY packages/go/fufu ./packages/go/fufu
COPY apps/fufu-act/go.mod ./apps/fufu-act/go.mod
COPY apps/fufu-act/go.sum ./apps/fufu-act/go.sum
WORKDIR /src/apps/fufu-act
RUN go mod download
WORKDIR /src
COPY apps/fufu-act ./apps/fufu-act
WORKDIR /src/apps/fufu-act
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fufu-act .

FROM ${RUNTIME_IMAGE}
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fufu-act /app/fufu-act
COPY apps/fufu-act/public ./public
RUN mkdir -p /app/data
EXPOSE 18820
ENV SLOT_PORT=18820
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:18820/ || exit 1
CMD ["/app/fufu-act"]

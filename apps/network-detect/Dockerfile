ARG GO_IMAGE=golang:1.25-alpine
ARG RUNTIME_IMAGE=alpine:3.22

FROM ${GO_IMAGE} AS build
WORKDIR /src
ENV GOWORK=off
RUN apk add --no-cache ca-certificates
COPY go.work ./
COPY packages/go/fufu ./packages/go/fufu
COPY apps/network-detect/go.mod ./apps/network-detect/go.mod
COPY apps/network-detect/go.sum ./apps/network-detect/go.sum
WORKDIR /src/apps/network-detect
RUN go mod download
WORKDIR /src
COPY apps/network-detect ./apps/network-detect
WORKDIR /src/apps/network-detect
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/network-detect .

FROM ${RUNTIME_IMAGE}
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/network-detect /app/network-detect
COPY apps/network-detect/frontend ./frontend
COPY apps/network-detect/combine ./combine
COPY apps/network-detect/newapi-managed-api-sites.json ./
COPY apps/network-detect/newapi-managed-api-sites.example.json ./
RUN mkdir -p /app/data
EXPOSE 8080
ENV PORT=8080
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
CMD ["/app/network-detect"]

ARG GO_IMAGE=golang:1.25-alpine
ARG NODE_IMAGE=node:24-alpine
ARG RUNTIME_IMAGE=alpine:3.22

FROM ${NODE_IMAGE} AS ui-build
WORKDIR /src/apps/fufu-tool-site
COPY apps/fufu-tool-site/package.json apps/fufu-tool-site/package-lock.json ./
RUN npm ci
COPY apps/fufu-tool-site/ui ./ui
RUN npm run build:ui

FROM ${GO_IMAGE} AS build
WORKDIR /src
ENV GOWORK=off
RUN apk add --no-cache ca-certificates
COPY packages/go/fufu ./packages/go/fufu
COPY apps/fufu-act/go.mod ./apps/fufu-act/go.mod
COPY apps/fufu-act/go.sum ./apps/fufu-act/go.sum
COPY apps/fufu-tool-site/go.mod ./apps/fufu-tool-site/go.mod
COPY apps/fufu-tool-site/go.sum ./apps/fufu-tool-site/go.sum
WORKDIR /src/apps/fufu-tool-site
RUN go mod download
WORKDIR /src
COPY packages/go/fufu ./packages/go/fufu
COPY apps/fufu-act ./apps/fufu-act
COPY apps/fufu-tool-site ./apps/fufu-tool-site
WORKDIR /src/apps/fufu-tool-site
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fufu-tool-site .

FROM ${RUNTIME_IMAGE}
WORKDIR /app
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/fufu-tool-site /app/fufu-tool-site
COPY apps/fufu-tool-site/frontend ./frontend
COPY apps/fufu-tool-site/combine ./combine
COPY apps/fufu-tool-site/admin ./admin
COPY --from=ui-build /src/apps/fufu-tool-site/ui-dist ./ui-dist
COPY apps/fufu-tool-site/newapi-managed-api-sites.json ./
COPY apps/fufu-tool-site/newapi-managed-api-sites.example.json ./
COPY apps/y2k-nav/index.html ./nav/index.html
COPY apps/y2k-nav/nav-ui-tokens.css ./nav/nav-ui-tokens.css
COPY apps/y2k-nav/theme.mjs ./nav/theme.mjs
COPY apps/y2k-nav/latency.mjs ./nav/latency.mjs
COPY apps/fufu-act/public ./activity/public
RUN mkdir -p /app/data /app/activity/data
EXPOSE 8080
ENV PORT=8080
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1
CMD ["/app/fufu-tool-site"]

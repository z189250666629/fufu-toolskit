ARG BASE_IMAGE=node:22-bookworm-slim
FROM ${BASE_IMAGE}

WORKDIR /app
ENV NODE_ENV=production

RUN apt-get update \
  && apt-get install -y --no-install-recommends python3 make g++ ca-certificates \
  && rm -rf /var/lib/apt/lists/*

COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force

COPY public ./public
COPY server ./server
COPY scripts ./scripts
RUN mkdir -p /app/data

EXPOSE 18820
ENV SLOT_PORT=18820
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD node -e "fetch('http://127.0.0.1:18820/').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"
CMD ["node", "server/index.mjs"]

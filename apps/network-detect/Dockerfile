ARG BASE_IMAGE=node:22-alpine
FROM ${BASE_IMAGE}

WORKDIR /app
ENV NODE_ENV=production

COPY package.json ./
COPY backend ./backend
COPY frontend ./frontend
COPY newapi-managed-api-sites.json ./
COPY newapi-managed-api-sites.example.json ./

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=10s \
  CMD node -e "fetch('http://127.0.0.1:8080/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"

CMD ["node", "backend/server.js"]

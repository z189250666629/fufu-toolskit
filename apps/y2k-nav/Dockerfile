ARG NGINX_IMAGE=nginx:1.27-alpine
FROM ${NGINX_IMAGE}

COPY index.html /usr/share/nginx/html/index.html

EXPOSE 80
HEALTHCHECK --interval=15s --timeout=5s --retries=5 --start-period=5s \
  CMD wget -q -O /dev/null http://127.0.0.1/ || exit 1
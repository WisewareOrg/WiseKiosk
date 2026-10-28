FROM node:26-alpine@sha256:aadf416b2cdce311a8811ba3f0608a61b77dbf997500e2eafe781b51f6a0b019 AS web
WORKDIR /src
COPY frontend/ frontend/
COPY boundary/ boundary/
RUN npm --prefix frontend ci
RUN cd frontend && node_modules/.bin/orval
RUN frontend/node_modules/.bin/json2ts --input frontend/src/config/schema.json --output frontend/src/config/types.ts --additionalProperties false
RUN frontend/node_modules/.bin/vite build frontend
# vite copies frontend/public/ into dist/, the gitignored local config.json with it; where a
# deployment's configuration comes from instead is ADR 0021 rev 4's and ADR 0020 rev 5's.
RUN rm -f frontend/dist/config.json

FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build
WORKDIR /src/backend
COPY backend/ ./
COPY boundary/ ../boundary/
RUN go generate ./internal/boundary
RUN CGO_ENABLED=0 go build -o /out/wisekiosk ./cmd

FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a
RUN apk upgrade --no-cache
# Why the image carries trust anchors is docs/ARCHITECTURE.md § Deployment's.
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 kiosk
COPY --from=build /out/wisekiosk /usr/local/bin/wisekiosk
COPY --from=web /src/frontend/dist /srv/kiosk
USER kiosk
EXPOSE 8080
# No --timeout: the bound the self-check applies is ADR 0020 rev 5's.
HEALTHCHECK CMD ["/usr/local/bin/wisekiosk", "-health-check"]
ENTRYPOINT ["/usr/local/bin/wisekiosk", "-static-root", "/srv/kiosk"]

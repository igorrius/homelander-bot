# syntax=docker/dockerfile:1

FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.27-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/homelander ./cmd/homelander \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/homelander /homelander
# The database directory must belong to the unprivileged user; a fresh volume
# mounted here takes these permissions.
COPY --from=go --chown=nonroot:nonroot /out/data /data
ENV DATABASE_PATH=/data/homelander.db
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/homelander"]

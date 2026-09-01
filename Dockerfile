FROM node:22-bookworm-slim AS web-build
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/pushsdk-gateway ./cmd/gateway

FROM debian:bookworm-slim
RUN useradd --system --uid 10001 --create-home --home-dir /app gateway
WORKDIR /app
COPY --from=go-build /out/pushsdk-gateway /app/pushsdk-gateway
COPY --from=web-build /build/web/dist /app/web
COPY db/migrations /app/migrations
USER gateway
EXPOSE 8080
ENTRYPOINT ["/app/pushsdk-gateway"]

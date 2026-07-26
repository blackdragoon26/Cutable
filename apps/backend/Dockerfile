FROM golang:1.26-alpine AS build

WORKDIR /src/apps/backend
COPY apps/backend/go.mod apps/backend/go.sum ./
RUN go mod download

COPY apps/backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/cutable-api \
    ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/cutable-api /cutable-api

EXPOSE 3010
USER nonroot:nonroot
ENTRYPOINT ["/cutable-api"]

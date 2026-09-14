FROM golang:1.27 AS builder
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/app /app
COPY api/openapi.yaml /api/openapi.yaml
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/app"]

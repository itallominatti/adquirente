FROM golang:1.27 AS builder
ARG SERVICE
WORKDIR /src
ENV GOFLAGS=-mod=vendor GOTOOLCHAIN=local CGO_ENABLED=0 GOOS=linux
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/app /app
COPY api/openapi.yaml /api/openapi.yaml
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/app"]

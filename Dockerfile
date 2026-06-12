FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /build/publish-policy-default ./cmd/module

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/publish-policy-default /
ENTRYPOINT ["/publish-policy-default"]

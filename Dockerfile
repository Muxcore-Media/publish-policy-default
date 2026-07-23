FROM golang:1.26-alpine AS builder
COPY core/ /build/core/
COPY publish-policy-default/ /build/publish-policy-default/
WORKDIR /build/publish-policy-default
RUN go mod download
RUN CGO_ENABLED=0 go build -o /publish-policy-default ./cmd/module

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
RUN adduser -D -h /app policy
USER policy
WORKDIR /app
COPY --from=builder /publish-policy-default .
COPY publish-policy-default/policies.yaml .
EXPOSE 9300
ENTRYPOINT ["./publish-policy-default"]

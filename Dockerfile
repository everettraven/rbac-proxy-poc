FROM golang:1.18 as builder

WORKDIR /app

COPY . ./

RUN go mod download

RUN go build -o /rbac-proxy-poc

FROM gcr.io/distroless/base-debian10

COPY --from=builder /rbac-proxy-poc /rbac-proxy-poc

ENTRYPOINT ["/rbac-proxy-poc"]
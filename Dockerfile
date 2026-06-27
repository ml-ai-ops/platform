FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go/ ./go/
RUN cd go && CGO_ENABLED=0 go build -o /mlaiops-gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /mlaiops-gateway /mlaiops-gateway
EXPOSE 8080
ENTRYPOINT ["/mlaiops-gateway"]

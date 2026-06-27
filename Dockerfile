FROM golang:1.23-alpine AS build
ARG SERVICE=gateway
WORKDIR /src
COPY go/ ./go/
RUN cd go && CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /service /service
EXPOSE 8080
ENTRYPOINT ["/service"]

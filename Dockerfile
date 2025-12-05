# syntax=docker/dockerfile:1
FROM golang:1.25.1-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o tezos-delegation-service ./cmd

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/tezos-delegation-service /app/tezos-delegation-service
COPY --from=build /app/db /app/db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/tezos-delegation-service"]

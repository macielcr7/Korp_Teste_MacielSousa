FROM golang:1.26.6 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app && \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && \
    addgroup -S inventory && adduser -S inventory -G inventory
WORKDIR /app
COPY --from=build /out/app /app/bin/app
COPY --from=build /out/migrate /app/bin/migrate
COPY --from=build /src/migrations /app/migrations

USER inventory
EXPOSE 8081 8083

CMD ["/app/bin/app"]

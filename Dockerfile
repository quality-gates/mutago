# Runtime image: docker build -t mutago . && docker run --rm -v "$PWD":/code -w /code mutago ./...
FROM golang:alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/mutago ./cmd/mutago

FROM golang:alpine
RUN apk add --no-cache git diffutils ca-certificates
COPY --from=build /app/mutago /usr/local/bin/mutago
WORKDIR /code
ENTRYPOINT ["mutago"]
CMD ["--help"]

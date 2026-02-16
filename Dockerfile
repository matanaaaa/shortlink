FROM golang:1.25 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /
COPY --from=build /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/core ./cmd/core

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/core /core
EXPOSE 8080
ENTRYPOINT ["/core"]

FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /atlas ./cmd/atlas
FROM alpine:3.21
WORKDIR /app
COPY --from=build /atlas /atlas
COPY --from=build /app/db ./db
EXPOSE 8080
CMD ["/atlas"]

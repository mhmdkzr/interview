FROM golang:1.24.0-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o interview .


FROM debian:bookworm-slim

WORKDIR /app
COPY --from=builder /app/interview .

RUN useradd -m appuser && chown -R appuser:appuser /app
USER appuser

EXPOSE 8080
CMD ["./interview"]
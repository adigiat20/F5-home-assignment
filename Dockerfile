FROM golang:1.22-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go test ./...

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o redactor .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

RUN addgroup -g 101 -S appgroup && adduser -u 100 -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/redactor .
COPY rules.yaml .

RUN chmod +x /app/redactor && chown -R 100:101 /app

USER 100:101

EXPOSE 8080

ENTRYPOINT ["./redactor"]
CMD ["--serve", "--addr", ":8080", "--rules", "rules.yaml"]
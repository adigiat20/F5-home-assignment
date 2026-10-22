FROM golang:1.22-alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

COPY go.mod go.su[m] ./
RUN go mod download


COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o redactor .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/redactor .

EXPOSE 8080

ENTRYPOINT ["./redactor"]
CMD ["--serve", "--addr", ":8080", "--rules", "rules.yaml"]
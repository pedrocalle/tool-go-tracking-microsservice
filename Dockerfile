# Etapa 1: build da aplicação
FROM golang:1.20 AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

# Compila para Linux com arquitetura compatível
RUN GOOS=linux GOARCH=amd64 go build -o main .

# Etapa 2: imagem final
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main .

# Confirma que o binário é executável
RUN chmod +x main

EXPOSE 8080

CMD ["./main"]

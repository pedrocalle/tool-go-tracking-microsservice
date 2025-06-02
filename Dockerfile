# Etapa 1: build da aplicação
FROM golang:1.20 AS builder

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./

# Binário estático compatível com Alpine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main .

# Etapa 2: imagem final leve com Alpine
FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/main .

# Marca como executável
RUN chmod +x ./main

EXPOSE 8080

CMD ["./main"]

# Usa imagem oficial do Go
FROM golang:1.25

WORKDIR /app

# Copia o código
COPY . .

# Compila o cliente
RUN go build -o client

# Garante permissão de execução no binário
RUN chmod +x ./client

# Comando para iniciar o cliente
CMD ["./client"]

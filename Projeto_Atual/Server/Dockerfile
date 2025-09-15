# Usa imagem oficial do Go
FROM golang:1.25

WORKDIR /app

# Copia o código
COPY . .

# Compila o servidor
RUN go build -o server

# Garante permissão de execução no binário
RUN chmod +x ./server

# Expõe a porta usada no servidor
EXPOSE 8080

# Comando para iniciar o servidor
CMD ["./server"]

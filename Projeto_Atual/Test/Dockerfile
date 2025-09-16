# Usa imagem oficial do Go
FROM golang:1.25

WORKDIR /app

# Copia o código
COPY . .

# Compila o teste/bot
RUN go build -o test

# Garante permissão de execução no binário
RUN chmod +x ./test

# Comando para iniciar o teste.
CMD ["./stress-tester", "-clientes=10", "-cenario=chaos", "-duracao=30s"]
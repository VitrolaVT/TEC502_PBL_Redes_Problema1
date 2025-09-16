# Batalha de Tanques - Jogo de Cartas em Go

> Projeto desenvolvido para a disciplina TEC502 - Concorrência e Conectividade na UEFS. Problema 1: Desenvolvimento de jogo de cartas.

Este repositório contém o código-fonte de um jogo de cartas multiplayer "Batalha de Tanques", desenvolvido em Go. O projeto utiliza comunicação via TCP para as ações do jogo (pareamento, chat, batalhas) e UDP para medição de latência (ping/pong).

O sistema é composto por três componentes principais:

* **Servidor (`/Server`):** O backend que gerencia as conexões, estado dos jogadores, pareamentos e a lógica das batalhas.
* **Cliente (`/Client`):** Uma aplicação de console interativa que permite ao jogador se conectar ao servidor e jogar.
* **Teste de Estresse (`/Test`):** Um script automatizado para simular múltiplos jogadores e testar a performance e robustez do servidor sob carga.

O projeto é totalmente containerizado usando Docker e Docker Compose, facilitando a execução de todos os componentes de forma isolada e consistente.

## Como Executar

Existem duas maneiras de executar o projeto: localmente, rodando os arquivos Go diretamente, ou utilizando Docker e Docker Compose para uma execução mais prática e isolada.

### 1. Executando Localmente (Sem Docker)

Este método é útil para desenvolvimento rápido e prático caso não tenha condições de rodar o docker no computador. 
OBS: Você precisará ter o Go instalado em sua máquina.

**Passo 1: Ajustar o Endereço do Servidor no Cliente**

Como o cliente e o servidor rodarão na mesma máquina, o cliente precisa se conectar a `localhost`. Antes de executar, você deve alterar dois endereços no código do cliente:

* **Arquivo a ser modificado:** `Client/client.go`

* **Alteração 1 (Conexão TCP):**
    ```go
    // Mude de:
    conn, err := net.Dial("tcp", "server:8080")
    // Para:
    conn, err := net.Dial("tcp", "localhost:8080")
    ```

* **Alteração 2 (Conexão UDP para Latência):**
    ```go
    // Mude de:
    iniciarLoopDeLatencia("server:8081", reader)
    // Para:
    iniciarLoopDeLatencia("localhost:8081", reader)
    ```

**Passo 2: Iniciar o Servidor**

Abra um terminal, navegue até a pasta do servidor e execute o seguinte comando:

```bash
cd Server
go run server.go
```
Você verá as mensagens de log indicando que os servidores TCP e UDP estão rodando.

**Passo 3: Iniciar o Cliente**

Abra um **novo terminal**, navegue até a pasta do cliente e execute o mesmo comando. Você pode iniciar quantos clientes quiser, cada um em seu próprio terminal.

```bash
cd Client
go run client.go
```
Agora você pode interagir com o jogo através do terminal do cliente.

### 2. Executando com Docker e Docker Compose
Este é o método recomendado, pois ele gerencia todas as dependências e redes automaticamente. Você só precisa ter o Docker e o Docker Compose instalados.

> **Importante:** Para a execução com Docker, o endereço do servidor no código do cliente **deve ser** `"server:8080"` e `"server:8081"`, pois `server` é o nome do serviço definido no `docker-compose.yml`, e o Docker o resolverá para o IP correto do container do servidor.

#### Nota para Usuários Windows: Instalação do Docker
Para executar este projeto com Docker no Windows, você primeiro precisa instalar o **Docker Desktop**. Ele é a ferramenta oficial que contém tudo o que é necessário.

* **Link para Instalação:** Você pode baixá-lo e ver os pré-requisitos (como a necessidade do WSL 2) no site oficial da Docker:
    * **[Instalar Docker Desktop no Windows](https://docs.docker.com/desktop/install/windows-install/)**

Após a instalação, abra seu terminal de preferência (**PowerShell** ou **Windows Terminal** são recomendados) na pasta do projeto e **siga exatamente os mesmos comandos** descritos abaixo. Os comandos `docker-compose build`, `docker-compose up`, etc., são universais e funcionam da mesma forma no Windows, Linux e macOS.

---

**Passo 1: Construir as Imagens**

Na raiz do projeto (onde está o arquivo `docker-compose.yml`), execute o comando para construir as imagens Docker para todos os serviços (servidor, cliente e teste). Você só precisa fazer isso na primeira vez ou caso quiser alterar o código.

```bash
docker-compose build
```

**Passo 2: Iniciar o Servidor e o Cliente**

Para jogar o jogo, você pode iniciar o servidor e um ou mais clientes.

```bash
# Inicia o servidor 
# A flag --build é opcional se você já construiu as imagens.
docker-compose up --build server 
```
Se você quiser rodar um segundo cliente, abra um novo terminal e use o comando `docker-compose run`:

```bash
# Executa um novo container para um cliente
docker-compose run --rm client
```

### 3. Executando os Testes de Estresse
Os testes de estresse são executados usando o serviço `test` definido no `docker-compose.yml`. Eles simulam múltiplos bots conectando e interagindo com o servidor para testar sua performance.

**Passo 1: Iniciar o Servidor em Background**

Primeiro, inicie apenas o serviço do servidor.

```bash
docker-compose up server
```

**Passo 2: Executar um Cenário de Teste**

Em outro terminal, use `docker-compose run` para executar o serviço de teste. Você pode passar diferentes parâmetros para personalizar o teste.

O comando base é: `docker-compose run --rm test ./test [parâmetros]`
Os parâmetros são: 
* cenario: Descreve qual tipo de teste será, podendo ser *logins*, *pacotes*, *batalhas*, e *geral*
* duracao: Duração do teste
* clientes: Número de bots para o teste 

**Exemplos de Cenários:**

* **Cenário geral com 50 bots por 30 segundos:**
    ```bash
    docker-compose run --rm test ./test -cenario=geral -clientes=50 -duracao=30s
    ```
* **Cenário focado em Batalhas com 20 bots por 1 minuto:**
    ```bash
    docker-compose run --rm test ./test -cenario=batalhas -clientes=20 -duracao=1m
    ```
* **Cenário focado em Abrir Pacotes com 100 bots por 20 segundos:**
    ```bash
    docker-compose run --rm test ./test -cenario=pacotes -clientes=100 -duracao=20s
    ```

**Passo 4: Encerrar o Ambiente**

Quando terminar todos os testes, use o comando abaixo para parar o container do servidor e remover a rede criada pelo Docker Compose.

```bash
docker-compose down
```

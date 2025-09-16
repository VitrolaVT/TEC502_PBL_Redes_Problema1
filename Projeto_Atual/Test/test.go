package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Estruturas de dados no fluxo de dados do cliente e servidor
type Requisicao struct {
	Tipo            string `json:"tipo"`
	Id_remetente    string `json:"id_remetente"`
	Id_destinatario string `json:"id_destinatario"`
	Mensagem        string `json:"mensagem"`
	Carta           Tanque `json:"carta"`
}

type Resposta struct {
	Tipo     string   `json:"tipo"`
	Mensagem string   `json:"mensagem"`
	Cartas   []Tanque `json:"cartas"`
}

type Tanque struct {
	Modelo     string `json:"modelo"`
	Id_jogador string `json:"id_jogador"`
	Vida       int    `json:"vida"`
	Ataque     int    `json:"ataque"`
}

// =============================================================================
// ESTRUTURAS DE CONTROLE DO TESTE
// =============================================================================

// Struct para guardar informações de um bot/cliente simulado.
type Bot struct {
	id         int
	serverID   string
	conn       net.Conn
	opponentID string
	deck       []Tanque
}

// Mapa global para que os bots possam se encontrar para parear.
var (
	botStates = make(map[string]*Bot)
	mu        sync.Mutex
)

// Contadores para o relatório final do teste.
var (
	botsSucedidos int32
	botsFalharam  int32
)

func main() {
	// Parâmetros para configurar o teste via linha de comando.
	numClientes := flag.Int("clientes", 50, "Número de clientes simultâneos a simular.")
	duracaoTeste := flag.Duration("duracao", 30*time.Second, "Duração total do teste.")
	cenario := flag.String("cenario", "chaos", "Cenário de teste a ser executado: logins, packs, battles, chaos.")
	flag.Parse()

	fmt.Printf("Iniciando teste de estresse com %d clientes por %v no cenário '%s'.\n", *numClientes, *duracaoTeste, *cenario)
	fmt.Println("Certifique-se de que o servidor está rodando.")

	var wg sync.WaitGroup

	// Garante que o número de clientes seja par para o cenário de batalha.
	if *cenario == "battles" && *numClientes%2 != 0 {
		*numClientes--
		fmt.Printf("Cenário 'battles' requer um número par de clientes. Ajustando para %d.\n", *numClientes)
	}

	// Laço para criar e rodar os bots em goroutines.
	for i := 0; i < *numClientes; i++ {
		wg.Add(1)
		go func(botID int) {
			defer wg.Done()
			runBot(botID, *cenario)
		}(i)
	}

	// Timer para controlar a duração total do teste.
	fmt.Printf("Teste em andamento... Pressione CTRL+C para parar antes ou aguarde %v.\n", *duracaoTeste)
	time.Sleep(*duracaoTeste)

	fmt.Println("\n=============================================")
	fmt.Println("Teste de Estresse Concluído.")
	fmt.Printf("Bots Sucedidos: %d\n", atomic.LoadInt32(&botsSucedidos))
	fmt.Printf("Bots Falharam: %d\n", atomic.LoadInt32(&botsFalharam))
	fmt.Println("=============================================")
}

// =============================================================================
// LÓGICA DE EXECUÇÃO DOS BOTS
// =============================================================================

// Função principal que executa a lógica de um bot individual.
func runBot(botID int, cenario string) {
	conn, err := net.DialTimeout("tcp", "localhost:8080", 5*time.Second)
	if err != nil {
		fmt.Printf("[Bot %d] Falha ao conectar: %v\n", botID, err)
		atomic.AddInt32(&botsFalharam, 1)
		return
	}
	defer conn.Close()

	bot := &Bot{
		id:   botID,
		conn: conn,
		deck: gerarDeckInicial(fmt.Sprintf("bot_%d", botID)),
	}

	// Goroutine para escutar continuamente as respostas do servidor para este bot.
	resChan := make(chan Resposta)
	errChan := make(chan error)
	go func() {
		for {
			res, err := lerResposta(bot.conn)
			if err != nil {
				errChan <- err
				return
			}
			resChan <- res
		}
	}()

	// Primeiro passo: aguardar o servidor enviar o ID de conexão.
	select {
	case res := <-resChan:
		if res.Tipo == "Criaçao_Id" {
			bot.serverID = res.Mensagem
			mu.Lock()
			botStates[bot.serverID] = bot
			mu.Unlock()
		}
	case <-time.After(5 * time.Second):
		fmt.Printf("[Bot %d] Timeout: Não recebeu ID do servidor.\n", bot.id)
		atomic.AddInt32(&botsFalharam, 1)
		return
	case err := <-errChan:
		fmt.Printf("[Bot %d] Erro ao esperar por ID: %v\n", bot.id, err)
		atomic.AddInt32(&botsFalharam, 1)
		return
	}

	// Executa a lógica do bot baseada no cenário escolhido.
	var success bool
	switch cenario {
	case "logins":
		success = cenarioLogin(bot)
	case "packs":
		success = cenarioPacks(bot, resChan, errChan)
	case "battles":
		success = cenarioBattle(bot, resChan, errChan)
	case "chaos":
		success = cenarioChaos(bot, resChan, errChan)
	default:
		fmt.Printf("Cenário desconhecido: %s\n", cenario)
		success = false
	}

	if success {
		atomic.AddInt32(&botsSucedidos, 1)
	} else {
		atomic.AddInt32(&botsFalharam, 1)
	}
}

// Cenário de Login: Bot apenas conecta e fica ocioso.
func cenarioLogin(bot *Bot) bool {
	fmt.Printf("[Bot %d | ID %s] Login bem-sucedido. Ficando ocioso.\n", bot.id, bot.serverID)
	time.Sleep(15 * time.Second) // Simula um jogador que fica parado no menu.
	return true
}

// Cenário de Pacotes: Bot conecta e solicita a abertura de pacotes.
func cenarioPacks(bot *Bot, resChan <-chan Resposta, errChan <-chan error) bool {
	fmt.Printf("[Bot %d | ID %s] Iniciando cenário de abrir pacotes.\n", bot.id, bot.serverID)
	for i := 0; i < 5; i++ { // Tenta abrir 5 pacotes.
		enviarRequisicao(bot.conn, Requisicao{Tipo: "Abrir_Pacote", Id_remetente: bot.serverID})
		time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond) // Espera um tempo aleatório.
	}
	return true
}

// Cenário de Batalha: Bots são criados em pares para batalhar.
func cenarioBattle(bot *Bot, resChan <-chan Resposta, errChan <-chan error) bool {
	// Lógica de pareamento.
	// Bots com ID par (0, 2, 4...) serão os responsáveis por iniciar o pareamento.
	if bot.id%2 == 0 {
		time.Sleep(1 * time.Second) // Dá um tempo para o bot oponente se registrar no mapa.
		mu.Lock()
		for _, b := range botStates {
			if b.id == bot.id+1 {
				bot.opponentID = b.serverID
				break
			}
		}
		mu.Unlock()

		if bot.opponentID == "" {
			return false // Falha se não encontrou oponente.
		}

		enviarRequisicao(bot.conn, Requisicao{Tipo: "Parear", Id_remetente: bot.serverID, Id_destinatario: bot.opponentID})
	}

	// Loop para tratar os eventos recebidos durante a batalha.
	for {
		select {
		case res := <-resChan:
			switch res.Tipo {
			case "Pareamento":
				fmt.Printf("[Bot %d] Pareado com sucesso!\n", bot.id)
				// Apenas o bot par (iniciador) envia a requisição para batalhar.
				if bot.id%2 == 0 {
					enviarRequisicao(bot.conn, Requisicao{Tipo: "Batalhar", Id_remetente: bot.serverID, Id_destinatario: bot.opponentID})
				}
			case "Inicio_Batalha":
				fmt.Printf("[Bot %d] Batalha iniciada!\n", bot.id)
			case "Enviar_Próxima_Carta":
				indice, _ := strconv.Atoi(res.Mensagem)
				if indice < len(bot.deck) {
					carta := bot.deck[indice]
					enviarRequisicao(bot.conn, Requisicao{Tipo: "Próxima_Carta", Id_remetente: bot.serverID, Carta: carta})
				}
			case "Fim_Batalha":
				fmt.Printf("[Bot %d] Batalha finalizada. %s\n", bot.id, res.Mensagem)
				return true // Sucesso!
			}
		case <-time.After(45 * time.Second): // Timeout para evitar que a batalha prenda o bot para sempre.
			fmt.Printf("[Bot %d] Timeout no cenário de batalha.\n", bot.id)
			return false
		case err := <-errChan:
			fmt.Printf("[Bot %d] Conexão perdida na batalha: %v\n", bot.id, err)
			return false
		}
	}
}

// Cenário de Caos: Mistura todos os cenários e adiciona desconexões aleatórias.
func cenarioChaos(bot *Bot, resChan <-chan Resposta, errChan <-chan error) bool {
	// Sorteia uma chance de desconectar logo após o login.
	if rand.Intn(10) == 0 {
		fmt.Printf("[Bot Caos %d] Desconectando aleatoriamente!\n", bot.id)
		return true // Consideramos sucesso, pois testou a resiliência do servidor à desconexão.
	}

	// Sorteia uma "personalidade" para o bot.
	personalidade := rand.Intn(3)
	switch personalidade {
	case 0: // Personalidade "Ocioso".
		return cenarioLogin(bot)
	case 1: // Personalidade "Abre Pacotes".
		return cenarioPacks(bot, resChan, errChan)
	case 2: // Personalidade "Batalhador".
		// Apenas bots pares tentam batalhar para formar pares.
		if bot.id%2 == 0 {
			return cenarioBattle(bot, resChan, errChan)
		}
		// Se o bot for ímpar, ele age como "Ocioso" neste cenário.
		return cenarioLogin(bot)
	}
	return true
}

// =============================================================================
// FUNÇÕES AUXILIARES
// =============================================================================

func enviarRequisicao(conn net.Conn, req Requisicao) {
	if conn == nil {
		return
	}
	reqBytes, _ := json.Marshal(req)
	_, _ = conn.Write(append(reqBytes, '\n'))
}

func lerResposta(conn net.Conn) (Resposta, error) {
	reader := bufio.NewReader(conn)
	// Timeout de leitura para evitar que um bot fique preso para sempre se o servidor parar de responder.
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	msg, err := reader.ReadBytes('\n')
	if err != nil {
		return Resposta{}, err
	}
	var res Resposta
	_ = json.Unmarshal(msg, &res)
	return res, nil
}

// Função para gerar um deck simples para cada bot.
func gerarDeckInicial(idJogador string) []Tanque {
	deck := make([]Tanque, 5)
	for i := 0; i < 5; i++ {
		deck[i] = Tanque{
			Modelo:     fmt.Sprintf("BotTank-T%d", i+1),
			Id_jogador: idJogador,
			Vida:       100 + rand.Intn(50),
			Ataque:     20 + rand.Intn(10),
		}
	}
	return deck
}

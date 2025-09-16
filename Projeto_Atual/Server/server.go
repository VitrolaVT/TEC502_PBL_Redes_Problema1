package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Struct como modelo de requisição do cliente para servidor
type Requisicao struct {
	Tipo            string `json:"tipo"`
	Id_remetente    string `json:"id_remetente"`
	Id_destinatario string `json:"id_destinatario"`
	Mensagem        string `json:"mensagem"`
	Carta           Tanque `json:"carta"`
}

// Struct modelo de resposta do servidor para cliente
type Resposta struct {
	Tipo     string   `json:"tipo"`
	Mensagem string   `json:"mensagem"`
	Cartas   []Tanque `json:"cartas"`
}

// Carta do jogo
type Tanque struct {
	Modelo     string `json:"modelo"`
	Id_jogador string `json:"id_jogador"`
	Vida       int    `json:"vida"`
	Ataque     int    `json:"ataque"`
}

// Struct para dados de uma batalha
type Batalha struct {
	Jogador1         string
	Jogador2         string
	Canal1           chan Tanque
	Canal2           chan Tanque
	Encerramento     chan bool
	EncerramentoOnce sync.Once
}

// Variáveis do server
var (
	clientes      = make(map[string]net.Conn) //Map para guardar conexões através dos IDs
	muClientes    sync.RWMutex                //Mutex para sincronização dos jogadores
	pares         = make(map[string]string)   //Pares de jogadores conectados
	muPares       sync.RWMutex                //Mutex para sincronização de jogadores pareados
	idCounter     int                         //Contador do ID
	pacoteCounter = 10                        //Contador de pacotes disponíveis
	muPacote      sync.Mutex                  //Mutex para sincronização do estoques
	batalhas      = make(map[string]*Batalha) //Map para guardar batalhas em andamento
	muBatalhas    sync.RWMutex                //Mutex para sincronizar as batalhas
)

// Pacote 1 de cartas
var pacote_1 = []Tanque{
	//5 leves
	{"Coyote (Light)", "server", 50, 10},
	{"Falcon (Light)", "server", 55, 12},
	{"Wolf (Light)", "server", 60, 15},
	{"Fox (Light)", "server", 52, 11},
	{"Hawk (Light)", "server", 58, 14},

	//3 médios
	{"Rhino (Medium)", "server", 100, 25},
	{"Jaguar (Medium)", "server", 110, 27},
	{"Panther (Medium)", "server", 120, 30},

	//2 pesados
	{"Mammoth (Heavy)", "server", 200, 50},
	{"Titan (Heavy)", "server", 220, 55},
}

// Constantes dos estados possíveis para uma batalha
const (
	EstadoEsperandoCarta = iota
	EstadoRealizandoTurno
)

func main() {
	//Criação de porta
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		color.Red("Erro na criação da porta")
		panic(err)
	}
	color.Green("Servidor rodando na porta 8080")

	//Ouvir constantemente requisições de conexão
	for {
		conn, err := ln.Accept()
		if err != nil {
			color.Red("Erro na aceitação da conexão")
			continue
		}
		//Goroutine para cada conexão/jogador
		go criarConexao(conn)
	}
}

// Função para criar a conexão e garantir transferência de informações cliente/servidor
func criarConexao(conn net.Conn) {
	defer conn.Close()

	//Criar id e guardar no map
	muClientes.Lock()
	idCounter++
	id_cliente := fmt.Sprintf("%d", idCounter)
	clientes[id_cliente] = conn
	muClientes.Unlock()
	color.Cyan("Jogador conectado! ID = %s", id_cliente)

	resposta := Resposta{Tipo: "Criaçao_Id", Mensagem: id_cliente}
	enviarResposta(conn, resposta)

	//Ler constantemente coisas enviados pelo outro lado da conexão
	reader := bufio.NewReader(conn)
	for {
		msg, err := reader.ReadBytes('\n')
		if err != nil {
			tratarDesconexao(id_cliente)
			return
		}

		var requisicao Requisicao
		err = json.Unmarshal(msg, &requisicao) //Deserializar requisição do json
		if err != nil {
			resposta.Tipo = "Erro"
			resposta.Mensagem = "Erro no recebimento do json"
			enviarResposta(conn, resposta)
		}

		//Decodificar o tipo da requisição
		switch requisicao.Tipo {
		case "Parear":
			parearClientes(conn, requisicao.Id_remetente, requisicao.Id_destinatario)

		case "Mensagem":
			transmitirMensagem(conn, requisicao.Id_remetente, requisicao.Id_destinatario, requisicao.Mensagem)

		case "Abrir_Pacote":
			sortearCartas(conn, requisicao.Id_remetente)

		case "Batalhar":
			batalha := Batalha{
				Jogador1:     requisicao.Id_remetente,
				Jogador2:     requisicao.Id_destinatario,
				Canal1:       make(chan Tanque),
				Canal2:       make(chan Tanque),
				Encerramento: make(chan bool),
			}
			muBatalhas.Lock()
			batalhas[requisicao.Id_remetente] = &batalha
			batalhas[requisicao.Id_destinatario] = &batalha
			muBatalhas.Unlock()

			go realizarBatalha(&batalha)

		case "Próxima_Carta":
			muBatalhas.RLock()
			batalha, existe := batalhas[requisicao.Id_remetente]
			muBatalhas.RUnlock()

			if !existe {
				resposta.Tipo = "Erro"
				resposta.Mensagem = "Você não está em uma batalha"
				enviarResposta(conn, resposta)
			} else {
				//Verifica qual jogador é para mandar no canal correto
				if requisicao.Id_remetente == batalha.Jogador1 {
					select {
					case batalha.Canal1 <- requisicao.Carta:
						//Apenas envia
					default:
						color.Red("Canal1 cheio ou encerrado para %s", batalha.Jogador1)
						batalha.Encerramento <- true
					}
				} else if requisicao.Id_remetente == batalha.Jogador2 {
					select {
					case batalha.Canal2 <- requisicao.Carta:
						//Apenas envia
					default:
						color.Red("Canal2 cheio ou encerrado para %s", batalha.Jogador2)
						select {
						case batalha.Encerramento <- true:
						default:
						}
					}
				}
			}

		default:
			resposta.Tipo = "Erro"
			resposta.Mensagem = "Comando inválido"
			enviarResposta(conn, resposta)
		}
	}
}

// Função para enviar resposta serializada em formato json para o cliente
func enviarResposta(conn net.Conn, resposta Resposta) {
	resposta_json, _ := json.Marshal(resposta)
	conn.Write(append(resposta_json, '\n'))
}

// Função para parear 2 jodadores
func parearClientes(conn net.Conn, id_remetente, id_destinatario string) {
	var resposta Resposta

	if id_remetente == id_destinatario {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Id destinatário não pode ser igual ao Id remetente"
		enviarResposta(conn, resposta)
		return
	} else if _, existe := clientes[id_destinatario]; !existe {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Id destinatário não existe"
		enviarResposta(conn, resposta)
		return
	} else if _, existe := pares[id_remetente]; existe {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Já existe um pareamento existente para o remetente"
		enviarResposta(conn, resposta)
		return
	} else if _, existe := pares[id_destinatario]; existe {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Já existe um pareamento existente para o destinatário"
		enviarResposta(conn, resposta)
		return
	}

	//Bloquear acesso da variáveis de pares e jogadores para sincronização
	muPares.Lock()
	pares[id_remetente] = id_destinatario
	pares[id_destinatario] = id_remetente
	muPares.Unlock()

	resposta.Tipo = "Pareamento"
	resposta.Mensagem = id_destinatario
	enviarResposta(conn, resposta)

	//Garantir leitura sincronizada entre goroutines que também lêem a variável
	muClientes.RLock()
	resposta.Mensagem = id_remetente
	enviarResposta(clientes[id_destinatario], resposta)
	muClientes.RUnlock()
}

// Função para mandar mensagem de um jogador para o outro pareado
func transmitirMensagem(conn net.Conn, id_remetente, idDestinatario, mensagem string) {
	//Garantir leitura sincronizada entre goroutines que também lêem a variável
	muPares.RLock()
	idPar, existe := pares[id_remetente]
	muPares.RUnlock()

	var resposta Resposta
	if idPar != idDestinatario || !existe {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Id do destinatário difente da conexão existente ou não existe conexão"
		enviarResposta(conn, resposta)
		return
	}

	//Garantir leitura sincronizada entre goroutines que também lêem a variável
	muClientes.RLock()
	idParConn := clientes[idPar]
	muClientes.RUnlock()

	resposta.Tipo = "Mensagem"
	resposta.Mensagem = mensagem
	enviarResposta(idParConn, resposta)
}

// Função de sortear cartas do pacote escolhido
func sortearCartas(conn net.Conn, id string) {
	//Cria um gerador aleatório independente usando tempo da chamada da função
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	//Bloquear acesso ao contador de pacotes disponíveis
	muPacote.Lock()
	defer muPacote.Unlock()

	var resposta Resposta
	if pacoteCounter <= 0 {
		resposta.Tipo = "Erro"
		resposta.Mensagem = "Não há mais pacotes disponíveis"
		enviarResposta(conn, resposta)
		return
	}

	pacoteCounter--

	//Sorteia 5 índices usando o gerador independente
	n := len(pacote_1)
	indices := r.Perm(n)[:5]

	cartasSorteadas := make([]Tanque, 0, 5)

	for _, i := range indices {
		cartasSorteadas = append(cartasSorteadas, pacote_1[i])
	}

	for i := range cartasSorteadas { //Trocar ID para o do jogador
		cartasSorteadas[i].Id_jogador = id
	}

	resposta.Tipo = "Sorteio"
	resposta.Mensagem = "Sorteio realizado com sucesso"
	resposta.Cartas = cartasSorteadas

	enviarResposta(conn, resposta)
}

// Função para tratar desconexão de jogador
func tratarDesconexao(idDesconectado string) {
	//Atualizar lista e jogadores conectados
	muClientes.Lock()
	if conn, ok := clientes[idDesconectado]; ok {
		conn.Close()
		delete(clientes, idDesconectado)
	}
	muClientes.Unlock()

	//Atualizar lista de jogadores pareados se necessário
	muPares.Lock()
	if idPar, ok := pares[idDesconectado]; ok {
		muClientes.RLock()
		conn2 := clientes[idPar]
		resposta := Resposta{Tipo: "Desconexão", Mensagem: "Jogador desconectou"}
		enviarResposta(conn2, resposta)
		muClientes.RUnlock()

		delete(pares, idDesconectado)
		delete(pares, idPar)
	}
	muPares.Unlock()

	//Atualizar lista batalhas se existirem
	muBatalhas.Lock()
	if batalhaExistente, ok := batalhas[idDesconectado]; ok {
		select {
		case batalhaExistente.Encerramento <- true:
		default:
		}
		delete(batalhas, idDesconectado)
		delete(batalhas, batalhaExistente.Jogador2)
	}
	muBatalhas.Unlock()

	color.Magenta("Jogador %s desconectou", idDesconectado)
}

// Função para realizar partida/batalha entre jogadores
func realizarBatalha(batalha *Batalha) {
	color.Yellow("Iniciando batalha entre %s e %s", batalha.Jogador1, batalha.Jogador2)

	//Pegar conexão de cada jogador para não dar RLock e RUnlock várias vezes
	muClientes.RLock()
	connJogador1 := clientes[batalha.Jogador1]
	connJogador2 := clientes[batalha.Jogador2]
	muClientes.RUnlock()

	//Envia início da batalha para os jogadores
	resposta := Resposta{Tipo: "Inicio_Batalha", Mensagem: batalha.Jogador2}
	enviarResposta(connJogador1, resposta) //Jogador 1

	resposta.Mensagem = batalha.Jogador1
	enviarResposta(connJogador2, resposta) //Jogador 2

	time.Sleep(2 * time.Second)

	//Estado inicial de partida
	turno := 0
	indice1, indice2 := 0, 0
	var carta1, carta2 *Tanque
	var ok1, ok2 bool
	estadoAtual := EstadoEsperandoCarta
	var cartasTurno []Tanque

	for {
		select {
		//Canal para caso ocorra uma desconexão de um jogador
		case <-batalha.Encerramento:
			color.Red("Batalha encerrada à força!")
			encerrarBatalha(batalha, batalha.Jogador1, batalha.Jogador2, "desconexão/força")
			return
		default:
		}

		switch estadoAtual {
		case EstadoEsperandoCarta:
			if carta1 == nil {
				resposta.Tipo = "Enviar_Próxima_Carta"
				resposta.Mensagem = fmt.Sprintf("%d", indice1)
				enviarResposta(connJogador1, resposta)
				color.Magenta("Aqui 1")
				carta1, ok1 = esperarCarta(batalha.Canal1, 30*time.Second)
				color.Magenta("Pedir nova carta jogador 1: %d", indice1)
			} else {
				ok1 = true
			}

			if carta2 == nil {
				resposta.Tipo = "Enviar_Próxima_Carta"
				resposta.Mensagem = fmt.Sprintf("%d", indice2)
				enviarResposta(connJogador2, resposta)
				color.Magenta("Aqui 2")
				carta2, ok2 = esperarCarta(batalha.Canal2, 30*time.Second)
				color.Magenta("Pedir nova carta jogador 2: %d", indice2)
			} else {
				ok2 = true
			}

			if !ok1 || !ok2 {
				encerrarBatalha(batalha, batalha.Jogador1, batalha.Jogador2, "timeout")
				return
			}

			estadoAtual = EstadoRealizandoTurno
		case EstadoRealizandoTurno:
			cartasTurno = nil
			if turno%2 == 0 { //Jogador1 ataca
				carta2.Vida -= carta1.Ataque
				resposta.Mensagem = fmt.Sprintf("Jogador 1 jogou no turno %d", turno)
			} else { //Jogador2 ataca
				carta1.Vida -= carta2.Ataque
				resposta.Mensagem = fmt.Sprintf("Jogador 2 jogou no turno %d", turno)
			}

			//Enviar resultado do turno
			cartasTurno = append(cartasTurno, *carta1, *carta2)
			resposta.Tipo = "Turno_Realizado"
			resposta.Cartas = cartasTurno
			enviarResposta(connJogador1, resposta)
			enviarResposta(connJogador2, resposta)

			//Verificar vida das cartas dos jogadores e preparar para próxima carta
			carta1Destruida := carta1.Vida <= 0
			carta2Destruida := carta2.Vida <= 0

			if carta1Destruida {
				carta1 = nil
				indice1++
				estadoAtual = EstadoEsperandoCarta
				color.Magenta("Pedir nova carta jogador 1: %d", indice1)
			}
			if carta2Destruida {
				carta2 = nil
				indice2++
				estadoAtual = EstadoEsperandoCarta
				color.Magenta("Pedir nova carta jogador 2: %d", indice2)
			}

			//Voltar para o estado de esperar carta
			estadoAtual = EstadoEsperandoCarta

			//Checa se acabou o deck de batalha de um jogador
			if indice1 >= 5 {
				encerrarBatalha(batalha, batalha.Jogador2, batalha.Jogador1, "sem cartas restantes")
				return
			}
			if indice2 >= 5 {
				encerrarBatalha(batalha, batalha.Jogador1, batalha.Jogador2, "sem cartas restantes")
				return
			}

			turno++
		}
	}
}

// Função de timeout para espera de carta
func esperarCarta(canal chan Tanque, tempo time.Duration) (*Tanque, bool) {
	timeout := time.After(tempo)
	select {
	case c := <-canal:
		return &c, true
	case <-timeout:
		return nil, false
	}
}

// Função centralizada para finalizar corretamente uma batalha(fechar canais e atualizar map)
func encerrarBatalha(batalha *Batalha, vencedor, perdedor string, motivo string) {
	//Remover batalha do map
	muBatalhas.Lock()
	delete(batalhas, batalha.Jogador1)
	delete(batalhas, batalha.Jogador2)
	muBatalhas.Unlock()

	//Notificar para as conexões existentes a mensagem e fim de partida
	var resposta Resposta
	resposta.Tipo = "Fim_Batalha"
	resposta.Mensagem = fmt.Sprintf("Batalha encerrada! %s venceu (%s).", vencedor, motivo)

	muClientes.RLock()
	if conn1, ok := clientes[batalha.Jogador1]; ok {
		enviarResposta(conn1, resposta)
	}
	if conn2, ok := clientes[batalha.Jogador2]; ok {
		enviarResposta(conn2, resposta)
	}
	muClientes.RUnlock()

	//Fechar canais com segurança
	batalha.EncerramentoOnce.Do(func() {
		close(batalha.Canal1)
		close(batalha.Canal2)
		close(batalha.Encerramento)
	})
}

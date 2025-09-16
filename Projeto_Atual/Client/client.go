package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
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

// Estados da máquina de estados
const (
	EstadoLivre = iota
	EstadoPareado
	EstadoEsperandoResposta
	Estado_Batalhando
)

// Variável IP do server
var ipServer string = "172.16.103.1"

// Variáveis para informações pertinentes ao jogador
var idPessoal, idParceiro string
var minhasCartas []Tanque

func main() {
	//Conexão do tipo TCP com o servidor
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	//Estado atual do jogador
	var estadoAtual int
	estadoAtual = EstadoEsperandoResposta

	//Lista para guardar deck de batalha de uma possível batalha
	deckBatalha := make([]Tanque, 0, 5)

	idPessoal = "none"
	idParceiro = "none"
	//Goroutine (thread) para ouvir respostas do servidor
	go func() {
		for {
			resposta := lerResposta(conn)
			switch resposta.Tipo {
			case "Erro":
				color.Red("Erro: %s", resposta.Mensagem)
				if idParceiro == "none" {
					estadoAtual = EstadoLivre
				} else {
					estadoAtual = EstadoPareado
				}

			case "Desconexão":
				color.Yellow("Parece que seu jogador pareado desconectou :(")
				estadoAtual = EstadoLivre
				idParceiro = "none"

			case "Criaçao_Id":
				color.Yellow("Seu ID é %s", resposta.Mensagem)
				idPessoal = resposta.Mensagem
				estadoAtual = EstadoLivre

			case "Pareamento":
				color.Green("Pareamento realizado com %s", resposta.Mensagem)
				idParceiro = resposta.Mensagem
				estadoAtual = EstadoPareado

			case "Mensagem":
				color.Cyan("Mensagem recebida: %s", resposta.Mensagem)

			case "Sorteio":
				minhasCartas = append(minhasCartas, resposta.Cartas...)

				color.Green(resposta.Tipo, resposta.Mensagem)
				imprimirTanques(resposta.Cartas)

			case "Inicio_Batalha":
				color.Yellow("Batalha iniciada com %s", resposta.Mensagem)
				deckBatalha = nil
				if len(minhasCartas) > 0 {
					deckBatalha = append(deckBatalha, sortearDeck()...)
				} else {
					//Inicializa deck de batalha com cartas inoperantes se não abriu um pacote
					for i := 0; i < 5; i++ {
						deckBatalha = append(deckBatalha, Tanque{Modelo: "Treinamento", Id_jogador: idPessoal, Vida: 1 + i, Ataque: 1})
					}
				}

				color.Cyan("Seu deck de batalha é:")
				imprimirTanques(deckBatalha)
				estadoAtual = Estado_Batalhando

			case "Fim_Batalha":
				color.Yellow("Batalha finalizada!")
				color.Cyan(resposta.Mensagem)
				estadoAtual = EstadoPareado

			case "Enviar_Próxima_Carta":
				indice, err := strconv.Atoi(resposta.Mensagem)
				color.Red("Indice %d", indice)

				if err != nil {
					fmt.Println("Erro ao converter:", err)
					panic(err)
				}

				//Verificar se indice é válido
				if indice < 0 || indice >= len(deckBatalha) {
					color.Red("ERRO: Índice %d fora do range do deck (0-%d)", indice, len(deckBatalha)-1)
					//Enviar carta padrão para não travar
					enviarRequisicao(conn, Requisicao{
						Tipo:            "Próxima_Carta",
						Id_remetente:    idPessoal,
						Id_destinatario: idParceiro,
						Mensagem:        "Carta",
						Carta:           Tanque{Modelo: "Padrão", Vida: 10, Ataque: 1}})
				} else {
					enviarRequisicao(conn, Requisicao{
						Tipo:            "Próxima_Carta",
						Id_remetente:    idPessoal,
						Id_destinatario: idParceiro,
						Mensagem:        "Carta",
						Carta:           deckBatalha[indice]})
				}

			case "Turno_Realizado":
				color.Yellow("Turno Realizado!")
				color.Yellow(resposta.Mensagem)
				imprimirTanques(resposta.Cartas)

			default:
				color.Red("Resposta recebida com tipo desconhecido: ", resposta.Tipo)
				os.Exit(0)
			}
		}
	}()

	//Loop infinito e centralizado que lê do terminal
	reader := bufio.NewReader(os.Stdin)
	for {

		switch estadoAtual {
		case EstadoLivre:
			fmt.Println("Comando Parear <id> / Abrir / sair: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "sair" {
				os.Exit(0)
			}

			if strings.HasPrefix(line, "Parear ") {
				idDestinatario := strings.TrimPrefix(line, "Parear ")
				enviarRequisicao(conn, Requisicao{Tipo: "Parear", Id_remetente: idPessoal, Id_destinatario: idDestinatario, Mensagem: "None"})
				estadoAtual = EstadoEsperandoResposta
			} else if strings.HasPrefix(line, "Abrir ") {
				enviarRequisicao(conn, Requisicao{Tipo: "Abrir_Pacote", Id_remetente: idPessoal, Id_destinatario: "None", Mensagem: "None"})
			} else {
				color.Red("Comando inválido")
			}

		case EstadoPareado:
			fmt.Println("Comando Abrir / Mensagem / Batalhar / sair: ")
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(line)

			if line == "sair" {
				os.Exit(0)
			}

			if strings.HasPrefix(line, "Abrir ") {
				enviarRequisicao(conn, Requisicao{Tipo: "Abrir_Pacote", Id_remetente: idPessoal, Id_destinatario: "None", Mensagem: "None"})
			} else if strings.HasPrefix(line, "Batalhar ") {
				if len(minhasCartas) < 5 {
					color.Red("Você não tem cartas suficientes para montar um deck")
				} else {
					enviarRequisicao(conn, Requisicao{Tipo: "Batalhar", Id_remetente: idPessoal, Id_destinatario: idParceiro, Mensagem: "None"})
					estadoAtual = EstadoEsperandoResposta
				}
			} else if strings.HasPrefix(line, "Mensagem ") {
				mensagem := strings.TrimPrefix(line, "Mensagem ")
				enviarRequisicao(conn, Requisicao{Tipo: "Mensagem", Id_remetente: idPessoal, Id_destinatario: idParceiro, Mensagem: mensagem})

			} else {
				color.Red("Comando inválido")
			}

		case EstadoEsperandoResposta:
			color.Yellow("Esperando resposta do server")
			time.Sleep(1 * time.Second)

		case Estado_Batalhando:
			color.Yellow("Batalha ocorrendo!!")
			time.Sleep(5 * time.Second)

		default:
			color.Red("Estado indefinido")
		}
	}
}

// Função para enviar requisição através de um pacote formato json via conexão TCP
func enviarRequisicao(conn net.Conn, requisicao Requisicao) {
	requisicao_json, _ := json.Marshal(requisicao)
	conn.Write(append(requisicao_json, '\n'))
}

// Função para ler da conexão uma resposta do servidor e transformar de volta em struct
func lerResposta(conn net.Conn) Resposta {
	reader := bufio.NewReader(conn)
	mensagem, _ := reader.ReadBytes('\n')
	var resposta Resposta
	json.Unmarshal(mensagem, &resposta)
	return resposta
}

// Função para sortear 5 cartas da coleção de cartas do jogador
func sortearDeck() []Tanque {
	//Cria um gerador aleatório independente usando tempo da chamada da função
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	//Sorteia 5 índices usando o gerador independente
	n := len(minhasCartas)
	indices := r.Perm(n)[:5]

	deck := make([]Tanque, 0, 5)
	for _, i := range indices {
		deck = append(deck, minhasCartas[i])
	}

	return deck
}

// Função para imprimir a lista de tanques/cartas
func imprimirTanques(lista []Tanque) {
	for i, t := range lista {
		fmt.Printf("Tanque %d:\n", i+1)
		fmt.Printf("  Modelo: %s\n", t.Modelo)
		color.Yellow("  Jogador: %s", t.Id_jogador)
		color.Green("  Vida: %d", t.Vida)
		color.Red("  Ataque: %d", t.Ataque)
	}
}

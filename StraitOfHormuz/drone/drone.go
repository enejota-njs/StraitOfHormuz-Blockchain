package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Estrutura que representa uma Requisição no sistema
type Request struct {
	AttendingDroneID int     `json:"attending_drone_id"` // Identificador do Drone que está atendendo
	Clock            int     `json:"clock"`              // Relógio lógico associado à Requisição
	ID               int     `json:"origin_id"`          // Identificador da Requisição (origem)
	IsCritical       bool    `json:"is_critical"`        // Indica se a Requisição é Crítica
	SectorID         int     `json:"sector_id"`          // Identificador do Setor de origem
	Status           string  `json:"status"`             // Estado atual da Requisição (ex.: PENDING, ATTENDING, DONE)
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Estrutura que representa um Drone
type Drone struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação entre Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação com Setores
	ID               int     `json:"id"`                 // Identificador do Drone
	IsBusy           bool    `json:"is_busy"`            // Indica se o Drone está ocupado
	IsOn             bool    `json:"is_on"`              // Indica se o Drone está ligado
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Estrutura utilizada como Mensagem de comunicação entre processos.
type Message struct {
	Clock    int       `json:"clock"`    // Relógio lógico da Mensagem
	Drone    Drone     `json:"drone"`    // Dados de Drone
	Request  Request   `json:"request"`  // Requisição individual
	Requests []Request `json:"requests"` // Lista de Requisições
	Text     string    `json:"text"`     // Tipo da Mensagem
}

// Estrutura que representa um Setor e seus limites no mapa.
type Sector struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação com Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação entre Setores
	AddressForSensor string  `json:"address_for_sensor"` // Endereço para comunicação com Sensores
	Bottom           float64 `json:"bottom"`             // Limite inferior
	ID               int     `json:"ID"`                 // Identificador do Setor
	Left             float64 `json:"left"`               // Limite esquerdo
	Right            float64 `json:"right"`              // Limite direito
	Top              float64 `json:"top"`                // Limite superior
}

// Variáveis globais utilizadas para manter o estado local do processo
var (
	clock    int        // Relógio lógico local
	drone    Drone      // Drone atual
	drones   []Drone    // Lista de Drones conhecidos
	mu       sync.Mutex // Exclusão mútua para acesso concorrente ao estado
	requests []Request  // Fila/Lista local de Requisições
	sectors  []Sector   // Lista de Setores conhecidos
)

// == CLOCK

// incrementClock: Incrementa o Relógio lógico local e retorna o novo valor
func incrementClock() int {
	clock++
	return clock
}

// updateClock: Atualiza o Relógio local com base no valor recebido (se necessário) e incrementa para registrar o evento atual
func updateClock(receivedClock int) int {
	if receivedClock > clock {
		clock = receivedClock
	}

	incrementClock()

	return clock
}

// == REQUEST

// addRequestToQueue adiciona uma Requisição na fila local, evitando duplicatas e mantendo ordenação por prioridade e desempates
func addRequestToQueue(request Request) {
	for _, r := range requests {
		// Evita inserir a mesma Requisição duas vezes, usando (SectorID, ID) como chave
		if r.SectorID == request.SectorID && r.ID == request.ID {
			return
		}
	}

	index := len(requests) // Posição padrão é no fim, caso não exista item com prioridade maior

	for i, r := range requests {
		// Requisições críticas entram antes das não críticas
		if request.IsCritical && !r.IsCritical {
			index = i
			break
		}

		// Requisições não críticas não passam na frente de críticas
		if !request.IsCritical && r.IsCritical {
			continue
		}

		// Entre requisições com mesma prioridade, ordena por Clock menor primeiro
		if request.Clock < r.Clock {
			index = i
			break
		}

		// Desempate: se Clock igual, menor SectorID primeiro
		if request.Clock == r.Clock && request.SectorID < r.SectorID {
			index = i
			break
		}

		// Desempate final: se Clock e SectorID iguais, menor ID primeiro
		if request.Clock == r.Clock &&
			request.SectorID == r.SectorID &&
			request.ID < r.ID {
			index = i
			break
		}
	}

	// Insere na posição calculada sem perder os elementos existentes
	requests = append(requests[:index], append([]Request{request}, requests[index:]...)...)

	fmt.Println("\nFila atual:\n")
	for i, r := range requests {
		fmt.Println(" ", i, "->",
			"Sector:", r.SectorID,
			"ID:", r.ID,
			"Status:", r.Status,
			"Critical:", r.IsCritical,
			"Clock:", r.Clock,
		)
	}
}

// syncRequestsFromSectors consulta todos os Setores conhecidos, coleta suas filas e mescla na fila local, ajustando o Relógio lógico
func syncRequestsFromSectors() {
	mu.Lock()
	clockValue := incrementClock()
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	message := Message{
		Text:  "SYNC_REQUESTS",
		Clock: clockValue,
	}

	for _, s := range currentSectors {
		// Conecta no endereço do Setor com timeout para não travar a execução
		conn, err := net.DialTimeout("tcp", s.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se conectar com setor: ", err)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		// Solicita a lista de Requisições do Setor
		if err := encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		// Lê a resposta do Setor
		var response Message
		if err := decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		// Atualiza o Relógio local com base no Relógio recebido
		updateClock(response.Clock)

		// Mescla as Requisições recebidas na fila local, mantendo ordenação
		for _, r := range response.Requests {
			addRequestToQueue(r)
		}

		mu.Unlock()

		_ = conn.Close()
	}
}

// markRequestAsPending Atualiza uma Requisição para PENDING no estado local e informa a Interface
func markRequestAsPending(request Request) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		// Identificação por (ID, SectorID)
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0
			break
		}
	}

	// Mantém o objeto enviado para a Interface coerente com o estado local
	request.Status = "PENDING"
	request.AttendingDroneID = 0

	go sendRequestToInterface("data/initialization/interface.json", request)
}

// markRequestAsAttending Marca uma Requisição como ATTENDING e atualiza o Drone responsável
func markRequestAsAttending(request Request, attendingDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	for i := range requests {
		if requests[i].ID == request.ID && requests[i].SectorID == request.SectorID {
			requests[i].Status = "ATTENDING"
			requests[i].AttendingDroneID = attendingDrone.ID
			break
		}
	}

	// Atualiza o estado do Drone que assumiu a Requisição
	if drone.ID == attendingDrone.ID {
		drone.IsBusy = true
	} else {
		for i := range drones {
			if drones[i].ID == attendingDrone.ID {
				drones[i].IsBusy = true
				drones[i].IsOn = true
				drones[i].X = attendingDrone.X
				drones[i].Y = attendingDrone.Y
				break
			}
		}
	}

	request.Status = "ATTENDING"
	go sendRequestToInterface("data/initialization/interface.json", request)
}

// removeRequestDone Remove a Requisição finalizada da fila local e libera o Drone que concluiu
func removeRequestDone(request Request, finishedDrone Drone) {
	mu.Lock()
	defer mu.Unlock()

	var filtered []Request

	// Filtra a Requisição concluída, mantendo as demais
	for _, r := range requests {
		if r.ID == request.ID && r.SectorID == request.SectorID {
			continue
		}

		filtered = append(filtered, r)
	}

	requests = filtered

	// Atualiza o estado do Drone finalizador no contexto local
	if drone.ID == finishedDrone.ID {
		drone.IsBusy = false
		drone.X = finishedDrone.X
		drone.Y = finishedDrone.Y
	}

	for i := range drones {
		if drones[i].ID == finishedDrone.ID {
			drones[i].IsBusy = false
			drones[i].X = finishedDrone.X
			drones[i].Y = finishedDrone.Y

			break
		}
	}

	request.Status = "DONE"
	go sendRequestToInterface("data/initialization/interface.json", request)
}

// dispatchRequests Sincroniza Requisições com Setores e decide qual Drone atende cada Requisição PENDING
func dispatchRequests() {
	for {
		syncRequestsFromSectors()

		mu.Lock()

		currentDrone := drone
		currentRequests := append([]Request(nil), requests...)
		currentDrones := append([]Drone(nil), drones...)

		mu.Unlock()

		// Só despacha quando este Drone está apto a trabalhar
		if currentDrone.IsOn && !currentDrone.IsBusy {
			for _, r := range currentRequests {
				// Só considera Requisições ainda não atendidas
				if r.Status != "PENDING" {
					continue
				}

				closer := currentDrone
				distance := calculateDistance(currentDrone, r)

				for _, d := range currentDrones {
					// Ignora Drones indisponíveis
					if !d.IsOn || d.IsBusy {
						continue
					}

					// Escolhe o Drone mais próximo, com desempate por ID
					tempDistance := calculateDistance(d, r)

					if tempDistance < distance {
						distance = tempDistance
						closer = d
					}

					if tempDistance == distance && d.ID < closer.ID {
						closer = d
					}
				}

				fmt.Println(
					"\nDrone escolhido",
					"| Drone: ", closer.ID,
					"| Request: ", r.ID,
					"| Sector: ", r.SectorID,
				)

				// Se este processo for o vencedor, executa o atendimento completo
				if closer.ID == currentDrone.ID {
					markRequestAsAttending(r, currentDrone)

					warnDrones("ATTENDING", r)
					warnSectors("ATTENDING", r)

					fulfillRequest(r)

					mu.Lock()
					updatedDrone := drone
					mu.Unlock()

					removeRequestDone(r, updatedDrone)

					warnDrones("DONE", r)
					warnSectors("DONE", r)

					// Processa uma Requisição por vez por ciclo
					break
				}
			}
		}

		time.Sleep(1 * time.Second)
	}
}

// == DRONE

// handleDroneCrash Marca um Drone como desligado e libera as Requisições que ele estava atendendo
func handleDroneCrash(crashedDroneID int) {
	mu.Lock()
	defer mu.Unlock()

	droneCrashed := false
	var deadDrone Drone

	for i := range drones {
		if drones[i].ID == crashedDroneID && drones[i].IsOn {
			drones[i].IsOn = false
			drones[i].IsBusy = false
			droneCrashed = true
			deadDrone = drones[i]
			break
		}
	}

	if !droneCrashed {
		return
	}

	go sendDeadDroneToInterface("data/initialization/interface.json", deadDrone)

	for i := range requests {
		// Se a Requisição estava vinculada ao Drone que caiu, ela volta para PENDING
		if requests[i].Status == "ATTENDING" && requests[i].AttendingDroneID == crashedDroneID {
			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0

			pendingRequest := requests[i]

			go sendRequestToInterface(
				"data/initialization/interface.json",
				pendingRequest,
			)

			// Notifica os demais componentes para convergirem o estado
			go warnDrones("PENDING", pendingRequest)
			go warnSectors("PENDING", pendingRequest)
		}
	}
}

// monitorDrones Faz checagens periódicas e trata Drone como “caído” quando não responde
func monitorDrones() {
	for {
		mu.Lock()
		currentDrones := append([]Drone(nil), drones...)
		mu.Unlock()

		for _, d := range currentDrones {
			// Evita checar o próprio Drone
			if d.IsOn && d.ID != drone.ID {
				conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
				if err != nil {
					fmt.Println("Drone não respondeu: ", d.ID)
					handleDroneCrash(d.ID)
				} else {
					_ = conn.Close()
				}
			}
		}
		time.Sleep(3 * time.Second)
	}
}

// calculateDistance Calcula a distância euclidiana entre o Drone e uma Requisição
func calculateDistance(d Drone, r Request) float64 {
	distance := math.Sqrt(
		math.Pow(d.X-r.X, 2) +
			math.Pow(d.Y-r.Y, 2),
	)

	return distance
}

// fulfillRequest Simula o deslocamento do Drone até a posição da Requisição e o tempo de atendimento
func fulfillRequest(request Request) {
	speed := 5.0
	delay := 100 * time.Millisecond

	for {
		mu.Lock()
		dx := request.X - drone.X
		dy := request.Y - drone.Y
		distance := math.Sqrt(dx*dx + dy*dy)

		// Chegada ao destino dentro de um passo de movimento
		if distance <= speed {
			drone.X = request.X
			drone.Y = request.Y
			mu.Unlock()
			break
		}

		// Movimento proporcional na direção do alvo
		drone.X += (dx / distance) * speed
		drone.Y += (dy / distance) * speed
		mu.Unlock()

		time.Sleep(delay)
	}

	// Tempo fixo de “atendimento” após chegar ao local
	time.Sleep(10 * time.Second)
}

// warnDrones Envia uma notificação para todos os Drones e ajusta o Relógio lógico com as respostas
func warnDrones(text string, request Request) {
	mu.Lock()
	clockValue := incrementClock()
	currentDrones := append([]Drone(nil), drones...)
	currentDrone := drone
	mu.Unlock()

	message := Message{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
		Clock:   clockValue,
	}

	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com Drone ID: ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Aguarda confirmação para atualizar o Relógio local
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		_ = conn.Close()
	}
}

// handleDrones Processa mensagens recebidas na porta de comunicação entre Drones e aplica atualizações locais
func handleDrones(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message
	if err := decoder.Decode(&message); err != nil {
		return
	}

	// Mensagem ATTENDING registra que algum Drone assumiu a Requisição
	if message.Text == "ATTENDING" {
		fmt.Printf("\nAviso de ATTENDING recebido -> DroneID: %d | SectorID: %d | RequestID: %d\n", message.Drone.ID, message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		markRequestAsAttending(message.Request, message.Drone)

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}

	// Mensagem DONE remove a Requisição e libera o Drone finalizador
	if message.Text == "DONE" {
		fmt.Printf("\nAviso de DONE recebido -> DroneID: %d | SectorID: %d | RequestID: %d\n", message.Drone.ID, message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		removeRequestDone(message.Request, message.Drone)

		_ = encoder.Encode(Message{
			Text:  "REMOVED",
			Clock: currentClock,
		})
	}

	// Mensagem PENDING reabre a Requisição, normalmente por falha de atendimento
	if message.Text == "PENDING" {
		fmt.Printf("\nAviso de PENDING recebido -> SectorID: %d | RequestID: %d\n", message.Request.SectorID, message.Request.ID)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		mu.Unlock()

		markRequestAsPending(message.Request)

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}
}

// listenDrones Inicia o servidor TCP do Drone para receber avisos e atualizações de outros Drones
func listenDrones() {
	_, port, _ := net.SplitHostPort(drone.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones: ", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleDrones(conn)
	}
}

// == SECTOR

// warnSectors Notifica todos os Setores sobre uma mudança de estado de Requisição e sincroniza o Relógio lógico
func warnSectors(text string, request Request) {
	mu.Lock()
	clockValue := incrementClock()
	currentDrone := drone
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	message := Message{
		Text:    text,
		Request: request,
		Drone:   currentDrone,
		Clock:   clockValue,
	}

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForDrone, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com Setor ID: ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Aguarda confirmação para atualizar o Relógio local
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		_ = conn.Close()
	}
}

// handleSector Processa conexões vindas de Setores e enfileira novas Requisições recebidas
func handleSector(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if err := decoder.Decode(&message); err != nil {
		return
	}

	switch message.Text {
	case "REQUEST":
		request := message.Request

		fmt.Printf("\nRequisição recebida -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", request.SectorID, request.ID, request.X, request.Y, request.IsCritical, request.Clock)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		addRequestToQueue(request)
		mu.Unlock()

		// Confirma ao Setor que a Requisição foi enfileirada
		_ = encoder.Encode(Message{
			Text:  "QUEUED",
			Clock: currentClock,
		})
	}
}

// listenSectors Inicia o servidor TCP do Drone para receber Requisições encaminhadas por Setores
func listenSectors() {
	_, port, _ := net.SplitHostPort(drone.AddressForSector)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos setores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSector(conn)
	}
}

// == LOAD DATA

// loadSectors Carrega a lista de Setores a partir de um arquivo JSON e atualiza o estado global
func loadSectors(path string) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos setores: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	sectors = config

	return nil
}

// loadDrones Carrega a lista de Drones, inicializa valores padrão e separa o Drone atual dos demais
func loadDrones(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos drones: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Drone
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	var filtered []Drone
	for _, d := range config {
		// Normaliza estado inicial para evitar lixo de configuração
		d.X = 0
		d.Y = 0
		d.IsBusy = false

		// Identifica o Drone deste processo e o marca como ativo
		if d.ID == myID {
			d.IsOn = true
			drone = d
			continue
		}

		// Mantém os demais como conhecidos, porém desligados até prova em contrário
		d.IsOn = false
		filtered = append(filtered, d)
	}

	drones = filtered

	return nil
}

// == SAVE DATA

// sendDroneToInterface Envia periodicamente o estado do Drone atual para o servidor de Interface
func sendDroneToInterface(path string) {
	mu.Lock()
	currentDrone := drone
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
		return
	}
	defer func() {
		_ = file.Close()
	}()

	var config []struct {
		Sectors string `json:"sectors"`
		Drones  string `json:"drones"`
		Sensors string `json:"sensors"`
	}

	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	for {
		// Tenta até conseguir conectar na Interface, para não perder o envio inicial
		conn, err := net.DialTimeout("tcp", config[0].Drones, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se conectar com servidor da interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err := json.NewEncoder(conn).Encode(currentDrone); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

// sendRequestToInterface Envia uma Requisição para o servidor da Interface
func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
		return
	}
	defer file.Close()

	var config []struct {
		Sectors  string `json:"sectors"`
		Drones   string `json:"drones"`
		Sensors  string `json:"sensors"`
		Requests string `json:"requests"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Requests, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao se conectar com servidor da interface: ", err)
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(request)
}

// sendDeadDroneToInterface Notifica a Interface sobre um Drone considerado desligado
func sendDeadDroneToInterface(path string, deadDrone Drone) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo da interface: ", err)
		return
	}
	defer file.Close()

	var config []struct {
		Drones string `json:"drones"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return
	}

	conn, err := net.DialTimeout("tcp", config[0].Drones, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao se comunicar com servidor da interface: ", err)
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(deadDrone)
}

// sendDroneLoop Mantém um loop de envio do estado do Drone para a Interface em intervalos fixos
func sendDroneLoop(path string) {
	for {
		sendDroneToInterface(path)
		time.Sleep(500 * time.Millisecond)
	}
}

// == MAIN

// main Inicializa o Drone a partir do ID informado, carrega configurações e inicia as rotinas principais do processo
func main() {
	if len(os.Args) < 2 {
		return
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return
	}

	dronesPath := "data/initialization/drones.json"
	sectorsPath := "data/initialization/sectors.json"
	intefacePath := "data/initialization/interface.json"

	// Carrega a configuração do Drone atual e dos demais Drones conhecidos
	if loadDrones(dronesPath, id) != nil {
		return
	}
	// Carrega os Setores conhecidos para comunicação e sincronização
	if loadSectors(sectorsPath) != nil {
		return
	}

	// Inicia os servidores de comunicação e as rotinas de coordenação
	go listenDrones()
	go listenSectors()
	go dispatchRequests()
	go monitorDrones()
	go sendDroneLoop(intefacePath)

	// Mantém o processo ativo indefinidamente
	select {}
}

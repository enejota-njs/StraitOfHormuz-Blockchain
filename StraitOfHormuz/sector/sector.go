package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Representa um Drone e seus dados de comunicação e estado
type Drone struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação entre Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação com Setores
	ID               int     `json:"id"`                 // Identificador do Drone
	IsBusy           bool    `json:"is_busy"`            // Indica se o Drone está ocupado
	IsOn             bool    `json:"is_on"`              // Indica se o Drone está ligado
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Representa uma Requisição gerada e processada no sistema
type Request struct {
	AttendingDroneID int     `json:"attending_drone_id"` // Identificador do Drone que está atendendo
	Clock            int     `json:"clock"`              // Relógio lógico associado à Requisição
	ID               int     `json:"origin_id"`          // Identificador da Requisição na origem
	IsCritical       bool    `json:"is_critical"`        // Indica se a Requisição é Crítica
	SectorID         int     `json:"sector_id"`          // Identificador do Setor de origem
	Status           string  `json:"status"`             // Estado atual da Requisição
	X                float64 `json:"x"`                  // Coordenada X
	Y                float64 `json:"y"`                  // Coordenada Y
}

// Representa um Setor do mapa e suas portas de comunicação
type Sector struct {
	AddressForDrone  string  `json:"address_for_drone"`  // Endereço para comunicação com Drones
	AddressForSector string  `json:"address_for_sector"` // Endereço para comunicação entre Setores
	AddressForSensor string  `json:"address_for_sensor"` // Endereço para comunicação com Sensores
	Bottom           float64 `json:"bottom"`             // Limite inferior
	ID               int     `json:"id"`                 // Identificador do Setor
	Left             float64 `json:"left"`               // Limite esquerdo
	Right            float64 `json:"right"`              // Limite direito
	Top              float64 `json:"top"`                // Limite superior
}

// Representa um Sensor e suas características e estado de ativação
type Sensor struct {
	ID         int     `json:"id"`          // Identificador do Sensor
	IsActive   bool    `json:"is_active"`   // Indica se o Sensor está ativo
	IsCritical bool    `json:"is_critical"` // Indica se o Sensor gera Requisição Crítica
	Type       string  `json:"type"`        // Tipo do Sensor
	X          float64 `json:"x"`           // Coordenada X
	Y          float64 `json:"y"`           // Coordenada Y
}

type Message struct {
	Clock      int       `json:"clock"`
	Drone      Drone     `json:"drone"`
	Request    Request   `json:"request"`
	Requests   []Request `json:"requests"`
	Text       string    `json:"text"`
	Block      Block     `json:"block"`
	Chain      []Block   `json:"chain"`
	CompanyID  int       `json:"company_id"`
	Amount     float64   `json:"amount"`
	TargetID   int       `json:"target_id"`
	BlockIndex int       `json:"block_index"`
	ProposerID int       `json:"proposer_id"`
}

var (
	clock     int
	drones    []Drone
	mu        sync.Mutex
	requestID int
	requests  []Request
	sector    Sector
	sectors   []Sector
	// Variáveis da Blockchain
	blockchain        *Blockchain     = novablockchain()
	committedHash     map[string]bool = make(map[string]bool)
	authorizedBrokers                 = map[int]bool{
		1: true,
		2: true,
	}
)

// == CLOCK

// incrementClock Incrementa o Relógio lógico local e retorna o novo valor
func incrementClock() int {
	clock++
	return clock
}

// updateClock Atualiza o Relógio local com base no valor recebido e incrementa para registrar o evento atual
func updateClock(receivedClock int) int {
	if receivedClock > clock {
		clock = receivedClock
	}

	incrementClock()

	return clock
}

// == REQUEST

// addRequestToQueue Adiciona uma Requisição na fila local, evitando duplicatas e mantendo ordenação por prioridade e desempates
func addRequestToQueue(request Request) {
	// Evita duplicidade usando (SectorID, ID) como chave
	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			return
		}
	}

	index := len(requests) // Inserção padrão no final

	for i, r := range requests {
		// Prioriza Requisições Críticas
		if request.IsCritical && !r.IsCritical {
			index = i
			break
		}

		// Mantém Críticas na frente de não Críticas
		if !request.IsCritical && r.IsCritical {
			continue
		}

		// Ordena por Clock para preservar causalidade
		if request.Clock < r.Clock {
			index = i
			break
		}

		// Desempate por Setor
		if request.Clock == r.Clock && request.SectorID < r.SectorID {
			index = i
			break
		}

		// Desempate final por ID da Requisição
		if request.Clock == r.Clock &&
			request.SectorID == r.SectorID &&
			request.ID < r.ID {
			index = i
			break
		}
	}

	// Insere na posição calculada mantendo o restante da fila
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

func sendRequest(sensor Sensor) {
	mu.Lock()
	custoMissao := 50.0
	saldoAtual := blockchain.GetBalance(sector.ID)
	mu.Unlock()

	if saldoAtual < custoMissao {
		fmt.Printf("\nRequisição do Sensor %d bloqueada! Saldo insuficiente: R$ %.2f\n", sensor.ID, saldoAtual)
		return
	}

	// TENTA COBRAR O VALOR PRIMEIRO
	fmt.Printf("\nCobrando R$ %.2f para autorizar missão do Sensor %d...\n", custoMissao, sensor.ID)
	sucesso := proposeTransaction("DEDUCTION", sector.ID, custoMissao, fmt.Sprintf("Pagamento da missão do Sensor %d", sensor.ID))

	if !sucesso {
		fmt.Printf("\nMissão cancelada por falha na cobrança.\n")
		return
	}

	// SE O PAGAMENTO PASSOU, CRIA A REQUISIÇÃO
	mu.Lock()
	clockValue := incrementClock()
	requestID++

	request := Request{
		SectorID:   sector.ID,
		ID:         requestID,
		Status:     "PENDING",
		X:          sensor.X,
		Y:          sensor.Y,
		IsCritical: sensor.IsCritical,
		Clock:      clockValue,
	}

	fmt.Printf("\nNova requisição criada -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", request.SectorID, request.ID, request.X, request.Y, request.IsCritical, request.Clock)

	addRequestToQueue(request)

	// Atualiza a Interface para visualização do estado
	go sendRequestToInterface("data/initialization/interface.json", request)

	message := Message{
		Text:    "REQUEST",
		Request: request,
		Clock:   clockValue,
	}

	currentSectors := append([]Sector(nil), sectors...)
	currentDrones := append([]Drone(nil), drones...)
	mu.Unlock()

	// Propaga a Requisição para os demais Setores
	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nSetor indisponível: ID ", s.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Resposta é usada para sincronizar o Relógio lógico
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}

	// Propaga a Requisição para os Drones, permitindo que eles iniciem o despacho
	for _, d := range currentDrones {
		conn, err := net.DialTimeout("tcp", d.AddressForSector, 2*time.Second)
		if err != nil {
			fmt.Println("\nDrone indisponível: ID ", d.ID)
			continue
		}

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err = encoder.Encode(message); err != nil {
			_ = conn.Close()
			continue
		}

		var response Message

		// Resposta é usada para sincronizar o Relógio lógico
		if err = decoder.Decode(&response); err != nil {
			_ = conn.Close()
			continue
		}

		mu.Lock()
		updateClock(response.Clock)
		mu.Unlock()

		if response.Text == "QUEUED" {
			_ = conn.Close()
		}
	}
}

// markRequestAsAttending Marca a Requisição como em atendimento e registra o Drone responsável
func markRequestAsAttending(request Request, attendingDrone Drone) {
	fmt.Printf("\nDrone aceitou requisição -> DroneID: %d | SectorID: %d | RequestID: %d\n", attendingDrone.ID, request.SectorID, request.ID)

	// Atualiza a fila local para refletir o início do atendimento
	for i := range requests {
		if requests[i].SectorID == request.SectorID && requests[i].ID == request.ID {
			requests[i].Status = "ATTENDING"
			requests[i].AttendingDroneID = attendingDrone.ID
			break
		}
	}
}

// removeRequestDone Remove a Requisição concluída da fila local do Setor
func removeRequestDone(request Request) {
	fmt.Printf("\nRequisição finalizada -> SectorID: %d | RequestID: %d\n", request.SectorID, request.ID)
	var filtered []Request
	for _, r := range requests {
		if r.SectorID == request.SectorID && r.ID == request.ID {
			continue
		}
		filtered = append(filtered, r)
	}
	requests = filtered
}

// == DRONE

// handleDroneCrash Atualiza o estado do Drone como indisponível e reabre Requisições que estavam em atendimento por ele
func handleDroneCrash(crashedDroneID int) {
	mu.Lock()
	defer mu.Unlock()

	for i := range drones {
		// Marca o Drone como desligado e livre para evitar seleção futura
		if drones[i].ID == crashedDroneID {
			drones[i].IsOn = false
			drones[i].IsBusy = false
			break
		}
	}

	for i := range requests {
		// Reverte para PENDING quando a Requisição ficou sem Drone responsável
		if requests[i].Status == "ATTENDING" &&
			requests[i].AttendingDroneID == crashedDroneID {

			requests[i].Status = "PENDING"
			requests[i].AttendingDroneID = 0

			pendingRequest := requests[i]

			// Atualiza a Interface para refletir a reabertura da Requisição
			go sendRequestToInterface(
				"data/initialization/interface.json",
				pendingRequest,
			)
		}
	}
}

// monitorDrones Verifica periodicamente se os Drones respondem e aciona tratamento de falha quando necessário
func monitorDrones() {
	for {
		mu.Lock()
		currentDrones := append([]Drone(nil), drones...)
		mu.Unlock()

		for _, d := range currentDrones {
			// Ignora Drones já marcados como desligados
			if !d.IsOn {
				continue
			}

			conn, err := net.DialTimeout("tcp", d.AddressForDrone, 2*time.Second)
			if err != nil {
				fmt.Println("Drone não respondeu:", d.ID)
				handleDroneCrash(d.ID)
			} else {
				_ = conn.Close()
			}
		}

		time.Sleep(3 * time.Second)
	}
}

// handleDrone Processa mensagens vindas de Drones e mantém a fila local consistente
func handleDrone(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "ATTENDING":
		// Registra que a Requisição entrou em atendimento e sincroniza o Relógio lógico
		mu.Lock()
		currentClock := updateClock(message.Clock)
		markRequestAsAttending(message.Request, message.Drone)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})

	case "DONE":
		mu.Lock()
		currentClock := updateClock(message.Clock)
		removeRequestDone(message.Request) // Remove da fila do setor atual
		mu.Unlock()

		_ = encoder.Encode(Message{Text: "REMOVED", Clock: currentClock})

		msgFinalizacao := Message{
			Text:    "REMOVE_DONE_REQUEST", // Nova tag de mensagem para os setores
			Request: message.Request,
			Clock:   currentClock,
		}
		go broadcastToSectors(msgFinalizacao)

		if message.Request.SectorID == sector.ID {
			laudo := fmt.Sprintf("Missão reqID %d concluída com sucesso pelo Drone %d", message.Request.ID, message.Drone.ID)
			go proposeTransaction("REPORT", sector.ID, 0.0, laudo)
		}

	case "SYNC_REQUESTS":
		// Responde com a fila atual para permitir sincronização entre componentes
		mu.Lock()
		currentClock := updateClock(message.Clock)
		currentRequests := append([]Request(nil), requests...)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:     "REQUESTS_SYNCED",
			Requests: currentRequests,
			Clock:    currentClock,
		})

	case "PENDING":
		// Reabre a Requisição no estado local e remove o vínculo com o Drone anterior
		mu.Lock()

		currentClock := updateClock(message.Clock)

		for i := range requests {
			if requests[i].SectorID == message.Request.SectorID &&
				requests[i].ID == message.Request.ID {

				requests[i].Status = "PENDING"
				requests[i].AttendingDroneID = 0

				break
			}
		}

		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "UPDATED",
			Clock: currentClock,
		})
	}
}

// listenDrone Inicia o servidor TCP do Setor para receber mensagens enviadas por Drones
func listenDrone() {
	_, port, _ := net.SplitHostPort(sector.AddressForDrone)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos drones: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (drone)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleDrone(conn)
	}
}

// == SENSOR

// handleSensor Recebe leituras do Sensor via conexão TCP e dispara Requisições quando o Sensor está ativo
func handleSensor(conn net.Conn) {
	decoder := json.NewDecoder(conn)

	var sensor Sensor

	for {
		// Mantém leitura contínua até a conexão falhar
		if err := decoder.Decode(&sensor); err != nil {
			_ = conn.Close()
			return
		}

		// Apenas Sensores ativos geram Requisições
		if sensor.IsActive {
			go sendRequest(sensor)
		}
	}
}

// listenSensor Inicia o servidor TCP do Setor para receber eventos enviados por Sensores
func listenSensor() {
	_, port, _ := net.SplitHostPort(sector.AddressForSensor)

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos sensores: ", err)
		return
	}
	defer func() {
		_ = listener.Close()
	}()

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go handleSensor(conn)
	}
}

// == SECTOR

// handleSector Processa mensagens vindas de outros Setores e enfileira Requisições recebidas
func handleSector(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	var message Message

	if decoder.Decode(&message) != nil {
		return
	}

	switch message.Text {
	case "REQUEST":
		// Registra recebimento e garante ordem pelo Relógio lógico
		fmt.Printf("\nRequisição recebida -> SectorID: %d | RequestID: %d | X: %.2f | Y: %.2f | Critical: %t | Clock: %d\n", message.Request.SectorID, message.Request.ID, message.Request.X, message.Request.Y, message.Request.IsCritical, message.Request.Clock)

		mu.Lock()
		currentClock := updateClock(message.Clock)
		addRequestToQueue(message.Request)
		mu.Unlock()

		_ = encoder.Encode(Message{
			Text:  "QUEUED",
			Clock: currentClock,
		})
	case "DEPOSIT": // Vem da company.go
		fmt.Printf("\nPedido de depósito recebido: R$ %.2f\n", message.Amount)
		go proposeTransaction("DEPOSIT", message.CompanyID, message.Amount, "Depósito via Companhia")

	case "TRANSFER":
		fmt.Printf("\nPedido de transferência de R$ %.2f para a Companhia %d\n", message.Amount, message.TargetID)
		go proposeTransfer(message.CompanyID, message.TargetID, message.Amount)

	case "REMOVE_DONE_REQUEST":
		fmt.Printf("\nNotificação de finalização recebida do Setor %d -> RequestID: %d\n", message.Request.SectorID, message.Request.ID)
		mu.Lock()
		_ = updateClock(message.Clock)
		removeRequestDone(message.Request)
		mu.Unlock()

	case "TAMPER":
		mu.Lock()
		if message.BlockIndex < len(blockchain.Chain) && message.BlockIndex > 0 {
			if len(blockchain.Chain[message.BlockIndex].Transactions) > 0 {

				blockchain.Chain[message.BlockIndex].Transactions[0].Amount = message.Amount
				fmt.Printf("\nBloco %d adulterado! Novo valor forçado: R$ %.2f\n", message.BlockIndex, message.Amount)
			}
		} else {
			fmt.Println("\nBloco inválido para adulteração.")
		}
		mu.Unlock()

	case "REQUEST_CHAIN":
		mu.Lock()
		chainCopy := append([]Block(nil), blockchain.getchain()...)
		mu.Unlock()
		_ = encoder.Encode(Message{Text: "CHAIN_RESPONSE", Chain: chainCopy})

	case "PROPOSE_BLOCK":
		mu.Lock()

		// VERIFICAÇÃO DE AUTENTICAÇÃO (PERMISSIONAMENTO)
		if !authorizedBrokers[message.ProposerID] {
			mu.Unlock()
			_ = encoder.Encode(Message{Text: "REJECT_BLOCK"})
			fmt.Printf("\nBloco %d rejeitado: Setor %d não possui autorização de escrita no Ledger.\n", message.Block.Index, message.ProposerID)
			return
		}

		// VALIDAÇÃO MATEMÁTICA PADRÃO
		isValid := validarbloco(message.Block, blockchain.getbloco())
		mu.Unlock()

		if isValid {
			_ = encoder.Encode(Message{Text: "VOTE_BLOCK", Block: message.Block})
			fmt.Printf("\nBloco %d proposto pelo Setor %d foi aprovado.\n", message.Block.Index, message.ProposerID)
		} else {
			_ = encoder.Encode(Message{Text: "REJECT_BLOCK"})
			fmt.Printf("\nBloco %d reprovado por erro matemático.\n", message.Block.Index)
		}

	case "COMMIT_BLOCK":
		mu.Lock()
		if !committedHash[message.Block.Hash] {
			// Valida uma última vez por segurança antes de adicionar
			if validarbloco(message.Block, blockchain.getbloco()) {
				blockchain.adicionarbloco(message.Block)
				committedHash[message.Block.Hash] = true
				fmt.Printf("\nBloco %d commitado via rede.\n", message.Block.Index)
			}
		}
		mu.Unlock()
		// Retorna um ACK para liberar a conexão TCP de quem avisou do COMMIT
		_ = encoder.Encode(Message{Text: "ACK"})
	}
}

// listenSectors Inicia o servidor TCP do Setor para receber mensagens de outros Setores
func listenSectors() {
	_, port, _ := net.SplitHostPort(sector.AddressForSector)

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

// loadSectors Carrega a configuração de Setores, seleciona o Setor atual e mantém os demais como Setores conhecidos
func loadSectors(path string, myID int) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de setores: ", err)
		return err
	}
	defer func() { _ = file.Close() }()

	var config []Sector
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	var filtered []Sector

	for _, s := range config {
		// Separa o Setor deste processo do restante da lista
		if s.ID == myID {
			sector = s
			continue
		}

		filtered = append(filtered, s)
	}

	sectors = filtered

	return nil
}

// loadDrones Carrega a lista de Drones conhecidos a partir de um arquivo JSON
func loadDrones(path string) error {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de drones: ", err)
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Drone
	if err = json.NewDecoder(file).Decode(&config); err != nil {
		return err
	}

	drones = config

	return nil
}

// SAVE DATA

// sendRequestToInterface Envia uma Requisição para o arquivo de Requisições da Interface
func sendRequestToInterface(path string, request Request) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
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
		fmt.Println("Erro ao conectar com interface para enviar requisição: ", err)
		return
	}
	defer conn.Close()

	_ = json.NewEncoder(conn).Encode(request)
}

// sendSectorToInterface Envia o estado do Setor atual para a Interface, com tentativa até conseguir conexão
func sendSectorToInterface(path string) {
	mu.Lock()
	currentSector := sector
	mu.Unlock()

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo de interface: ", err)
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
		// Mantém tentativa para não iniciar sem registrar na Interface
		conn, err := net.DialTimeout("tcp", config[0].Sectors, 2*time.Second)
		if err != nil {
			fmt.Println("Interface indisponível, tentando novamente...")
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSector); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

// MAIN

// main Inicializa o Setor pelo ID informado, carrega configurações e inicia as rotinas de rede e monitoramento
func main() {
	if len(os.Args) < 2 {
		return
	}

	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("Erro no Atoi")
		return
	}

	sectorsPath := "data/initialization/sectors.json"
	dronesPath := "data/initialization/drones.json"
	intefacePath := "data/initialization/interface.json"

	// Carrega Setor atual e lista de Setores remotos
	if err := loadSectors(sectorsPath, id); err != nil {
		fmt.Println("ERRO AO CARREGAR SECTORS:", err)
		return
	}
	// Carrega lista de Drones conhecidos
	if err := loadDrones(dronesPath); err != nil {
		fmt.Println("ERRO AO CARREGAR DRONES:", err)
		return
	}

	// Publica o Setor na Interface para visualização
	go sendSectorToInterface(intefacePath)

	// Inicia servidores e rotinas principais do Setor
	go listenSensor()
	go listenSectors()
	go listenDrone()
	go monitorDrones()

	syncLedger()
	go auditLedger()

	select {}
}

func syncLedger() {
	mu.Lock()
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			continue
		}

		encoder := json.NewEncoder(conn)
		_ = encoder.Encode(Message{Text: "REQUEST_CHAIN"})

		var response Message
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&response); err == nil && response.Text == "CHAIN_RESPONSE" {
			mu.Lock()
			// Substitui se a cadeia remota for maior e válida
			if len(response.Chain) > len(blockchain.getchain()) && validarchain(response.Chain) {
				blockchain.Chain = response.Chain
				fmt.Println("\nCadeia de blocos sincronizada com sucesso. Tamanho:", len(blockchain.getchain()))
			}
			mu.Unlock()
			conn.Close()
			break
		}
		conn.Close()
	}
}

// proposeTransaction cria um bloco, pede votos ativamente e propaga se houver consenso
func proposeTransaction(txType string, companyID int, amount float64, data string) bool {
	tx := Transaction{
		Type:      txType,
		CompanyID: companyID,
		Amount:    amount,
		Data:      data,
		Timestamp: time.Now().String(),
	}

	mu.Lock()
	newBlock := blockchain.criarbloco([]Transaction{tx})
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	msg := Message{Text: "PROPOSE_BLOCK", Block: newBlock, ProposerID: sector.ID}

	votes := 1       // O próprio criador do bloco já aprova a transação
	activeNodes := 1 // O próprio criador já conta como um nó ativo na rede

	// Coleta votos de forma síncrona aguardando a resposta
	for _, s := range currentSectors {
		// Tenta conectar; se falhar, o nó está offline e é ignorado na contagem
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			continue
		}

		// Se a conexão foi aceita, contabilizamos como um nó ativo na rede
		activeNodes++

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err := encoder.Encode(msg); err == nil {
			var response Message
			// Trava e espera o outro setor responder com o voto
			if err := decoder.Decode(&response); err == nil {
				if response.Text == "VOTE_BLOCK" && response.Block.Hash == newBlock.Hash {
					votes++
				}
			}
		}
		_ = conn.Close()
	}

	// Calcula a maioria necessária: Metade dos nós ATIVOS + 1
	majority := (activeNodes / 2) + 1

	// Verifica se os votos atingiram ou superaram a maioria necessária
	if votes >= majority {
		mu.Lock()
		if !committedHash[newBlock.Hash] {
			blockchain.adicionarbloco(newBlock)
			committedHash[newBlock.Hash] = true
			fmt.Printf("\nConsenso atingido! Bloco %d salvo. Saldo atual: R$ %.2f (Votos: %d/%d)\n", newBlock.Index, blockchain.GetBalance(sector.ID), votes, activeNodes)
		}
		mu.Unlock()

		printFullLedger()

		// Propaga o COMMIT definitivo para que os outros nós salvem em suas blockchains
		commitMsg := Message{Text: "COMMIT_BLOCK", Block: newBlock}
		for _, s := range currentSectors {
			conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
			if err == nil {
				_ = json.NewEncoder(conn).Encode(commitMsg)
				_ = conn.Close()
			}
		}
		return true
	} else {
		fmt.Printf("\nFalha no consenso para o bloco %d (Votos: %d, Nós Ativos: %d, Maioria necessária: %d)\n", newBlock.Index, votes, activeNodes, majority)
		return false
	}
}

// proposeTransfer Cria um bloco com duas transações para mover dinheiro entre empresas
func proposeTransfer(senderID int, targetID int, amount float64) {
	mu.Lock()
	if blockchain.GetBalance(senderID) < amount {
		fmt.Printf("\nTransferência negada. Saldo insuficiente (R$ %.2f).\n", blockchain.GetBalance(senderID))
		mu.Unlock()
		return
	}

	txOut := Transaction{Type: "DEDUCTION", CompanyID: senderID, Amount: amount, Data: fmt.Sprintf("Enviou dinheiro para Setor %d", targetID), Timestamp: time.Now().String()}
	txIn := Transaction{Type: "DEPOSIT", CompanyID: targetID, Amount: amount, Data: fmt.Sprintf("Recebeu dinheiro do Setor %d", senderID), Timestamp: time.Now().String()}

	newBlock := blockchain.criarbloco([]Transaction{txOut, txIn})
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	msg := Message{Text: "PROPOSE_BLOCK", Block: newBlock, ProposerID: sector.ID}
	votes, activeNodes := 1, 1

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			continue
		}
		activeNodes++
		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)
		if encoder.Encode(msg) == nil {
			var response Message
			if decoder.Decode(&response) == nil && response.Text == "VOTE_BLOCK" && response.Block.Hash == newBlock.Hash {
				votes++
			}
		}
		_ = conn.Close()
	}

	majority := (activeNodes / 2) + 1

	if votes >= majority {
		mu.Lock()
		if !committedHash[newBlock.Hash] {
			blockchain.adicionarbloco(newBlock)
			committedHash[newBlock.Hash] = true
			fmt.Printf("\nConsenso atingido! Transferência salva no Bloco %d.\n", newBlock.Index)
		}
		mu.Unlock()
		printFullLedger()
		for _, s := range currentSectors {
			conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
			if err == nil {
				json.NewEncoder(conn).Encode(Message{Text: "COMMIT_BLOCK", Block: newBlock})
				conn.Close()
			}
		}
	}
}

func broadcastToSectors(msg Message) {
	mu.Lock()
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err == nil {
			json.NewEncoder(conn).Encode(msg)
			conn.Close()
		}
	}
}

// printFullLedger formata e imprime todo o histórico da blockchain no terminal
func printFullLedger() {
	mu.Lock()
	chainCopy := append([]Block(nil), blockchain.getchain()...)
	mu.Unlock()

	fmt.Println("\n==================== LEDGER COMPLETO ====================")
	for _, block := range chainCopy {
		shortHash := "N/A"
		if len(block.Hash) >= 8 {
			shortHash = block.Hash[:8]
		}

		fmt.Printf("Bloco %d Hash: %s...\n", block.Index, shortHash)

		if len(block.Transactions) == 0 {
			fmt.Println("   -> Gênesis ")
		} else {
			for _, tx := range block.Transactions {
				fmt.Printf("   -> %s | Setor: %d | Valor: R$ %.2f | %s\n", tx.Type, tx.CompanyID, tx.Amount, tx.Data)
			}
		}
	}
	fmt.Println("=========================================================\n")
}

// auditLedger Fica verificando a integridade da blockchain de tempo em tempo
func auditLedger() {
	for {
		time.Sleep(5 * time.Second)
		mu.Lock()
		isValid := validarchain(blockchain.getchain())
		mu.Unlock()

		if !isValid {
			fmt.Println("\nHACK DETECTADO! A Blockchain local foi adulterada.")
			fmt.Println("Solicitando cópia autêntica da rede para corrigir a divergência")
			forceSyncLedger()
		}
	}
}

// forceSyncLedger Pede a blockchain aos vizinhos e substitui a local imediatamente
func forceSyncLedger() {
	mu.Lock()
	currentSectors := append([]Sector(nil), sectors...)
	mu.Unlock()

	for _, s := range currentSectors {
		conn, err := net.DialTimeout("tcp", s.AddressForSector, 2*time.Second)
		if err != nil {
			continue
		}

		json.NewEncoder(conn).Encode(Message{Text: "REQUEST_CHAIN"})

		var response Message
		if json.NewDecoder(conn).Decode(&response) == nil && response.Text == "CHAIN_RESPONSE" {
			// Se a blockchain do vizinho for válida matematicamente, sobrescrevemos a nossa corrompida
			if validarchain(response.Chain) {
				mu.Lock()
				blockchain.Chain = response.Chain
				mu.Unlock()
				fmt.Printf("\nBlockchain restaurada copiando dados do Setor %d.\n", s.ID)
				printFullLedger()
				conn.Close()
				return
			}
		}
		conn.Close()
	}
}

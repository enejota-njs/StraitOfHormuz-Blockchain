package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
)

// Representa uma Requisição gerada por um Sensor e processada no sistema
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

// mu Garante exclusão mútua durante persistência em arquivos JSON compartilhados
var mu sync.Mutex

// saveDrone Persiste o estado do Drone no arquivo de Interface, removendo Drones desligados e substituindo entradas existentes
func saveDrone(path string, drone Drone) error {
	var list []Drone

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			// Falha na leitura é tratada como concorrência de escrita no mesmo arquivo
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	var filtered []Drone
	exists := false

	for _, d := range list {
		// Atualiza o Drone existente e remove se ele estiver desligado
		if d.ID == drone.ID {
			exists = true
			if drone.IsOn {
				filtered = append(filtered, drone)
			}
		} else {
			filtered = append(filtered, d)
		}
	}

	// Insere novo Drone apenas se ele estiver ligado
	if !exists && drone.IsOn {
		filtered = append(filtered, drone)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(filtered)
}

// saveSector Persiste o estado do Setor no arquivo de Interface, substituindo o Setor existente pelo mesmo ID
func saveSector(path string, sector Sector) error {
	var list []Sector

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			// Falha na leitura é tratada como concorrência de escrita no mesmo arquivo
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	exists := false
	for i := range list {
		// Substitui registro existente pelo mesmo ID
		if list[i].ID == sector.ID {
			list[i] = sector
			exists = true
			break
		}
	}

	if !exists {
		list = append(list, sector)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(list)
}

// saveSensor Persiste o estado do Sensor no arquivo de Interface, usando (Type, X, Y) como chave de identificação
func saveSensor(path string, sensor Sensor) error {
	var list []Sensor

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			// Falha na leitura é tratada como concorrência de escrita no mesmo arquivo
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	exists := false
	for i := range list {
		// Mantém um Sensor único por tipo e coordenadas
		if list[i].Type == sensor.Type && list[i].X == sensor.X && list[i].Y == sensor.Y {
			list[i] = sensor
			exists = true
			break
		}
	}

	if !exists {
		list = append(list, sensor)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(list)
}

// saveRequest Persiste o estado da Requisição no arquivo de Interface, removendo-a quando Status for DONE
func saveRequest(path string, request Request) error {
	var list []Request

	file, err := os.Open(path)
	if err == nil {
		if stat, _ := file.Stat(); stat.Size() > 0 {
			// Falha na leitura é tratada como concorrência de escrita no mesmo arquivo
			if err := json.NewDecoder(file).Decode(&list); err != nil {
				file.Close()
				return fmt.Errorf("arquivo ocupado")
			}
		}
		file.Close()
	}

	var filtered []Request
	exists := false

	for _, r := range list {
		// Atualiza a Requisição existente e remove quando finalizada
		if r.SectorID == request.SectorID && r.ID == request.ID {
			if request.Status == "DONE" {
				exists = true
				continue
			}
			filtered = append(filtered, request)
			exists = true
		} else {
			filtered = append(filtered, r)
		}
	}

	// Insere nova Requisição apenas se ela não estiver finalizada
	if !exists && request.Status != "DONE" {
		filtered = append(filtered, request)
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()

	encoder := json.NewEncoder(outFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(filtered)
}

// listenDrones Inicia um servidor TCP para receber estados de Drones e persistir no arquivo da Interface
func listenDrones(port string, path string) {
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

		go func(conn net.Conn) {
			defer conn.Close()
			var drone Drone

			if err := json.NewDecoder(conn).Decode(&drone); err != nil {
				return
			}

			// Protege a leitura e escrita do arquivo compartilhado
			mu.Lock()
			_ = saveDrone(path, drone)
			mu.Unlock()
		}(conn)
	}
}

// listenSectors Inicia um servidor TCP para receber estados de Setores e persistir no arquivo da Interface
func listenSectors(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos setores:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (setor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var sector Sector
			if err := json.NewDecoder(conn).Decode(&sector); err != nil {
				return
			}
			// Protege a leitura e escrita do arquivo compartilhado
			mu.Lock()
			_ = saveSector(path, sector)
			mu.Unlock()
		}(conn)
	}
}

// listenSensors Inicia um servidor TCP para receber estados de Sensores e persistir no arquivo da Interface
func listenSensors(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta dos sensores:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (sensor)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var sensor Sensor
			if err := json.NewDecoder(conn).Decode(&sensor); err != nil {
				return
			}
			// Protege a leitura e escrita do arquivo compartilhado
			mu.Lock()
			_ = saveSensor(path, sensor)
			mu.Unlock()
		}(conn)
	}
}

// listenRequests Inicia um servidor TCP para receber estados de Requisições e persistir no arquivo da Interface
func listenRequests(port string, path string) {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Erro ao iniciar porta das requisições:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Servidor inicializado (requisição)")

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			var request Request
			if err := json.NewDecoder(conn).Decode(&request); err != nil {
				return
			}
			// Protege a leitura e escrita do arquivo compartilhado
			mu.Lock()
			_ = saveRequest(path, request)
			mu.Unlock()
		}(conn)
	}
}

// loadInterfacePorts Carrega os endereços de portas da Interface a partir do arquivo de configuração
func loadInterfacePorts(path string) (string, string, string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", "", "", err
	}
	defer file.Close()

	var config []struct {
		Sectors  string `json:"sectors"`
		Drones   string `json:"drones"`
		Sensors  string `json:"sensors"`
		Requests string `json:"requests"`
	}

	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return "", "", "", "", err
	}

	return config[0].Sectors, config[0].Drones, config[0].Sensors, config[0].Requests, nil
}

// clearFile Reinicializa o arquivo JSON com uma lista vazia para evitar lixo de execuções anteriores
func clearFile(path string) {
	file, err := os.Create(path)
	if err != nil {
		fmt.Println("Erro ao limpar arquivo:", path, err)
		return
	}
	defer file.Close()
	_, _ = file.WriteString("[]")
}

// main Limpa os arquivos de saída, carrega portas da Interface e inicia os servidores de recebimento
func main() {
	// Garante estado limpo para consumo pela Interface gráfica
	clearFile("data/interface/drones.json")
	clearFile("data/interface/sectors.json")
	clearFile("data/interface/sensors.json")
	clearFile("data/interface/requests.json")

	interfacePath := "data/initialization/interface.json"

	sectorsPort, dronesPort, sensorsPort, requestsPort, err := loadInterfacePorts(interfacePath)
	if err != nil {
		fmt.Println("Erro ao ler interface.json:", err)
		return
	}

	// Extrai somente a porta para o net.Listen
	_, sectorsPort, _ = net.SplitHostPort(sectorsPort)
	_, dronesPort, _ = net.SplitHostPort(dronesPort)
	_, sensorsPort, _ = net.SplitHostPort(sensorsPort)
	_, requestsPort, _ = net.SplitHostPort(requestsPort)

	go listenDrones(dronesPort, "data/interface/drones.json")
	go listenSectors(sectorsPort, "data/interface/sectors.json")
	go listenSensors(sensorsPort, "data/interface/sensors.json")
	go listenRequests(requestsPort, "data/interface/requests.json")

	select {}
}

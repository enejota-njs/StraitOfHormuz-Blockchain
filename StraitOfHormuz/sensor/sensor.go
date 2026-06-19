package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Representa um Setor do mapa e suas portas de comunicação
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

// Representa um Sensor e suas características e estado de ativação
type Sensor struct {
	ID         int     `json:"id"`          // Identificador do Sensor
	IsActive   bool    `json:"is_active"`   // Indica se o Sensor está ativo
	IsCritical bool    `json:"is_critical"` // Indica se o Sensor gera Requisição Crítica
	Type       string  `json:"type"`        // Tipo do Sensor
	X          float64 `json:"x"`           // Coordenada X
	Y          float64 `json:"y"`           // Coordenada Y
}

// Variáveis globais utilizadas para manter o estado local do processo
var (
	mu     sync.Mutex // Exclusão mútua para acesso concorrente ao estado
	sensor Sensor     // Sensor atual
	sector string     // Endereço do Setor de destino
)

// == SENSOR

// runSensor Mantém uma conexão com o Setor responsável e envia periodicamente o estado do Sensor
func runSensor() {
	conn, err := net.DialTimeout("tcp", sector, 2*time.Second)
	if err != nil {
		fmt.Println("Erro ao se comunicar com servidor do setor: ", err)
		return
	}

	encoder := json.NewEncoder(conn)

	for {
		r := rand.Float64()

		mu.Lock()
		// Define ativação e criticidade com base em um valor aleatório para simulação
		sensor.IsActive = r > 0.6
		sensor.IsCritical = r > 0.8
		currentSensor := sensor
		mu.Unlock()

		fmt.Printf("\nSensor enviando -> ID: %d | Type: %s | X: %.2f | Y: %.2f | Active: %t | Critical: %t\n", currentSensor.ID, currentSensor.Type, currentSensor.X, currentSensor.Y, currentSensor.IsActive, currentSensor.IsCritical)

		if err := encoder.Encode(currentSensor); err != nil {
			_ = conn.Close()

			for {
				// Reestabelece a conexão para manter o envio contínuo
				conn, err = net.DialTimeout("tcp", sector, 2*time.Second)
				if err == nil {
					fmt.Println("Erro ao se comunicar com servidor do setor: ", err)

					break
				}

				fmt.Println("Tentando reconectar")
				time.Sleep(2 * time.Second)
			}
		}

		time.Sleep(15 * time.Second)
	}
}

// findSector Encontra qual Setor contém a posição do Sensor e configura o endereço de destino
func findSector(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos setores: ", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sector
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return false
	}

	x := sensor.X
	y := sensor.Y

	for _, s := range config {
		// Seleciona o Setor cujo retângulo de cobertura contém as coordenadas do Sensor
		if x >= s.Left &&
			x <= s.Right &&
			y <= s.Top &&
			y >= s.Bottom {
			sector = s.AddressForSensor
			return true
		}
	}

	return false
}

// register Carrega a configuração do Sensor pelo ID informado e inicializa seu estado local
func register(path string) bool {
	id, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return false
	}

	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Erro ao abrir arquivo dos sensores: ", err)
		return false
	}
	defer func() {
		_ = file.Close()
	}()

	var config []Sensor
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return false
	}

	for _, s := range config {
		// Seleciona o Sensor correspondente ao ID fornecido na linha de comando
		if s.ID == id {
			sensor = Sensor{
				ID:         s.ID,
				Type:       s.Type,
				X:          s.X,
				Y:          s.Y,
				IsActive:   false,
				IsCritical: false,
			}

			return true
		}
	}

	return false
}

// == SAVE DATA

// sendSensorToInterface Envia o estado do Sensor atual para a Interface, com retentativa até conseguir conexão
func sendSensorToInterface(path string) {
	mu.Lock()
	currentSensor := sensor
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
		// Mantém tentativa para não iniciar sem registrar na Interface
		conn, err := net.DialTimeout("tcp", config[0].Sensors, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao se comunicar com servidor da interface: ", err)
			time.Sleep(1 * time.Second)
			continue
		}

		if err = json.NewEncoder(conn).Encode(currentSensor); err != nil {
			_ = conn.Close()
			continue
		}

		_ = conn.Close()
		break
	}
}

// main Inicializa o Sensor pelo ID informado, resolve o Setor responsável e inicia as rotinas do processo
func main() {
	if len(os.Args) < 2 {
		return
	}

	sensorsPath := "data/initialization/sensors.json"
	sectorsPath := "data/initialization/sectors.json"
	intefacePath := "data/initialization/interface.json"

	// Carrega a configuração do Sensor atual
	if !register(sensorsPath) {
		fmt.Println("Erro ao registrar sensor")
		return
	}
	// Determina o Setor que deve receber eventos deste Sensor
	if !findSector(sectorsPath) {
		fmt.Println("Erro ao procurar setor")
		return
	}

	// Publica o Sensor na Interface para visualização
	go sendSensorToInterface(intefacePath)

	// Inicia o loop de envio de eventos para o Setor responsável
	go runSensor()

	select {}
}

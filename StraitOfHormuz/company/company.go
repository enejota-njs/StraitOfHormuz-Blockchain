package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

type SectorConfig struct {
	ID               int    `json:"id"`
	AddressForSector string `json:"address_for_sector"`
}

func getBrokerIP(path string, targetID int) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var config []SectorConfig
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return "", err
	}

	for _, s := range config {
		if s.ID == targetID {
			return s.AddressForSector, nil
		}
	}
	return "", fmt.Errorf("setor não encontrado")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Uso: go run company.go <ID_DO_SETOR>")
		return
	}

	companyID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		return
	}

	address, err := getBrokerIP("data/initialization/sectors.json", companyID)
	if err != nil {
		fmt.Println("\nErro ao buscar o broker. O Setor está rodando?")
		return
	}

	fmt.Printf("\nCompanhia%d Conectada ao Broker: %s\n", companyID, address)

	for {
		fmt.Println("\n================ MENU ================")
		fmt.Println("1 - Depositar dinheiro")
		fmt.Println("2 - Transferir para outra Companhia")
		fmt.Println("3 - Adulterar Bloco (Simular Hack)")
		fmt.Println("0 - Sair")
		fmt.Print("Escolha uma opção: ")

		var opcao int
		fmt.Scan(&opcao)

		msg := map[string]interface{}{
			"company_id": companyID,
		}

		switch opcao {
		case 0:
			fmt.Println("Saindo...")
			return
		case 1:
			var amount float64
			fmt.Print("Valor do depósito: R$ ")
			fmt.Scan(&amount)
			msg["text"] = "DEPOSIT"
			msg["amount"] = amount

		case 2:
			var targetID int
			var amount float64
			fmt.Print("ID da Companhia destino: ")
			fmt.Scan(&targetID)
			fmt.Print("Valor da transferência: R$ ")
			fmt.Scan(&amount)
			msg["text"] = "TRANSFER"
			msg["target_id"] = targetID
			msg["amount"] = amount

		case 3:
			var blockIndex int
			var fakeAmount float64
			fmt.Print("Qual o número do bloco que deseja fraudar? ")
			fmt.Scan(&blockIndex)
			fmt.Print("Qual o novo valor (falso) para colocar lá? R$ ")
			fmt.Scan(&fakeAmount)
			msg["text"] = "TAMPER"
			msg["block_index"] = blockIndex
			msg["amount"] = fakeAmount

		default:
			fmt.Println("Opção inválida.")
			continue
		}

		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			fmt.Println("Erro ao conectar no broker.")
			continue
		}

		json.NewEncoder(conn).Encode(msg)
		conn.Close()
		fmt.Println("Comando enviado para o Setor com sucesso!")
		time.Sleep(1 * time.Second)
	}
}

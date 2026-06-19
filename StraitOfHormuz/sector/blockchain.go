package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Representa uma transação financeira ou de registro no ledger
type Transaction struct {
	Type      string  `json:"type"`       // "DEPOSIT", "DEDUCTION", "REPORT"
	CompanyID int     `json:"company_id"` // ID do Setor/Companhia
	Amount    float64 `json:"amount"`     // Valor financeiro
	Data      string  `json:"data"`       // Laudos ou dados extras
	Timestamp string  `json:"timestamp"`
}

// Representa um bloco na Blockchain
type Block struct {
	Index        int           `json:"index"`
	Timestamp    string        `json:"timestamp"`
	Transactions []Transaction `json:"transactions"`
	PreviousHash string        `json:"previous_hash"`
	Hash         string        `json:"hash"`
}

// Representa a cadeia de blocos principal
type Blockchain struct {
	Chain []Block `json:"chain"`
}

// calcularhash Gera o hash SHA256 de um bloco
func calcularhash(block Block) string {
	record := fmt.Sprintf("%d%s%v%s", block.Index, block.Timestamp, block.Transactions, block.PreviousHash)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// criarblocogenesis Inicializa o primeiro bloco da rede
func criarblocogenesis() Block {
	genesisBlock := Block{
		Index:        0,
		Timestamp:    "0000-00-00 00:00:00", // Data estática para que o Hash seja idêntico em todos os nós
		Transactions: []Transaction{},
		PreviousHash: "0",
	}
	genesisBlock.Hash = calcularhash(genesisBlock)
	return genesisBlock
}

// novablockchain Instancia a blockchain com o bloco genesis
func novablockchain() *Blockchain {
	return &Blockchain{
		Chain: []Block{criarblocogenesis()},
	}
}

// getchain Retorna a cadeia completa
func (bc *Blockchain) getchain() []Block {
	return bc.Chain
}

// getbloco Retorna o último bloco da cadeia
func (bc *Blockchain) getbloco() Block {
	return bc.Chain[len(bc.Chain)-1]
}

// criarbloco Instancia um novo bloco baseado no bloco anterior
func (bc *Blockchain) criarbloco(txs []Transaction) Block {
	prevBlock := bc.getbloco()
	newBlock := Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now().String(),
		Transactions: txs,
		PreviousHash: prevBlock.Hash,
	}
	newBlock.Hash = calcularhash(newBlock)
	return newBlock
}

// validarhash Verifica se o hash fornecido condiz com os dados do bloco
func validarhash(block Block) bool {
	return block.Hash == calcularhash(block)
}

// validarbloco Checa se o bloco é válido em relação ao anterior
func validarbloco(newBlock Block, prevBlock Block) bool {
	if prevBlock.Index+1 != newBlock.Index {
		return false
	}
	if prevBlock.Hash != newBlock.PreviousHash {
		return false
	}
	if !validarhash(newBlock) {
		return false
	}
	return true
}

// validarchain Checa a integridade de uma cadeia inteira recebida de outro nó
func validarchain(chain []Block) bool {
	if len(chain) == 0 {
		return false
	}
	// Opcional: verificar o bloco genesis
	for i := 1; i < len(chain); i++ {
		if !validarbloco(chain[i], chain[i-1]) {
			return false
		}
	}
	return true
}

// adicionarbloco Insere o bloco na cadeia local se ele for válido
func (bc *Blockchain) adicionarbloco(newBlock Block) bool {
	if validarbloco(newBlock, bc.getbloco()) {
		bc.Chain = append(bc.Chain, newBlock)
		return true
	}
	return false
}

// GetBalance Calcula o saldo atual de uma companhia baseando-se no ledger
func (bc *Blockchain) GetBalance(companyID int) float64 {
	balance := 0.0
	for _, block := range bc.Chain {
		for _, tx := range block.Transactions {
			if tx.CompanyID == companyID {
				if tx.Type == "DEPOSIT" {
					balance += tx.Amount
				} else if tx.Type == "DEDUCTION" {
					balance -= tx.Amount
				}
			}
		}
	}
	return balance
}

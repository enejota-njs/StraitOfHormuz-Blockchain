# Strait of Hormuz Blockchain

> Universidade Estadual de Feira de Santana
>
> TEC502 - MI - Concorrência e Conectividade
>
> Problema 3: Economia e Auditoria de Guerra

### Infraestrutura distribuída para coordenação de drones autônomos de monitoramento marítimo com blockchain própria para autorizar e pagar missões

## Sumário

- [📍 Descrição do Projeto](#-descrição-do-projeto)
- [📂 Estrutura de Pastas](#-estrutura-de-pastas)
- [📐 Arquitetura](#-arquitetura)
- [⛓️ Blockchain](#-blockchain)
- [🚀 Fluxo Completo de uma Missão Paga](#-fluxo-completo-de-uma-missão-paga)
- [🔧 Configuração de Ambiente](#-configuração-de-ambiente)
- [▶️ Como Executar](#️-como-executar)
- [📌 Observações e Limitações](#-observações-e-limitações)

---

## 📍 Descrição do Projeto

### Origem

Este repositório parte da arquitetura criada em [`enejota-njs/StraitOfHormuz`](https://github.com/enejota-njs/StraitOfHormuz), que implementa a coordenação distribuída entre setores, drones e sensores para monitoramento marítimo no Estreito de Hormuz: relógio lógico de Lamport, fila de requisições ordenada por criticidade, eleição do drone mais próximo e replanejamento automático quando um drone falha.

A partir dessa base, esta versão adiciona uma **blockchain própria** (`sector/blockchain.go`) e um cliente de depósitos (`company/company.go`), transformando cada setor também em um nó de um ledger distribuído: nenhuma missão é aberta sem saldo, e nenhuma transação é gravada sem o voto da maioria dos setores ativos. É essa camada que este README detalha; para a lógica P2P herdada, o repositório original é a referência mais completa.

### Objetivos

* **Solução Descentralizada**: Tecnologia distríbuida, descentralizada e com a ausência de Ponto Único de Falha (**SPOF**).
* **Gestão de Ativos:** O sistema deve criar uma moeda digital (token) usada pelas
companhias para requisitar os drones. O Ledger distribuído deve registrar a posse e a transferência
desses créditos de forma imutável, garantindo que seja impossível realizar o **"duplo gasto"** (usar o
mesmo saldo para duas escoltas diferentes).
* **Log de Operações Imutável:** Toda vez que um drone for despachado e concluir sua missão de
reconhecimento, o "laudo" da missão deve ser registrado,
tornando a informação pública e à prova de adulteração para todas as companhias do consórcio.

## 📂 Estrutura de Pastas

```
StraitOfHormuz/
├── company/                  # [novo] cliente CLI para depositar saldo na blockchain de um setor
│   └── company.go
├── data/
│   ├── images/               # Asset do painel (drone.webp)
│   ├── initialization/       # Configuração de entrada (IDs, endereços, limites)
│   └── interface/            # Estado em tempo de execução, consumido pelo painel Pygame
├── drone/
│   └── drone.go
├── interface/
│   ├── interface.go           # Hub TCP que persiste o estado recebido em JSON
│   └── interface.py           # Painel visual (Pygame)
├── sector/
│   ├── sector.go
│   └── blockchain.go         # [novo] blocos, hash, validação de cadeia e saldo
├── sensor/
│   └── sensor.go
├── go.mod
└── go.sum
```

## 📐 Arquitetura

### Componentes

| Componente | Arquivo(s) | Nesta versão | Papel |
|---|---|---|---|
| Blockchain | `sector/blockchain.go` | **Novo** | Blocos, hash (SHA-256), validação de cadeia e cálculo de saldo por companhia. |
| Company | `company/company.go` | **Novo** | Cliente CLI para depositar saldo na blockchain de um setor. |
| Setor | `sector/sector.go` | Estendido | Fila de requisições e despacho (herdado) + proposta/voto/gravação de blocos (novo). |
| Drone | `drone/drone.go` | Herdado | Sincroniza a fila, decide por proximidade quem atende, executa e libera a missão. |
| Sensor | `sensor/sensor.go` | Herdado | Gera eventos aleatórios e os envia ao setor responsável pela sua área. |
| Interface | `interface/interface.go` + `interface.py` | Herdado | Hub TCP + painel Pygame para observabilidade; não participa do consenso. |

---

## ⛓️ Blockchain

### Visão Geral

- **Estrutura:** `Transaction` guarda tipo (`DEPOSIT`, `DEDUCTION`, `REPORT`), `CompanyID`, valor e um campo livre de dados (usado como laudo da missão). `Block` encadeia um índice, timestamp, lista de transações, o hash do bloco anterior e o próprio hash (SHA-256 sobre os demais campos). `Blockchain` é simplesmente a lista de blocos (`Chain`).
- **Bloco gênese determinístico:** O primeiro bloco usa um timestamp fixo (`"0000-00-00 00:00:00"`), garantindo que todos os setores comecem com exatamente o mesmo hash — pré-requisito para que a validação entre nós diferentes funcione.
- **Conta = Setor:** O saldo (`GetBalance`) soma depósitos e subtrai deduções filtrando pelo `CompanyID`; na prática, o ID do setor é também o ID da companhia que o financia, então cada setor tem sua própria "conta" dentro do mesmo livro-contábil compartilhado.
- **Gate financeiro da missão:** Antes de transformar uma leitura de sensor em requisição, o setor verifica `blockchain.GetBalance(sector.ID)`. Se o saldo for menor que o custo fixo de R$ 50,00 de uma missão, a requisição é bloqueada e nem entra na fila.
- **Depósitos via `company.go`:** O cliente lê `sectors.json`, descobre o endereço do setor/companhia alvo e envia uma mensagem `DEPOSIT` repetidamente, a partir de valores digitados no terminal.
- **Dedução automática:** Ao receber a confirmação de que um drone concluiu uma missão, o próprio setor de origem propõe uma transação `DEDUCTION` no valor da missão.

### Consenso entre Setores

Toda transação só entra na cadeia depois de uma rodada de votação por maioria entre os setores que estão de fato acessíveis:

1. O setor que originou a transação monta um bloco e o envia (`PROPOSE_BLOCK`) a cada setor conhecido, na mesma conexão TCP em que aguardará a resposta.
2. Cada setor que recebe o bloco o valida (índice sequencial, hash anterior compatível com o seu próprio último bloco, e integridade do hash) e responde `VOTE_BLOCK` (aprova) ou `REJECT_BLOCK` (recusa).
3. O proponente conta como "nó ativo" qualquer setor que respondeu à conexão e como "voto" apenas quem retornou `VOTE_BLOCK` com o hash correto. A maioria exigida é `(nós ativos / 2) + 1` — calculada sobre quem está no ar naquela rodada, não sobre o total configurado em `sectors.json`.
4. Atingida a maioria, o bloco é gravado localmente e um `COMMIT_BLOCK` é propagado para que os demais setores também o adicionem às suas cópias da cadeia.
5. Sem maioria, a transação é simplesmente descartada (com logs de quantos votos foram obtidos versus quantos eram necessários).

Ao iniciar, cada setor pede a cadeia completa a um setor conhecido (`REQUEST_CHAIN` / `CHAIN_RESPONSE`) e a adota caso seja **mais longa e válida** que a sua própria cadeia local — uma versão simplificada da regra da "cadeia mais longa válida".

### Mensagens da Blockchain (TCP + JSON)

| Mensagem | Direção | Finalidade |
|---|---|---|
| `DEPOSIT` | Company → Setor | Solicita o registro de um depósito para uma companhia/setor. |
| `REQUEST_CHAIN` / `CHAIN_RESPONSE` | Setor ↔ Setor | Sincroniza a cadeia completa quando um setor inicia. |
| `PROPOSE_BLOCK` | Setor → Setor | Propõe um novo bloco (depósito ou dedução) para votação. |
| `VOTE_BLOCK` / `REJECT_BLOCK` | Setor → Setor | Resposta de aprovação ou recusa ao bloco proposto. |
| `COMMIT_BLOCK` | Setor → Setor | Confirma o bloco aprovado para gravação definitiva em toda a rede. |

> [!NOTE]
> As demais mensagens do protocolo (`REQUEST`, `ATTENDING`, ...) pertencem ao fluxo P2P herdado do projeto original e não foram alteradas pela camada de blockchain.

## 🚀 Fluxo Completo de uma Missão Paga

1. `company.go` deposita saldo em um setor específico — esse depósito também é tratado como uma transação, proposta e votada como qualquer outra.
2. Um sensor da área desse setor reporta um evento; havendo saldo, o setor cria a requisição e segue o fluxo P2P herdado (fila por criticidade/relógio lógico, drone mais próximo assume, executa, conclui).
3. Ao receber a confirmação de `DONE`, o setor de origem propõe uma `DEDUCTION` de R$ 50,00, que passa pela votação descrita acima e é gravada na cadeia.
4. O saldo do setor cai; uma nova missão só será aceita se ainda houver saldo suficiente para cobrir o custo.

## 🔧 Configuração de Ambiente

### Pré-requisitos

- Go/Golang.
- Docker.
- Python 3 com `pygame` instalado (`pip install pygame`), apenas para o painel visual — opcional para a simulação em si.

>[!IMPORTANT]
> O `go.mod` declara `go 1.26`; ajuste essa diretiva se estiver usando outra versão.

Os arquivos em `data/initialization/` definem a topologia da rede: `sectors.json` (id, três endereços de comunicação e limites geográficos de cada setor), `drones.json` (id e endereços de cada drone), `sensors.json` (id, tipo e coordenadas de cada sensor) e `interface.json` (endereços do hub de observabilidade). Os endereços de exemplo apontam todos para o mesmo host (`172.16.201.8`); ajuste-os para `127.0.0.1` ou para os IPs reais de cada máquina, conforme o ambiente.

## ▶️ Como Executar

Todos os processos usam caminhos relativos (`../data/...`), então cada comando deve ser executado dentro da respectiva pasta do componente. Como cada setor nasce com saldo zero, **o passo de depósito (`company.go`) não é opcional** — sem ele, toda requisição de sensor será bloqueada por saldo insuficiente.

```bash
# 1. Hub de interface (recebe e persiste o estado)
cd interface
go run interface.go

# 2. Painel visual (opcional)
cd interface
python interface.py

# 3. Setores — repita para cada ID definido em sectors.json
#    (sector.go e blockchain.go formam o mesmo pacote, por isso "go run ." e não "go run sector.go <id>")
cd sector
go run . 1

# 4. Deposite saldo no setor antes de gerar missões
cd company
go run company.go 1

# 5. Drones — repita para cada ID definido em drones.json
cd drone
go run drone.go 1

# 6. Sensores — repita para cada ID definido em sensors.json
cd sensor
go run sensor.go 1
```

## 📌 Observações e Limitações

- A cadeia é única e compartilhada por toda a rede de setores; contas diferentes (uma por `CompanyID`/setor) convivem no mesmo livro-contábil, distinguidas apenas no momento de somar o saldo.
- A maioria exigida no consenso é dinâmica — baseada em quantos setores responderam durante aquela rodada de votação —, então a tolerância a falhas depende de quem está de fato acessível no instante da proposta.
- O código também trata, dentro do switch de mensagens de cada setor, um caso de voto recebido de forma assíncrona (`VOTE_BLOCK` como mensagem independente, acumulando em `blockVotes`); esse caminho está preparado para um modelo de votação por broadcast, mas não chega a ser acionado pelo fluxo atual de `proposeTransaction`, que já resolve a votação de forma síncrona na própria conexão.
- O estado da blockchain e da fila de requisições vive em memória de cada processo; os arquivos em `data/interface/` são apenas o reflexo usado pelo painel visual.
- Limitações herdadas do projeto original (IPs fixos nos exemplos de configuração, ausência de persistência em disco, etc.) continuam valendo — o repositório base traz mais detalhes sobre a parte P2P.

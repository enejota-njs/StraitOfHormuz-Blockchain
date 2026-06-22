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
- [⛓️ Blockchain](#%EF%B8%8F-blockchain)
- [🚀 Fluxo Completo de uma Missão Paga](#-fluxo-completo-de-uma-missão-paga)
- [🔧 Configuração de Ambiente](#-configuração-de-ambiente)
- [▶️ Como Executar](#️-como-executar)
- [🧪 Testes](#-testes)
- [📌 Resultados e Observações](#-resultados-e-observações)

---

## 📍 Descrição do Projeto

### Origem

Este repositório parte da arquitetura criada em [`enejota-njs/StraitOfHormuz`](https://github.com/enejota-njs/StraitOfHormuz), que implementa a coordenação distribuída entre setores, drones e sensores para monitoramento marítimo no Estreito de Hormuz: relógio lógico de Lamport, fila de requisições ordenada por criticidade, eleição do drone mais próximo e replanejamento automático quando um drone falha.

A partir dessa base, esta versão adiciona uma **blockchain própria** (`sector/blockchain.go`) e um cliente de depósitos (`company/company.go`), transformando cada setor também em um nó de um ledger distribuído: nenhuma missão é aberta sem saldo, e nenhuma transação é gravada sem o voto da maioria dos setores ativos. É essa camada que este README detalha; para a lógica P2P herdada, o repositório original é a referência mais completa.

### Objetivos

* **Solução Descentralizada**: Tecnologia distribuída, descentralizada e com a ausência de Ponto Único de Falha (**SPOF**).
* **Gestão de Ativos:** O sistema deve criar uma moeda digital (token) usada pelas
companhias para requisitar os drones. O Ledger distribuído deve registrar a posse e a transferência
desses créditos de forma imutável, garantindo que seja impossível realizar o **"duplo gasto"** (usar o
mesmo saldo para duas escoltas diferentes).
* **Log de Operações Imutável:** Toda vez que um drone for despachado e concluir sua missão de
reconhecimento, o "laudo" da missão deve ser registrado,
tornando a informação pública e à prova de adulteração para todas as companhias do consórcio.

### Principais Extensões Implementadas

- Blockchain própria
- Consenso distribuído entre setores
- Sistema de depósitos
- Controle de saldo para autorização de missões
- Registro imutável de operações

---

## 📂 Estrutura de Pastas

```
StraitOfHormuz/
├── company/                  # [novo] cliente CLI para depositar saldo na blockchain de um setor
│   └── company.go
├── data/
│   ├── images/               # Asset do painel
│   ├── initialization/       # Configuração de entrada (IDs, endereços, limites)
│   └── interface/            # Estado em tempo de execução, consumido pelo painel Pygame
├── drone/
│   └── drone.go
├── interface/
│   ├── interface.go           # Hub que persiste o estado recebido em JSON
│   └── interface.py           # Painel visual (Pygame)
├── sector/
│   ├── sector.go
│   └── blockchain.go         # [novo] blocos, hash, validação de cadeia e saldo
├── sensor/
│   └── sensor.go
├── go.mod
└── go.sum
```

---

## 📐 Arquitetura

### Diagrama

<div align="center">
    <img width="680" height="610" alt="architeture" src="https://github.com/user-attachments/assets/9cb657b1-4dfe-4e19-91a5-94a48b3d8367" />
</div>

> [!NOTE]
> Cada sensor e companhia são associados a um setor.

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

---

## 🚀 Fluxo Completo de uma Missão Paga

1. `company.go` deposita saldo em um setor específico — esse depósito também é tratado como uma transação, proposta e votada como qualquer outra.
2. Um sensor da área desse setor reporta um evento; havendo saldo, o setor cria a requisição e segue o fluxo P2P herdado (fila por criticidade/relógio lógico, drone mais próximo assume, executa, conclui).
3. Ao receber a confirmação de `DONE`, o setor de origem propõe uma `DEDUCTION` de R$ 50,00, que passa pela votação descrita acima e é gravada na cadeia.
4. O saldo do setor cai; uma nova missão só será aceita se ainda houver saldo suficiente para cobrir o custo.

---

## 🔧 Configuração de Ambiente

### Topologia da Rede

Os arquivos em `data/initialization/` definem a topologia da rede. Os endereços de exemplo apontam todos para o mesmo host (`172.16.201.1`); **ajuste-os para os IPs reais de cada máquina**, conforme o ambiente.

> [!IMPORTANT]
> Para rodar o sistema em máquinas diferentes, certifique-se de que ambas estejam na mesma rede local e que o firewall permita conexões nas portas utilizadas.

* `sectors.json`

```json
{
    // ID
    "id": 1,                                    
    // Endereços de comunicação
    "address_for_drone" : "172.16.201.2:5000",  // para os drones
    "address_for_sector": "172.16.201.2:5001",  // para outros setores
    "address_for_sensor": "172.16.201.2:5002",  // para o sensor
    // Limites geográficos
    "left": 1,                                  
    "right": 50,
    "top": 100,
    "bottom": 0
  }
```

* `drones.json`

```json
{
    // ID
    "id": 1,                                   
    // Endereços de comunicação
    "address_for_sector": "172.16.201.1:5012",  // para os setores 
    "address_for_drone": "172.16.201.1:5013"    // para outros drones
  }
```

* `sensors.json` 

```json
{
    "id" : 1,    // ID
    "type" : "RadarCosteiro",    // Tipo
    // Coordenadas
    "x" : 10,    
    "y" : 20
  }
```

* `interface.json`

```json
{
    // Endereços do hub de observabilidade
    "sectors" : "172.16.201.1:9001",
    "drones" : "172.16.201.1:9002",
    "sensors" : "172.16.201.1:9003",
    "requests": "172.16.201.1:9004"
  }
```

### Docker Compose

O arquivo `docker-compose.yml` é responsável por orquestrar a construção e execução de múltiplos serviços simultaneamente. Dessa forma, o usuário não precisa iniciar cada serviço por meio de comandos individuais.

Novos serviços podem ser adicionados ou removidos por meio da edição desse arquivo. A seguir é apresentada a estrutura básica de um serviço:

```yaml
sector1:
  build:
    context: .
    dockerfile: ./sector/Dockerfile
  container_name: sector1
  command: ["./sector_bin", "1"]
  volumes:
    - ./data:/app/data
  ports:
    - "5000:5000"
    - "5001:5001"
    - "5002:5002"
```

- `sector1`: nome do serviço.
- `dockerfile`: caminho para o arquivo Dockerfile utilizado na construção da imagem.
- `command`: comando executado ao iniciar o contêiner. Quando aplicável, o segundo argumento representa o ID da entidade.
- `ports`: portas utilizadas para comunicação entre os serviços e o ambiente externo.
- `volumes`: diretórios compartilhados entre o host e o contêiner.

> [!IMPORTANT]
> Certifique-se de que serviços do mesmo tipo não utilizem o mesmo identificador, nome de contêiner ou conjunto de portas. Configurações duplicadas podem causar conflitos e inconsistências durante a execução do sistema.

### authorizedBrokers

Para imitar uma blockchain federada, o sistema conta com uma variável chamada `authorizedBrokers` em `./sector/sector.go`. Essa variável define quais companhias têm o direito de realizar depósitos e transferência de créditos. Caso uma companhia não esteja definida lá, ela não possui a habilidade de realizar suas atividades. Essa variável pode ser encontrada no campo `var` e pode ser editada pelo usuário adicionando ou removendo IDs de companhias:

```go
var (
	// ...

  authorizedBrokers                 = map[int]bool{
		1: true,
		2: true,      // ID da companhia (2) e autorização (true)
	}
)
```

---

## ▶️ Como Executar

### Requisitos

- Go/Golang.
- Docker + Docker Compose.
- Python 3 com `pygame` instalado (`pip install pygame`), apenas para o painel visual — opcional para a simulação em si.

>[!IMPORTANT]
> O `go.mod` declara `go 1.26`; ajuste essa diretiva se estiver usando outra versão.

### Compilação e Execução

A compilação e execução dos serviços são realizadas com o auxílio do `docker-compose.yml`. O primeiro passo é executar o seguinte comando na raiz do projeto:

```bash
docker compose up -d --build
```

Após a conclusão do comando, todos os serviços definidos no arquivo `docker-compose.yml` estarão em execução. Para visualizar o log de algum deles, usamos o seguinte comando:

```bash
docker logs -f <service_name>
```

Onde `<service_name>` corresponde ao nome do serviço definido no arquivo `docker-compose.yml` (por exemplo, `sector1`).

Os serviços de companhia exigem interação direta do usuário. Por esse motivo, eles devem ser iniciados em modo interativo utilizando o comando:

```bash
docker compose run company1
```

---

## 🧪 Testes

### Gestão de Ativos

Verificou-se se as companhias conseguiam realizar depósitos para si mesma e para outras companhias. O resultado foi positivo, considerando que a companhia está registrada como autorizada:

1. Depósito de R$100,00 para própria companhia:

```
================ MENU ================
1 - Depositar dinheiro
2 - Transferir para outra Companhia
3 - Adulterar Bloco (Simular Hack)
0 - Sair
Escolha uma opção: 1
Valor do depósito: R$ 100
Comando enviado para o Setor com sucesso!
```

2. Depósito de R$50,00 para outra companhia

```
Escolha uma opção: 2
ID da Companhia destino: 2
Valor da transferência: R$ 50
Comando enviado para o Setor com sucesso!
```

3. Resultados em `sector`

```
// Resultado do primeiro teste
Pedido de depósito recebido: R$ 100.00
Consenso atingido! Bloco 1 salvo. Saldo atual: R$ 100.00 (Votos: 4/4)

// Resultado do segundo teste
Pedido de transferência de R$ 50.00 para a Companhia 2
Consenso atingido! Transferência salva no Bloco 2.

// Mensagem caso não haja saldo suficiente para transferência
Pedido de transferência de R$ 1000.00 para a Companhia 2
Transferência negada. Saldo insuficiente (R$ 0.00).

// Mensagem caso a companhia não esteja autorizada
Pedido de depósito recebido: R$ 50.00
Falha no consenso para o bloco 7 (Votos: 1, Nós Ativos: 4, Maioria necessária: 3)  // Não atinge votos suficientes
```

### Outros Testes

| Teste | Objetivo | Resultado? | Detalhamento |
|---|---|---|---|
| Desligamento de diferentes entidades | Testar ausência de **SPOF** | ✅ | O sistema continua funcionando  |
| Assinatura digital | Testar a segurança da blockchain | ⚠️ | As companhias não têm uma assinatura digital, o que significa que qualquer companhia tem acesso ao laudo completo das missões |
| Alteração de dados da blockchain | Testar se uma companhia é impossibilitada de alterar algum bloco da blockchain | ✅ | O setor reconhece uma alteração no ledger e solicita uma cópia de um setor vizinho |
| Recuperação de ledger | Testar se um setor consegue recuperar a blockchain após ser desligado | ✅ | O setor verifica que seu ledger é menor que os outros e solicita uma cópia a outro setor |

---

## 📌 Resultados e Observações

### Considerações Importantes

- A cadeia é única e compartilhada por toda a rede de setores; contas diferentes (uma por `CompanyID`/setor) convivem no mesmo livro-contábil, distinguidas apenas no momento de somar o saldo.
- A maioria exigida no consenso é dinâmica — baseada em quantos setores responderam durante aquela rodada de votação —, então a tolerância a falhas depende de quem está de fato acessível no instante da proposta.
- O código também trata, dentro do switch de mensagens de cada setor, um caso de voto recebido de forma assíncrona (`VOTE_BLOCK` como mensagem independente, acumulando em `blockVotes`); esse caminho está preparado para um modelo de votação por broadcast, mas não chega a ser acionado pelo fluxo atual de `proposeTransaction`, que já resolve a votação de forma síncrona na própria conexão.
- O estado da blockchain e da fila de requisições vive em memória de cada processo; os arquivos em `data/interface/` são apenas o reflexo usado pelo painel visual.
- Limitações herdadas do projeto original (IPs fixos nos exemplos de configuração, ausência de persistência em disco, etc.) continuam valendo — o repositório base traz mais detalhes sobre a parte P2P.

### Objetivos alcançados

* O sistema é distribuído, descentralizado e com ausência de **SPOF**
* A gestão de créditos foi bem implementada
* A alteração de blocos no ledger por parte dos setores é impossibilitada, garantindo transparência nas informações

### Melhorias Futuras

* As companhias podem possuir uma assinatura digital, de forma a manter o laudo das missões privado apenas para quem solicitou o drone
* Pode haver um filtro para o ledger, em vez de apenas imprimir a cópia completa no terminal. Poderia buscar por setor, ID do drone ou ID da requisição
* Interface própria do ledger para facilitar visualização
* Método de registro mais sofisticado que `authorizedBrokers`, evitando que o usuário precise realizar alterações no código-fonte

---

## 🎯 Contribuidores

- [Levi Nogueira Vasconcelos](https://github.com/levi-vasc)
- [Nathan de Jesus dos Santos](https://github.com/enejota-njs)

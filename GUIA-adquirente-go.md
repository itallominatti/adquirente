# Guia completo: construindo uma ADQUIRENTE do zero com Go

Este guia parte do zero. Ele não assume que você sabe o que é uma adquirente, nem que conhece Go, REST, gRPC, Kafka, Kubernetes, banco de dados, segurança, design patterns, nuvem ou as regras do Banco Central. Cada conceito é explicado antes de ser usado, com analogias, exemplos numéricos e código.

O guia tem três partes:

- **Partes 1 a 14 — os conceitos.** Primeiro o negócio (o que uma adquirente faz e por quê), depois a regulação, depois cada tecnologia. Cada parte termina com um bloco **"No nosso projeto"** dizendo exatamente onde aquele conceito entra na adquirente que você vai construir.
- **Parte 15 — o roteiro, arquivo por arquivo.** Você constrói o projeto inteiro: cada arquivo com seu caminho, o código completo e a explicação. **Todo o código foi compilado com Go 1.27 e passou em `go vet` e em 31 funções de teste (45 casos, contando os subtestes)** antes de entrar no guia.
- **Parte 16 — a nuvem.** A infraestrutura de produção na AWS, escrita em Terraform e explicada recurso por recurso, com segurança em primeiro lugar. **Todo o Terraform foi validado (`terraform validate`) com o provider AWS 6.x.**

**Pré-requisito sugerido:** o guia `simple_bank/GUIA-api-banco-go.md`. Lá você construiu uma API bancária com Go, Gin, PostgreSQL, DDD e Clean Architecture. Este guia usa a mesma organização de pastas (`domain`, `application`, `handler`, `infrastructure`) e assume que você já rodou aquele projeto. Se não rodou, tudo bem: a Parte 3 e a Parte 6 revisam o necessário, só que mais rápido.

**Sobre o código deste guia.** Os trechos das Partes 1 a 14 são exemplos curtos, escritos para explicar uma ideia por vez; alguns são esboços. A Parte 15 traz o projeto real, completo. Você vai digitar, compilar e testar na sua máquina, fase a fase; é assim que se aprende.

**Sobre as normas do Banco Central.** Regulação muda. Os números de resoluções e circulares citados aqui estavam corretos quando o guia foi escrito, mas antes de tomar qualquer decisão de negócio consulte o site do BCB (`bcb.gov.br`) e um advogado especializado. Este guia ensina engenharia; ele não substitui assessoria jurídica.

Dica de leitura: não tente decorar tudo. Leia a Parte 1 com calma (é a mais importante, porque todo o resto existe para servir a ela), passe o olho nas Partes 2 a 14, comece a Parte 15 e volte às partes de conceito quando o roteiro pedir. A Parte 16 é para quando o projeto já roda no seu computador.

---

## O que vamos construir

Uma **adquirente simulada**: o sistema que fica entre a maquininha do comerciante e o banco emissor do cartão, autoriza a compra, guarda a "agenda" do que o comerciante tem a receber, liquida (paga) esse valor na data certa e permite antecipar recebíveis.

Como não temos acesso à Visa, à Mastercard, ao Banco Central nem a um banco de verdade, vamos **simular** as pontas externas (emissor, câmara de liquidação, registradora). Tudo o que fica no meio, que é a adquirente de fato, será real: regras de negócio, banco de dados, APIs, eventos, segurança, observabilidade, deploy e infraestrutura de nuvem.

| Capacidade | Como | Partes do guia |
|---|---|---|
| Credenciar um comerciante (EC) com plano de taxas e conta para receber | API REST (`POST /v1/merchants`) + análise (KYC) | 4, 8, 15 |
| Tokenizar um cartão sem nunca guardar o número | Serviço `vault` (gRPC + AES-256-GCM) | 9, 11, 15 |
| Autorizar uma transação de cartão (débito, crédito à vista, parcelado) | API REST (`POST /v1/transactions`) + emissor simulado via gRPC | 4, 11, 15 |
| Capturar / cancelar uma transação | API REST | 15 |
| Gerar a agenda de recebíveis (o que o EC vai receber e quando) | Regras de domínio + eventos Kafka (outbox) | 1, 12, 15 |
| Liquidar (pagar) os recebíveis no vencimento, respeitando ônus | Worker diário + registradora simulada + banco simulado | 1, 2, 12, 15 |
| Antecipar recebíveis com desconto | API REST + cálculo financeiro | 1, 15 |
| Tratar chargeback (contestação) | API interna + ajuste na agenda e no ledger | 1, 15 |
| Contabilizar cada centavo em partidas dobradas | Ledger append-only | 7, 15 |
| Avisar o comerciante (webhook assinado) | Consumidor Kafka + HMAC | 9, 12, 15 |
| Autenticar e autorizar quem chama a API | OAuth2 client credentials → JWT RS256 + scopes | 10, 15 |
| Documentar a API | OpenAPI 3.1 | 5, 15 |
| Enxergar o que está acontecendo | Logs estruturados, métricas Prometheus, tracing OpenTelemetry | 13, 15 |
| Rodar em produção | Docker + Kubernetes | 14, 15 |
| Ter uma infraestrutura segura na nuvem | AWS (VPC, EKS, RDS, ElastiCache, MSK, S3, KMS, WAF, CloudTrail, GuardDuty...) com Terraform | 16 |

Tecnologias: **Go 1.27**, **Gin**, **PostgreSQL**, **GORM** e **`database/sql`** (para você comparar ORM com SQL explícito), **gRPC + Protocol Buffers (buf)**, **Kafka**, **Redis**, **OpenTelemetry + Jaeger**, **Prometheus + Grafana**, **Docker**, **Kubernetes**, **Terraform + AWS**.

---

## Mapa do guia

| Parte | Assunto | Por que existe |
|---|---|---|
| 1 | O negócio: o que é uma adquirente | Sem entender o negócio, nenhuma decisão técnica faz sentido |
| 2 | Compliance e Banco Central | O que a lei exige do sistema |
| 3 | Go | A linguagem |
| 4 | HTTP, REST, APIs e Gin | Como o mundo fala com a adquirente |
| 5 | OpenAPI / Swagger | Como documentar essa conversa |
| 6 | SOLID e Clean Architecture | Como organizar o código para ele sobreviver a anos de mudança |
| 7 | Design patterns | As soluções com nome que aparecem o tempo todo em pagamentos |
| 8 | Banco de dados e ORM | Onde o dinheiro (na forma de registros) mora |
| 9 | Segurança | Como não vazar cartão de ninguém |
| 10 | Autenticação e autorização | Quem pode fazer o quê |
| 11 | gRPC | Como serviços internos conversam rápido |
| 12 | Kafka e eventos | Como desacoplar "aconteceu" de "reagir" |
| 13 | Observabilidade: logs, métricas, tracing | Como enxergar o sistema rodando |
| 14 | Kubernetes | Como rodar e escalar em produção |
| 15 | O roteiro, arquivo por arquivo | Mão na massa: o projeto completo |
| 16 | AWS com Terraform | A infraestrutura de produção, segura, como código |
| — | Glossário e referências | Para consulta |

---

# PARTE 1 — O negócio: o que é uma adquirente

## 1.1 Quem é quem numa compra com cartão

Você passa o cartão numa padaria. Em menos de dois segundos aparece "APROVADA". Nesse intervalo, cinco empresas diferentes conversaram entre si. Vamos dar nome a cada uma:

| Papel | Quem é | O que faz | Exemplos |
|---|---|---|---|
| **Portador** | Você | Tem o cartão e quer pagar | — |
| **Estabelecimento comercial (EC)** | A padaria | Vende e quer receber | Qualquer loja |
| **Emissor** | O banco que te deu o cartão | Sabe se você tem limite/saldo; aprova ou nega; te cobra na fatura | Itaú, Nubank, Bradesco |
| **Bandeira** (ou *instituidor do arranjo*) | A "rede" impressa no cartão | Define as regras do jogo e roteia mensagens entre adquirente e emissor | Visa, Mastercard, Elo |
| **Adquirente** (ou *credenciadora*) | A empresa da maquininha | Credencia a padaria, recebe a transação, leva até a bandeira, e depois **paga a padaria** | Cielo, Rede, Stone, Getnet, PagSeguro |

Existem ainda alguns coadjuvantes:

- **Subadquirente** (ou *facilitador*, *PSP*): uma empresa que se credencia numa adquirente e revende o serviço para lojas menores, com um cadastro mais simples. O Mercado Pago, quando um pequeno vendedor recebe por ele, atua assim. Para a bandeira, o subadquirente é "um cliente da adquirente".
- **Gateway de pagamento**: só transporta a mensagem da loja virtual até a adquirente; não toca no dinheiro.
- **Registradora de recebíveis**: guarda o registro oficial de "quem tem a receber quanto, de quem e quando" (veremos em 1.6).
- **Câmara de liquidação**: a instituição que centraliza a movimentação do dinheiro entre bancos e adquirentes no fim do dia (veremos em 1.7).

O conjunto de regras que une todos esses papéis chama-se **arranjo de pagamento**. "Arranjo Visa crédito" é um arranjo; "arranjo Elo débito" é outro. Quem cria o arranjo é a bandeira; quem regula é o Banco Central.

## 1.2 A vida de uma transação

Uma transação de cartão passa por etapas bem definidas. Decore estes quatro nomes; eles vão virar estados no nosso código.

```
 Portador       EC / maquininha        ADQUIRENTE           Bandeira          Emissor
    |                 |                    |                    |                 |
    |-- passa cartão->|                    |                    |                 |
    |                 |-- 1. AUTORIZAÇÃO ->|                    |                 |
    |                 |                    |-- pedido --------->|-- pedido ------>|
    |                 |                    |                    |     (tem limite?)|
    |                 |                    |<-- aprovada -------|<-- aprovada ----|
    |                 |<-- "APROVADA" -----|                    |                 |
    |                 |                    |                    |                 |
    |                 |-- 2. CAPTURA ----->|  (confirma que a venda aconteceu)    |
    |                 |                    |                    |                 |
    |                 |                    |-- 3. LIQUIDAÇÃO (dias depois): o dinheiro sai do
    |                 |                    |    emissor, passa pela câmara e chega na conta do EC
    |                 |                    |                    |                 |
    |                 |                    |<-- 4. CHARGEBACK (às vezes): o portador contesta
```

**1. Autorização.** A maquininha manda os dados para a adquirente: valor, tipo (débito/crédito), número de parcelas e os dados do cartão. A adquirente valida (o EC existe? está ativo? o valor é positivo?), monta uma mensagem no padrão da bandeira e envia. A bandeira descobre qual é o emissor pelo início do número do cartão (o **BIN**, os primeiros 6 ou 8 dígitos) e repassa. O emissor verifica limite, senha, fraude, e responde "aprovado" com um **código de autorização** ou "negado" com um motivo. Tudo isso leva menos de dois segundos. Neste momento **nenhum dinheiro se move**; o emissor só **reservou** o valor no limite do portador.

**2. Captura.** A confirmação de que a venda de fato aconteceu. Na maquininha física, autorização e captura acontecem juntas (é o normal). No comércio eletrônico é comum autorizar na compra e capturar só quando o produto é enviado; se o pedido for cancelado antes, a reserva é desfeita e nada aparece na fatura. Só transações **capturadas** entram na agenda de recebíveis.

**3. Liquidação (settlement).** É quando o dinheiro anda de verdade. No dia combinado, o emissor paga a bandeira/câmara, a câmara paga a adquirente e a adquirente paga o EC, já descontando as taxas. Detalhes em 1.4 e 1.7.

**4. Chargeback.** O portador liga para o banco e diz "não reconheço essa compra". O emissor abre uma contestação, o valor é estornado do EC (via adquirente), e o EC pode se defender apresentando provas. Detalhes em 1.8.

Duas mensagens auxiliares que você também vai implementar:

- **Cancelamento (void)**: desfaz uma transação autorizada/capturada **no mesmo dia**, antes da liquidação.
- **Estorno (refund)**: devolve o dinheiro de uma transação **já liquidada**; gera uma nova movimentação, no sentido inverso.

## 1.3 De onde vem o dinheiro da adquirente: MDR e intercâmbio

O comerciante não recebe 100% da venda. Ele paga uma taxa chamada **MDR** (*Merchant Discount Rate*, "taxa de desconto do lojista"). Essa taxa é dividida em três:

| Fatia | Vai para | Nome |
|---|---|---|
| A maior | Emissor | **Tarifa de intercâmbio** (*interchange fee*) |
| Pequena | Bandeira | Taxa de bandeira (*scheme fee*) |
| O que sobra | Adquirente | Margem da adquirente |

Exemplo com uma venda de **R$ 100,00 no crédito à vista** e MDR de **2,5%**:

```
Valor da venda .................. R$ 100,00
MDR 2,5% ........................ R$   2,50   (o EC recebe R$ 97,50)
   └─ intercâmbio (ex.: 1,70%) .. R$   1,70   → emissor
   └─ bandeira   (ex.: 0,10%) ... R$   0,10   → bandeira
   └─ adquirente (o resto) ...... R$   0,70   → nossa receita
```

O intercâmbio é tabelado pela bandeira e varia por produto (débito é mais barato, crédito parcelado é mais caro) e por segmento do comércio (o **MCC**, *Merchant Category Code*: supermercado, posto, restaurante...). No débito, o Banco Central limita o intercâmbio (na regra vigente quando este guia foi escrito, teto médio de 0,5%). Isso significa que a adquirente **não controla a maior parte do seu custo**; ela só controla a margem. Por isso, no nosso modelo, o **plano de taxas** de cada EC guarda o MDR por produto, e o cálculo de "quanto o EC recebe" é uma regra de domínio central.

## 1.4 Débito, crédito à vista, parcelado e os prazos

O produto muda o prazo em que o EC recebe:

| Produto | O que acontece | Prazo padrão de mercado no Brasil |
|---|---|---|
| **Débito** | O dinheiro sai da conta do portador na hora | **D+1** (um dia útil depois) |
| **Crédito à vista** | O portador paga na fatura; o emissor "empresta" o prazo | **D+30** |
| **Crédito parcelado lojista** | O EC parcela sem juros; o emissor paga uma parcela por mês | Parcela 1 em D+30, parcela 2 em D+60, ... parcela *n* em D+30·*n* |
| **Crédito parcelado emissor** | O emissor parcela com juros para o portador; para o EC, é uma venda à vista | D+30 |

"D" é a data da captura. "D+30" quer dizer 30 dias corridos depois, ajustado para o próximo dia útil se cair em fim de semana/feriado. Esse ajuste de dia útil é uma regra simples de escrever, mas fácil de esquecer — e é onde muitos bugs de agenda moram.

Exemplo: venda de R$ 1.200,00 em **12x** no crédito, capturada em 10/03, MDR parcelado de 3,5%:

```
Valor bruto ..................... R$ 1.200,00
MDR 3,5% ........................ R$    42,00
Valor líquido total ............. R$ 1.158,00
Parcela líquida ................. R$    96,50  (1.158 / 12)

Agenda de recebíveis:
  parcela  1/12  R$ 96,50  vence 09/04
  parcela  2/12  R$ 96,50  vence 09/05
  ...
  parcela 12/12  R$ 96,50  vence 05/03 do ano seguinte
```

Repare no problema de arredondamento: 1.158 / 12 = 96,5 exato, mas 1.000 / 3 = 333,333... Quando não divide certo, a diferença de centavos vai para a primeira (ou última) parcela. Dinheiro em software **nunca** usa números com vírgula flutuante (`float`); usa **inteiros em centavos**. A Parte 8 explica o porquê.

## 1.5 Antecipação de recebíveis

O EC vendeu R$ 1.200,00 em 12x e vai receber ao longo de um ano. Mas ele precisa pagar o fornecedor hoje. A adquirente oferece: "eu te pago tudo agora, com um desconto". Isso é **antecipação de recebíveis** (*ARV*, antecipação de recebíveis de cartão). É uma das maiores fontes de lucro das adquirentes brasileiras.

O desconto é uma **taxa de juros** aplicada ao tempo que falta até o vencimento. A fórmula usual é a de valor presente com juros compostos, pro-rata pelos dias:

```
valor_presente = valor_futuro / (1 + taxa_mensal) ^ (dias_até_vencer / 30)
```

Exemplo com taxa de **2% ao mês**:

| Parcela | Valor futuro | Dias até vencer | Cálculo | Valor hoje |
|---|---|---|---|---|
| 1 | R$ 96,50 | 30 | 96,50 / 1,02¹ | R$ 94,61 |
| 2 | R$ 96,50 | 60 | 96,50 / 1,02² | R$ 92,75 |
| 3 | R$ 96,50 | 90 | 96,50 / 1,02³ | R$ 90,93 |
| ... | | | | |
| 12 | R$ 96,50 | 360 | 96,50 / 1,02¹² | R$ 76,09 |

Somando as 12, o EC recebe hoje **R$ 1.020,51** em vez de R$ 1.158,00 ao longo do ano. A diferença (R$ 137,49) é receita da adquirente, que vai receber os R$ 1.158,00 do emissor normalmente. (Na Parte 15 há um teste automatizado que reproduz esta tabela centavo a centavo.)

Regras que vamos codificar:
- Só se antecipa recebível **capturado** e ainda **não liquidado** e não **contestado**.
- Um recebível antecipado muda de estado (`ANTICIPATED`) e, quando o emissor pagar no vencimento, o dinheiro fica com a adquirente, não vai para o EC de novo.
- A antecipação pode ser **avulsa** (o EC pede quando quer) ou **automática** (todo dia, tudo o que entrar é antecipado; muito comum em maquininhas de pequenos comércios, onde o "recebe em 1 dia" é na verdade antecipação automática).

Um recebível antecipado é um ativo financeiro. É por isso que a antecipação é regulada e por isso existe o **registro de recebíveis**.

## 1.6 Agenda de recebíveis e as registradoras

A **agenda de recebíveis** é a lista de tudo o que um EC tem a receber: cada parcela de cada venda, com data e valor. É o coração da adquirente. Tudo gira em torno dela: liquidação paga a agenda, antecipação compra a agenda, chargeback desconta da agenda.

Desde junho de 2021 essa agenda precisa ser **registrada** em uma entidade autorizada pelo Banco Central, a **registradora** (as principais: **CERC**, **TAG**, **B3** e **Núclea**, esta última a antiga CIP). A regra foi criada pela Resolução CMN 4.734/2019 e pela Circular BCB 3.952/2019 e tem um objetivo: permitir que o lojista use a agenda de recebíveis como **garantia** de empréstimo em qualquer banco, e não só na adquirente. Antes disso, a agenda ficava "presa" na adquirente, que era a única que podia antecipar.

O que se registra é a **Unidade de Recebível (UR)**: o total que um EC (CNPJ) tem a receber de uma adquirente, num arranjo (ex.: Visa crédito), numa data de liquidação. Todas as parcelas do mesmo EC, mesma bandeira/produto, mesma data se somam numa UR.

A adquirente precisa: (a) enviar as URs à registradora todo dia, (b) consultar se há **ônus** (a UR foi dada em garantia a um banco?) antes de liquidar ou antecipar, e (c) se houver, pagar para o credor indicado e não para o EC. No nosso projeto, a registradora será simulada como um serviço interno; a regra de "não liquidar para o EC se houver ônus" será real.

## 1.7 Liquidação: como o dinheiro chega ao EC

"Liquidar" é pagar. Todo dia útil a adquirente precisa:

1. Selecionar todos os recebíveis que **vencem hoje** e não foram antecipados nem contestados.
2. Agrupar por EC e por **domicílio bancário** (a conta onde o EC quer receber).
3. Verificar na registradora se há ônus/troca de titularidade.
4. Gerar as ordens de pagamento e enviá-las.
5. Marcar os recebíveis como **liquidados** e registrar o comprovante.
6. Conciliar: conferir que o que o emissor pagou bate com o que foi pago ao EC.

No Brasil, a movimentação entre emissores e adquirentes é **centralizada** na câmara (a Núclea, por meio do sistema de liquidação de cartões, o SLC). O Banco Central obriga isso para reduzir risco: em vez de cada emissor pagar cada adquirente, todos pagam/recebem o saldo líquido pela câmara. Subadquirentes acima de um certo volume também são obrigados a participar.

O nosso projeto terá um **worker de liquidação**: um programa que roda todo dia, executa os seis passos acima e gera um "arquivo de liquidação" (uma lista de ordens de pagamento). A câmara e o banco serão simulados.

## 1.8 Chargeback e conciliação

**Chargeback** é a contestação. O emissor devolve o dinheiro ao portador e cobra da adquirente, que cobra do EC. Motivos comuns: fraude (cartão clonado), produto não entregue, cobrança duplicada. Há prazos (em geral o portador tem até 120 dias) e um fluxo de defesa: o EC apresenta comprovantes, o emissor aceita ou não, e pode haver arbitragem pela bandeira. Para a adquirente, chargeback é **risco de crédito**: se o EC já recebeu, já antecipou e fechou as portas, quem paga é a adquirente. Por isso adquirentes analisam o risco do EC no credenciamento e podem reter parte da agenda como garantia.

**Conciliação** é a conferência: o que a adquirente autorizou = o que a bandeira capturou = o que o emissor pagou = o que foi pago ao EC. Todo dia, arquivos vêm e vão entre as partes e um sistema de conciliação aponta diferenças. É trabalho de contador feito por software. No nosso projeto, faremos uma conciliação simples: somar o que foi liquidado por dia e comparar com o que a "câmara" simulada informou.

## 1.9 Outros temas que aparecem no dia a dia (e que vamos só citar)

- **ISO 8583**: o formato binário das mensagens entre maquininha, adquirente e bandeira. Antigo, cheio de campos numerados ("bit 4 = valor", "bit 39 = código de resposta"). No projeto usaremos JSON/gRPC, mas os nomes dos campos (NSU, código de autorização, código de resposta) vêm de lá.
- **NSU**: Número Sequencial Único da transação, gerado pela adquirente. Todo comprovante tem.
- **3-D Secure (3DS)**: autenticação extra no e-commerce (aquele redirecionamento para o banco). Quando usada, o risco de chargeback por fraude passa para o emissor.
- **Tokenização de cartão**: trocar o número real por um "token" que só serve para aquele EC. Essencial para segurança (Parte 9).
- **Pix**: as adquirentes hoje também oferecem Pix. É outro arranjo, com liquidação instantânea, e fica de fora deste guia.

## No nosso projeto

Tudo o que você leu vira estas entidades no domínio:

| Conceito | Entidade / regra no código |
|---|---|
| Estabelecimento comercial | `Merchant` (com `Document`, `MCC`, `BankAccount`, `FeePlan`, `Status`) |
| Plano de taxas | `FeePlan` (MDR por produto e número de parcelas, taxa de antecipação) |
| Transação e seus estados | `Transaction` com máquina de estados `PENDING → AUTHORIZED → CAPTURED → SETTLED`, e ramos `DENIED`, `CANCELED`, `CHARGEBACKED` |
| Agenda | `Receivable` (uma por parcela: bruto, taxa, líquido, vencimento, estado) |
| Liquidação | `Settlement` (lote diário por EC) e o worker |
| Antecipação | `Anticipation` (conjunto de recebíveis, desconto, líquido) |
| Contestação | `Chargeback` |
| Registradora | serviço `registry` simulado + regra "verifica ônus antes de pagar" |
| Emissor/bandeira | serviço `issuer-sim` via gRPC que aprova/nega por regras simples (ex.: cartão terminado em 0000 nega) |

---

# PARTE 2 — Compliance e as regras do Banco Central

## 2.1 O que é compliance

*Compliance* vem de *to comply*, "cumprir". É o conjunto de práticas para garantir que a empresa cumpre leis, normas do regulador e regras contratuais (como as da bandeira). Numa adquirente, compliance não é um departamento distante: ele **define requisitos do sistema**. "Guardar o histórico de cada transação por 5 anos", "não armazenar o código de segurança do cartão", "reportar operações suspeitas" são frases de compliance que viram tabela, validação e job no seu código.

Pense assim: o produto é "pagar o lojista"; compliance é "pagar o lojista de um jeito que o Banco Central, a bandeira e a Justiça aceitem".

## 2.2 A lei-mãe: Lei 12.865/2013

Até 2013 as adquirentes viviam num limbo regulatório. A **Lei 12.865/2013** criou os conceitos de **arranjo de pagamento** e **instituição de pagamento (IP)** e deu ao Banco Central (BCB) e ao Conselho Monetário Nacional (CMN) o poder de regular o setor. A partir daí vieram dezenas de resoluções e circulares. As que mais afetam uma adquirente:

| Norma | Assunto | O que exige na prática |
|---|---|---|
| **Resolução BCB 80/2021** | Constituição, autorização e funcionamento das IPs | Define as modalidades (abaixo), quando a autorização é obrigatória, capital mínimo, governança |
| **Resolução BCB 81/2021** | Arranjos de pagamento | Regras de participação, interoperabilidade, o que a bandeira pode exigir |
| **Resolução CMN 4.734/2019 + Circular BCB 3.952/2019** | Registro de recebíveis de cartão | Registrar as URs em uma registradora, respeitar ônus e garantias |
| **Resolução BCB 85/2021** | Política de segurança cibernética das IPs | Política formal, plano de resposta a incidentes, controles sobre nuvem, reporte de incidentes ao BCB |
| **Circular BCB 3.978/2020** | Prevenção à lavagem de dinheiro e financiamento do terrorismo (PLD/FT) | Conhecer o cliente (KYC), monitorar, comunicar ao COAF operações suspeitas |
| **Lei 13.709/2018 (LGPD)** | Proteção de dados pessoais | Base legal para tratar dados, minimização, direito de exclusão, relatório de impacto |
| **Regras das bandeiras + PCI DSS** | Contratual (não é lei) | Segurança dos dados do cartão; auditoria anual |

## 2.3 Instituição de pagamento e suas modalidades

A Resolução BCB 80 define modalidades de IP. Uma empresa pode ser mais de uma:

| Modalidade | O que faz | Exemplo |
|---|---|---|
| **Emissor de moeda eletrônica** | Mantém contas de pagamento pré-pagas | Carteiras digitais, contas de fintechs |
| **Emissor de instrumento de pagamento pós-pago** | Emite cartão de crédito | Bancos e fintechs de cartão |
| **Credenciador** | **É a adquirente.** Habilita ECs a aceitar cartão e participa da liquidação | Cielo, Stone |
| **Iniciador de transação de pagamento (ITP)** | Inicia pagamentos em nome do usuário (Open Finance) | Apps que "puxam" um Pix da sua conta |

A adquirente do nosso guia é um **credenciador**. Pontos que importam para o sistema:

- **Autorização do BCB.** Não é qualquer empresa que pode "ser" credenciador. Acima de um volume (na regra vigente na escrita, R$ 500 milhões em transações acumuladas em 12 meses) a autorização prévia do BCB é obrigatória; abaixo, a empresa opera como participante do arranjo sob as regras da bandeira, mas já está sujeita à regulação. **Verifique os valores atuais no site do BCB.**
- **Subcredenciador (subadquirente).** Se você não tem contrato direto com a bandeira e opera "dentro" de uma adquirente, você é subcredenciador. Acima de certo volume, é obrigado a liquidar de forma centralizada pela câmara. Muitos projetos começam assim.
- **Segregação de recursos.** O dinheiro dos ECs que passa pela adquirente **não é da adquirente**. Precisa ficar separado do patrimônio dela (em conta específica) e não pode ser usado para pagar as contas da empresa. No sistema, isso vira contabilidade separada: "saldo a repassar" nunca se mistura com "receita".
- **Relatórios ao BCB.** IPs enviam informações periódicas (volumes, reclamações, incidentes). No sistema: dados precisam ser consultáveis por período com facilidade.

## 2.4 Registro de recebíveis: o que o sistema precisa fazer

Já vimos o conceito em 1.6. Aqui, as obrigações:

1. **Registrar diariamente** todas as URs (EC + arranjo + adquirente + data de liquidação, valor total) na registradora escolhida. As registradoras são interoperáveis: registrar em uma vale para todas.
2. **Consultar antes de pagar.** No dia da liquidação, para cada UR, perguntar à registradora: há **ônus** (garantia dada a um banco)? Há **troca de titularidade** (o EC vendeu o recebível)? Se sim, pagar ao credor indicado, no valor indicado.
3. **Registrar as antecipações.** Quando a adquirente antecipa, ela passa a ser dona daquele recebível; isso é uma troca de titularidade e deve constar no registro.
4. **Manter trilha de auditoria.** Cada alteração na agenda precisa ter quem, quando e por quê.

Regra de ouro do domínio: **nenhum pagamento ao EC sai sem passar pela verificação de ônus**. No nosso código, o caso de uso `SettleReceivables` chama o port `ReceivablesRegistry.CheckLiens(...)` antes de gerar qualquer ordem.

## 2.5 KYC, PLD/FT e o credenciamento

**KYC** (*Know Your Customer*, "conheça seu cliente") é o processo de saber quem é o EC antes de credenciar: CNPJ ativo, sócios, atividade (MCC) compatível com o faturamento, se está em listas de sanções, se é pessoa politicamente exposta (PEP). **PLD/FT** é a prevenção à lavagem de dinheiro e ao financiamento do terrorismo: monitorar padrões estranhos (um salão de beleza que fatura R$ 2 milhões/mês em transações de R$ 9.999,00) e comunicar ao **COAF**.

No sistema:
- Credenciamento tem um estado `UNDER_REVIEW` antes de `ACTIVE`; a ativação é uma decisão, não um `INSERT`.
- Dados cadastrais têm histórico (você precisa provar o que sabia sobre o EC na data X).
- Existe um job de monitoramento que gera alertas (regras simples: valor médio, quantidade por dia, transações fora do horário).

## 2.6 LGPD

A **Lei Geral de Proteção de Dados** vale para todo dado que identifique uma pessoa: nome, CPF, e-mail, e também o **número do cartão**. Princípios que viram requisito:

- **Finalidade e minimização**: só colete o que precisa. Não guarde o CVV; não guarde o número do cartão se um token resolve.
- **Base legal**: para processar pagamento, a base é "execução de contrato" e "obrigação legal"; não precisa de consentimento, mas precisa estar documentado.
- **Direitos do titular**: acesso, correção, exclusão. Exclusão em pagamentos conflita com a obrigação de guardar histórico; a saída é **anonimizar** (manter a transação, remover o vínculo com a pessoa) após o prazo legal.
- **Segurança**: criptografia, controle de acesso, registro de quem acessou o quê.
- **Incidentes**: vazou, tem que comunicar a ANPD e os titulares.

## 2.7 PCI DSS: a regra das bandeiras sobre dados de cartão

O **PCI DSS** (*Payment Card Industry Data Security Standard*) é um padrão criado pelas bandeiras. Não é lei, é contrato: quem não cumpre perde o direito de processar cartão. A versão vigente é a 4.0. Ele tem 12 requisitos; os que mais mudam o seu código:

| Requisito | Tradução para o desenvolvedor |
|---|---|
| Proteger dados de conta armazenados | **Nunca** guardar o CVV/CVC, a trilha magnética ou o PIN, nem "temporariamente", nem em log. O PAN (número do cartão), se precisar guardar, só cifrado ou tokenizado, com chaves gerenciadas separadamente |
| Mascarar o PAN ao exibir | Mostrar no máximo o BIN e os 4 últimos dígitos: `4111 11** **** 1111` |
| Criptografar em trânsito | TLS 1.2+ em tudo; nada de HTTP puro, nem interno |
| Controle de acesso por necessidade | Quem opera suporte não vê PAN completo; cada acesso a dado de cartão é registrado |
| Registrar e monitorar todo acesso | Logs de auditoria imutáveis, com data/hora sincronizada (NTP), guardados por pelo menos 1 ano |
| Testar segurança regularmente | Scans de vulnerabilidade, pentest anual, revisão de código |
| Desenvolver software com segurança | Revisão de código, dependências atualizadas, ambientes separados, sem dados reais em teste |

O jeito mais inteligente de cumprir o PCI é **reduzir o escopo**: quanto menos partes do sistema tocam no número real do cartão, menos partes precisam de auditoria. A técnica é a **tokenização**: um componente pequeno e isolado (o *vault*) recebe o PAN, guarda cifrado e devolve um token aleatório. Todo o resto do sistema só vê o token. No nosso projeto, o vault será um serviço separado, e a tabela `transactions` guarda `card_token`, `card_brand`, `card_last4` e `card_bin`, nunca o PAN.

## 2.8 Checklist: como compliance vira código

Use esta tabela quando estiver escrevendo o projeto. Cada linha é um requisito que você vai implementar em alguma fase da Parte 14.

| Exigência | Onde entra no sistema | Fase |
|---|---|---|
| Nunca guardar CVV; PAN só tokenizado | Vault + validação no handler que rejeita `cvv` no corpo depois da autorização | 3 |
| Mascarar PAN em toda saída (API, log, tela) | Função `MaskPAN`; `slog` com *redaction* | 3, 8 |
| Trilha de auditoria de alterações na agenda | Tabela `receivable_events` (append-only) | 4 |
| Verificar ônus antes de liquidar | Port `ReceivablesRegistry` no caso de uso de liquidação | 5 |
| Segregar dinheiro do EC da receita | Contas contábeis separadas: `payable_to_merchant`, `revenue_fees`, `revenue_anticipation` | 4, 5, 6 |
| Credenciamento com análise (KYC) | Estado `UNDER_REVIEW`, endpoint de aprovação com permissão específica | 2 |
| Monitoramento PLD/FT | Job com regras simples de alerta | 7 |
| Guardar histórico por 5 anos; anonimizar dados pessoais depois | Política de retenção, job de anonimização | 10 |
| Registrar acessos a dados sensíveis | Middleware de auditoria em rotas sensíveis | 8 |
| TLS em tudo, inclusive interno | Ingress com TLS; mTLS entre serviços | 9, 10 |
| Plano de resposta a incidente; comunicar BCB/ANPD | Runbook (documento) + alertas | 8, 10 |
| Ambientes separados; sem dado real em teste | Docker Compose para dev; dados sintéticos | 0 |

---

# PARTE 3 — Go: o que você precisa dominar

O guia do `simple_bank` deu uma visão de cinco minutos de Go. Aqui vamos mais fundo, porque uma adquirente usa concorrência, erros ricos, interfaces bem desenhadas e testes de verdade.

## 3.1 Instalação, módulos e ferramentas

Instale o Go em `go.dev/dl`. Confira com `go version`. Um projeto Go é um **módulo**, identificado pelo arquivo `go.mod`:

```bash
mkdir adquirente && cd adquirente
go mod init github.com/seu-usuario/adquirente
```

O nome do módulo é o prefixo de todos os imports internos (`github.com/seu-usuario/adquirente/internal/domain`). Comandos que você vai usar todo dia:

| Comando | O que faz |
|---|---|
| `go run ./cmd/api` | Compila e executa |
| `go build ./...` | Compila tudo (`./...` = "esta pasta e todas as subpastas") |
| `go test ./...` | Roda todos os testes |
| `go test -race ./...` | Roda os testes procurando *race conditions* (bugs de concorrência) |
| `go vet ./...` | Procura erros prováveis (formatação errada em `Printf`, etc.) |
| `go fmt ./...` | Formata o código no estilo oficial (não há discussão de estilo em Go) |
| `go mod tidy` | Adiciona/remove dependências conforme o código usa |
| `go get pacote@versao` | Adiciona uma dependência |

Instale também o **golangci-lint** (agrega dezenas de linters) e o **staticcheck**. Rode-os no CI.

## 3.2 Tipos, structs, ponteiros, slices e maps

```go
// Tipos básicos
var idade int = 30        // int, int64, uint8...
var preco int64 = 1050    // dinheiro: inteiro em centavos (R$ 10,50)
var nome string = "Ana"
var ativo bool = true

// Tipo nomeado: dá significado e permite métodos
type Money int64

func (m Money) String() string { /* formata "R$ 10,50" */ return "" }

// Struct: agrupa campos
type Merchant struct {
    ID       string
    Document string
    Balance  Money
}

// Ponteiro: "endereço de". Métodos que ALTERAM a struct precisam de ponteiro.
func (m *Merchant) Credit(v Money) { m.Balance += v }   // altera o original
func (m Merchant) Snapshot() Merchant { return m }      // recebe cópia

// Slice: lista de tamanho variável
ids := []string{"a", "b"}
ids = append(ids, "c")
for i, id := range ids { fmt.Println(i, id) }

// Map: dicionário chave → valor
mdr := map[string]float64{"DEBIT": 1.5, "CREDIT": 2.5}
taxa, existe := mdr["PIX"] // existe == false, taxa == 0
```

Quando usar ponteiro? Regra prática: entidades (coisas com identidade que mudam, como `Merchant`, `Transaction`) andam como `*Merchant`. Objetos de valor pequenos e imutáveis (`Money`, `Document`) andam por cópia.

**Zero value.** Toda variável não inicializada tem um valor padrão: `0`, `""`, `false`, `nil`. Um `*Merchant` não inicializado é `nil`, e acessar um campo dele derruba o programa (*nil pointer dereference*). Verifique `if m == nil`.

## 3.3 Erros: o jeito Go

Em Go, erro é um valor devolvido, não uma exceção. Três técnicas que você vai usar o tempo todo:

**Erros sentinela** (constantes para comparar):

```go
var ErrInsufficientFunds = errors.New("saldo insuficiente")

if errors.Is(err, ErrInsufficientFunds) { /* 422 */ }
```

**Embrulhar (wrap) com contexto**, mantendo a causa:

```go
if err := repo.Save(ctx, tx); err != nil {
    return fmt.Errorf("salvar transação %s: %w", tx.ID, err) // %w preserva a cadeia
}
// Lá em cima, errors.Is(err, ErrInsufficientFunds) continua funcionando.
```

**Erros com dados** (tipo próprio) e `errors.As`:

```go
type DeclinedError struct {
    Code   string // "51" = saldo insuficiente, no padrão ISO 8583
    Reason string
}

func (e *DeclinedError) Error() string { return "negada: " + e.Reason }

var de *DeclinedError
if errors.As(err, &de) {
    fmt.Println(de.Code)
}
```

Regras: trate o erro **uma vez** (ou repassa com contexto, ou loga e decide; nunca os dois); nunca ignore com `_`; não use `panic` para erro de negócio.

## 3.4 Interfaces e composição

Uma interface é um contrato de métodos. Em Go a implementação é **implícita**: não existe `implements`. Isso permite definir a interface **no pacote que usa**, e não no que implementa, que é exatamente o que a Clean Architecture pede.

```go
// domain/repository.go — o domínio define O QUE precisa
type MerchantRepository interface {
    FindByID(ctx context.Context, id string) (*Merchant, error)
    Save(ctx context.Context, m *Merchant) error
}

// infrastructure/postgres/merchant_repository.go — a infraestrutura diz COMO
type MerchantRepository struct{ db *sql.DB }
func (r *MerchantRepository) FindByID(ctx context.Context, id string) (*domain.Merchant, error) { /* SQL */ return nil, nil }
func (r *MerchantRepository) Save(ctx context.Context, m *domain.Merchant) error { return nil }

// Garantia em tempo de compilação de que implementa o contrato:
var _ domain.MerchantRepository = (*MerchantRepository)(nil)
```

Mantenha interfaces **pequenas** (1 a 4 métodos). Interfaces grandes são difíceis de implementar e de mockar.

**Composição em vez de herança.** Go não tem herança. Você "embute" uma struct em outra:

```go
type Auditable struct{ CreatedAt, UpdatedAt time.Time }
type Merchant struct {
    Auditable          // Merchant "ganha" CreatedAt/UpdatedAt
    ID string
}
```

## 3.5 Generics (o mínimo)

Desde o Go 1.18 dá para escrever funções que valem para vários tipos:

```go
func Map[T, U any](in []T, f func(T) U) []U {
    out := make([]U, 0, len(in))
    for _, v := range in { out = append(out, f(v)) }
    return out
}

netAmounts := Map(receivables, func(r Receivable) Money { return r.Net })
```

Use com moderação: para coleções e utilitários, sim; para modelar o domínio, quase nunca.

## 3.6 Concorrência: goroutines, channels, sync e context

Uma adquirente processa milhares de transações por segundo e roda workers em paralelo. Go torna isso barato.

**Goroutine** é uma função rodando concorrentemente. Custa poucos KB; você pode ter milhares.

```go
go processar(tx) // dispara e não espera
```

**Channel** é um cano tipado por onde goroutines trocam dados com segurança:

```go
jobs := make(chan Receivable, 100)     // canal com buffer de 100
results := make(chan error, 100)

// 5 workers lendo do mesmo canal
for i := 0; i < 5; i++ {
    go func() {
        for r := range jobs {          // sai do loop quando o canal é fechado
            results <- settle(r)
        }
    }()
}

for _, r := range receivables { jobs <- r }
close(jobs)
```

**sync.WaitGroup** espera um grupo terminar; **sync.Mutex** protege um dado compartilhado:

```go
var wg sync.WaitGroup
var mu sync.Mutex
total := Money(0)

for _, r := range receivables {
    wg.Add(1)
    go func(r Receivable) {
        defer wg.Done()
        mu.Lock()
        total += r.Net
        mu.Unlock()
    }(r)
}
wg.Wait()
```

Sem o `mu`, dois workers somando ao mesmo tempo produzem um total errado. Isso é uma **race condition**. `go test -race` detecta.

**errgroup** (`golang.org/x/sync/errgroup`) é a forma prática de rodar N tarefas e pegar o primeiro erro:

```go
g, ctx := errgroup.WithContext(ctx)
for _, batch := range batches {
    g.Go(func() error { return settleBatch(ctx, batch) })
}
if err := g.Wait(); err != nil { return err }
```

**context.Context** carrega cancelamento, prazo (*deadline*) e valores de escopo de requisição (como o *trace id*). Regra: primeiro parâmetro de toda função que faz I/O; nunca guardado em struct.

```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Second) // emissor tem 2s para responder
defer cancel()
resp, err := issuer.Authorize(ctx, req)
if errors.Is(err, context.DeadlineExceeded) { /* tratar como "negada por timeout" */ }
```

Em pagamentos, o **timeout é uma decisão de negócio**: se o emissor não respondeu em 2 segundos, a maquininha vai mostrar "erro"; se a resposta chegar depois e for "aprovada", você precisa **desfazer** (mandar um *reversal*), senão o portador é cobrado por uma compra que não saiu. Isso se chama *late response* e é um dos problemas clássicos do setor.

## 3.7 Testes

Go tem testes na biblioteca padrão. Arquivo `x_test.go`, função `TestX(t *testing.T)`.

**Table-driven tests** (o padrão da comunidade):

```go
func TestCalculateNet(t *testing.T) {
    cases := []struct {
        name     string
        gross    Money
        mdrBps   int   // taxa em "basis points": 250 = 2,50%
        wantNet  Money
    }{
        {"crédito 2,5%", 10000, 250, 9750},
        {"débito 1,5%", 10000, 150, 9850},
        {"arredonda para baixo", 1001, 250, 976}, // 1001 - 25,025 → taxa 25 (ou 26, decida e documente)
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := CalculateNet(tc.gross, tc.mdrBps)
            if got != tc.wantNet {
                t.Fatalf("got %d want %d", got, tc.wantNet)
            }
        })
    }
}
```

**Mocks/fakes**: como as interfaces são pequenas, você escreve um fake à mão:

```go
type fakeIssuer struct{ approve bool }
func (f fakeIssuer) Authorize(ctx context.Context, r AuthRequest) (AuthResponse, error) {
    if f.approve { return AuthResponse{Approved: true, Code: "123456"}, nil }
    return AuthResponse{Approved: false, ReasonCode: "51"}, nil
}
```

**Testes de integração** com banco real usam **testcontainers-go** (sobe um PostgreSQL em Docker só para o teste). Marque-os com uma *build tag* (`//go:build integration`) para não rodar no `go test` normal.

**Cobertura**: `go test -cover ./...`. Mire alto no domínio (regras de dinheiro merecem 100%) e seja pragmático nas camadas externas.

## 3.8 Layout de projeto

Convenção da comunidade (não imposta pela linguagem):

```
adquirente/
├── cmd/                    # um main.go por executável
│   ├── api/main.go         # servidor REST
│   ├── issuer-sim/main.go  # emissor simulado (gRPC)
│   └── settlement/main.go  # worker de liquidação
├── internal/               # código privado do módulo (o Go impede import de fora)
│   ├── domain/             # entidades, value objects, regras, interfaces (ports)
│   ├── application/        # casos de uso
│   ├── infrastructure/     # postgres, kafka, grpc clients, redis...
│   └── handler/            # HTTP (Gin), gRPC servers
├── api/                    # openapi.yaml, *.proto
├── deploy/                 # Dockerfiles, k8s/, helm/
├── migrations/
├── go.mod
└── Makefile
```

## No nosso projeto

- Dinheiro é `Money int64` em centavos; taxas são `int` em *basis points* (1 bps = 0,01%). Nada de `float64` no domínio.
- Todo caso de uso recebe `ctx` e propaga para repositórios e clientes.
- O autorizador chama o emissor com `context.WithTimeout` e trata *late response* com reversal.
- O worker de liquidação usa `errgroup` para processar lotes de ECs em paralelo, com limite de concorrência.
- Domínio: testes table-driven com cobertura próxima de 100%. Infra: testes de integração com testcontainers.

---

# PARTE 4 — HTTP, REST, APIs e Gin

## 4.1 HTTP a fundo

HTTP é o protocolo de "pedido e resposta" da web. Um pedido (*request*) e uma resposta (*response*) são texto com uma estrutura fixa:

```
POST /v1/transactions HTTP/1.1            ← linha de pedido: método, caminho, versão
Host: api.adquirente.com                  ← cabeçalhos (headers): metadados
Content-Type: application/json
Authorization: Bearer eyJhbGciOi...
Idempotency-Key: 7c3a9f2e-...
                                          ← linha em branco separa cabeçalhos do corpo
{"merchant_id":"m_123","amount":10000,"product":"CREDIT","installments":1,"card_token":"tok_abc"}
```

```
HTTP/1.1 201 Created                      ← linha de status
Content-Type: application/json
X-Request-Id: 9b1f...
Location: /v1/transactions/tx_987

{"id":"tx_987","status":"AUTHORIZED","authorization_code":"123456","nsu":"000000123"}
```

**Métodos** e o que prometem:

| Método | Uso | Seguro? (não altera nada) | Idempotente? (repetir dá o mesmo resultado) |
|---|---|---|---|
| `GET` | Ler | Sim | Sim |
| `POST` | Criar / executar ação | Não | **Não** (por padrão) |
| `PUT` | Substituir inteiro | Não | Sim |
| `PATCH` | Alterar parte | Não | Não (por padrão) |
| `DELETE` | Remover | Não | Sim |

**Status codes** que uma API de pagamentos usa:

| Código | Quando |
|---|---|
| 200 OK | Leitura ou ação concluída |
| 201 Created | Recurso criado (devolve `Location`) |
| 202 Accepted | Recebi, vou processar depois (ex.: pedido de antecipação em lote) |
| 400 Bad Request | JSON inválido, campo faltando |
| 401 Unauthorized | Sem credencial ou credencial inválida |
| 403 Forbidden | Credencial válida, mas sem permissão |
| 404 Not Found | Recurso não existe |
| 409 Conflict | Estado impede (capturar transação já cancelada; chave de idempotência reutilizada com corpo diferente) |
| 422 Unprocessable Entity | Regra de negócio (EC inativo, valor acima do limite) |
| 429 Too Many Requests | Passou do *rate limit* |
| 500 Internal Server Error | Bug nosso |
| 502/503/504 | Dependência fora (emissor não respondeu) |

Cabeçalhos importantes: `Content-Type` (formato do corpo), `Authorization` (credencial), `X-Request-Id` (identificador para rastrear), `Idempotency-Key` (ver 4.3), `Retry-After` (em 429/503).

## 4.2 REST: desenhando uma boa API

**REST** organiza a API em **recursos** (substantivos) manipulados por **verbos HTTP**. Regras práticas:

- Recursos no plural: `/merchants`, `/transactions`, `/receivables`.
- Identificador na rota: `/transactions/tx_987`.
- Sub-recursos para relações: `/merchants/m_123/receivables`.
- Ações que não cabem em CRUD viram um sub-recurso com `POST`: `/transactions/tx_987/capture`, `/transactions/tx_987/cancel`, `/anticipations`.
- **Versione** na rota: `/v1/...`. Nunca quebre um cliente que já está em produção.
- **Pagine** listas: `?limit=50&cursor=abc`. Paginação por cursor é melhor que por página quando a lista muda.
- **Filtre** com query string: `/receivables?status=SCHEDULED&due_date=2026-10-01`.
- **Erros padronizados**. Use o formato *Problem Details* (RFC 9457):

```json
{
  "type": "https://api.adquirente.com/errors/insufficient-limit",
  "title": "Transação negada",
  "status": 422,
  "detail": "Emissor negou a transação (código 51: saldo insuficiente)",
  "instance": "/v1/transactions/tx_987",
  "code": "ISSUER_DECLINED",
  "issuer_response_code": "51"
}
```

O cliente decide o que fazer pelo campo `code` (estável), não pelo `detail` (texto para humanos).

## 4.3 Idempotência: a regra número um de uma API de pagamentos

Cenário: a maquininha manda `POST /transactions`, a rede cai antes da resposta chegar. A maquininha não sabe se a compra foi autorizada. Ela tenta de novo. Sem cuidado, o portador é cobrado duas vezes.

Solução: o cliente manda um cabeçalho **`Idempotency-Key`** com um identificador único gerado por ele (um UUID). O servidor:

1. Procura a chave. Se já existe **com o mesmo corpo**, devolve a **mesma resposta** de antes, sem executar de novo.
2. Se existe com corpo diferente, devolve `409 Conflict`.
3. Se não existe, executa, salva `(chave → status + corpo da resposta)` e responde.

Isso precisa ser atômico: dois pedidos iguais **ao mesmo tempo** não podem passar os dois. A implementação usa uma tabela com `UNIQUE (merchant_id, idempotency_key)` ou um `SET NX` no Redis com prazo (24h é comum). Vamos implementar como middleware do Gin.

## 4.4 Gin

O Gin cuida de rotas, leitura de JSON, middlewares e respostas. O que você precisa saber além do `simple_bank`:

**Grupos e middlewares por grupo:**

```go
r := gin.New()
r.Use(gin.Recovery(), RequestID(), Logger(), otelgin.Middleware("api"))

v1 := r.Group("/v1")
v1.Use(Authenticate(jwks))                              // todo /v1 exige token

merchants := v1.Group("/merchants")
merchants.POST("", Authorize("merchants:write"), h.Create)
merchants.POST("/:id/approve", Authorize("merchants:approve"), h.Approve)

tx := v1.Group("/transactions")
tx.Use(Idempotency(store))                              // só aqui
tx.POST("", Authorize("transactions:write"), h.Authorize)
tx.POST("/:id/capture", Authorize("transactions:write"), h.Capture)
```

**Binding e validação** com tags:

```go
type AuthorizeRequest struct {
    MerchantID   string `json:"merchant_id"  binding:"required"`
    Amount       int64  `json:"amount"       binding:"required,gt=0"`
    Product      string `json:"product"      binding:"required,oneof=DEBIT CREDIT"`
    Installments int    `json:"installments" binding:"required,min=1,max=12"`
    CardToken    string `json:"card_token"   binding:"required"`
}

func (h *TransactionHandler) Authorize(c *gin.Context) {
    var req AuthorizeRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
        return
    }
    // ... chama o caso de uso
}
```

Validação de formato fica no handler (Gin). Validação de **regra de negócio** ("EC inativo", "parcelas acima do permitido no plano") fica no domínio. Não misture.

**Middleware** é uma função que roda antes/depois do handler. Um `RequestID`:

```go
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        id := c.GetHeader("X-Request-Id")
        if id == "" { id = uuid.NewString() }
        c.Set("request_id", id)
        c.Header("X-Request-Id", id)
        c.Next() // chama o próximo da cadeia
    }
}
```

## 4.5 Timeouts, rate limit e desligamento gracioso

**Timeouts no servidor** evitam que uma conexão lenta segure recursos para sempre:

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           r,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      15 * time.Second,
    IdleTimeout:       60 * time.Second,
}
```

**Rate limit** protege de abuso e de clientes com bug em loop: N pedidos por segundo por credencial, resposta `429` com `Retry-After`. Implementação comum: *token bucket* no Redis.

**Graceful shutdown**: quando o Kubernetes manda parar (SIGTERM), pare de aceitar conexões novas, termine as em andamento e só então saia. Sem isso, um deploy no meio de uma autorização gera transação "no limbo".

```go
go func() { _ = srv.ListenAndServe() }()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()
_ = srv.Shutdown(ctx)
```

## 4.6 Webhooks: quando a adquirente é quem chama

Quando algo acontece de forma assíncrona (liquidação concluída, chargeback recebido), a adquirente avisa o EC chamando uma URL que ele cadastrou. Isso é um **webhook**. Regras:

- Assine o corpo com HMAC (Parte 9) para o EC saber que foi você.
- Inclua um `event_id` para o EC descartar duplicatas.
- Tente de novo com *backoff* exponencial se o EC responder erro ou não responder (1min, 5min, 30min, 2h...).
- Não bloqueie o fluxo principal: o envio sai de um consumidor Kafka, não do handler.

## No nosso projeto

- API REST `/v1` com Gin, erros em *Problem Details* com campo `code`.
- Middleware `Idempotency` em todo `POST` que cria dinheiro ou movimento (transações, capturas, antecipações, estornos).
- Middlewares: `RequestID`, `Logger`, `Recovery`, `Authenticate`, `Authorize(scope)`, `RateLimit`, `otelgin`.
- Webhooks assinados e com retry, enviados por um serviço `notifier` que consome Kafka.
- Timeouts em todo servidor e cliente; graceful shutdown em todo binário.

---

# PARTE 5 — OpenAPI / Swagger: documentando a API

## 5.1 O que é

**OpenAPI** é um formato padrão (YAML ou JSON) para descrever uma API REST: rotas, parâmetros, corpos, respostas, erros, autenticação. **Swagger** é o nome antigo do padrão e o nome atual de um conjunto de ferramentas (Swagger UI, Swagger Editor) que leem esse formato.

Por que importa: o EC que vai integrar com você não vai ler seu código Go. Ele vai ler a documentação. Com OpenAPI, essa documentação é **executável**: gera uma tela interativa para testar, gera clientes em várias linguagens, valida se a implementação cumpre o contrato e alimenta ferramentas de teste.

## 5.2 Duas formas de trabalhar

**Design-first**: você escreve o `openapi.yaml` antes do código, discute com quem vai consumir, e depois implementa. É o que grandes APIs de pagamento fazem. Vantagem: o contrato é pensado, não "sai" do código.

**Code-first**: você anota o código Go com comentários e uma ferramenta (`swaggo/swag`) gera o `openapi.yaml`. Vantagem: nunca fica desatualizado. Desvantagem: a documentação sai com a cara do código.

Vamos fazer **design-first** para a API pública (a que o EC usa) e gerar o servidor Go a partir do spec com o **oapi-codegen**. Assim o contrato manda, e o Go obedece.

## 5.3 O spec

Um trecho do `api/openapi.yaml` da adquirente:

```yaml
openapi: 3.1.0
info:
  title: Adquirente API
  version: 1.0.0
  description: API pública para credenciamento, transações, recebíveis e antecipação.
servers:
  - url: https://api.adquirente.com/v1
security:
  - bearerAuth: []

paths:
  /transactions:
    post:
      operationId: authorizeTransaction
      summary: Autoriza uma transação de cartão
      parameters:
        - $ref: '#/components/parameters/IdempotencyKey'
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/AuthorizeRequest'
      responses:
        '201':
          description: Transação autorizada
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Transaction'
        '422':
          $ref: '#/components/responses/BusinessError'
        '409':
          $ref: '#/components/responses/Conflict'

  /transactions/{id}/capture:
    post:
      operationId: captureTransaction
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string }
      responses:
        '200':
          description: Capturada
          content:
            application/json:
              schema: { $ref: '#/components/schemas/Transaction' }

components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
  parameters:
    IdempotencyKey:
      name: Idempotency-Key
      in: header
      required: true
      schema: { type: string, format: uuid }
  schemas:
    AuthorizeRequest:
      type: object
      required: [merchant_id, amount, product, installments, card_token]
      properties:
        merchant_id:  { type: string, example: m_01HZX }
        amount:       { type: integer, minimum: 1, description: Valor em centavos, example: 10000 }
        product:      { type: string, enum: [DEBIT, CREDIT] }
        installments: { type: integer, minimum: 1, maximum: 12 }
        card_token:   { type: string, description: Token gerado pelo vault; nunca o PAN }
    Transaction:
      type: object
      properties:
        id:                 { type: string }
        status:             { type: string, enum: [AUTHORIZED, CAPTURED, DENIED, CANCELED, SETTLED, CHARGEBACKED] }
        authorization_code: { type: string }
        nsu:                { type: string }
        card_last4:         { type: string, example: "1111" }
    Problem:
      type: object
      properties:
        type: { type: string }
        title: { type: string }
        status: { type: integer }
        detail: { type: string }
        code: { type: string }
  responses:
    BusinessError:
      description: Regra de negócio impediu
      content:
        application/problem+json:
          schema: { $ref: '#/components/schemas/Problem' }
    Conflict:
      description: Conflito de estado ou de idempotência
      content:
        application/problem+json:
          schema: { $ref: '#/components/schemas/Problem' }
```

Repare que o spec **documenta decisões de segurança**: `card_token` diz explicitamente "nunca o PAN"; `card_last4` mostra que só os 4 últimos são expostos.

## 5.4 Gerando código Go a partir do spec

```bash
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
oapi-codegen -generate gin,types,spec -package openapi -o internal/handler/openapi/gen.go api/openapi.yaml
```

Isso gera: as structs de request/response, uma interface `ServerInterface` com um método por `operationId` (`AuthorizeTransaction(c *gin.Context, params ...)`), e o registro de rotas. Você implementa a interface; se o spec mudar e você esquecer de implementar algo, **o compilador avisa**.

## 5.5 Swagger UI e validação

- Sirva o spec em `/openapi.yaml` e o **Swagger UI** em `/docs` (há um middleware pronto; ou sirva o HTML estático do Swagger UI apontando para o spec).
- Em desenvolvimento, ative o middleware `OapiRequestValidator` do oapi-codegen: ele rejeita pedidos que não batem com o spec **antes** de chegar ao handler.
- No CI, rode um *linter* de OpenAPI (Spectral) para pegar rotas sem exemplos, erros não documentados, etc.

## No nosso projeto

- `api/openapi.yaml` é a fonte da verdade da API pública. Alterar a API = alterar o spec primeiro.
- Na Parte 15 os handlers Gin são escritos à mão, para você ver cada linha; gerar a interface do servidor com `oapi-codegen` a partir do spec fica como exercício da Fase 10.
- A API serve o spec em `/openapi.yaml` fora de produção; aponte o Swagger Editor (`editor.swagger.io`) ou o Swagger UI para ele.
- A API interna entre serviços **não** usa OpenAPI: usa gRPC, que tem seu próprio contrato (`.proto`).

---

# PARTE 6 — SOLID e Clean Architecture

## 6.1 Por que arquitetura importa numa adquirente

Uma adquirente vive décadas. A bandeira muda o protocolo, o Banco Central cria uma regra nova, o banco troca o formato do arquivo de liquidação, o time troca PostgreSQL por outro banco. Se o cálculo do MDR estiver misturado com SQL e com JSON do Gin, cada uma dessas mudanças vira uma cirurgia. Arquitetura é a arte de **isolar o que muda por motivos diferentes**.

## 6.2 SOLID, com exemplos do nosso domínio

**S — Single Responsibility (responsabilidade única).** Cada tipo tem **um motivo para mudar**. `Transaction` muda quando a regra de negócio da transação muda. `TransactionRepository` muda quando o banco muda. `TransactionHandler` muda quando a API muda. Se um arquivo muda por três motivos, divida.

**O — Open/Closed (aberto para extensão, fechado para modificação).** Adicionar um produto novo (Pix, voucher) não deve exigir editar um `switch` gigante. Use interfaces:

```go
type FeeCalculator interface {
    Calculate(gross Money, installments int) (fee Money, err error)
}

type DebitFee struct{ bps int }
func (d DebitFee) Calculate(g Money, _ int) (Money, error) { return g * Money(d.bps) / 10000, nil }

type InstallmentCreditFee struct{ table map[int]int } // parcelas → bps
func (i InstallmentCreditFee) Calculate(g Money, n int) (Money, error) {
    bps, ok := i.table[n]
    if !ok { return 0, ErrInstallmentsNotAllowed }
    return g * Money(bps) / 10000, nil
}
```

Um produto novo = um tipo novo; nada existente é tocado.

**L — Liskov Substitution.** Quem implementa uma interface precisa **honrar o contrato**, não só ter os métodos. Se `IssuerClient.Authorize` promete "devolve erro só em falha de comunicação; negativa vem em `Approved=false`", o emissor simulado e o real precisam seguir isso. Senão o autorizador trata "negada" como "fora do ar" e manda reversal à toa.

**I — Interface Segregation.** Interfaces pequenas, do ponto de vista de quem usa. O caso de uso de liquidação precisa `ListDueOn(date)` e `MarkSettled(ids)`; ele não precisa de `Create`. Dê a ele uma interface só com o que ele usa:

```go
type SettlementReceivables interface {
    ListDueOn(ctx context.Context, day time.Time) ([]*Receivable, error)
    MarkSettled(ctx context.Context, ids []string, settlementID string) error
}
```

Testar fica trivial (fake de 2 métodos) e a dependência fica explícita.

**D — Dependency Inversion.** Regras de alto nível (domínio, casos de uso) **não dependem** de detalhes (PostgreSQL, Kafka, HTTP). Ambos dependem de **abstrações** (interfaces), e as interfaces pertencem ao alto nível. É o que fizemos no `simple_bank`: `domain.AccountRepository` é definida no domínio e implementada em `infrastructure/postgres`.

## 6.3 Clean Architecture: as camadas e a regra de dependência

```
┌──────────────────────────────────────────────────────────────┐
│  Frameworks & Drivers   (Gin, gRPC server, Kafka, Postgres,   │
│                          Redis, clientes HTTP externos)       │
│  ┌────────────────────────────────────────────────────────┐  │
│  │  Interface Adapters  (handlers, DTOs, repositórios,     │  │
│  │                       presenters, consumers)            │  │
│  │  ┌──────────────────────────────────────────────────┐  │  │
│  │  │  Application  (casos de uso: AuthorizeTransaction,│  │  │
│  │  │                SettleReceivables, RequestAnticip.)│  │  │
│  │  │  ┌────────────────────────────────────────────┐  │  │  │
│  │  │  │  Domain  (Merchant, Transaction, Receivable, │  │  │  │
│  │  │  │           Money, regras, eventos, ports)     │  │  │  │
│  │  │  └────────────────────────────────────────────┘  │  │  │
│  │  └──────────────────────────────────────────────────┘  │  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
          As setas de dependência apontam SEMPRE para dentro.
```

**Regra de dependência**: código de uma camada só pode importar camadas mais internas. O domínio não importa nada do projeto (só a biblioteca padrão). A aplicação importa o domínio. Handlers e repositórios importam aplicação e domínio. `main.go` importa tudo e **monta** (injeção de dependência).

O que vai em cada camada:

| Camada | Contém | Não contém |
|---|---|---|
| **Domain** | Entidades, value objects, máquinas de estado, serviços de domínio (`CalculateSchedule`), erros, interfaces de repositório e de gateways (*ports*), eventos de domínio | SQL, JSON, HTTP, Kafka, `time.Now()` solto (injete um `Clock`) |
| **Application** | Casos de uso (um por ação de negócio), orquestração, transações (Unit of Work), publicação de eventos | Regras de cálculo (isso é domínio), detalhes de transporte |
| **Adapters** | Handlers Gin, servidores gRPC, DTOs, mapeamento JSON↔domínio, repositórios, consumidores Kafka, clientes de serviços externos | Regras de negócio |
| **Frameworks** | `main.go`, configuração, conexões | — |

## 6.4 Ports & Adapters (Arquitetura Hexagonal)

É a mesma ideia com outro desenho. **Port** = interface que o núcleo define. **Adapter** = implementação concreta. Ports de entrada (*driving*): "alguém quer autorizar uma transação" (o caso de uso). Ports de saída (*driven*): "preciso salvar", "preciso perguntar ao emissor", "preciso publicar evento".

```go
// ports de saída, definidos no domínio/aplicação
type IssuerGateway interface {
    Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResponse, error)
    Reverse(ctx context.Context, txID string) error
}
type EventPublisher interface {
    Publish(ctx context.Context, events ...DomainEvent) error
}
type ReceivablesRegistry interface {
    Register(ctx context.Context, units []ReceivableUnit) error
    CheckLiens(ctx context.Context, units []ReceivableUnit) ([]Lien, error)
}
```

Adapters: `grpc.IssuerClient`, `kafka.Publisher`, `registry.FakeRegistry`. Trocar o emissor simulado pelo real = escrever um adapter novo, zero mudança no núcleo.

## 6.5 DDD leve: o vocabulário que vamos usar

- **Entidade**: tem identidade e ciclo de vida (`Merchant`, `Transaction`, `Receivable`).
- **Value Object**: definido pelos valores, imutável, sem identidade (`Money`, `Document` (CNPJ validado), `BankAccount`, `FeeRate`).
- **Agregado**: um grupo de entidades tratado como unidade de consistência, com uma **raiz** por onde tudo entra. `Transaction` é raiz do agregado que contém seus `Receivables`: você não altera um recebível direto; pede à transação (`tx.Capture()` gera os recebíveis; `tx.Chargeback()` os cancela).
- **Serviço de domínio**: regra que não pertence a uma entidade só (`ScheduleCalculator`, que precisa de calendário de dias úteis).
- **Evento de domínio**: fato que aconteceu, no passado (`TransactionCaptured`, `ReceivableAnticipated`). A entidade acumula eventos; o caso de uso publica depois de salvar.
- **Repositório**: coleção de agregados, com interface no domínio.
- **Linguagem ubíqua**: o código usa as palavras do negócio. `Receivable`, não `Item`; `Chargeback`, não `Reversal2`.

## 6.6 Como fica o projeto

```
internal/
├── domain/
│   ├── shared/          money.go, product.go, clock.go, calendar.go, event.go, id.go, errors.go
│   ├── merchant/        document.go, bank_account.go, fee_plan.go, merchant.go, repository.go
│   ├── transaction/     card.go, transaction.go (máquina de estados), events.go, ports.go
│   ├── receivable/      receivable.go, schedule.go (D+n, dias úteis, URs), ports.go
│   ├── anticipation/    pricing.go (valor presente), anticipation.go, ports.go
│   ├── settlement/      settlement.go, ports.go
│   ├── chargeback/      chargeback.go
│   └── ledger/          entry.go (partidas dobradas)
├── application/
│   ├── uow.go  errors.go  idempotency.go
│   ├── merchant/        onboard.go, review.go
│   ├── transaction/     authorize.go, lifecycle.go
│   ├── receivable/      schedule.go (consumidor), query.go
│   ├── settlement/      settle_due.go
│   ├── anticipation/    anticipate.go
│   ├── chargeback/      open.go
│   └── notification/    notify.go
├── infrastructure/
│   ├── postgres/        db.go, uow.go, *_repository.go, idempotency_store.go, card_vault_store.go
│   ├── gormrepo/        merchant_repository.go (ORM, para comparar)
│   ├── kafka/           publisher.go, relay.go (outbox), consumer.go (DLQ)
│   ├── grpcclient/      tls.go, issuer.go (breaker), vault.go
│   ├── redis/           lock.go
│   ├── registry/        fake.go            (registradora simulada)
│   ├── bank/            file_gateway.go    (banco liquidante simulado)
│   ├── cryptoutil/      aesgcm.go
│   ├── webhook/         sender.go
│   ├── config/          config.go
│   └── observability/   logging.go, otel.go, metrics.go
└── handler/
    ├── http/            router.go, problem.go, dto.go, merchants.go, transactions.go, receivables.go, anticipations.go
    │   └── middleware/  request_id.go, logger.go, auth.go, idempotency.go
    └── grpc/            issuer_server.go, vault_server.go
```

O domínio é dividido por **contexto** (merchant, transaction, receivable...), não por tipo de arquivo. Cada pasta de domínio é um pacote Go pequeno e coeso.

---

# PARTE 7 — Design Patterns: as soluções que se repetem

## 7.1 O que é um design pattern

Um *design pattern* (padrão de projeto) é uma solução com nome para um problema que aparece sempre. Os 23 padrões clássicos vêm do livro "Design Patterns" (1994, a "Gangue dos Quatro", GoF) e se dividem em **criacionais** (como criar objetos), **estruturais** (como compor objetos) e **comportamentais** (como objetos colaboram). Depois vieram os padrões **de arquitetura corporativa** (Fowler: Repository, Unit of Work, etc.) e os **de sistemas distribuídos** (Outbox, Saga, Circuit Breaker).

Um aviso: Go é uma linguagem simples, e muitos padrões que em Java exigem classes abstratas aqui viram "uma interface e uma função". Não force padrões; use quando o problema aparecer. Abaixo, os que **de fato** aparecem numa adquirente, com o problema que cada um resolve.

## 7.2 Padrões criacionais

**Factory (fábrica).** Centraliza a criação de um objeto que tem invariantes. Em Go, é o construtor `NewX` que valida:

```go
func NewTransaction(merchant *Merchant, amount Money, product Product, installments int, card CardToken) (*Transaction, error) {
    if !merchant.IsActive() { return nil, ErrMerchantInactive }
    if amount <= 0 { return nil, ErrInvalidAmount }
    if product == Debit && installments != 1 { return nil, ErrDebitCannotInstall }
    return &Transaction{id: newID("tx"), status: Pending, /* ... */}, nil
}
```

Nunca deixe alguém criar uma `Transaction{}` inválida na mão: campos privados + construtor.

**Builder.** Para objetos com muitas opções. Em Go, o idioma é *functional options*:

```go
type ServerOption func(*Server)
func WithTimeout(d time.Duration) ServerOption { return func(s *Server) { s.timeout = d } }
func WithTLS(cfg *tls.Config) ServerOption      { return func(s *Server) { s.tls = cfg } }

srv := NewServer(addr, WithTimeout(5*time.Second), WithTLS(cfg))
```

**Singleton.** Uma instância só. Em Go, evite o global mágico; crie **uma vez no `main`** e injete. Se precisar de inicialização preguiçosa segura, `sync.Once`.

## 7.3 Padrões estruturais

**Adapter.** Converte a interface de um componente para a que o seu código espera. É exatamente o "adapter" de Ports & Adapters: `grpcclient.IssuerAdapter` transforma `domain.AuthorizationRequest` em `pb.AuthorizeRequest` e vice-versa. Quando o emissor real chegar, com ISO 8583, você escreve `iso8583.IssuerAdapter`.

**Decorator.** Envolve um objeto acrescentando comportamento, mantendo a interface. Middlewares HTTP são decorators. E dá para decorar um repositório com cache, ou um gateway com métricas:

```go
type instrumentedIssuer struct {
    next    IssuerGateway
    latency metric.Float64Histogram
}
func (i instrumentedIssuer) Authorize(ctx context.Context, r AuthorizationRequest) (AuthorizationResponse, error) {
    start := time.Now()
    resp, err := i.next.Authorize(ctx, r)
    i.latency.Record(ctx, time.Since(start).Seconds())
    return resp, err
}
```

**Facade.** Uma interface simples na frente de um subsistema complexo. O caso de uso `AuthorizeTransaction` é uma facade: o handler chama um método e por trás há validação, chamada ao emissor, persistência e evento.

**Proxy.** Um substituto que controla acesso. O **vault de cartões** é um proxy: o resto do sistema fala com tokens, e só o vault fala com o PAN.

**Composite.** Tratar um grupo como um item. Regras de fraude compostas: `AllOf(rule1, rule2)`, `AnyOf(...)`.

## 7.4 Padrões comportamentais

**Strategy.** Algoritmos intercambiáveis atrás de uma interface. O `FeeCalculator` da Parte 6 é Strategy: débito, crédito à vista, parcelado, cada um com sua estratégia, escolhida pelo produto. Também: estratégia de precificação da antecipação (juros compostos vs. simples).

**State.** Um objeto muda de comportamento conforme o estado. A **máquina de estados da transação** é o padrão State. Em Go, a forma mais legível é uma tabela de transições permitidas:

```go
type Status string
const (
    Pending      Status = "PENDING"
    Authorized   Status = "AUTHORIZED"
    Denied       Status = "DENIED"
    Captured     Status = "CAPTURED"
    Canceled     Status = "CANCELED"
    Settled      Status = "SETTLED"
    Chargebacked Status = "CHARGEBACKED"
)

var transitions = map[Status][]Status{
    Pending:    {Authorized, Denied},
    Authorized: {Captured, Canceled},
    Captured:   {Canceled, Settled, Chargebacked},
    Settled:    {Chargebacked},
}

func (t *Transaction) transition(to Status) error {
    for _, allowed := range transitions[t.status] {
        if allowed == to {
            t.status = to
            return nil
        }
    }
    return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, t.status, to)
}

func (t *Transaction) Capture(now time.Time) error {
    if err := t.transition(Captured); err != nil { return err }
    t.capturedAt = now
    t.events = append(t.events, TransactionCaptured{ID: t.id, At: now})
    return nil
}
```

Toda mudança de estado passa por `transition`. Capturar uma transação cancelada é impossível por construção, não por `if` espalhado.

**Observer / Publish-Subscribe.** Quem gera um fato não sabe quem reage. Eventos de domínio + Kafka são Observer em escala de sistema: `TransactionCaptured` é publicado; o serviço de agenda, o de notificação e o de antifraude reagem, sem que o autorizador os conheça.

**Command.** Encapsular uma ação como objeto, com tudo o que precisa para executar. Os *inputs* dos casos de uso (`AuthorizeCommand{MerchantID, Amount, ...}`) são Commands; permitem enfileirar, logar e reprocessar.

**Chain of Responsibility.** Uma cadeia de verificadores, cada um decide se barra ou passa adiante. Pipeline de validação pré-autorização: `merchantActive → amountWithinLimits → velocityCheck → fraudScore → issuer`. Cada elo é uma função `func(ctx, *AuthContext) error`.

**Template Method.** Esqueleto de algoritmo com passos variáveis. Em Go, uma função que recebe funções: o worker de liquidação tem o esqueleto fixo (listar → verificar ônus → agrupar → pagar → marcar) e o passo "pagar" varia por banco.

**Specification.** Regras de negócio como objetos combináveis: `IsAnticipable = Captured.And(NotSettled).And(NotLiened).And(DueAfter(today))`. Útil para filtrar recebíveis elegíveis à antecipação de forma testável.

## 7.5 Padrões de arquitetura corporativa

**Repository.** Coleção em memória "de mentira" na frente do banco. Já usamos.

**Unit of Work.** "Tudo ou nada" numa transação de banco. Já usamos (`uow.Do`). Numa adquirente é vital: autorizar = salvar transação + salvar evento na outbox + salvar chave de idempotência, tudo junto ou nada.

**DTO (Data Transfer Object).** Structs só para transporte (JSON de entrada/saída, mensagens Kafka), separadas das entidades. A entidade tem campos privados e regras; o DTO é público e burro.

**Domain Events.** Já explicado. A entidade acumula; o caso de uso publica após o commit.

**Optimistic Locking.** Coluna `version` na tabela; `UPDATE ... WHERE id = ? AND version = ?`; se afetou 0 linhas, alguém alterou antes, tente de novo. Alternativa ao `SELECT FOR UPDATE` quando conflito é raro.

## 7.6 Padrões de sistemas distribuídos

**Transactional Outbox.** O problema: você salva a transação no banco e publica no Kafka. Se o Kafka cair entre os dois, o evento se perde; se o banco falhar após publicar, você publicou uma mentira. Solução: gravar o evento numa tabela `outbox` **na mesma transação** do banco, e um processo separado (*relay*) lê a outbox e publica no Kafka, marcando como enviado. Garantia: se a transação existe, o evento vai sair (pelo menos uma vez).

**Idempotent Consumer.** Como o evento pode chegar duas vezes, o consumidor guarda os `event_id` já processados (tabela `processed_events`) e ignora repetidos. Outbox + consumidor idempotente = "exatamente uma vez" na prática.

**Saga.** Uma operação que atravessa vários serviços, sem transação distribuída, com **compensações**. Autorização com timeout: (1) reservar no emissor; (2) salvar localmente; se (2) falhar, compensar com (3) reversal no emissor. Cada passo tem seu "desfazer".

**Circuit Breaker.** Se o emissor está falhando, pare de chamá-lo por um tempo (o "disjuntor" abre) e responda rápido "indisponível"; depois de N segundos, teste uma chamada (meio-aberto); se passar, fecha. Evita que uma dependência lenta derrube você junto. Biblioteca: `sony/gobreaker`.

**Retry com backoff e jitter.** Tentar de novo, esperando cada vez mais (100ms, 200ms, 400ms...) com um pouco de aleatoriedade para não sincronizar todos os clientes. Só para operações **idempotentes**. Nunca faça retry cego de uma autorização sem chave de idempotência.

**Bulkhead.** Isolar recursos: o pool de conexões do emissor A não pode ser esgotado por lentidão do emissor B. Em Go: semáforos (`chan struct{}` com buffer) por dependência.

**CQRS (Command Query Responsibility Segregation).** Separar o modelo de escrita (entidades ricas) do de leitura (views desnormalizadas). O extrato do EC ("tudo o que tenho a receber por dia") é uma leitura pesada; uma tabela `receivables_daily_summary` alimentada por eventos responde em milissegundos sem tocar o modelo de escrita. Use só quando a leitura pedir.

**Ledger de partidas dobradas.** Não é um GoF, mas é o padrão contábil: todo movimento de dinheiro é um par de lançamentos (débito numa conta, crédito noutra) e a soma é sempre zero. `payable_to_merchant` cresce quando captura; `revenue_fees` recebe a taxa; na liquidação, `payable_to_merchant` cai e `bank_outgoing` sobe. Com isso, "quanto devemos aos ECs hoje?" é uma soma, e uma auditoria confere.

## 7.7 Anti-padrões para evitar

- **Modelo anêmico**: entidades só com getters/setters e regras em "services". A `Transaction` deve saber se pode ser capturada.
- **God object**: um `PaymentService` com 40 métodos. Um caso de uso por arquivo.
- **Lógica no handler**: o handler traduz HTTP ↔ caso de uso, nada mais.
- **Float para dinheiro**: sempre inteiro em centavos.
- **`time.Now()` no domínio**: injete `Clock`; senão testes de "D+30 cai em feriado" ficam impossíveis.
- **Retry sem idempotência**: cobra o portador duas vezes.

## No nosso projeto

| Padrão | Onde |
|---|---|
| Factory | Todos os `New*` do domínio |
| Functional options | Servidores HTTP/gRPC, clientes Kafka |
| Adapter | `grpcclient.Issuer`, `kafka.Publisher`, `registry.Fake` |
| Decorator | Middlewares Gin; `instrumentedIssuer`; repositório com cache de `Merchant` |
| Strategy | `FeeCalculator` por produto; `AnticipationPricer` |
| State | Máquinas de estado de `Transaction`, `Receivable`, `Merchant`, `Chargeback` |
| Chain of Responsibility | Pipeline de pré-autorização |
| Specification | Elegibilidade de antecipação |
| Repository + Unit of Work | Toda persistência |
| Domain Events + Outbox + Idempotent Consumer | Toda comunicação entre serviços |
| Saga (compensação) | Autorização com timeout/reversal |
| Circuit Breaker + Retry + Bulkhead | Cliente do emissor e da registradora |
| Ledger de partidas dobradas | Contabilidade de saldos a repassar e receitas |
| CQRS (leve) | Resumo diário de recebíveis por EC |

---

# PARTE 8 — Banco de dados e ORM

## 8.1 Relacional, e por que uma adquirente usa

Um banco **relacional** guarda dados em **tabelas** (linhas e colunas) e garante regras entre elas (uma transação sempre pertence a um EC que existe). Ele fala **SQL**. Bancos **não relacionais** (MongoDB, DynamoDB, Redis) abrem mão de algumas garantias em troca de flexibilidade ou velocidade.

Dinheiro exige garantias. Por isso o núcleo de uma adquirente é relacional (**PostgreSQL** no nosso caso). Redis entra como auxiliar (cache, idempotência, rate limit), e um banco analítico pode entrar depois para relatórios.

## 8.2 SQL essencial

```sql
-- Criar tabela
CREATE TABLE merchants (
    id            TEXT PRIMARY KEY,
    document      TEXT NOT NULL UNIQUE,         -- CNPJ
    legal_name    TEXT NOT NULL,
    mcc           TEXT NOT NULL,
    status        TEXT NOT NULL,                -- UNDER_REVIEW, ACTIVE, BLOCKED
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Inserir, ler, alterar
INSERT INTO merchants (id, document, legal_name, mcc, status) VALUES ('m_1', '12345678000190', 'Padaria Pão Quente', '5462', 'UNDER_REVIEW');
SELECT id, legal_name FROM merchants WHERE status = 'ACTIVE' ORDER BY created_at DESC LIMIT 50;
UPDATE merchants SET status = 'ACTIVE', updated_at = now() WHERE id = 'm_1';

-- Juntar tabelas
SELECT r.due_date, SUM(r.net_amount) AS total
FROM receivables r
JOIN merchants m ON m.id = r.merchant_id
WHERE m.id = 'm_1' AND r.status = 'SCHEDULED'
GROUP BY r.due_date
ORDER BY r.due_date;
```

**Chaves e restrições** são a primeira linha de defesa: `PRIMARY KEY`, `FOREIGN KEY` (a transação aponta para um EC que existe), `UNIQUE` (uma chave de idempotência por EC), `CHECK (amount > 0)`, `NOT NULL`. O banco recusa dado inválido mesmo que o código tenha bug.

**Índices** tornam buscas rápidas. Sem índice, `WHERE due_date = ?` lê a tabela inteira. Crie índices para as consultas que o sistema faz de verdade: `(merchant_id, status)`, `(due_date, status)`, `(idempotency_key)`. Cada índice custa espaço e deixa a escrita um pouco mais lenta; não indexe tudo.

## 8.3 ACID e transações

Uma **transação de banco** (não confundir com transação de cartão) é um grupo de comandos que vale tudo ou nada. As garantias têm a sigla **ACID**:

- **Atomicidade**: ou todos os comandos são aplicados, ou nenhum.
- **Consistência**: as restrições valem antes e depois.
- **Isolamento**: transações simultâneas não veem estados intermediários umas das outras (com níveis; abaixo).
- **Durabilidade**: depois do `COMMIT`, sobreviveu, mesmo que a luz caia.

```sql
BEGIN;
UPDATE receivables SET status = 'ANTICIPATED', anticipation_id = 'ant_9' WHERE id IN ('r_1','r_2') AND status = 'SCHEDULED';
INSERT INTO anticipations (id, merchant_id, gross, discount, net) VALUES ('ant_9', 'm_1', 19300, 700, 18600);
INSERT INTO ledger_entries (...) VALUES (...), (...);
COMMIT;   -- ou ROLLBACK se algo deu errado
```

**Níveis de isolamento** e o problema clássico: dois pedidos de antecipação do mesmo recebível ao mesmo tempo. Ambos leem `status = 'SCHEDULED'`, ambos gravam. Resultado: antecipou duas vezes. Soluções:

- **Lock pessimista**: `SELECT ... FOR UPDATE` trava as linhas até o commit; o segundo pedido espera e, quando lê, já vê `ANTICIPATED`. Foi o que o `simple_bank` fez com `FindByIDForUpdate`. Cuidado com **deadlock**: dois processos travando as mesmas linhas em ordem diferente. Regra: sempre trave em ordem determinística (por `id` crescente), como fizemos no `Transfer`.
- **Lock otimista**: coluna `version`; `UPDATE ... WHERE id = ? AND version = 3`; se afetou 0 linhas, houve conflito; recarregue e tente de novo. Bom quando conflito é raro.
- **Condição no UPDATE**: `UPDATE ... WHERE status = 'SCHEDULED'` e checar linhas afetadas. Simples e eficaz para transições de estado.

O PostgreSQL usa `READ COMMITTED` por padrão. Para o worker de liquidação, que lê e grava lotes grandes, `REPEATABLE READ` ou `SELECT FOR UPDATE SKIP LOCKED` (vários workers pegam linhas diferentes sem esperar) resolvem bem.

## 8.4 Migrações

Migração é um arquivo SQL versionado que evolui o esquema. Nunca altere tabela na mão em produção. Usamos **golang-migrate**:

```
migrations/
├── 000001_create_merchants.up.sql
├── 000001_create_merchants.down.sql
├── 000002_create_transactions.up.sql
└── ...
```

```bash
migrate -path migrations -database "$DATABASE_URL" up
```

Regras: toda migração tem o `down`; migrações nunca são editadas depois de aplicadas; mudanças que quebram (renomear coluna) são feitas em dois deploys (adicionar nova → migrar dados → remover velha).

## 8.5 Dinheiro no banco: nunca `float`

`float64` não representa 0,10 exatamente (é 0,1000000000000000055...). Some 10 vezes 0,10 e você não tem 1,00. Numa adquirente, um centavo errado multiplicado por milhões de transações é um problema regulatório. Duas opções corretas:

- **Inteiro em centavos** (`BIGINT` no banco, `int64` no Go). Simples, rápido, sem surpresa. Escolha do guia.
- **`NUMERIC(18,2)`** no banco com uma biblioteca decimal no Go (`shopspring/decimal`). Necessário se você tiver várias moedas com casas decimais diferentes ou cálculos com muitas casas.

Taxas em **basis points** (`INTEGER`; 250 = 2,50%). Ao calcular `gross * bps / 10000`, faça a multiplicação antes da divisão e defina a regra de arredondamento (o guia usa "arredonda para baixo, e a diferença vai para a primeira parcela"; documente a sua).

## 8.6 `database/sql`, sqlx, sqlc, GORM: qual usar

| Opção | O que é | Prós | Contras |
|---|---|---|---|
| **`database/sql`** (padrão) | Você escreve SQL, faz `Scan` coluna a coluna | Controle total; sem mágica; o que o `simple_bank` usou | Verboso |
| **sqlx** | `database/sql` + mapeia colunas para struct por tag | Menos código | Ainda SQL na mão |
| **sqlc** | Você escreve SQL em arquivos; ele **gera** funções Go tipadas | SQL explícito, tipagem forte, zero reflexão em runtime | Passo de geração |
| **GORM** | ORM completo: define structs, ele gera SQL | Rápido para CRUD; migrações automáticas; hooks | Esconde o SQL; consultas complexas ficam estranhas; fácil gerar N+1 |

**ORM** (*Object-Relational Mapping*) é uma biblioteca que traduz structs em tabelas e métodos em SQL, para você não escrever SQL. É produtivo para cadastro (CRUD) e perigoso para o núcleo financeiro, onde você precisa saber exatamente qual SQL roda, com qual lock.

Decisão do guia, que também é a mais comum em fintechs Go: **`database/sql` puro para o núcleo** (transações, recebíveis, liquidação, ledger), para você ver cada SQL e cada lock, e **GORM para o cadastro** (merchants). Assim você aprende os dois e sente a diferença. `sqlc` é a evolução natural do núcleo quando o volume de SQL cresce (seção 8.8 mostra como fica).

## 8.7 GORM na prática

```go
type MerchantModel struct {
    ID        string `gorm:"primaryKey"`
    Document  string `gorm:"uniqueIndex;not null"`
    LegalName string `gorm:"not null"`
    MCC       string `gorm:"not null"`
    Status    string `gorm:"not null;index"`
    Version   int    `gorm:"not null;default:1"`
    CreatedAt time.Time
    UpdatedAt time.Time
}

func (MerchantModel) TableName() string { return "merchants" }

// Repositório: converte Model ↔ entidade de domínio. O domínio nunca vê o Model.
type MerchantRepository struct{ db *gorm.DB }

func (r *MerchantRepository) FindByID(ctx context.Context, id string) (*merchant.Merchant, error) {
    var m MerchantModel
    err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
    if errors.Is(err, gorm.ErrRecordNotFound) { return nil, merchant.ErrNotFound }
    if err != nil { return nil, err }
    return toDomain(m), nil
}

func (r *MerchantRepository) Save(ctx context.Context, m *merchant.Merchant) error {
    model := toModel(m)
    res := r.db.WithContext(ctx).
        Model(&MerchantModel{}).
        Where("id = ? AND version = ?", model.ID, model.Version-1). // lock otimista
        Updates(model)
    if res.Error != nil { return res.Error }
    if res.RowsAffected == 0 { return merchant.ErrConcurrentUpdate }
    return nil
}
```

Cuidados com GORM: use `WithContext`; desligue `AutoMigrate` em produção (use migrações versionadas); ative o log de SQL em dev para ver o que ele gera; nunca exponha o `Model` fora da infraestrutura.

## 8.8 sqlc para o núcleo

```sql
-- queries/receivables.sql
-- name: ListDueOn :many
SELECT * FROM receivables
WHERE due_date = $1 AND status = 'SCHEDULED'
ORDER BY merchant_id, id
FOR UPDATE SKIP LOCKED;

-- name: MarkSettled :execrows
UPDATE receivables SET status = 'SETTLED', settlement_id = $2, updated_at = now()
WHERE id = ANY($1::text[]) AND status = 'SCHEDULED';
```

`sqlc generate` produz `func (q *Queries) ListDueOn(ctx, dueDate time.Time) ([]Receivable, error)` e `MarkSettled(ctx, ids []string, settlementID string) (int64, error)`. Você vê o SQL, o lock e o tipo. Para dinheiro, é o ideal.

## 8.9 Modelagem da adquirente

As tabelas do núcleo (simplificadas; as migrações completas ficam na Parte 15):

```
merchants ─────────┐
  id, document, legal_name, mcc, status, version
merchant_bank_accounts (domicílio)
  merchant_id, bank_code, branch, account, kind
fee_plans
  id, merchant_id, product, installments, mdr_bps, anticipation_bps_month
transactions
  id, merchant_id, amount, product, installments, status,
  card_token, card_brand, card_bin, card_last4,
  authorization_code, nsu, issuer_response_code,
  authorized_at, captured_at, canceled_at, version
receivables
  id, transaction_id, merchant_id, installment_no, gross_amount, fee_amount, net_amount,
  due_date, status (SCHEDULED|ANTICIPATED|SETTLED|CANCELED|CHARGEBACKED),
  settlement_id, anticipation_id
settlements
  id, settlement_date, merchant_id, bank_account_id, total_amount, status, external_ref
anticipations
  id, merchant_id, requested_at, gross_amount, discount_amount, net_amount, rate_bps_month, status
chargebacks
  id, transaction_id, reason_code, amount, status, opened_at, deadline
ledger_entries (append-only)
  id, account (payable_to_merchant|revenue_fees|revenue_anticipation|bank_outgoing|...),
  merchant_id, debit, credit, reference_type, reference_id, created_at
idempotency_keys
  merchant_id, key, request_hash, response_status, response_body, expires_at   UNIQUE(merchant_id, key)
outbox
  id, aggregate_type, aggregate_id, event_type, payload (JSONB), created_at, published_at
processed_events
  consumer, event_id, processed_at   PRIMARY KEY(consumer, event_id)
```

Observe: `card_token`, `card_bin`, `card_last4` e **nenhuma coluna de PAN ou CVV**. `ledger_entries`, `outbox` e `processed_events` são *append-only*: nunca se faz `UPDATE`/`DELETE` nelas (só o `published_at` da outbox).

## 8.10 Redis

Redis é um banco em memória, chave → valor, muito rápido, com expiração por chave. Usos aqui:

- **Idempotência**: `SET idem:{merchant}:{key} {hash} NX EX 86400`. O `NX` ("só se não existir") é atômico, então dois pedidos simultâneos não passam.
- **Rate limit**: contador com expiração por credencial.
- **Cache** de `Merchant` e `FeePlan` (mudam raramente, são lidos em toda autorização).
- **Locks distribuídos** curtos (ex.: garantir que só um worker roda a liquidação do dia).

Regra: Redis nunca é a única cópia de nada que importa. Se cair, o sistema fica mais lento, não errado.

## No nosso projeto

- PostgreSQL com migrações golang-migrate; `database/sql` (driver pgx) no núcleo financeiro; GORM no cadastro. O plano de taxas fica em JSONB dentro de `merchants`.
- Dinheiro em `BIGINT` centavos; taxas em bps.
- `SELECT ... FOR UPDATE` para capturar/cancelar/antecipar; `FOR UPDATE SKIP LOCKED` no worker de liquidação; verificação de versão em todo `UPDATE` (lock otimista).
- Tabelas `outbox`, `processed_events`, `idempotency_keys`, `ledger_entries` e `card_vault` desde a primeira fase.
- Idempotência na tabela (atômica via `ON CONFLICT`); Redis para o lock diário da liquidação (e, como evolução, cache e rate limit).

---

# PARTE 9 — Segurança

## 9.1 Os conceitos

Segurança da informação se resume a três propriedades, a tríade **CIA**:

- **Confidencialidade**: só quem deve vê. O número do cartão não pode vazar.
- **Integridade**: ninguém altera sem autorização. Um valor de R$ 10,00 não pode virar R$ 1.000,00 no caminho.
- **Disponibilidade**: o sistema responde quando precisa. Uma adquirente fora do ar às 18h de sexta é uma cidade sem comércio.

Vocabulário: **ameaça** (quem/o que pode atacar), **vulnerabilidade** (a fraqueza), **superfície de ataque** (tudo que está exposto: portas, endpoints, dependências, pessoas), **defesa em profundidade** (várias camadas, porque uma vai falhar), **menor privilégio** (cada componente só tem a permissão que precisa).

## 9.2 Criptografia, sem matemática

**Hash** transforma dados de qualquer tamanho numa "impressão digital" fixa, e não dá para voltar. `SHA-256("hello")` é sempre o mesmo, e mudar uma letra muda tudo. Usos: verificar integridade, comparar sem revelar, gerar identificadores. **Para senha**, use um hash **lento e com sal** (**bcrypt** ou **argon2id**), não SHA-256 puro; a lentidão é proposital, para inviabilizar chute em massa.

```go
hash, _ := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
err := bcrypt.CompareHashAndPassword(hash, []byte(tentativa)) // nil se bate
```

**Criptografia simétrica**: a mesma chave cifra e decifra. Rápida. Padrão: **AES-256-GCM** (o GCM também garante integridade). Uso: cifrar o PAN em repouso no vault.

```go
block, _ := aes.NewCipher(key)         // key: 32 bytes vindos do gerenciador de segredos
gcm, _ := cipher.NewGCM(block)
nonce := make([]byte, gcm.NonceSize())
_, _ = rand.Read(nonce)                // nonce novo a cada cifragem, nunca reutilizar
ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
```

**Criptografia assimétrica**: um par de chaves; o que uma cifra, só a outra decifra. A **pública** pode ser distribuída; a **privada** nunca sai de onde nasceu. Usos: **assinatura digital** (assino com a privada, qualquer um verifica com a pública: é a base do JWT e do TLS) e troca de chaves. Algoritmos: RSA, ECDSA, Ed25519.

**HMAC**: um hash com chave secreta compartilhada. Prova que a mensagem veio de quem tem a chave e não foi alterada. É como assinamos webhooks:

```go
mac := hmac.New(sha256.New, secret)
mac.Write(body)
signature := hex.EncodeToString(mac.Sum(nil))
// enviar no header: X-Signature: sha256=<signature>
// o EC recalcula e compara com hmac.Equal (comparação em tempo constante)
```

**Regras que não se discutem**: nunca invente criptografia; use a biblioteca padrão do Go (`crypto/*`) ou `golang.org/x/crypto`; gere aleatoriedade com `crypto/rand`, nunca `math/rand`; compare segredos com `subtle.ConstantTimeCompare`/`hmac.Equal`.

## 9.3 TLS, HTTPS e mTLS

**TLS** é o protocolo que cifra a conexão (o "S" de HTTPS). O servidor apresenta um **certificado** (sua chave pública assinada por uma autoridade confiável); o cliente verifica e os dois combinam uma chave simétrica de sessão. Garante confidencialidade e integridade em trânsito, e autentica o servidor.

**mTLS** (*mutual TLS*): o cliente **também** apresenta certificado. Serve para autenticar máquinas: o serviço de liquidação só aceita conexão de quem tem certificado emitido pela nossa autoridade interna. Padrão em comunicação entre serviços de pagamento e no *service mesh* (Parte 14).

Na prática: TLS 1.2 no mínimo, 1.3 preferido; certificados públicos via Let's Encrypt/ACM no *ingress*; certificados internos via cert-manager ou o mesh. Nada de HTTP puro, nem entre pods.

## 9.4 Segredos

Senhas de banco, chaves de API, chaves de cifragem. Regras:

- Nunca no código nem no git (nem "só no commit inicial").
- Em desenvolvimento: variáveis de ambiente lidas de `.env` (que está no `.gitignore`).
- Em produção: um **gerenciador de segredos** (HashiCorp Vault, AWS Secrets Manager, ou os `Secrets` do Kubernetes cifrados em repouso). O serviço recebe o segredo em runtime, com permissão só para o que precisa.
- **Rotação**: segredos trocam periodicamente; o código precisa suportar dois válidos ao mesmo tempo (o velho e o novo) durante a troca.
- A **chave que cifra o PAN** (DEK) é cifrada por outra chave (KEK) que fica num HSM/KMS. Isso é *envelope encryption* e é exigência do PCI.

## 9.5 OWASP Top 10, aplicado

O OWASP publica a lista das falhas mais comuns em aplicações web. As que mais importam aqui:

| Falha | O que é | Defesa no nosso código |
|---|---|---|
| **Injeção (SQL)** | Montar SQL concatenando texto do usuário | Sempre parâmetros (`$1`, `?`); sqlc/GORM já fazem; nunca `fmt.Sprintf` em SQL |
| **Quebra de autenticação** | Token fraco, sem expiração, sem revogação | JWT curto (5–15 min), refresh, rotação de chaves, `alg` fixo |
| **Quebra de controle de acesso** | Usuário do EC A consulta transação do EC B mudando o `id` na URL | Toda consulta filtra por `merchant_id` do token, não da URL |
| **Exposição de dados sensíveis** | PAN em log, em erro, em URL | Mascaramento; nunca dados sensíveis em query string; logs com *redaction* |
| **Configuração insegura** | Swagger aberto em produção, `debug=true`, CORS `*` | Perfis por ambiente; Gin em `ReleaseMode` |
| **Componentes vulneráveis** | Dependência com CVE | `govulncheck` no CI; Dependabot |
| **Falhas de log e monitoramento** | Atacante age e ninguém vê | Auditoria + alertas (Parte 13) |
| **SSRF** | O servidor faz requisição a uma URL controlada pelo atacante (ex.: URL de webhook apontando para a rede interna) | Validar URL de webhook: só `https`, sem IP privado, resolver DNS e checar |

## 9.6 Dados de cartão: PAN, tokenização e o vault

O **PAN** (*Primary Account Number*) é o número do cartão. Os dados sensíveis segundo o PCI:

| Dado | Pode guardar? | Como |
|---|---|---|
| PAN | Sim, se precisar | Cifrado (AES-GCM) ou tokenizado, em componente isolado |
| Nome do portador, validade | Sim | Protegido |
| **CVV/CVC** | **Nunca** após a autorização | — |
| **Trilha magnética / chip completo** | **Nunca** | — |
| **PIN** | **Nunca** | — |

**Tokenização** é o padrão: um serviço isolado (o **vault**) recebe o PAN, gera um token aleatório (não derivado do PAN!), guarda `token → PAN cifrado` e devolve o token. O resto do sistema só conhece tokens. O vault expõe duas operações: `Tokenize(pan) → token` e, só para o autorizador, `Detokenize(token) → pan` (para montar a mensagem ao emissor). Ele é o único componente no escopo "pesado" do PCI.

**Mascaramento**: em toda saída (API, log, tela) o PAN aparece como BIN + últimos 4: `411111******1111`. Função única, testada, usada em todo lugar.

**Validação**: o algoritmo de **Luhn** verifica o dígito de controle do PAN (pega erro de digitação, não fraude). O BIN identifica bandeira e emissor.

## 9.7 Segurança no código, no build e na operação

- **Validação de entrada** em toda borda (Gin binding + regras de domínio). Rejeite o que não conhece.
- **Logs**: nunca `log.Printf("%+v", req)` num request com dados de cartão. Use tipos com `String()`/`LogValue()` que mascaram (Parte 13).
- **Dependências**: `go mod verify`, `govulncheck ./...`, versões fixas.
- **Imagens Docker**: base mínima (`distroless` ou `scratch`), usuário não-root, sem shell, escaneadas (Trivy).
- **Kubernetes**: `NetworkPolicy` (o vault só aceita conexão do autorizador), `readOnlyRootFilesystem`, sem privilégios.
- **Ambientes**: dados de produção nunca em teste; gere cartões de teste (`4111 1111 1111 1111` é o clássico).
- **Auditoria**: quem acessou dado sensível, quando, de onde. Append-only, com relógio sincronizado.
- **Resposta a incidente**: um documento (runbook) com quem chamar, como revogar chaves, como comunicar BCB/ANPD/bandeira. Escreva antes de precisar.

## No nosso projeto

- Serviço `vault` (gRPC, mTLS, `NetworkPolicy` restrita): `Tokenize`/`Detokenize`, AES-256-GCM com envelope encryption; chave via variável de ambiente em dev e Secret em K8s.
- `shared.MaskPAN` e tipos `CardToken`, `MaskedPAN` que implementam `slog.LogValuer` para nunca logar em claro.
- Webhooks assinados com HMAC-SHA256; validação anti-SSRF na URL cadastrada.
- `govulncheck` e Trivy no CI; imagens `distroless`; Gin em `ReleaseMode` fora de dev.
- Tabela `audit_log` e middleware que registra acesso a rotas sensíveis.

---

# PARTE 10 — Autenticação e autorização

## 10.1 A diferença

- **Autenticação** (*authn*): **quem** é você? Provar identidade (senha, token, certificado).
- **Autorização** (*authz*): **o que** você pode fazer? Permissões.

Um EC autenticado (sabemos que é a Padaria) ainda não está autorizado a aprovar o próprio credenciamento. Erros: `401` quando falha a autenticação; `403` quando falha a autorização.

## 10.2 Formas de autenticar uma API

| Método | Como funciona | Quando usar |
|---|---|---|
| **API Key** | Uma string secreta no header | Integrações simples; sempre com hash no banco e possibilidade de revogar |
| **Basic** | `usuário:senha` em base64 no header | Só sobre TLS; legado |
| **Bearer token (JWT)** | Um token assinado, com validade, no header `Authorization: Bearer ...` | O padrão para APIs |
| **OAuth 2.0 client credentials** | O cliente troca `client_id` + `client_secret` por um token de curta duração | Máquina-máquina (o sistema do EC chamando a adquirente) |
| **mTLS** | Certificado do cliente | Entre serviços; parceiros críticos (bancos, registradoras) |

Para a API pública usaremos **OAuth 2.0 client credentials** emitindo **JWT**. Para serviços internos, **mTLS**.

## 10.3 JWT por dentro

Um **JWT** (*JSON Web Token*) são três partes em base64 separadas por ponto: `header.payload.signature`.

```json
// header
{ "alg": "RS256", "kid": "2026-09-key-1", "typ": "JWT" }
// payload (claims)
{
  "iss": "https://auth.adquirente.com",
  "sub": "client_padaria",
  "aud": "adquirente-api",
  "exp": 1760000000,
  "iat": 1759999100,
  "merchant_id": "m_01HZX",
  "scope": "transactions:write receivables:read"
}
```

A assinatura é feita com a chave **privada** do servidor de autenticação (RS256 ou ES256). Qualquer serviço com a chave **pública** verifica que o token é legítimo e não foi alterado, **sem consultar banco**. Por isso JWT escala.

Regras:
- `exp` curto (5 a 15 minutos). Token vazado vale pouco tempo.
- Verificar **`iss`, `aud`, `exp` e o algoritmo** (fixe `RS256`/`ES256`; ataques clássicos trocam `alg` para `none`).
- Publicar as chaves públicas num endpoint **JWKS** (`/.well-known/jwks.json`) com `kid`; rotacionar chaves sem derrubar ninguém.
- JWT **não é cifrado**: não coloque segredo dentro. Só identificadores e permissões.
- Revogação imediata não existe em JWT puro; para isso, `exp` curto + lista de bloqueio no Redis para casos graves.

Em Go, `github.com/golang-jwt/jwt/v5` e `github.com/MicahParks/keyfunc` (busca JWKS).

## 10.4 OAuth 2.0 e OpenID Connect

**OAuth 2.0** é um protocolo de **autorização delegada**: define como um cliente obtém um token para acessar um recurso. Tem vários "fluxos"; o que importa para máquina-máquina é o **client credentials**:

```
POST /oauth/token
Content-Type: application/x-www-form-urlencoded

grant_type=client_credentials&client_id=client_padaria&client_secret=***&scope=transactions:write
```

Resposta: `{"access_token":"eyJ...","token_type":"Bearer","expires_in":900}`.

**OpenID Connect (OIDC)** é OAuth 2.0 + identidade de **pessoas** (login com Google é OIDC). Você vai usar para o **painel interno** da adquirente (operadores de suporte, analistas de risco), não para a API máquina-máquina. Em produção, use um provedor pronto (Keycloak, Auth0, Cognito); escrever servidor OAuth é um projeto em si. No nosso projeto, um `auth-sim` mínimo emite JWTs para desenvolvimento.

## 10.5 Autorização: RBAC, ABAC e scopes

**RBAC** (*Role-Based Access Control*): usuários têm **papéis**; papéis têm permissões. `analista_risco` pode `merchants:approve`; `suporte` pode `transactions:read`.

**ABAC** (*Attribute-Based*): a decisão usa atributos do sujeito, do recurso e do contexto. "Pode antecipar se o EC do token == EC do recebível E o EC está ativo E é horário comercial."

**Scopes** (OAuth): permissões granulares carregadas no token: `transactions:write`, `receivables:read`, `anticipations:write`.

Na prática combinamos: scopes no token (o que o cliente pode chamar) + regra de **posse** no caso de uso (só o próprio EC vê seus dados) + RBAC no painel interno.

Regra de ouro: **a autorização de posse acontece na consulta**, não num `if` depois. `SELECT ... WHERE id = $1 AND merchant_id = $2` (com o `merchant_id` vindo do token) torna impossível ler dado alheio, mesmo com bug no handler.

## 10.6 Middlewares em Gin

```go
func Authenticate(keys jwt.Keyfunc) gin.HandlerFunc {
    return func(c *gin.Context) {
        raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
        if raw == "" {
            problem(c, http.StatusUnauthorized, "UNAUTHENTICATED", "token ausente")
            c.Abort()
            return
        }
        claims := &Claims{}
        tok, err := jwt.ParseWithClaims(raw, claims, keys,
            jwt.WithValidMethods([]string{"RS256"}),
            jwt.WithAudience("adquirente-api"),
            jwt.WithIssuer("https://auth.adquirente.com"),
        )
        if err != nil || !tok.Valid {
            problem(c, http.StatusUnauthorized, "INVALID_TOKEN", "token inválido")
            c.Abort()
            return
        }
        c.Set("principal", Principal{ClientID: claims.Subject, MerchantID: claims.MerchantID, Scopes: claims.ScopeSet()})
        c.Next()
    }
}

func Authorize(scope string) gin.HandlerFunc {
    return func(c *gin.Context) {
        p := c.MustGet("principal").(Principal)
        if !p.Scopes.Has(scope) {
            problem(c, http.StatusForbidden, "FORBIDDEN", "escopo insuficiente: "+scope)
            c.Abort()
            return
        }
        c.Next()
    }
}
```

O handler pega `principal.MerchantID` e passa ao caso de uso; o caso de uso passa ao repositório; o repositório filtra. Nenhuma camada confia no `id` da URL sozinho.

## 10.7 Entre serviços: mTLS e identidade de serviço

Dentro do cluster, o autorizador chama o vault e o emissor; o worker chama a registradora. Nada disso usa JWT de EC. Usa **mTLS**: cada serviço tem um certificado com seu nome (`spiffe://adquirente/authorizer`), e cada servidor tem uma lista de quem pode chamar. Com um *service mesh* (Istio/Linkerd) isso vem pronto; sem mesh, `cert-manager` emite os certificados e o servidor gRPC exige `tls.RequireAndVerifyClientCert`.

## No nosso projeto

- API pública: OAuth 2.0 client credentials → JWT RS256, `exp` 15 min, JWKS; `auth-sim` em dev.
- Scopes: `merchants:read|write|approve`, `transactions:write|read`, `receivables:read`, `anticipations:write`, `chargebacks:write`.
- Posse verificada na consulta (`merchant_id` do token).
- Painel interno (fase opcional): OIDC com Keycloak e RBAC.
- Interno: gRPC com mTLS; `NetworkPolicy` como segunda camada.

---

# PARTE 11 — gRPC

## 11.1 O que é e quando usar

**gRPC** é um framework de chamadas remotas criado pelo Google. Em vez de "mandar JSON por HTTP para uma rota", você define **funções** num arquivo de contrato e chama como se fossem locais: `resp, err := issuer.Authorize(ctx, req)`. Por baixo, usa HTTP/2 e um formato binário compacto, o **Protocol Buffers** (protobuf).

| | REST/JSON | gRPC/protobuf |
|---|---|---|
| Contrato | OpenAPI (opcional) | `.proto` (obrigatório) |
| Formato | Texto (JSON) | Binário |
| Velocidade | Boa | Melhor (menos bytes, sem parse de texto, conexão multiplexada) |
| Tipagem | Gerada a partir do spec, se você quiser | Gerada sempre, em qualquer linguagem |
| Streaming | Difícil | Nativo (cliente, servidor, bidirecional) |
| Navegador / EC externo | Natural | Precisa de gateway |
| Depuração | `curl` | `grpcurl`, menos amigável |

Regra: **REST para fora** (ECs, parceiros, painel), **gRPC para dentro** (autorizador ↔ vault, autorizador ↔ emissor, worker ↔ registradora). É a divisão que a maioria das fintechs usa.

## 11.2 Protocol Buffers

O arquivo `.proto` define **mensagens** (structs) e **serviços** (interfaces):

```protobuf
syntax = "proto3";
package issuer.v1;
option go_package = "github.com/seu-usuario/adquirente/gen/issuer/v1;issuerv1";

service IssuerService {
  rpc Authorize(AuthorizeRequest) returns (AuthorizeResponse);
  rpc Reverse(ReverseRequest) returns (ReverseResponse);
}

message AuthorizeRequest {
  string transaction_id = 1;   // o número é a "tag" do campo; nunca reutilize
  string pan = 2;              // só o autorizador chama, depois de detokenizar
  string expiry = 3;           // "MM/YY"
  int64  amount = 4;           // centavos
  string currency = 5;         // "BRL"
  Product product = 6;
  int32  installments = 7;
  string merchant_mcc = 8;
}

enum Product {
  PRODUCT_UNSPECIFIED = 0;
  PRODUCT_DEBIT = 1;
  PRODUCT_CREDIT = 2;
}

message AuthorizeResponse {
  bool   approved = 1;
  string authorization_code = 2;
  string response_code = 3;    // "00" aprovada, "51" saldo insuficiente, "05" não honrar...
}

message ReverseRequest  { string transaction_id = 1; }
message ReverseResponse { bool accepted = 1; }
```

Regras de evolução: nunca mude o número de um campo; nunca mude o tipo; para remover, marque `reserved`; adicionar campo é sempre seguro (o lado antigo ignora). Isso é o que permite atualizar cliente e servidor em momentos diferentes.

## 11.3 Gerando o código Go

Instale o compilador `protoc` e os plugins, e use o **buf** (facilita muito):

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# buf.yaml e buf.gen.yaml na raiz; depois:
buf generate
```

Saem dois arquivos: `issuer.pb.go` (as mensagens) e `issuer_grpc.pb.go` (interfaces `IssuerServiceServer`/`IssuerServiceClient`).

## 11.4 Servidor (o emissor simulado)

```go
type issuerServer struct {
    issuerv1.UnimplementedIssuerServiceServer
}

func (s *issuerServer) Authorize(ctx context.Context, req *issuerv1.AuthorizeRequest) (*issuerv1.AuthorizeResponse, error) {
    // regras de simulação: cartão terminado em 0000 nega; valor > R$ 5.000 nega por limite
    if strings.HasSuffix(req.Pan, "0000") {
        return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "05"}, nil
    }
    if req.Amount > 500_000 {
        return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "51"}, nil
    }
    return &issuerv1.AuthorizeResponse{Approved: true, AuthorizationCode: randomDigits(6), ResponseCode: "00"}, nil
}

func main() {
    lis, _ := net.Listen("tcp", ":9090")
    srv := grpc.NewServer(
        grpc.Creds(credentials.NewTLS(tlsConfig)),                 // mTLS
        grpc.ChainUnaryInterceptor(otelgrpc.UnaryServerInterceptor(), loggingInterceptor),
    )
    issuerv1.RegisterIssuerServiceServer(srv, &issuerServer{})
    _ = srv.Serve(lis)
}
```

Repare: **negativa é resposta normal** (`Approved=false`), não erro. Erro gRPC é só para falha técnica. Isso é o contrato de Liskov da Parte 6.

## 11.5 Cliente (o adapter no autorizador)

```go
conn, err := grpc.NewClient("issuer-sim:9090",
    grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
    grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
)
client := issuerv1.NewIssuerServiceClient(conn)

// Adapter: implementa domain.IssuerGateway
type IssuerAdapter struct{ client issuerv1.IssuerServiceClient }

func (a *IssuerAdapter) Authorize(ctx context.Context, r transaction.AuthorizationRequest) (transaction.AuthorizationResponse, error) {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()
    resp, err := a.client.Authorize(ctx, &issuerv1.AuthorizeRequest{ /* mapeia campos */ })
    if err != nil {
        return transaction.AuthorizationResponse{}, fmt.Errorf("issuer: %w", err)
    }
    return transaction.AuthorizationResponse{Approved: resp.Approved, Code: resp.AuthorizationCode, ResponseCode: resp.ResponseCode}, nil
}
```

## 11.6 Erros, deadlines e interceptors

**Status codes gRPC**: `OK`, `INVALID_ARGUMENT`, `NOT_FOUND`, `DEADLINE_EXCEEDED`, `UNAVAILABLE`, `INTERNAL`... O cliente lê com `status.Code(err)`. `UNAVAILABLE` e `DEADLINE_EXCEEDED` são os que disparam circuit breaker/reversal.

**Deadline** é o `context.WithTimeout` propagado pela rede: o servidor sabe quanto tempo o cliente ainda vai esperar e pode desistir de trabalho inútil.

**Interceptors** são os middlewares do gRPC: autenticação (checar certificado/cliente), logging, métricas, tracing, recuperação de panic, *retry*.

## 11.7 Streaming (quando faz sentido)

- **Server streaming**: a registradora envia milhares de URs registradas em resposta a uma consulta, uma por vez, sem montar tudo em memória.
- **Bidirecional**: um canal aberto com a bandeira, com mensagens indo e voltando (como os links ISO 8583 reais).

No projeto, usaremos unário em tudo e um server-stream opcional na consulta de ônus em lote.

## No nosso projeto

- `api/proto/issuer/v1/issuer.proto`, `vault/v1/vault.proto`, `registry/v1/registry.proto`, gerados com `buf`.
- `cmd/issuer-sim`, `cmd/vault`, `cmd/registry-sim` são servidores gRPC com mTLS.
- Adapters em `internal/infrastructure/grpcclient/` implementam os ports do domínio; timeouts, breaker e métricas ficam no adapter (Decorator).
- Interceptors: OpenTelemetry, logging, recovery.

---

# PARTE 12 — Kafka e arquitetura orientada a eventos

## 12.1 Síncrono vs. assíncrono

Quando o autorizador aprova uma transação, várias coisas precisam acontecer: gerar a agenda, avisar o EC, alimentar o antifraude, atualizar relatórios. Se o autorizador fizesse tudo isso **antes de responder** à maquininha (síncrono), a resposta demoraria e qualquer um desses passos fora do ar derrubaria a autorização.

A alternativa é o autorizador só **registrar o fato** ("transação tx_987 foi capturada") e responder. Outros serviços leem esse fato **depois** e reagem, cada um no seu ritmo (assíncrono). O fato é um **evento**, e o lugar onde eventos ficam é um **broker de mensagens**. O nosso é o **Kafka**.

Vantagens: a autorização fica rápida e independente; um consumidor fora do ar não perde nada (o evento espera); adicionar um consumidor novo (um relatório novo) não toca o autorizador. Custo: **consistência eventual**: por alguns milissegundos (ou segundos), a agenda ainda não existe. A API precisa deixar isso claro (`202` ou um estado `PROCESSING`).

## 12.2 Kafka: os conceitos

- **Tópico**: um "diário" nomeado, onde eventos são **anexados** em ordem e ficam por um tempo (retenção, ex.: 7 dias). `transactions.captured`, `receivables.scheduled`, `settlements.completed`.
- **Partição**: um tópico é dividido em N partições para paralelizar. A **ordem só é garantida dentro de uma partição**. A partição é escolhida pela **chave** da mensagem; mesma chave → mesma partição → ordem. Chave = `merchant_id` garante que os eventos de um EC chegam em ordem.
- **Offset**: a posição de cada mensagem na partição. O consumidor guarda "até onde li".
- **Producer**: quem escreve. **Consumer**: quem lê.
- **Consumer group**: um conjunto de instâncias do mesmo consumidor que dividem as partições entre si. Duas instâncias do `notifier` em um grupo leem partições diferentes; se uma cai, a outra assume (*rebalance*). Grupos diferentes (`notifier`, `scheduler`) leem **todos** os eventos, cada um independentemente.
- **Retenção**: o Kafka não apaga ao consumir; apaga por tempo/tamanho. Isso permite **reprocessar** (voltar o offset) quando um consumidor tinha bug.

```
Tópico transactions.captured (3 partições)
 P0: [e1][e4][e7]...        ← consumer group "scheduler": instância A lê P0 e P1
 P1: [e2][e5][e8]...                                     instância B lê P2
 P2: [e3][e6][e9]...        ← consumer group "notifier": instância única lê P0, P1, P2
```

## 12.3 Garantias de entrega

- **At-most-once**: pode perder, nunca duplica. Inaceitável aqui.
- **At-least-once**: nunca perde, pode duplicar. É o que se usa: o produtor tenta de novo até ter confirmação (`acks=all`), o consumidor só confirma o offset **depois** de processar. A duplicata é tratada pelo **consumidor idempotente**.
- **Exactly-once**: o Kafka tem um modo transacional, mas só cobre Kafka → Kafka. Com banco de dados no meio, a garantia prática vem de **Outbox + consumidor idempotente**.

## 12.4 Outbox, na prática

```go
// dentro da Unit of Work, na mesma transação SQL:
err := s.uow.Do(ctx, func(repos Repositories) error {
    if err := repos.Transactions.Save(ctx, tx); err != nil { return err }
    for _, ev := range tx.PullEvents() {                 // eventos acumulados na entidade
        if err := repos.Outbox.Append(ctx, ev); err != nil { return err }
    }
    return nil
})
```

Um processo **relay** (uma goroutine em cada instância da API, ou um binário separado) faz, em loop:

```sql
SELECT id, event_type, payload FROM outbox WHERE published_at IS NULL ORDER BY id LIMIT 100 FOR UPDATE SKIP LOCKED;
-- publica cada um no Kafka (chave = aggregate_id), espera ack
UPDATE outbox SET published_at = now() WHERE id = ANY($1);
```

Alternativa madura: **Debezium** (CDC) lê o *write-ahead log* do PostgreSQL e publica a outbox no Kafka sem código seu.

## 12.5 Consumidor idempotente, retries e DLQ

```go
func (c *SchedulerConsumer) Handle(ctx context.Context, msg kafka.Message) error {
    var ev TransactionCaptured
    if err := json.Unmarshal(msg.Value, &ev); err != nil { return Permanent(err) } // não adianta tentar de novo

    return c.uow.Do(ctx, func(repos Repositories) error {
        seen, err := repos.Processed.MarkIfNew(ctx, "scheduler", ev.EventID) // INSERT ... ON CONFLICT DO NOTHING
        if err != nil { return err }
        if !seen { return nil }                                            // duplicata: ignora
        receivables := receivable.Schedule(ev, c.calendar, c.feePlans)      // regra de domínio
        return repos.Receivables.SaveAll(ctx, receivables)
    })
}
```

Se `Handle` retorna erro **transitório** (banco fora), o consumidor **não** confirma o offset e tenta de novo com backoff. Se retorna erro **permanente** (mensagem malformada, regra impossível), a mensagem vai para um tópico **DLQ** (*dead-letter queue*, `transactions.captured.dlq`) para análise humana, e o offset avança para não travar a partição. Um alerta dispara quando a DLQ recebe algo.

## 12.6 Desenhando os eventos

Um evento é um **fato passado**, nomeado no passado, com tudo o que o consumidor precisa (para não ter que consultar de volta):

```json
{
  "event_id": "evt_01J...",
  "event_type": "transaction.captured",
  "version": 1,
  "occurred_at": "2026-09-11T14:03:22Z",
  "aggregate_id": "tx_987",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "data": {
    "transaction_id": "tx_987",
    "merchant_id": "m_01HZX",
    "amount": 120000,
    "product": "CREDIT",
    "installments": 12,
    "brand": "VISA",
    "captured_at": "2026-09-11T14:03:22Z",
    "fee_plan_id": "fp_3"
  }
}
```

Nunca coloque PAN em evento. `version` permite evoluir o formato. Use um **schema registry** (Avro/Protobuf) quando houver muitos consumidores; para começar, JSON com versão basta.

Tópicos e quem consome:

| Tópico | Produzido por | Consumido por |
|---|---|---|
| `transaction.authorized` / `.captured` / `.canceled` | API (autorizador) | scheduler (agenda), notifier, antifraude, analytics |
| `receivable.scheduled` / `.anticipated` / `.settled` / `.canceled` | scheduler, anticipation, settlement | registry-sync, notifier, ledger, analytics |
| `settlement.completed` / `.failed` | settlement worker | notifier, reconciliation |
| `chargeback.opened` / `.resolved` | API | scheduler (cancela recebível), notifier, risco |

## 12.7 Kafka em Go

Bibliotecas: `segmentio/kafka-go` (simples) ou `twmb/franz-go` (mais rápido e completo). Exemplo com kafka-go:

```go
writer := &kafka.Writer{
    Addr:         kafka.TCP("kafka:9092"),
    Balancer:     &kafka.Hash{},        // partição pela chave
    RequiredAcks: kafka.RequireAll,     // acks=all
    Async:        false,
}
err := writer.WriteMessages(ctx, kafka.Message{
    Topic: "transaction.captured",
    Key:   []byte(ev.MerchantID),
    Value: payload,
    Headers: []kafka.Header{{Key: "traceparent", Value: []byte(traceparent)}},
})

reader := kafka.NewReader(kafka.ReaderConfig{
    Brokers: []string{"kafka:9092"}, GroupID: "scheduler", Topic: "transaction.captured",
})
for {
    msg, err := reader.FetchMessage(ctx)       // não confirma ainda
    if err != nil { break }
    if err := handler.Handle(ctx, msg); err != nil { /* retry/DLQ */ continue }
    _ = reader.CommitMessages(ctx, msg)        // confirma só depois de processar
}
```

Em desenvolvimento, suba o Kafka pelo Docker Compose (imagem `apache/kafka` em modo KRaft, sem ZooKeeper) com uma UI (Kafka UI / Redpanda Console) para ver os tópicos.

## No nosso projeto

- Tabela `outbox` + relay em goroutine na API; `processed_events` em todo consumidor.
- Chave de partição = `merchant_id` (ordem por EC).
- Tópicos com nome `dominio.fato`, payload JSON versionado, `trace_id` propagado no header.
- DLQ por tópico + alerta.
- Consumidores: `scheduler` (gera agenda, lança no ledger e registra as URs na registradora) e `notifier` (webhooks). O ledger é escrito pelos próprios casos de uso, na mesma transação do dado; um `reconciliation` fica como exercício.

---

# PARTE 13 — Observabilidade: logs, métricas e tracing

## 13.1 O que é observabilidade

**Monitorar** é olhar painéis de coisas que você previu ("CPU alta"). **Observabilidade** é conseguir responder perguntas que você **não** previu ("por que as autorizações do EC m_123 estão 3x mais lentas desde as 14h só no produto parcelado?") usando os dados que o sistema emite. Três tipos de dado, os "três pilares":

| Pilar | O que é | Responde |
|---|---|---|
| **Logs** | Eventos discretos com texto e campos | "O que aconteceu com a requisição X?" |
| **Métricas** | Números agregados ao longo do tempo | "Quantas autorizações/segundo? Qual a taxa de erro?" |
| **Traces** | O caminho de uma requisição por todos os serviços, com tempo de cada passo | "Onde os 800ms foram gastos?" |

O que une os três é a **correlação**: um `trace_id` que aparece no log, no trace e como *exemplar* na métrica.

## 13.2 Logs estruturados com `slog`

Log de texto livre (`log.Printf("erro ao autorizar %s", id)`) é bom para humano ler no terminal e péssimo para máquina buscar. **Log estruturado** é JSON com campos:

```json
{"time":"2026-09-11T14:03:22Z","level":"INFO","msg":"transaction authorized","trace_id":"4bf9...","request_id":"9b1f...","merchant_id":"m_01HZX","transaction_id":"tx_987","amount":120000,"product":"CREDIT","issuer_code":"00","duration_ms":312}
```

Go tem isso na biblioteca padrão desde a 1.21: `log/slog`.

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
slog.SetDefault(logger)

// no handler/caso de uso, com contexto (trace_id vem do ctx via um Handler customizado)
slog.InfoContext(ctx, "transaction authorized",
    "merchant_id", tx.MerchantID(),
    "transaction_id", tx.ID(),
    "amount", tx.Amount(),
    "issuer_code", resp.ResponseCode,
    "duration_ms", time.Since(start).Milliseconds(),
)
```

Regras:
- **Níveis**: `DEBUG` (só em dev), `INFO` (fatos de negócio: autorizada, liquidada), `WARN` (algo estranho, mas seguiu: retry, emissor lento), `ERROR` (falhou e alguém precisa olhar). Não logue `ERROR` para negativa do emissor: é `INFO`, é negócio normal.
- **Um evento por linha**; sem `fmt.Sprintf` dentro da mensagem: os dados vão em campos.
- **Nunca dados sensíveis**. Tipos como `CardToken` e `MaskedPAN` implementam `slog.LogValuer` para que, mesmo que alguém logue a struct inteira, o PAN saia mascarado:

```go
func (p PAN) LogValue() slog.Value { return slog.StringValue(Mask(string(p))) }
```

- Os logs vão para `stdout`; o Kubernetes coleta e envia para o backend (Loki, Elasticsearch, Datadog). O programa não escreve arquivo.

## 13.3 Métricas com Prometheus

O **Prometheus** coleta métricas fazendo `GET /metrics` em cada serviço a cada 15s e guarda séries temporais. O **Grafana** desenha. Tipos de métrica:

- **Counter**: só cresce. `transactions_authorized_total{product="CREDIT",result="approved"}`.
- **Gauge**: sobe e desce. `settlement_pending_amount_cents`, `kafka_consumer_lag`.
- **Histogram**: distribuição de valores em faixas. `issuer_authorize_duration_seconds` com buckets 0.05, 0.1, 0.25, 0.5, 1, 2. Permite calcular **p50/p95/p99** (o tempo abaixo do qual 50/95/99% das chamadas ficam). A média mente; o p99 conta a verdade.

Método **RED** para serviços: **R**ate (quantas/s), **E**rrors (quantas falham), **D**uration (quanto demoram). Para cada endpoint e cada dependência. Método **USE** para recursos: **U**tilization, **S**aturation, **E**rrors (pool de conexões, CPU).

```go
var authorizeDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
    Name:    "issuer_authorize_duration_seconds",
    Help:    "Tempo de resposta do emissor",
    Buckets: []float64{.05, .1, .25, .5, 1, 2, 5},
}, []string{"issuer", "result"})

timer := prometheus.NewTimer(authorizeDuration.WithLabelValues("sim", result))
defer timer.ObserveDuration()
```

Cuidado com **cardinalidade**: nunca use `merchant_id` ou `transaction_id` como label (milhões de séries). Labels são para poucas categorias: produto, resultado, código de resposta.

Métricas de **negócio** são as mais valiosas: valor autorizado por minuto, taxa de aprovação por bandeira, valor liquidado hoje vs. previsto, lag da outbox, tamanho da DLQ.

## 13.4 Tracing distribuído com OpenTelemetry

Uma autorização atravessa: Gin → caso de uso → Redis (idempotência) → PostgreSQL → vault (gRPC) → emissor (gRPC) → PostgreSQL → outbox → Kafka → scheduler. Um **trace** é o registro dessa viagem inteira; cada passo é um **span** com início, fim, atributos e o span pai. Vê-se como uma cascata:

```
trace 4bf92f35... (total 412ms)
└─ POST /v1/transactions                          [0 ─────────────── 412ms]
   ├─ idempotency.check (redis)                   [2 ─ 4ms]
   ├─ merchant.load (postgres)                    [5 ── 11ms]
   ├─ vault.Detokenize (grpc)                     [12 ──── 25ms]
   ├─ issuer.Authorize (grpc)                     [26 ─────────────────── 380ms]  ← aqui está o tempo
   ├─ transaction.save + outbox (postgres)        [382 ── 398ms]
   └─ response                                    [399 ─ 412ms]
```

**OpenTelemetry (OTel)** é o padrão aberto para gerar traces (e métricas e logs). O programa emite para um **Collector**, que envia para o backend (**Jaeger**, **Tempo**, Datadog...). A **propagação de contexto** é o que costura os serviços: o `trace_id` viaja no header HTTP `traceparent` (W3C), nos metadados gRPC e nos headers Kafka.

```go
// inicialização (uma vez, no main)
exp, _ := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint("otel-collector:4317"), otlptracegrpc.WithInsecure())
tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exp),
    sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("api"))),
)
otel.SetTracerProvider(tp)
otel.SetTextMapPropagator(propagation.TraceContext{})

// instrumentação automática: Gin, gRPC, database/sql, kafka
r.Use(otelgin.Middleware("api"))
// spans manuais para passos de negócio
ctx, span := otel.Tracer("application").Start(ctx, "AuthorizeTransaction")
defer span.End()
span.SetAttributes(attribute.String("product", string(cmd.Product)), attribute.Int("installments", cmd.Installments))
```

Com Kafka o span do consumidor é filho do span do produtor (`traceparent` no header da mensagem): você vê no Jaeger que a agenda foi gerada 230ms após a captura, num outro serviço.

## 13.5 Health checks, alertas e SLOs

**Health checks** para o Kubernetes (Parte 14):
- `/health/live` (**liveness**): "o processo está vivo?" Responde 200 sempre que o servidor HTTP responde. Se falhar, o pod é reiniciado.
- `/health/ready` (**readiness**): "posso receber tráfego?" Verifica banco, Redis, Kafka. Se falhar, o pod sai do balanceador, mas não reinicia.

**Alertas** são regras sobre métricas que acordam alguém. Poucos e bons:
- Taxa de erro 5xx > 1% por 5 min.
- p99 de autorização > 2s por 5 min.
- Taxa de aprovação caiu > 20% vs. mesma hora de ontem (pode ser emissor com problema).
- Lag da outbox > 1.000 eventos ou > 60s.
- DLQ recebeu mensagem.
- Liquidação do dia não concluída até 11h.

**SLO** (*Service Level Objective*): a promessa medível. "99,95% das autorizações respondem em < 1s no mês." O **error budget** (0,05%) é quanto você pode "gastar" com incidentes e deploys arriscados antes de parar tudo para estabilizar.

## 13.6 O que um bom dashboard da adquirente mostra

- Autorizações/min por produto e resultado; taxa de aprovação por bandeira/emissor.
- p50/p95/p99 do emissor, do vault, do banco.
- Valor capturado hoje; valor a liquidar hoje vs. já liquidado; valor antecipado hoje.
- Lag de consumidores Kafka; tamanho da outbox; DLQ.
- Erros por `code` de *Problem Details*.
- Saturação: pool de conexões, CPU/memória por pod, réplicas do HPA.

## No nosso projeto

- `slog` JSON em todos os binários; `trace_id`/`request_id`/`merchant_id` em todo log via handler que lê do `ctx`; `LogValuer` em tipos sensíveis.
- Prometheus em `/metrics` com métricas RED por endpoint e por dependência, mais métricas de negócio.
- OpenTelemetry com Collector → Jaeger (dev) e Tempo (prod); instrumentação de Gin, gRPC, `database/sql`, Kafka; spans manuais nos casos de uso.
- `/health/live` e `/health/ready` em todo serviço.
- Grafana com o dashboard acima e alertas via Alertmanager.

---

# PARTE 14 — Kubernetes

## 14.1 Containers, em uma página

Um **container** é um processo isolado que carrega consigo tudo o que precisa (binário, bibliotecas, configuração) numa **imagem**. O **Docker** constrói imagens (a partir de um `Dockerfile`) e roda containers. Você já fez isso no `simple_bank`. O que muda numa adquirente:

```dockerfile
# deploy/docker/api.Dockerfile
FROM golang:1.27 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot   # sem shell, sem root, ~2 MB
COPY --from=builder /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]
```

`distroless` + `nonroot` reduzem a superfície de ataque (Parte 9). Um Dockerfile por binário (`api`, `issuer-sim`, `vault`, `settlement`, `scheduler`, `notifier`...).

## 14.2 Por que orquestrar

Com Docker Compose você roda tudo numa máquina. Em produção você precisa de: várias réplicas da API em máquinas diferentes; reiniciar o que morrer; subir mais réplicas quando o tráfego cresce (sexta 18h) e descer depois; atualizar sem derrubar; segredos e configuração fora da imagem; rede entre serviços com nomes fixos. O **Kubernetes** (K8s) faz isso. Você descreve o **estado desejado** em YAML e ele trabalha para manter.

## 14.3 Os objetos que você vai usar

| Objeto | O que é | Analogia |
|---|---|---|
| **Pod** | A menor unidade: um ou mais containers que rodam juntos, com o mesmo IP | Um "computador" descartável |
| **Deployment** | "Quero N réplicas deste pod, com esta imagem"; faz *rolling update* | O gerente que mantém N garçons na sala |
| **Service** | Um nome DNS fixo e um IP que balanceia entre os pods de um Deployment | O telefone do restaurante (não muda quando troca o garçom) |
| **Ingress** | A porta de entrada HTTP(S) do cluster, com TLS e rotas por host/caminho | A fachada com o letreiro |
| **ConfigMap** | Configuração não sensível (URLs, níveis de log) | O quadro de avisos |
| **Secret** | Configuração sensível (senhas, chaves), em base64, cifrada em repouso se configurado | O cofre |
| **HorizontalPodAutoscaler (HPA)** | Sobe/desce réplicas por CPU, memória ou métrica custom | Chamar mais garçons quando enche |
| **CronJob** | Um pod que roda num horário | O alarme das 6h para o worker de liquidação |
| **NetworkPolicy** | Firewall entre pods | Quem pode entrar na cozinha |
| **Namespace** | Um "andar" do cluster para isolar ambientes/times | — |
| **PersistentVolumeClaim** | Disco persistente (para Postgres/Kafka, se rodarem no cluster) | — |

Em produção, banco e Kafka geralmente ficam **fora** do cluster (RDS, MSK, Confluent, ou operadores dedicados). No estudo, rodam dentro.

## 14.4 Manifests do projeto (o essencial)

```yaml
# deploy/k8s/api/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  namespace: adquirente
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate: { maxUnavailable: 0, maxSurge: 1 }     # nunca fica sem réplica durante deploy
  selector:
    matchLabels: { app: api }
  template:
    metadata:
      labels: { app: api }
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
        - name: api
          image: ghcr.io/seu-usuario/adquirente-api:1.4.2   # tag imutável, nunca "latest"
          ports: [{ containerPort: 8080 }]
          envFrom:
            - configMapRef: { name: api-config }
            - secretRef: { name: api-secrets }
          resources:
            requests: { cpu: "250m", memory: "128Mi" }       # o que reserva
            limits:   { cpu: "1",    memory: "512Mi" }       # o teto
          readinessProbe:
            httpGet: { path: /health/ready, port: 8080 }
            periodSeconds: 5
          livenessProbe:
            httpGet: { path: /health/live, port: 8080 }
            periodSeconds: 10
            failureThreshold: 3
          lifecycle:
            preStop:
              exec: { command: ["sleep", "5"] }              # dá tempo do balanceador parar de mandar tráfego
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
      terminationGracePeriodSeconds: 30                       # o graceful shutdown da Parte 4 tem 30s
---
apiVersion: v1
kind: Service
metadata: { name: api, namespace: adquirente }
spec:
  selector: { app: api }
  ports: [{ port: 80, targetPort: 8080 }]
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata: { name: api, namespace: adquirente }
spec:
  scaleTargetRef: { apiVersion: apps/v1, kind: Deployment, name: api }
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource: { name: cpu, target: { type: Utilization, averageUtilization: 60 } }
---
apiVersion: batch/v1
kind: CronJob
metadata: { name: settlement, namespace: adquirente }
spec:
  schedule: "0 6 * * 1-5"                     # 06:00, segunda a sexta
  concurrencyPolicy: Forbid                   # nunca duas liquidações ao mesmo tempo
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          restartPolicy: Never
          containers:
            - name: settlement
              image: ghcr.io/seu-usuario/adquirente-settlement:1.4.2
              envFrom: [{ secretRef: { name: api-secrets } }]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: vault-only-from-api, namespace: adquirente }
spec:
  podSelector: { matchLabels: { app: vault } }
  policyTypes: [Ingress]
  ingress:
    - from: [{ podSelector: { matchLabels: { app: api } } }]
      ports: [{ port: 9090 }]
```

Pontos que importam para pagamentos: `maxUnavailable: 0` (deploy sem queda), probes (o K8s só manda tráfego para quem está pronto), `preStop` + `terminationGracePeriodSeconds` (autorização em andamento termina), `concurrencyPolicy: Forbid` no CronJob (liquidação nunca roda em dobro), `NetworkPolicy` no vault.

## 14.5 Rodando localmente

- **kind** ou **minikube** criam um cluster na sua máquina.
- **Helm** empacota os YAMLs com variáveis (`values.yaml`) por ambiente; **Kustomize** faz o mesmo por sobreposição. Escolha um; o guia usa Kustomize (`base/` + `overlays/dev|prod`).
- Comandos do dia a dia: `kubectl get pods -n adquirente`, `kubectl logs -f deploy/api`, `kubectl describe pod X` (por que não sobe?), `kubectl port-forward svc/api 8080:80`, `kubectl rollout status deploy/api`, `kubectl rollout undo deploy/api`.

## 14.6 CI/CD

Pipeline (GitHub Actions ou similar), a cada push:

1. `go vet`, `golangci-lint`, `govulncheck`.
2. `go test -race ./...` + testes de integração com testcontainers.
3. `buf lint` / Spectral no OpenAPI.
4. Build das imagens, scan com Trivy, push com tag = versão/commit.
5. Deploy no ambiente de dev automaticamente; em prod, com aprovação e **canary** (10% do tráfego na versão nova, observar métricas, promover ou reverter). Ferramentas: Argo CD/Argo Rollouts, Flux.

## 14.7 Operação

- **Observabilidade** (Parte 13) instalada no cluster: Prometheus Operator, Grafana, Loki, Tempo/Jaeger, OTel Collector.
- **Service mesh** (Istio/Linkerd): mTLS automático, retries, circuit breaking e métricas por serviço sem código. Opcional, mas comum em pagamentos.
- **Backups** do PostgreSQL testados (restaurar de verdade, de vez em quando). PITR (*point-in-time recovery*).
- **Multi-AZ**: réplicas espalhadas por zonas de disponibilidade (`topologySpreadConstraints`).
- **Runbooks**: "emissor fora do ar", "liquidação falhou", "DLQ crescendo", cada um com passos.

## No nosso projeto

- Um Dockerfile por binário (`distroless`, `nonroot`).
- `deploy/k8s/` com Kustomize: `base/` (api, vault, issuer-sim, registry-sim, scheduler, notifier, settlement CronJob, postgres, redis, kafka, otel-collector, jaeger, prometheus, grafana) e `overlays/dev`.
- Cluster local com kind; `make k8s-up` sobe tudo.
- CI com lint, testes, build e scan.

---

# PARTE 15 — O roteiro: construindo a adquirente arquivo por arquivo

Aqui você constrói o projeto inteiro. **Todo o código desta parte foi compilado com Go 1.27, passou em `go vet` e em 31 funções de teste (45 casos, contando os subtestes)** antes de entrar no guia; o `docker-compose.yml` passou em `docker compose config` e os manifests seguem o esquema do Kubernetes 1.31. Você vai digitá-lo (não copie e cole em bloco: digitar é como o conhecimento entra), compilar a cada arquivo e rodar os testes a cada pacote.

**Como ler cada arquivo.** O título traz o caminho exato. Antes do código há um parágrafo dizendo o que o arquivo faz e qual conceito do guia ele materializa (com a parte para reler). Dentro do código, os comentários explicam as decisões linha a linha. Depois de alguns arquivos, há um bloco **Compile e teste**.

**As fases e o que cada uma entrega:**

| Fase | Entrega | Você vai perceber que aprendeu |
|---|---|---|
| 0 | Projeto criado, Docker Compose subindo, ferramentas instaladas | Layout Go, containers, módulos |
| 1 | O domínio inteiro, testado, sem banco nem HTTP | Regras de negócio da Parte 1 viradas código; DDD; State; Money |
| 2 | Os casos de uso | Clean Architecture, ports, Unit of Work, Saga, Outbox |
| 3 | Os contratos: OpenAPI e Protobuf | Design-first, gRPC |
| 4 | Banco de dados: migrações e repositórios | SQL, locks, ORM vs SQL explícito, idempotência |
| 5 | O resto da infraestrutura: Kafka, gRPC, Redis, cripto, observabilidade | Adapters, breaker, AES-GCM, slog/OTel/Prometheus |
| 6 | Handlers HTTP e gRPC | Gin, middlewares, JWT, Problem Details |
| 7 | Os sete binários | Composition root, graceful shutdown |
| 8 | O fluxo completo rodando | Você opera uma adquirente |
| 9 | Kubernetes | Manifests, probes, NetworkPolicy, CronJob |
| 10 | Endurecimento e compliance | O checklist da Parte 2.8 fechado |

**A árvore final**, para você saber onde está a qualquer momento:

```
adquirente/
├── api/
│   ├── openapi.yaml
│   └── proto/{issuer,vault}/v1/*.proto
├── buf.yaml  buf.gen.yaml
├── cmd/{api,auth-sim,issuer-sim,vault,scheduler,settlement,notifier}/main.go
├── deploy/
│   ├── docker/Dockerfile
│   ├── prometheus.yml
│   ├── k8s/base/*.yaml
│   └── terraform/...                      (Parte 16)
├── docker-compose.yml  Makefile  .env.example  .gitignore
├── gen/                                    (gerado pelo buf; não versionado)
├── internal/
│   ├── domain/{shared,merchant,transaction,receivable,anticipation,settlement,chargeback,ledger}/
│   ├── application/{merchant,transaction,receivable,settlement,anticipation,chargeback,notification}/ + uow.go errors.go idempotency.go
│   ├── infrastructure/{postgres,gormrepo,kafka,grpcclient,redis,registry,bank,cryptoutil,config,observability,webhook}/
│   └── handler/{http,http/middleware,grpc}/
├── migrations/*.sql
└── scripts/gen-dev-certs.sh
```

---

## Fase 0 — Ambiente e esqueleto

**Objetivo:** projeto criado, ferramentas instaladas, Docker Compose subindo tudo.

### 0.1 Ferramentas

```bash
# Go 1.27+: https://go.dev/dl  |  Docker Desktop  |  Git
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest      # migrações
go install github.com/bufbuild/buf/cmd/buf@latest                        # protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest # lint
go install golang.org/x/vuln/cmd/govulncheck@latest                      # vulnerabilidades
```

Confira que `$(go env GOPATH)/bin` está no seu `PATH` (`buf --version` deve responder).

### 0.2 Pastas e módulo

```bash
mkdir adquirente && cd adquirente
go mod init github.com/seu-usuario/adquirente
mkdir -p cmd/{api,auth-sim,issuer-sim,vault,scheduler,settlement,notifier}
mkdir -p internal/domain/{shared,merchant,transaction,receivable,anticipation,settlement,chargeback,ledger}
mkdir -p internal/application/{merchant,transaction,receivable,settlement,anticipation,chargeback,notification}
mkdir -p internal/infrastructure/{postgres,gormrepo,kafka,grpcclient,redis,registry,bank,cryptoutil,config,observability,webhook}
mkdir -p internal/handler/{http/middleware,grpc} api/proto/{issuer,vault}/v1 migrations deploy/{docker,k8s/base,terraform} scripts
```

O nome do módulo (`github.com/seu-usuario/adquirente`) aparece em todos os imports internos. Se você trocar, troque em todos os arquivos.

### 0.3 `.gitignore`

Segredos, saídas e código gerado não entram no git.

```bash
.env
certs/
out/
gen/
*.exe
```

### 0.4 `.env.example`

Copie para `.env`. É a lista de tudo que os binários leem do ambiente (Parte 9.4: segredos nunca no código).

```bash
# Copie para .env (que está no .gitignore) e ajuste. Nada aqui é segredo de produção.
APP_ENV=dev
LOG_LEVEL=debug
HTTP_ADDR=:8080
DATABASE_URL=postgres://adq:adq@localhost:5432/adquirente?sslmode=disable
REDIS_ADDR=localhost:6379
KAFKA_BROKERS=localhost:9092
ISSUER_ADDR=localhost:9091
VAULT_ADDR=localhost:9092
OTLP_ENDPOINT=localhost:4317
JWT_PUBLIC_KEY_FILE=certs/jwt-public.pem
JWT_PRIVATE_KEY_FILE=certs/jwt-private.pem
JWT_ISSUER=https://auth.adquirente.local
JWT_AUDIENCE=adquirente-api
REGISTRY_LIENS_FILE=certs/liens.json
SETTLEMENT_OUTPUT_DIR=./out/settlements
WEBHOOK_SECRET=dev-secret-change-me
MAX_TX_AMOUNT=5000000
# gere com: openssl rand -base64 32
VAULT_KEY_BASE64=
```

### 0.5 `Makefile`

Os comandos do dia a dia num lugar só.

```makefile
DATABASE_URL ?= postgres://adq:adq@localhost:5432/adquirente?sslmode=disable

.PHONY: run test lint generate migrate-up migrate-down compose-up compose-down certs settle

run:            ## roda a API local (precisa de compose-up)
	go run ./cmd/api

test:           ## testes com detector de race
	go test -race -cover ./...

lint:
	go vet ./...
	golangci-lint run ./...
	govulncheck ./...

generate:       ## gera código gRPC a partir dos .proto
	buf lint && buf generate

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down 1

compose-up:     ## sobe tudo (postgres, redis, kafka, jaeger, prometheus, grafana e os serviços)
	docker compose up -d --build

compose-down:
	docker compose down -v

certs:          ## chaves de dev: JWT (RSA) e um par de clientes; NUNCA use em produção
	./scripts/gen-dev-certs.sh

settle:         ## liquida a data informada: make settle DATE=2026-10-13
	SETTLEMENT_DATE=$(DATE) go run ./cmd/settlement
```

### 0.6 `docker-compose.yml`

Tudo o que a adquirente precisa em volta: PostgreSQL, Redis, Kafka (modo KRaft, sem ZooKeeper), Kafka UI, Jaeger (traces), Prometheus e Grafana (métricas), a migração, e os nossos serviços. Leia os comentários: cada serviço tem um motivo.

```yaml
# Ambiente de desenvolvimento completo. Sobe com: docker compose up -d --build

# Variáveis comuns a todos os nossos serviços (âncora YAML reutilizada com <<: *common-env).
# Chaves "x-" no topo do arquivo são ignoradas pelo Compose: servem só para reaproveitar.
x-common-env: &common-env
  APP_ENV: dev
  DATABASE_URL: postgres://adq:adq@postgres:5432/adquirente?sslmode=disable
  REDIS_ADDR: redis:6379
  KAFKA_BROKERS: kafka:19092
  OTLP_ENDPOINT: jaeger:4317
  JWT_PUBLIC_KEY_FILE: /certs/jwt-public.pem
  JWT_PRIVATE_KEY_FILE: /certs/jwt-private.pem
  REGISTRY_LIENS_FILE: /certs/liens.json
  WEBHOOK_SECRET: dev-secret-change-me

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: adq
      POSTGRES_PASSWORD: adq
      POSTGRES_DB: adquirente
    ports: ["5432:5432"]
    volumes: ["pgdata:/var/lib/postgresql/data"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U adq -d adquirente"]
      interval: 5s
      timeout: 3s
      retries: 10

  migrate:
    image: migrate/migrate:v4.18.1
    depends_on:
      postgres: { condition: service_healthy }
    volumes: ["./migrations:/migrations:ro"]
    command: ["-path", "/migrations", "-database", "postgres://adq:adq@postgres:5432/adquirente?sslmode=disable", "up"]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  kafka:
    image: apache/kafka:3.9.0          # modo KRaft: sem ZooKeeper
    ports: ["9092:9092"]
    environment:
      KAFKA_NODE_ID: 1
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
      KAFKA_LISTENERS: PLAINTEXT://:19092,CONTROLLER://:9093,EXTERNAL://:9092
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:19092,EXTERNAL://localhost:9092
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT,EXTERNAL:PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk

  kafka-ui:
    image: provectuslabs/kafka-ui:latest
    ports: ["8081:8080"]
    environment:
      KAFKA_CLUSTERS_0_NAME: local
      KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:19092
    depends_on: [kafka]

  jaeger:
    image: jaegertracing/all-in-one:1.62.0
    ports: ["16686:16686", "4317:4317"]   # UI e OTLP/gRPC
    environment:
      COLLECTOR_OTLP_ENABLED: "true"

  prometheus:
    image: prom/prometheus:v2.55.0
    ports: ["9090:9090"]
    volumes: ["./deploy/prometheus.yml:/etc/prometheus/prometheus.yml:ro"]

  grafana:
    image: grafana/grafana:11.3.0
    ports: ["3000:3000"]
    environment:
      GF_SECURITY_ADMIN_PASSWORD: admin
    depends_on: [prometheus]

  # ---------- nossos serviços ----------
  auth-sim:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: auth-sim } }
    environment: { <<: *common-env, HTTP_ADDR: ":8080", AUTH_CLIENTS_FILE: /certs/clients.json }
    volumes: ["./certs:/certs:ro"]
    ports: ["8082:8080"]

  issuer-sim:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: issuer-sim } }
    environment: { <<: *common-env, GRPC_ADDR: ":9090", ISSUER_SIM_LATENCY_MS: "50" }
    depends_on: [jaeger]

  vault:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: vault } }
    environment: { <<: *common-env, GRPC_ADDR: ":9090", VAULT_KEY_BASE64: "${VAULT_KEY_BASE64}" }
    depends_on:
      migrate: { condition: service_completed_successfully }

  api:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: api } }
    environment: { <<: *common-env, HTTP_ADDR: ":8080", ISSUER_ADDR: issuer-sim:9090, VAULT_ADDR: vault:9090 }
    volumes: ["./certs:/certs:ro"]
    ports: ["8080:8080"]
    depends_on:
      migrate: { condition: service_completed_successfully }
      issuer-sim: { condition: service_started }
      vault: { condition: service_started }
      kafka: { condition: service_started }

  scheduler:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: scheduler } }
    environment: { <<: *common-env }
    volumes: ["./certs:/certs:ro"]
    depends_on:
      migrate: { condition: service_completed_successfully }
      kafka: { condition: service_started }

  notifier:
    build: { context: ., dockerfile: deploy/docker/Dockerfile, args: { SERVICE: notifier } }
    environment: { <<: *common-env }
    depends_on:
      migrate: { condition: service_completed_successfully }
      kafka: { condition: service_started }

volumes:
  pgdata:
```

### 0.7 `deploy/prometheus.yml`

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: api
    static_configs:
      - targets: ["api:8080"]
```

### 0.8 `scripts/gen-dev-certs.sh`

Gera o par de chaves RSA do JWT, o arquivo de clientes OAuth, o arquivo de ônus da registradora simulada e a chave AES do cofre. **Só para desenvolvimento.**

```bash
#!/usr/bin/env sh
# Gera o material de desenvolvimento em ./certs. Em produção: KMS/Secrets Manager + cert-manager.
set -e
mkdir -p certs
cd certs

# Par RSA para o JWT (auth-sim assina com a privada; a API valida com a pública)
openssl genrsa -out jwt-private.pem 2048 2>/dev/null
openssl rsa -in jwt-private.pem -pubout -out jwt-public.pem 2>/dev/null

# Clientes OAuth de exemplo (segredos em claro SÓ em dev)
cat > clients.json <<'JSON'
{
  "operador": {
    "secret": "op-s3cr3t",
    "scope": "merchants:read merchants:write merchants:approve chargebacks:write"
  },
  "client_padaria": {
    "secret": "padaria-s3cr3t",
    "merchant_id": "SUBSTITUA_PELO_ID_DO_EC",
    "scope": "merchants:read merchants:write transactions:write transactions:read receivables:read anticipations:write"
  }
}
JSON

# Ônus de exemplo para a registradora simulada (vazio = sem ônus)
echo "[]" > liens.json

# Chave AES-256 do vault (32 bytes em base64) → cole no .env como VAULT_KEY_BASE64
echo "VAULT_KEY_BASE64=$(openssl rand -base64 32)" > vault.env

echo "ok: certs/jwt-*.pem, certs/clients.json, certs/liens.json, certs/vault.env"
```

```bash
chmod +x scripts/gen-dev-certs.sh && make certs
cat certs/vault.env >> .env       # VAULT_KEY_BASE64
```

**Pronto quando:** `go build ./...` compila um módulo vazio sem reclamar e `docker compose up -d postgres redis kafka jaeger` sobe (os nossos serviços ainda não existem; o Compose vai reclamar deles até a Fase 7, use `docker compose up -d <serviço>` por nome).

---

## Fase 1 — O domínio

**Objetivo:** todas as regras da Parte 1 em Go puro, sem importar nada além da biblioteca padrão (e o `uuid`). É a fase mais importante. Releia a Parte 1 e a Parte 6 antes.

```bash
go get github.com/google/uuid@latest
```

### 1.1 Pacote `shared`: o que todo contexto usa

#### `internal/domain/shared/money.go`

Dinheiro em centavos e taxas em basis points (Parte 8.5). `Split` resolve o problema dos centavos das parcelas (Parte 1.4).

```go
package shared

import "fmt"

// Money é dinheiro em centavos. R$ 10,50 = Money(1050).
// Nunca use float para dinheiro: 0,10 não tem representação exata em binário.
type Money int64

// Bps é uma taxa em "basis points": 1 bps = 0,01%. Assim, 250 bps = 2,50%.
// Guardar taxas como inteiro evita os mesmos problemas de arredondamento do float.
type Bps int

func (m Money) IsPositive() bool { return m > 0 }
func (m Money) IsZero() bool     { return m == 0 }

// String formata como moeda brasileira: "R$ 10,50" ou "-R$ 10,50".
func (m Money) String() string {
	v := int64(m)
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, v/100, v%100)
}

// ApplyBps calcula gross × rate, truncando (arredonda para baixo).
// A multiplicação vem antes da divisão para não perder precisão.
// Ex.: ApplyBps(10000, 250) = 10000*250/10000 = 250 (R$ 2,50 sobre R$ 100,00).
func ApplyBps(gross Money, rate Bps) Money {
	return Money(int64(gross) * int64(rate) / 10000)
}

// Split divide total em n partes inteiras. Quando não divide exato, os centavos
// que sobram vão para a PRIMEIRA parte, para que a soma bata sempre com o total.
// Ex.: Split(1000, 3) = [334, 333, 333].
func Split(total Money, n int) []Money {
	if n <= 0 {
		return nil
	}
	base := int64(total) / int64(n)
	rest := int64(total) - base*int64(n)
	parts := make([]Money, n)
	for i := range parts {
		parts[i] = Money(base)
	}
	parts[0] += Money(rest)
	return parts
}

// Sum soma uma lista de valores.
func Sum(values ...Money) Money {
	var total Money
	for _, v := range values {
		total += v
	}
	return total
}
```

#### `internal/domain/shared/money_test.go`

Table-driven (Parte 3.7). Repare no caso "trunca para baixo": a regra de arredondamento é uma **decisão** e está documentada no teste.

```go
package shared

import "testing"

func TestMoneyString(t *testing.T) {
	cases := []struct {
		in   Money
		want string
	}{
		{1050, "R$ 10,50"},
		{5, "R$ 0,05"},
		{0, "R$ 0,00"},
		{-1050, "-R$ 10,50"},
		{120000, "R$ 1200,00"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Money(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestApplyBps(t *testing.T) {
	cases := []struct {
		name  string
		gross Money
		rate  Bps
		want  Money
	}{
		{"2,5% de R$ 100,00", 10000, 250, 250},
		{"1,5% de R$ 100,00", 10000, 150, 150},
		{"3,5% de R$ 1.200,00", 120000, 350, 4200},
		{"trunca para baixo", 1001, 250, 25}, // 25,025 -> 25
		{"taxa zero", 10000, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ApplyBps(c.gross, c.rate); got != c.want {
				t.Fatalf("got %d want %d", got, c.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name  string
		total Money
		n     int
		want  []Money
	}{
		{"divide exato", 115800, 12, []Money{9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650, 9650}},
		{"sobra vai para a primeira", 1000, 3, []Money{334, 333, 333}},
		{"uma parcela", 999, 1, []Money{999}},
		{"n inválido", 999, 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Split(c.total, c.n)
			if len(got) != len(c.want) {
				t.Fatalf("len = %d want %d", len(got), len(c.want))
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("parte %d = %d want %d", i, got[i], c.want[i])
				}
			}
			if c.n > 0 && Sum(got...) != c.total {
				t.Fatalf("soma %d != total %d", Sum(got...), c.total)
			}
		})
	}
}
```

#### `internal/domain/shared/product.go`

```go
package shared

import "errors"

// Product é o produto do arranjo: débito ou crédito.
// Fica em shared porque merchant (plano de taxas) e transaction (autorização) usam.
type Product string

const (
	Debit  Product = "DEBIT"
	Credit Product = "CREDIT"
)

var ErrInvalidProduct = errors.New("produto inválido: use DEBIT ou CREDIT")

func ParseProduct(s string) (Product, error) {
	switch Product(s) {
	case Debit, Credit:
		return Product(s), nil
	}
	return "", ErrInvalidProduct
}
```

#### `internal/domain/shared/clock.go`

Injetar o relógio (Parte 7.7) é o que permite testar "D+30 caiu num sábado".

```go
package shared

import "time"

// Clock abstrai "que horas são". O domínio nunca chama time.Now() direto:
// assim um teste consegue dizer "hoje é sábado, 31 de janeiro" e conferir a agenda.
type Clock interface {
	Now() time.Time
}

// RealClock é o relógio de produção.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

// FixedClock devolve sempre o mesmo instante. Só para testes.
type FixedClock struct{ T time.Time }

func (f FixedClock) Now() time.Time { return f.T }

// DateOnly zera horas/minutos/segundos, mantendo só a data (em UTC).
func DateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
```

#### `internal/domain/shared/calendar.go`

O calendário de dias úteis, com feriados móveis calculados a partir da Páscoa. É o tipo de regra que parece detalhe e é onde moram os bugs de agenda.

```go
package shared

import "time"

// BusinessCalendar responde "esse dia é útil?" e ajusta datas.
// Liquidação só acontece em dia útil: D+30 que cai num sábado vira segunda.
type BusinessCalendar interface {
	IsBusinessDay(t time.Time) bool
	NextBusinessDay(t time.Time) time.Time // t, se for útil; senão o próximo útil
	AddBusinessDays(t time.Time, n int) time.Time
}

// BrazilCalendar conhece fins de semana e os feriados nacionais brasileiros,
// inclusive os móveis (Carnaval, Sexta-feira Santa, Corpus Christi), calculados
// a partir da Páscoa.
type BrazilCalendar struct{}

func (BrazilCalendar) IsBusinessDay(t time.Time) bool {
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}
	return !isNationalHoliday(t)
}

func (c BrazilCalendar) NextBusinessDay(t time.Time) time.Time {
	d := DateOnly(t)
	for !c.IsBusinessDay(d) {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func (c BrazilCalendar) AddBusinessDays(t time.Time, n int) time.Time {
	d := DateOnly(t)
	for n > 0 {
		d = d.AddDate(0, 0, 1)
		if c.IsBusinessDay(d) {
			n--
		}
	}
	return d
}

func isNationalHoliday(t time.Time) bool {
	m, d := t.Month(), t.Day()
	switch {
	case m == time.January && d == 1, // Confraternização Universal
		m == time.April && d == 21,    // Tiradentes
		m == time.May && d == 1,       // Dia do Trabalho
		m == time.September && d == 7, // Independência
		m == time.October && d == 12,  // Nossa Senhora Aparecida
		m == time.November && d == 2,  // Finados
		m == time.November && d == 15, // Proclamação da República
		m == time.November && d == 20, // Consciência Negra (nacional desde 2024)
		m == time.December && d == 25: // Natal
		return true
	}
	easter := easterSunday(t.Year())
	day := DateOnly(t)
	// segunda e terça de Carnaval, Sexta-feira Santa, Corpus Christi
	for _, offset := range []int{-48, -47, -2, 60} {
		if day.Equal(easter.AddDate(0, 0, offset)) {
			return true
		}
	}
	return false
}

// easterSunday calcula o domingo de Páscoa pelo algoritmo de Meeus/Jones/Butcher.
func easterSunday(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := ((h + l - 7*m + 114) % 31) + 1
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
```

#### `internal/domain/shared/calendar_test.go`

```go
package shared

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func TestBrazilCalendar(t *testing.T) {
	cal := BrazilCalendar{}

	if cal.IsBusinessDay(date(2026, time.September, 12)) { // sábado
		t.Fatal("sábado não é dia útil")
	}
	if cal.IsBusinessDay(date(2026, time.September, 7)) { // Independência
		t.Fatal("7 de setembro não é dia útil")
	}
	if !cal.IsBusinessDay(date(2026, time.September, 14)) { // segunda
		t.Fatal("segunda comum é dia útil")
	}

	// Páscoa 2026 = 05/04; Carnaval (ter.) = 17/02; Sexta Santa = 03/04; Corpus Christi = 04/06
	if got := easterSunday(2026); !got.Equal(date(2026, time.April, 5)) {
		t.Fatalf("Páscoa 2026 = %v", got)
	}
	for _, h := range []time.Time{date(2026, time.February, 17), date(2026, time.April, 3), date(2026, time.June, 4)} {
		if cal.IsBusinessDay(h) {
			t.Fatalf("%v é feriado móvel", h)
		}
	}

	// quinta comum fica como está
	if got := cal.NextBusinessDay(date(2026, time.April, 9)); !got.Equal(date(2026, time.April, 9)) {
		t.Fatalf("NextBusinessDay(quinta) = %v", got)
	}
	// sábado 12/09 -> segunda 14/09
	if got := cal.NextBusinessDay(date(2026, time.September, 12)); !got.Equal(date(2026, time.September, 14)) {
		t.Fatalf("NextBusinessDay(sábado) = %v", got)
	}
	// D+1 útil de sexta 04/09/2026 pula sábado, domingo e o feriado de segunda (07/09) -> terça 08/09
	if got := cal.AddBusinessDays(date(2026, time.September, 4), 1); !got.Equal(date(2026, time.September, 8)) {
		t.Fatalf("AddBusinessDays(sexta antes do 7/9, 1) = %v", got)
	}
}
```

#### `internal/domain/shared/event.go`

Eventos de domínio (Parte 6.5). Todo evento embute `BaseEvent`; os campos são exportados porque viram JSON na outbox.

```go
package shared

import "time"

// Event é um fato que aconteceu no domínio ("transação capturada").
// Entidades acumulam eventos; o caso de uso grava-os na outbox junto com o dado.
type Event interface {
	EventID() string
	EventType() string
	OccurredAt() time.Time
	AggregateID() string
}

// BaseEvent é embutido em todo evento concreto para não repetir os campos.
// Os campos são exportados porque o evento vai virar JSON na outbox/Kafka.
type BaseEvent struct {
	ID        string    `json:"event_id"`
	Type      string    `json:"event_type"`
	At        time.Time `json:"occurred_at"`
	Aggregate string    `json:"aggregate_id"`
	Version   int       `json:"version"`
}

func NewBaseEvent(eventType, aggregateID string, at time.Time) BaseEvent {
	return BaseEvent{ID: NewID("evt"), Type: eventType, At: at, Aggregate: aggregateID, Version: 1}
}

func (b BaseEvent) EventID() string       { return b.ID }
func (b BaseEvent) EventType() string     { return b.Type }
func (b BaseEvent) OccurredAt() time.Time { return b.At }
func (b BaseEvent) AggregateID() string   { return b.Aggregate }
```

#### `internal/domain/shared/id.go`

```go
package shared

import (
	"strings"

	"github.com/google/uuid"
)

// NewID gera identificadores legíveis com prefixo: "tx_5f1c...", "m_9a2b...".
// O prefixo deixa óbvio, num log ou numa URL, de que tipo é o id.
func NewID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
```

#### `internal/domain/shared/errors.go`

```go
package shared

import "errors"

// Erros genéricos compartilhados por todos os contextos do domínio.
var (
	ErrNotFound          = errors.New("registro não encontrado")
	ErrInvalidAmount     = errors.New("o valor deve ser maior que zero")
	ErrInvalidTransition = errors.New("transição de estado inválida")
	ErrConcurrentUpdate  = errors.New("o registro foi alterado por outra operação; tente novamente")
)
```

**Compile e teste:**

```bash
go test ./internal/domain/shared/
```

### 1.2 Pacote `merchant`: o estabelecimento comercial

#### `internal/domain/merchant/document.go`

CNPJ como Value Object: se existe, é válido. O algoritmo já aceita o formato alfanumérico da Receita.

```go
package merchant

import (
	"errors"
	"strings"
)

// Document é um CNPJ validado. É um Value Object: se existe, é válido.
// Desde 2026 a Receita Federal emite CNPJs alfanuméricos (os 12 primeiros
// caracteres podem ter letras; os 2 dígitos verificadores continuam numéricos).
// O algoritmo abaixo cobre os dois formatos.
type Document string

var ErrInvalidDocument = errors.New("CNPJ inválido")

func NewDocument(raw string) (Document, error) {
	clean := strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(raw))
	if len(clean) != 14 {
		return "", ErrInvalidDocument
	}
	for i, ch := range clean {
		isDigit := ch >= '0' && ch <= '9'
		isLetter := ch >= 'A' && ch <= 'Z'
		if i >= 12 && !isDigit { // verificadores são sempre dígitos
			return "", ErrInvalidDocument
		}
		if !isDigit && !isLetter {
			return "", ErrInvalidDocument
		}
	}
	if strings.Count(clean, string(clean[0])) == 14 { // "00000000000000" passa no cálculo, mas não é um CNPJ real
		return "", ErrInvalidDocument
	}
	if checkDigit(clean[:12]) != int(clean[12]-'0') || checkDigit(clean[:13]) != int(clean[13]-'0') {
		return "", ErrInvalidDocument
	}
	return Document(clean), nil
}

// checkDigit aplica o módulo 11 da Receita: pesos 2..9 da direita para a esquerda.
// Cada caractere vale (código ASCII - 48): '0'..'9' → 0..9 e 'A'..'Z' → 17..42.
func checkDigit(base string) int {
	weight := 2
	sum := 0
	for i := len(base) - 1; i >= 0; i-- {
		sum += int(base[i]-'0') * weight
		weight++
		if weight > 9 {
			weight = 2
		}
	}
	rest := sum % 11
	if rest < 2 {
		return 0
	}
	return 11 - rest
}

func (d Document) String() string { return string(d) }

// Masked esconde a raiz do CNPJ, deixando só filial e verificadores: "**.***.***/0001-81".
// Use em logs e telas de suporte.
func (d Document) Masked() string {
	s := string(d)
	if len(s) != 14 {
		return "**************"
	}
	return "**.***.***/" + s[8:12] + "-" + s[12:]
}
```

#### `internal/domain/merchant/bank_account.go`

```go
package merchant

import "errors"

// BankAccount é o "domicílio bancário": a conta em que o EC recebe a liquidação.
type BankAccount struct {
	BankCode string // código COMPE de 3 dígitos, ex.: "341" (Itaú), "260" (Nubank)
	Branch   string // agência, sem dígito
	Number   string // conta com dígito, ex.: "12345-6"
	Kind     AccountKind
}

type AccountKind string

const (
	Checking AccountKind = "CHECKING" // conta corrente
	Savings  AccountKind = "SAVINGS"  // poupança
	Payment  AccountKind = "PAYMENT"  // conta de pagamento (fintechs)
)

var ErrInvalidBankAccount = errors.New("dados bancários inválidos")

func NewBankAccount(bankCode, branch, number string, kind AccountKind) (BankAccount, error) {
	if len(bankCode) != 3 || branch == "" || number == "" {
		return BankAccount{}, ErrInvalidBankAccount
	}
	switch kind {
	case Checking, Savings, Payment:
	default:
		return BankAccount{}, ErrInvalidBankAccount
	}
	return BankAccount{BankCode: bankCode, Branch: branch, Number: number, Kind: kind}, nil
}

func (b BankAccount) IsZero() bool { return b.BankCode == "" }
```

#### `internal/domain/merchant/fee_plan.go`

O plano de taxas da Parte 1.3: MDR por produto e parcelas, mais a taxa de antecipação.

```go
package merchant

import (
	"errors"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// FeePlan é o plano de taxas do EC: quanto ele paga (MDR) em cada produto e
// número de parcelas, e a taxa mensal cobrada quando antecipa recebíveis.
//
// Exemplo: débito 150 bps (1,50%), crédito à vista 250 bps (2,50%),
// crédito em 2 a 6 parcelas 350 bps, 7 a 12 parcelas 450 bps, antecipação 200 bps/mês.
type FeePlan struct {
	debit                 shared.Bps
	creditOneShot         shared.Bps
	creditInstallments    map[int]shared.Bps // parcelas (2..12) → MDR
	anticipationRateMonth shared.Bps
}

var (
	ErrInstallmentsNotAllowed = errors.New("número de parcelas não permitido pelo plano de taxas")
	ErrInvalidFeePlan         = errors.New("plano de taxas inválido")
)

func NewFeePlan(debit, creditOneShot shared.Bps, creditInstallments map[int]shared.Bps, anticipationRateMonth shared.Bps) (FeePlan, error) {
	if debit < 0 || creditOneShot < 0 || anticipationRateMonth < 0 || debit > 10000 || creditOneShot > 10000 {
		return FeePlan{}, ErrInvalidFeePlan
	}
	rates := make(map[int]shared.Bps, len(creditInstallments))
	for n, bps := range creditInstallments {
		if n < 2 || n > 12 || bps < 0 || bps > 10000 {
			return FeePlan{}, ErrInvalidFeePlan
		}
		rates[n] = bps
	}
	return FeePlan{debit: debit, creditOneShot: creditOneShot, creditInstallments: rates, anticipationRateMonth: anticipationRateMonth}, nil
}

// MDR devolve a taxa para um produto e número de parcelas.
func (p FeePlan) MDR(product shared.Product, installments int) (shared.Bps, error) {
	switch {
	case product == shared.Debit && installments == 1:
		return p.debit, nil
	case product == shared.Credit && installments == 1:
		return p.creditOneShot, nil
	case product == shared.Credit && installments > 1:
		bps, ok := p.creditInstallments[installments]
		if !ok {
			return 0, ErrInstallmentsNotAllowed
		}
		return bps, nil
	}
	return 0, ErrInstallmentsNotAllowed
}

func (p FeePlan) AnticipationRateMonth() shared.Bps { return p.anticipationRateMonth }
func (p FeePlan) Debit() shared.Bps                 { return p.debit }
func (p FeePlan) CreditOneShot() shared.Bps         { return p.creditOneShot }

// CreditInstallments devolve uma cópia (o mapa interno é imutável de fora).
func (p FeePlan) CreditInstallments() map[int]shared.Bps {
	out := make(map[int]shared.Bps, len(p.creditInstallments))
	for k, v := range p.creditInstallments {
		out[k] = v
	}
	return out
}
```

#### `internal/domain/merchant/merchant.go`

A entidade. Campos privados; só os métodos mudam o estado (`UNDER_REVIEW → ACTIVE → BLOCKED`, Parte 2.5). `SetWebhook` já faz a validação anti-SSRF da Parte 9.5.

```go
package merchant

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Status do credenciamento. Um EC nasce em análise (KYC) e só transaciona quando ativo.
type Status string

const (
	UnderReview Status = "UNDER_REVIEW"
	Active      Status = "ACTIVE"
	Blocked     Status = "BLOCKED"
)

var (
	ErrInvalidLegalName = errors.New("a razão social é obrigatória")
	ErrInvalidMCC       = errors.New("o MCC deve ter 4 dígitos")
	ErrMerchantInactive = errors.New("estabelecimento não está ativo")
	ErrInvalidWebhook   = errors.New("URL de webhook inválida: use https e um host público")
)

// Merchant é o estabelecimento comercial (EC). Campos privados: só os métodos mudam o estado.
type Merchant struct {
	id               string
	document         Document
	legalName        string
	mcc              string
	status           Status
	bankAccount      BankAccount
	feePlan          FeePlan
	webhookURL       string
	autoAnticipation bool
	version          int
	createdAt        time.Time
	updatedAt        time.Time
}

func New(document Document, legalName, mcc string, bank BankAccount, plan FeePlan, now time.Time) (*Merchant, error) {
	legalName = strings.TrimSpace(legalName)
	if legalName == "" {
		return nil, ErrInvalidLegalName
	}
	if len(mcc) != 4 {
		return nil, ErrInvalidMCC
	}
	if bank.IsZero() {
		return nil, ErrInvalidBankAccount
	}
	return &Merchant{
		id:          shared.NewID("m"),
		document:    document,
		legalName:   legalName,
		mcc:         mcc,
		status:      UnderReview,
		bankAccount: bank,
		feePlan:     plan,
		version:     1,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

// Restore reconstrói a entidade a partir do banco, sem validar de novo (já foi validada ao nascer).
func Restore(id string, document Document, legalName, mcc string, status Status, bank BankAccount, plan FeePlan,
	webhookURL string, autoAnticipation bool, version int, createdAt, updatedAt time.Time) *Merchant {
	return &Merchant{
		id: id, document: document, legalName: legalName, mcc: mcc, status: status, bankAccount: bank,
		feePlan: plan, webhookURL: webhookURL, autoAnticipation: autoAnticipation, version: version,
		createdAt: createdAt, updatedAt: updatedAt,
	}
}

// Approve conclui a análise de KYC. Só um analista com permissão chama isto.
func (m *Merchant) Approve(now time.Time) error {
	if m.status != UnderReview {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, m.status, Active)
	}
	m.status = Active
	m.touch(now)
	return nil
}

// Block impede novas transações (suspeita de fraude, inadimplência, pedido do EC).
func (m *Merchant) Block(now time.Time) error {
	if m.status != Active {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, m.status, Blocked)
	}
	m.status = Blocked
	m.touch(now)
	return nil
}

func (m *Merchant) SetFeePlan(plan FeePlan, now time.Time) {
	m.feePlan = plan
	m.touch(now)
}

// SetWebhook valida a URL contra SSRF: só https e sem apontar para rede interna.
func (m *Merchant) SetWebhook(raw string, now time.Time) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return ErrInvalidWebhook
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "127.") ||
		strings.HasPrefix(host, "169.254.") || strings.HasPrefix(host, "172.") {
		return ErrInvalidWebhook
	}
	m.webhookURL = raw
	m.touch(now)
	return nil
}

func (m *Merchant) EnableAutoAnticipation(on bool, now time.Time) {
	m.autoAnticipation = on
	m.touch(now)
}

func (m *Merchant) touch(now time.Time) {
	m.updatedAt = now
	m.version++
}

func (m *Merchant) IsActive() bool { return m.status == Active }

func (m *Merchant) ID() string               { return m.id }
func (m *Merchant) Document() Document       { return m.document }
func (m *Merchant) LegalName() string        { return m.legalName }
func (m *Merchant) MCC() string              { return m.mcc }
func (m *Merchant) Status() Status           { return m.status }
func (m *Merchant) BankAccount() BankAccount { return m.bankAccount }
func (m *Merchant) FeePlan() FeePlan         { return m.feePlan }
func (m *Merchant) WebhookURL() string       { return m.webhookURL }
func (m *Merchant) AutoAnticipation() bool   { return m.autoAnticipation }
func (m *Merchant) Version() int             { return m.version }
func (m *Merchant) CreatedAt() time.Time     { return m.createdAt }
func (m *Merchant) UpdatedAt() time.Time     { return m.updatedAt }
```

#### `internal/domain/merchant/repository.go`

O port. Quem implementa é a infraestrutura (Fase 4).

```go
package merchant

import "context"

// Repository é o port de persistência do EC. Quem implementa é a infraestrutura.
// Update deve aplicar lock otimista: falhar com shared.ErrConcurrentUpdate se a
// versão gravada não for a esperada.
type Repository interface {
	Create(ctx context.Context, m *Merchant) error
	FindByID(ctx context.Context, id string) (*Merchant, error)
	Update(ctx context.Context, m *Merchant) error
}
```

#### `internal/domain/merchant/merchant_test.go`

```go
package merchant

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

func TestNewDocument(t *testing.T) {
	cases := []struct {
		in    string
		valid bool
	}{
		{"11.222.333/0001-81", true},
		{"11222333000181", true},
		{"11.222.333/0001-80", false}, // dígito verificador errado
		{"00000000000000", false},     // todos iguais
		{"123", false},
		{"11.222.333/0001-8A", false}, // verificador não pode ser letra
	}
	for _, c := range cases {
		_, err := NewDocument(c.in)
		if (err == nil) != c.valid {
			t.Errorf("NewDocument(%q): err=%v, valid=%v", c.in, err, c.valid)
		}
	}
	d, _ := NewDocument("11.222.333/0001-81")
	if d.Masked() != "**.***.***/0001-81" {
		t.Errorf("Masked() = %q", d.Masked())
	}
}

func testPlan(t *testing.T) FeePlan {
	t.Helper()
	plan, err := NewFeePlan(150, 250, map[int]shared.Bps{2: 350, 3: 350, 6: 350, 12: 450}, 200)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestFeePlanMDR(t *testing.T) {
	plan := testPlan(t)
	cases := []struct {
		product      shared.Product
		installments int
		want         shared.Bps
		wantErr      bool
	}{
		{shared.Debit, 1, 150, false},
		{shared.Credit, 1, 250, false},
		{shared.Credit, 6, 350, false},
		{shared.Credit, 12, 450, false},
		{shared.Credit, 7, 0, true},  // não está no plano
		{shared.Debit, 2, 0, true},   // débito não parcela
		{shared.Credit, 13, 0, true}, // acima do máximo
	}
	for _, c := range cases {
		got, err := plan.MDR(c.product, c.installments)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("MDR(%s,%d) = %d, %v; want %d, err=%v", c.product, c.installments, got, err, c.want, c.wantErr)
		}
	}
}

func newTestMerchant(t *testing.T) *Merchant {
	t.Helper()
	doc, _ := NewDocument("11.222.333/0001-81")
	bank, _ := NewBankAccount("341", "0001", "12345-6", Checking)
	m, err := New(doc, "Padaria Pão Quente LTDA", "5462", bank, testPlan(t), time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMerchantLifecycle(t *testing.T) {
	m := newTestMerchant(t)
	now := time.Date(2026, 9, 2, 9, 0, 0, 0, time.UTC)

	if m.Status() != UnderReview || m.IsActive() {
		t.Fatal("EC deve nascer em análise")
	}
	if err := m.Block(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatalf("bloquear em análise deve falhar, err=%v", err)
	}
	if err := m.Approve(now); err != nil {
		t.Fatal(err)
	}
	if !m.IsActive() || m.Version() != 2 {
		t.Fatalf("status=%s version=%d", m.Status(), m.Version())
	}
	if err := m.Approve(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("aprovar duas vezes deve falhar")
	}
	if err := m.Block(now); err != nil || m.IsActive() {
		t.Fatal("bloqueio falhou")
	}
}

func TestMerchantWebhookAntiSSRF(t *testing.T) {
	m := newTestMerchant(t)
	now := time.Now()
	for _, bad := range []string{"http://loja.com/hook", "https://localhost/hook", "https://10.0.0.5/hook", "https://169.254.169.254/latest", "ftp://x"} {
		if err := m.SetWebhook(bad, now); !errors.Is(err, ErrInvalidWebhook) {
			t.Errorf("%q deveria ser rejeitada", bad)
		}
	}
	if err := m.SetWebhook("https://loja.com.br/webhooks/adquirente", now); err != nil {
		t.Fatal(err)
	}
}

func TestNewMerchantValidation(t *testing.T) {
	doc, _ := NewDocument("11.222.333/0001-81")
	bank, _ := NewBankAccount("341", "0001", "12345-6", Checking)
	plan := testPlan(t)
	if _, err := New(doc, "  ", "5462", bank, plan, time.Now()); !errors.Is(err, ErrInvalidLegalName) {
		t.Error("razão social vazia deveria falhar")
	}
	if _, err := New(doc, "X", "54", bank, plan, time.Now()); !errors.Is(err, ErrInvalidMCC) {
		t.Error("MCC curto deveria falhar")
	}
	if _, err := New(doc, "X", "5462", BankAccount{}, plan, time.Now()); !errors.Is(err, ErrInvalidBankAccount) {
		t.Error("conta vazia deveria falhar")
	}
	if _, err := NewBankAccount("34", "0001", "1", Checking); !errors.Is(err, ErrInvalidBankAccount) {
		t.Error("código de banco curto deveria falhar")
	}
}
```

### 1.3 Pacote `transaction`: a transação de cartão

#### `internal/domain/transaction/card.go`

O que guardamos do cartão (token, bandeira, BIN, últimos 4) e o que **nunca** guardamos. `MaskPAN` e o tipo `PAN` com `LogValue` garantem que o número completo não aparece em saída nenhuma (Parte 9.6).

```go
package transaction

import (
	"errors"
	"log/slog"
	"strings"
)

// CardInfo é o que a adquirente GUARDA sobre o cartão: nunca o número completo (PAN),
// nunca o CVV. O Token é o que o vault devolveu; BIN e Last4 servem para exibição,
// relatórios e roteamento. Isso mantém o resto do sistema fora do escopo pesado do PCI DSS.
type CardInfo struct {
	Token string
	Brand string
	BIN   string // 6 primeiros dígitos
	Last4 string
}

var (
	ErrInvalidPAN   = errors.New("número de cartão inválido")
	ErrInvalidToken = errors.New("token de cartão inválido")
)

// CardInfoFromPAN deriva os dados públicos do cartão a partir do PAN.
// Só o vault e o autorizador chamam isto; o PAN nunca é guardado.
func CardInfoFromPAN(token, pan string) (CardInfo, error) {
	if token == "" {
		return CardInfo{}, ErrInvalidToken
	}
	if !Luhn(pan) {
		return CardInfo{}, ErrInvalidPAN
	}
	return CardInfo{Token: token, Brand: brandOf(pan), BIN: pan[:6], Last4: pan[len(pan)-4:]}, nil
}

// Luhn verifica o dígito de controle do PAN (pega erro de digitação, não fraude).
func Luhn(pan string) bool {
	if len(pan) < 13 || len(pan) > 19 {
		return false
	}
	sum := 0
	double := false
	for i := len(pan) - 1; i >= 0; i-- {
		c := pan[i]
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func brandOf(pan string) string {
	switch {
	case strings.HasPrefix(pan, "4"):
		return "VISA"
	case pan[0] == '5' && pan[1] >= '1' && pan[1] <= '5', strings.HasPrefix(pan, "2"):
		return "MASTERCARD"
	case strings.HasPrefix(pan, "6362"), strings.HasPrefix(pan, "5067"), strings.HasPrefix(pan, "4576"):
		return "ELO"
	case strings.HasPrefix(pan, "34"), strings.HasPrefix(pan, "37"):
		return "AMEX"
	}
	return "UNKNOWN"
}

// MaskPAN exibe só BIN + últimos 4: "411111******1111". Regra do PCI DSS para qualquer saída.
func MaskPAN(pan string) string {
	if len(pan) < 10 {
		return strings.Repeat("*", len(pan))
	}
	return pan[:6] + strings.Repeat("*", len(pan)-10) + pan[len(pan)-4:]
}

// PAN é o número do cartão em memória. Implementa slog.LogValuer para que,
// se alguém logar a struct inteira por engano, saia mascarado.
type PAN string

func (p PAN) LogValue() slog.Value { return slog.StringValue(MaskPAN(string(p))) }
func (p PAN) String() string       { return MaskPAN(string(p)) }
```

#### `internal/domain/transaction/transaction.go`

A máquina de estados da Parte 7.4 e a Factory da Parte 7.2. Repare que `transition` é a única função que muda `status`.

```go
package transaction

import (
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Status é o estado da transação de cartão. Veja a tabela transitions abaixo:
// toda mudança passa por lá, então um estado inválido é impossível por construção.
type Status string

const (
	Pending      Status = "PENDING"
	Authorized   Status = "AUTHORIZED"
	Denied       Status = "DENIED"
	Captured     Status = "CAPTURED"
	Canceled     Status = "CANCELED"
	Settled      Status = "SETTLED"
	Chargebacked Status = "CHARGEBACKED"
)

var transitions = map[Status][]Status{
	Pending:    {Authorized, Denied},
	Authorized: {Captured, Canceled},
	Captured:   {Canceled, Settled, Chargebacked},
	Settled:    {Chargebacked},
}

var (
	ErrDebitInstallments = errors.New("débito não pode ser parcelado")
	ErrMaxInstallments   = errors.New("número de parcelas deve estar entre 1 e 12")
)

type Transaction struct {
	id                string
	merchantID        string
	amount            shared.Money
	product           shared.Product
	installments      int
	card              CardInfo
	status            Status
	authorizationCode string
	nsu               string
	responseCode      string
	createdAt         time.Time
	authorizedAt      *time.Time
	capturedAt        *time.Time
	canceledAt        *time.Time
	version           int
	events            []shared.Event
}

// New é a Factory: valida tudo o que a adquirente consegue validar ANTES de falar com o emissor.
func New(m *merchant.Merchant, amount shared.Money, product shared.Product, installments int, card CardInfo, now time.Time) (*Transaction, error) {
	if !m.IsActive() {
		return nil, merchant.ErrMerchantInactive
	}
	if !amount.IsPositive() {
		return nil, shared.ErrInvalidAmount
	}
	if installments < 1 || installments > 12 {
		return nil, ErrMaxInstallments
	}
	if product == shared.Debit && installments != 1 {
		return nil, ErrDebitInstallments
	}
	if _, err := m.FeePlan().MDR(product, installments); err != nil {
		return nil, err // ErrInstallmentsNotAllowed: o plano do EC não permite
	}
	if card.Token == "" || card.Last4 == "" {
		return nil, ErrInvalidToken
	}
	return &Transaction{
		id:           shared.NewID("tx"),
		merchantID:   m.ID(),
		amount:       amount,
		product:      product,
		installments: installments,
		card:         card,
		status:       Pending,
		createdAt:    now,
		version:      1,
	}, nil
}

func Restore(id, merchantID string, amount shared.Money, product shared.Product, installments int, card CardInfo,
	status Status, authorizationCode, nsu, responseCode string, createdAt time.Time,
	authorizedAt, capturedAt, canceledAt *time.Time, version int) *Transaction {
	return &Transaction{
		id: id, merchantID: merchantID, amount: amount, product: product, installments: installments, card: card,
		status: status, authorizationCode: authorizationCode, nsu: nsu, responseCode: responseCode,
		createdAt: createdAt, authorizedAt: authorizedAt, capturedAt: capturedAt, canceledAt: canceledAt, version: version,
	}
}

func (t *Transaction) transition(to Status) error {
	for _, allowed := range transitions[t.status] {
		if allowed == to {
			t.status = to
			t.version++
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, t.status, to)
}

// Authorize registra a aprovação do emissor.
func (t *Transaction) Authorize(authorizationCode, nsu string, now time.Time) error {
	if err := t.transition(Authorized); err != nil {
		return err
	}
	t.authorizationCode = authorizationCode
	t.nsu = nsu
	t.responseCode = "00"
	t.authorizedAt = &now
	t.events = append(t.events, TransactionAuthorized{
		BaseEvent:  shared.NewBaseEvent("transaction.authorized", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, Product: t.product, Installments: t.installments,
		AuthorizationCode: authorizationCode, NSU: nsu, CardBrand: t.card.Brand, CardLast4: t.card.Last4,
	})
	return nil
}

// Deny registra a negativa (código de resposta do emissor, ex.: "51" saldo insuficiente).
func (t *Transaction) Deny(responseCode string, now time.Time) error {
	if err := t.transition(Denied); err != nil {
		return err
	}
	t.responseCode = responseCode
	t.events = append(t.events, TransactionDenied{
		BaseEvent:  shared.NewBaseEvent("transaction.denied", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, ResponseCode: responseCode,
	})
	return nil
}

// Capture confirma a venda. É a captura que gera recebíveis (via evento).
func (t *Transaction) Capture(now time.Time) error {
	if err := t.transition(Captured); err != nil {
		return err
	}
	t.capturedAt = &now
	t.events = append(t.events, TransactionCaptured{
		BaseEvent:  shared.NewBaseEvent("transaction.captured", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, Product: t.product, Installments: t.installments,
		CardBrand: t.card.Brand, CapturedAt: now,
	})
	return nil
}

// Cancel desfaz uma transação autorizada ou capturada (antes da liquidação).
func (t *Transaction) Cancel(now time.Time) error {
	if err := t.transition(Canceled); err != nil {
		return err
	}
	t.canceledAt = &now
	t.events = append(t.events, TransactionCanceled{
		BaseEvent:  shared.NewBaseEvent("transaction.canceled", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount,
	})
	return nil
}

// MarkSettled é chamado quando o último recebível da transação é liquidado.
func (t *Transaction) MarkSettled() error { return t.transition(Settled) }

// Chargeback registra a contestação vinda do emissor.
func (t *Transaction) Chargeback(reasonCode string, now time.Time) error {
	if err := t.transition(Chargebacked); err != nil {
		return err
	}
	t.events = append(t.events, TransactionChargebacked{
		BaseEvent:  shared.NewBaseEvent("transaction.chargebacked", t.id, now),
		MerchantID: t.merchantID, Amount: t.amount, ReasonCode: reasonCode,
	})
	return nil
}

// PullEvents devolve os eventos acumulados e limpa a lista. O caso de uso chama
// depois de salvar, para gravar na outbox dentro da mesma transação de banco.
func (t *Transaction) PullEvents() []shared.Event {
	evs := t.events
	t.events = nil
	return evs
}

func (t *Transaction) ID() string                { return t.id }
func (t *Transaction) MerchantID() string        { return t.merchantID }
func (t *Transaction) Amount() shared.Money      { return t.amount }
func (t *Transaction) Product() shared.Product   { return t.product }
func (t *Transaction) Installments() int         { return t.installments }
func (t *Transaction) Card() CardInfo            { return t.card }
func (t *Transaction) Status() Status            { return t.status }
func (t *Transaction) AuthorizationCode() string { return t.authorizationCode }
func (t *Transaction) NSU() string               { return t.nsu }
func (t *Transaction) ResponseCode() string      { return t.responseCode }
func (t *Transaction) CreatedAt() time.Time      { return t.createdAt }
func (t *Transaction) AuthorizedAt() *time.Time  { return t.authorizedAt }
func (t *Transaction) CapturedAt() *time.Time    { return t.capturedAt }
func (t *Transaction) CanceledAt() *time.Time    { return t.canceledAt }
func (t *Transaction) Version() int              { return t.version }
```

#### `internal/domain/transaction/events.go`

```go
package transaction

import (
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Eventos de domínio da transação. Nomes no passado: são fatos.
// Carregam tudo o que um consumidor precisa, para não ter que consultar de volta.
// Nunca carregam PAN.

type TransactionAuthorized struct {
	shared.BaseEvent
	MerchantID        string         `json:"merchant_id"`
	Amount            shared.Money   `json:"amount"`
	Product           shared.Product `json:"product"`
	Installments      int            `json:"installments"`
	AuthorizationCode string         `json:"authorization_code"`
	NSU               string         `json:"nsu"`
	CardBrand         string         `json:"card_brand"`
	CardLast4         string         `json:"card_last4"`
}

type TransactionDenied struct {
	shared.BaseEvent
	MerchantID   string       `json:"merchant_id"`
	Amount       shared.Money `json:"amount"`
	ResponseCode string       `json:"response_code"`
}

type TransactionCaptured struct {
	shared.BaseEvent
	MerchantID   string         `json:"merchant_id"`
	Amount       shared.Money   `json:"amount"`
	Product      shared.Product `json:"product"`
	Installments int            `json:"installments"`
	CardBrand    string         `json:"card_brand"`
	CapturedAt   time.Time      `json:"captured_at"`
}

type TransactionCanceled struct {
	shared.BaseEvent
	MerchantID string       `json:"merchant_id"`
	Amount     shared.Money `json:"amount"`
}

type TransactionChargebacked struct {
	shared.BaseEvent
	MerchantID string       `json:"merchant_id"`
	Amount     shared.Money `json:"amount"`
	ReasonCode string       `json:"reason_code"`
}
```

#### `internal/domain/transaction/ports.go`

Os ports de saída: emissor, cofre, gerador de NSU, repositório. Note o contrato de Liskov (Parte 6.2): negativa não é erro.

```go
package transaction

import (
	"context"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Ports de saída: o domínio diz O QUE precisa; a infraestrutura diz COMO.

// AuthorizationRequest é o que mandamos ao emissor (via bandeira). Aqui, e só aqui,
// o PAN aparece em claro, em memória, pelo tempo da chamada.
type AuthorizationRequest struct {
	TransactionID string
	PAN           PAN
	Expiry        string // "MM/YY"
	Amount        shared.Money
	Product       shared.Product
	Installments  int
	MerchantMCC   string
}

// AuthorizationResponse: negativa NÃO é erro. Erro é falha técnica (timeout, rede).
type AuthorizationResponse struct {
	Approved          bool
	AuthorizationCode string
	ResponseCode      string // "00" aprovada; "51" saldo insuficiente; "05" não honrar; "91" emissor indisponível
}

type IssuerGateway interface {
	Authorize(ctx context.Context, req AuthorizationRequest) (AuthorizationResponse, error)
	Reverse(ctx context.Context, transactionID string) error
}

// CardData é o que o vault devolve ao detokenizar.
type CardData struct {
	PAN    PAN
	Expiry string
}

type CardVault interface {
	Detokenize(ctx context.Context, token string) (CardData, error)
}

// NSUGenerator gera o Número Sequencial Único de cada transação autorizada.
type NSUGenerator interface {
	Next(ctx context.Context) (string, error)
}

// Repository é o port de persistência. Os métodos recebem merchantID para que a
// consulta já filtre por dono: um EC nunca enxerga transação de outro, mesmo com bug no handler.
type Repository interface {
	Create(ctx context.Context, t *Transaction) error
	FindByID(ctx context.Context, merchantID, id string) (*Transaction, error)
	FindByIDForUpdate(ctx context.Context, merchantID, id string) (*Transaction, error)
	Update(ctx context.Context, t *Transaction) error
}
```

#### `internal/domain/transaction/transaction_test.go`

```go
package transaction

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

var now = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func activeMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{2: 350, 3: 350, 12: 450}, 200)
	m, err := merchant.New(doc, "Padaria", "5462", bank, plan, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Approve(now); err != nil {
		t.Fatal(err)
	}
	return m
}

func testCard(t *testing.T) CardInfo {
	t.Helper()
	c, err := CardInfoFromPAN("tok_abc", "4111111111111111")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCardInfoFromPAN(t *testing.T) {
	c := testCard(t)
	if c.Brand != "VISA" || c.BIN != "411111" || c.Last4 != "1111" {
		t.Fatalf("%+v", c)
	}
	if _, err := CardInfoFromPAN("tok", "4111111111111112"); !errors.Is(err, ErrInvalidPAN) {
		t.Error("Luhn inválido deveria falhar")
	}
	if MaskPAN("4111111111111111") != "411111******1111" {
		t.Error(MaskPAN("4111111111111111"))
	}
	if PAN("4111111111111111").String() != "411111******1111" {
		t.Error("PAN.String() deve mascarar")
	}
}

func TestNewValidation(t *testing.T) {
	m := activeMerchant(t)
	card := testCard(t)

	if _, err := New(m, 0, shared.Credit, 1, card, now); !errors.Is(err, shared.ErrInvalidAmount) {
		t.Error("valor zero")
	}
	if _, err := New(m, 1000, shared.Debit, 2, card, now); !errors.Is(err, ErrDebitInstallments) {
		t.Error("débito parcelado")
	}
	if _, err := New(m, 1000, shared.Credit, 7, card, now); !errors.Is(err, merchant.ErrInstallmentsNotAllowed) {
		t.Error("7x não está no plano")
	}
	if _, err := New(m, 1000, shared.Credit, 13, card, now); !errors.Is(err, ErrMaxInstallments) {
		t.Error("13x")
	}
	if err := m.Block(now); err != nil {
		t.Fatal(err)
	}
	if _, err := New(m, 1000, shared.Credit, 1, card, now); !errors.Is(err, merchant.ErrMerchantInactive) {
		t.Error("EC bloqueado")
	}
}

func TestStateMachine(t *testing.T) {
	m := activeMerchant(t)
	tx, err := New(m, 120000, shared.Credit, 12, testCard(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != Pending {
		t.Fatal("deve nascer PENDING")
	}
	// capturar antes de autorizar é impossível
	if err := tx.Capture(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar PENDING deveria falhar")
	}
	if err := tx.Authorize("123456", "000000123", now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Capture(now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Authorize("x", "y", now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("autorizar CAPTURED deveria falhar")
	}
	evs := tx.PullEvents()
	if len(evs) != 2 || evs[0].EventType() != "transaction.authorized" || evs[1].EventType() != "transaction.captured" {
		t.Fatalf("eventos: %+v", evs)
	}
	if len(tx.PullEvents()) != 0 {
		t.Fatal("PullEvents deve limpar")
	}
	if err := tx.MarkSettled(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Cancel(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("cancelar SETTLED deveria falhar")
	}
	if err := tx.Chargeback("4837", now); err != nil || tx.Status() != Chargebacked {
		t.Fatal("chargeback após liquidação deve ser permitido")
	}
}

func TestDenyAndCancel(t *testing.T) {
	m := activeMerchant(t)
	tx, _ := New(m, 5000, shared.Debit, 1, testCard(t), now)
	if err := tx.Deny("51", now); err != nil || tx.Status() != Denied || tx.ResponseCode() != "51" {
		t.Fatal("negativa")
	}
	if err := tx.Capture(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar negada")
	}

	tx2, _ := New(m, 5000, shared.Debit, 1, testCard(t), now)
	_ = tx2.Authorize("1", "2", now)
	if err := tx2.Cancel(now); err != nil || tx2.Status() != Canceled || tx2.CanceledAt() == nil {
		t.Fatal("cancelar autorizada")
	}
}
```

### 1.4 Pacote `receivable`: a agenda

#### `internal/domain/receivable/receivable.go`

Uma parcela da agenda e seus estados (Parte 1.6).

```go
package receivable

import (
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Status do recebível (uma parcela da agenda).
type Status string

const (
	Scheduled    Status = "SCHEDULED"    // na agenda, aguardando o vencimento
	Anticipated  Status = "ANTICIPATED"  // o EC já recebeu (com desconto); no vencimento o dinheiro fica com a adquirente
	Settled      Status = "SETTLED"      // pago
	Canceled     Status = "CANCELED"     // a transação foi cancelada antes de liquidar
	Chargebacked Status = "CHARGEBACKED" // contestado; não será pago (ou será estornado)
)

var transitions = map[Status][]Status{
	Scheduled:   {Anticipated, Settled, Canceled, Chargebacked},
	Anticipated: {Settled, Chargebacked},
}

// Receivable é uma parcela que o EC tem a receber. É a linha da agenda.
type Receivable struct {
	id             string
	transactionID  string
	merchantID     string
	installmentNo  int
	installments   int
	product        shared.Product
	brand          string
	gross          shared.Money // valor bruto da parcela
	fee            shared.Money // MDR da parcela
	net            shared.Money // o que o EC recebe (gross - fee)
	dueDate        time.Time    // data de liquidação (dia útil)
	status         Status
	settlementID   string
	anticipationID string
	version        int
	createdAt      time.Time
	updatedAt      time.Time
}

func newReceivable(transactionID, merchantID string, installmentNo, installments int, product shared.Product, brand string,
	gross, fee shared.Money, dueDate, now time.Time) *Receivable {
	return &Receivable{
		id: shared.NewID("rcv"), transactionID: transactionID, merchantID: merchantID,
		installmentNo: installmentNo, installments: installments, product: product, brand: brand,
		gross: gross, fee: fee, net: gross - fee, dueDate: shared.DateOnly(dueDate),
		status: Scheduled, version: 1, createdAt: now, updatedAt: now,
	}
}

func Restore(id, transactionID, merchantID string, installmentNo, installments int, product shared.Product, brand string,
	gross, fee, net shared.Money, dueDate time.Time, status Status, settlementID, anticipationID string,
	version int, createdAt, updatedAt time.Time) *Receivable {
	return &Receivable{
		id: id, transactionID: transactionID, merchantID: merchantID, installmentNo: installmentNo, installments: installments,
		product: product, brand: brand, gross: gross, fee: fee, net: net, dueDate: dueDate, status: status,
		settlementID: settlementID, anticipationID: anticipationID, version: version, createdAt: createdAt, updatedAt: updatedAt,
	}
}

func (r *Receivable) transition(to Status, now time.Time) error {
	for _, allowed := range transitions[r.status] {
		if allowed == to {
			r.status = to
			r.version++
			r.updatedAt = now
			return nil
		}
	}
	return fmt.Errorf("%w: recebível %s %s → %s", shared.ErrInvalidTransition, r.id, r.status, to)
}

// IsAnticipable: só o que está na agenda e ainda não venceu pode ser antecipado.
func (r *Receivable) IsAnticipable(today time.Time) bool {
	return r.status == Scheduled && r.dueDate.After(shared.DateOnly(today))
}

// DaysUntilDue conta os dias corridos até o vencimento (base do cálculo de antecipação).
func (r *Receivable) DaysUntilDue(today time.Time) int {
	return int(r.dueDate.Sub(shared.DateOnly(today)).Hours() / 24)
}

func (r *Receivable) Anticipate(anticipationID string, now time.Time) error {
	if err := r.transition(Anticipated, now); err != nil {
		return err
	}
	r.anticipationID = anticipationID
	return nil
}

func (r *Receivable) Settle(settlementID string, now time.Time) error {
	if err := r.transition(Settled, now); err != nil {
		return err
	}
	r.settlementID = settlementID
	return nil
}

func (r *Receivable) Cancel(now time.Time) error     { return r.transition(Canceled, now) }
func (r *Receivable) Chargeback(now time.Time) error { return r.transition(Chargebacked, now) }

func (r *Receivable) ID() string              { return r.id }
func (r *Receivable) TransactionID() string   { return r.transactionID }
func (r *Receivable) MerchantID() string      { return r.merchantID }
func (r *Receivable) InstallmentNo() int      { return r.installmentNo }
func (r *Receivable) Installments() int       { return r.installments }
func (r *Receivable) Product() shared.Product { return r.product }
func (r *Receivable) Brand() string           { return r.brand }
func (r *Receivable) Gross() shared.Money     { return r.gross }
func (r *Receivable) Fee() shared.Money       { return r.fee }
func (r *Receivable) Net() shared.Money       { return r.net }
func (r *Receivable) DueDate() time.Time      { return r.dueDate }
func (r *Receivable) Status() Status          { return r.status }
func (r *Receivable) SettlementID() string    { return r.settlementID }
func (r *Receivable) AnticipationID() string  { return r.anticipationID }
func (r *Receivable) Version() int            { return r.version }
func (r *Receivable) CreatedAt() time.Time    { return r.createdAt }
func (r *Receivable) UpdatedAt() time.Time    { return r.updatedAt }
```

#### `internal/domain/receivable/schedule.go`

O serviço de domínio que transforma captura em agenda (Parte 1.4) e agrupa em Unidades de Recebível para a registradora (Parte 1.6).

```go
package receivable

import (
	"errors"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

var ErrNotCaptured = errors.New("só transações capturadas geram recebíveis")

// Schedule é o Serviço de Domínio que transforma uma transação capturada na agenda.
//
// Regras (Parte 1.4 do guia):
//   - débito: 1 recebível em D+1 útil;
//   - crédito em n parcelas: n recebíveis em D+30·k dias corridos, cada um ajustado
//     para o próximo dia útil;
//   - MDR vem do plano de taxas do EC; fee = gross × MDR; net = gross − fee;
//   - parcelas via Split: os centavos que sobram vão para a primeira.
func Schedule(tx *transaction.Transaction, plan merchant.FeePlan, cal shared.BusinessCalendar, now time.Time) ([]*Receivable, error) {
	if tx.Status() != transaction.Captured || tx.CapturedAt() == nil {
		return nil, ErrNotCaptured
	}
	mdr, err := plan.MDR(tx.Product(), tx.Installments())
	if err != nil {
		return nil, err
	}
	n := tx.Installments()
	fee := shared.ApplyBps(tx.Amount(), mdr)
	grossParts := shared.Split(tx.Amount(), n)
	feeParts := shared.Split(fee, n)
	captured := shared.DateOnly(*tx.CapturedAt())

	out := make([]*Receivable, 0, n)
	for k := 1; k <= n; k++ {
		var due time.Time
		if tx.Product() == shared.Debit {
			due = cal.AddBusinessDays(captured, 1)
		} else {
			due = cal.NextBusinessDay(captured.AddDate(0, 0, 30*k))
		}
		out = append(out, newReceivable(tx.ID(), tx.MerchantID(), k, n, tx.Product(), tx.Card().Brand,
			grossParts[k-1], feeParts[k-1], due, now))
	}
	return out, nil
}

// Unit é a Unidade de Recebível (UR) do registro: EC + arranjo (bandeira/produto) + data.
// É o que se envia à registradora; várias parcelas de vendas diferentes somam numa UR.
type Unit struct {
	MerchantID string
	Brand      string
	Product    shared.Product
	DueDate    time.Time
	Total      shared.Money
	Count      int
}

// GroupUnits agrupa recebíveis em URs.
func GroupUnits(list []*Receivable) []Unit {
	type key struct {
		merchant string
		brand    string
		product  shared.Product
		due      time.Time
	}
	index := map[key]int{}
	var units []Unit
	for _, r := range list {
		k := key{r.merchantID, r.brand, r.product, r.dueDate}
		if i, ok := index[k]; ok {
			units[i].Total += r.net
			units[i].Count++
			continue
		}
		index[k] = len(units)
		units = append(units, Unit{MerchantID: r.merchantID, Brand: r.brand, Product: r.product, DueDate: r.dueDate, Total: r.net, Count: 1})
	}
	return units
}
```

#### `internal/domain/receivable/ports.go`

Repositório, registradora e o evento `receivable.scheduled`.

```go
package receivable

import (
	"context"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// ListFilter filtra a agenda de um EC.
type ListFilter struct {
	Status Status    // vazio = todos
	From   time.Time // vencimento >= From (zero = sem limite)
	To     time.Time // vencimento <= To (zero = sem limite)
	Limit  int
}

// DailySummary é a leitura resumida (CQRS leve): total por dia e status.
type DailySummary struct {
	DueDate time.Time
	Status  Status
	Count   int
	Net     shared.Money
}

type Repository interface {
	SaveAll(ctx context.Context, list []*Receivable) error
	ListByMerchant(ctx context.Context, merchantID string, f ListFilter) ([]*Receivable, error)
	ListByTransaction(ctx context.Context, transactionID string) ([]*Receivable, error)
	// ListDueOnForUpdate trava as linhas que vencem no dia (FOR UPDATE SKIP LOCKED):
	// vários workers podem rodar em paralelo sem pegar o mesmo recebível.
	ListDueOnForUpdate(ctx context.Context, day time.Time, limit int) ([]*Receivable, error)
	// FindByIDs lê recebíveis específicos de um EC sem travar (simulação de antecipação).
	FindByIDs(ctx context.Context, merchantID string, ids []string) ([]*Receivable, error)
	// FindForUpdate trava recebíveis específicos de um EC (antecipação).
	FindForUpdate(ctx context.Context, merchantID string, ids []string) ([]*Receivable, error)
	UpdateAll(ctx context.Context, list []*Receivable) error
	Summary(ctx context.Context, merchantID string, from, to time.Time) ([]DailySummary, error)
}

// Lien é um ônus registrado sobre a agenda: parte do que venceria para o EC
// deve ir para um credor (um banco que deu empréstimo com a agenda em garantia).
type Lien struct {
	MerchantID   string
	DueDate      time.Time
	Amount       shared.Money
	CreditorName string
	Creditor     merchant.BankAccount
}

// Registry é o port da registradora de recebíveis (CERC, TAG, B3, Núclea).
type Registry interface {
	Register(ctx context.Context, units []Unit) error
	CheckLiens(ctx context.Context, merchantID string, dueDate time.Time) ([]Lien, error)
	// TransferOwnership informa que a adquirente passou a ser dona (antecipação).
	TransferOwnership(ctx context.Context, receivableIDs []string, newOwner string) error
}

// Eventos publicados pelos casos de uso que mexem na agenda.

type ReceivablesScheduled struct {
	shared.BaseEvent
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Count         int          `json:"count"`
	GrossTotal    shared.Money `json:"gross_total"`
	NetTotal      shared.Money `json:"net_total"`
	FirstDueDate  time.Time    `json:"first_due_date"`
	LastDueDate   time.Time    `json:"last_due_date"`
}

func NewReceivablesScheduled(list []*Receivable, now time.Time) ReceivablesScheduled {
	ev := ReceivablesScheduled{BaseEvent: shared.NewBaseEvent("receivable.scheduled", list[0].TransactionID(), now),
		TransactionID: list[0].TransactionID(), MerchantID: list[0].MerchantID(), Count: len(list)}
	ev.FirstDueDate, ev.LastDueDate = list[0].DueDate(), list[0].DueDate()
	for _, r := range list {
		ev.GrossTotal += r.Gross()
		ev.NetTotal += r.Net()
		if r.DueDate().Before(ev.FirstDueDate) {
			ev.FirstDueDate = r.DueDate()
		}
		if r.DueDate().After(ev.LastDueDate) {
			ev.LastDueDate = r.DueDate()
		}
	}
	return ev
}
```

#### `internal/domain/receivable/schedule_test.go`

Este teste **reproduz o exemplo da Parte 1.4**: R$ 1.200,00 em 12x, MDR 3,5%, capturada em 10/03/2026. Se a agenda do seu código bater com a tabela do guia, você entendeu a regra.

```go
package receivable

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func fixtures(t *testing.T) (*merchant.Merchant, transaction.CardInfo) {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{3: 350, 12: 350}, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, date(2026, 1, 1))
	_ = m.Approve(date(2026, 1, 1))
	card, _ := transaction.CardInfoFromPAN("tok_1", "4111111111111111")
	return m, card
}

func captured(t *testing.T, m *merchant.Merchant, card transaction.CardInfo, amount shared.Money, product shared.Product, n int, at time.Time) *transaction.Transaction {
	t.Helper()
	tx, err := transaction.New(m, amount, product, n, card, at)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Authorize("123456", "1", at)
	_ = tx.Capture(at)
	return tx
}

// Reproduz o exemplo da Parte 1.4: R$ 1.200,00 em 12x, MDR 3,5%, capturada em 10/03/2026.
func TestScheduleCredit12x(t *testing.T) {
	m, card := fixtures(t)
	cal := shared.BrazilCalendar{}
	tx := captured(t, m, card, 120000, shared.Credit, 12, date(2026, 3, 10))

	list, err := Schedule(tx, m.FeePlan(), cal, date(2026, 3, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 12 {
		t.Fatalf("len = %d", len(list))
	}
	var gross, fee, net shared.Money
	for i, r := range list {
		gross += r.Gross()
		fee += r.Fee()
		net += r.Net()
		if r.InstallmentNo() != i+1 || r.Status() != Scheduled || r.Net() != 9650 {
			t.Errorf("parcela %d: %+v", i+1, r)
		}
	}
	if gross != 120000 || fee != 4200 || net != 115800 {
		t.Fatalf("gross=%d fee=%d net=%d", gross, fee, net)
	}
	// parcela 1: 10/03 + 30 = 09/04 (quinta) → fica
	if !list[0].DueDate().Equal(date(2026, 4, 9)) {
		t.Errorf("parcela 1 vence %v", list[0].DueDate())
	}
	// parcela 2: 10/03 + 60 = 09/05 (sábado) → segunda 11/05
	if !list[1].DueDate().Equal(date(2026, 5, 11)) {
		t.Errorf("parcela 2 vence %v", list[1].DueDate())
	}
	// parcela 12: 10/03 + 360 = 05/03/2027 (sexta) → fica
	if !list[11].DueDate().Equal(date(2027, 3, 5)) {
		t.Errorf("parcela 12 vence %v", list[11].DueDate())
	}
}

func TestScheduleDebitAndRounding(t *testing.T) {
	m, card := fixtures(t)
	cal := shared.BrazilCalendar{}

	// débito sexta 04/09/2026 → D+1 útil pula fim de semana e 07/09 → 08/09
	tx := captured(t, m, card, 10000, shared.Debit, 1, date(2026, 9, 4))
	list, err := Schedule(tx, m.FeePlan(), cal, date(2026, 9, 4))
	if err != nil || len(list) != 1 {
		t.Fatal(err)
	}
	if !list[0].DueDate().Equal(date(2026, 9, 8)) || list[0].Fee() != 150 || list[0].Net() != 9850 {
		t.Fatalf("%+v", list[0])
	}

	// R$ 10,00 em 3x: bruto 334/333/333; fee 2,5%*... MDR 3x = 350 bps → fee 35 → 13/11/11; net 321/322/322
	tx3 := captured(t, m, card, 1000, shared.Credit, 3, date(2026, 9, 4))
	list3, _ := Schedule(tx3, m.FeePlan(), cal, date(2026, 9, 4))
	wantGross := []shared.Money{334, 333, 333}
	wantFee := []shared.Money{13, 11, 11}
	var net shared.Money
	for i, r := range list3 {
		if r.Gross() != wantGross[i] || r.Fee() != wantFee[i] {
			t.Errorf("parcela %d gross=%d fee=%d", i+1, r.Gross(), r.Fee())
		}
		net += r.Net()
	}
	if net != 965 {
		t.Fatalf("net total = %d", net)
	}
}

func TestScheduleRequiresCapture(t *testing.T) {
	m, card := fixtures(t)
	tx, _ := transaction.New(m, 1000, shared.Credit, 1, card, date(2026, 9, 4))
	_ = tx.Authorize("1", "2", date(2026, 9, 4))
	if _, err := Schedule(tx, m.FeePlan(), shared.BrazilCalendar{}, date(2026, 9, 4)); !errors.Is(err, ErrNotCaptured) {
		t.Fatal("autorizada sem captura não gera agenda")
	}
}

func TestReceivableTransitionsAndUnits(t *testing.T) {
	m, card := fixtures(t)
	tx := captured(t, m, card, 30000, shared.Credit, 3, date(2026, 9, 4))
	list, _ := Schedule(tx, m.FeePlan(), shared.BrazilCalendar{}, date(2026, 9, 4))
	now := date(2026, 9, 5)

	r := list[0]
	if !r.IsAnticipable(now) || r.DaysUntilDue(now) <= 0 {
		t.Fatal("parcela futura deve ser antecipável")
	}
	if err := r.Anticipate("ant_1", now); err != nil || r.Status() != Anticipated || r.IsAnticipable(now) {
		t.Fatal("antecipar")
	}
	if err := r.Cancel(now); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("cancelar antecipado deve falhar")
	}
	if err := r.Settle("stl_1", now); err != nil || r.SettlementID() != "stl_1" {
		t.Fatal("liquidar antecipado (dinheiro fica com a adquirente)")
	}
	if err := list[1].Chargeback(now); err != nil {
		t.Fatal(err)
	}

	units := GroupUnits(list)
	if len(units) != 3 { // 3 datas diferentes, mesma bandeira/produto
		t.Fatalf("units = %d", len(units))
	}
	ev := NewReceivablesScheduled(list, now)
	if ev.Count != 3 || ev.GrossTotal != 30000 || ev.EventType() != "receivable.scheduled" {
		t.Fatalf("%+v", ev)
	}
}
```

### 1.5 Pacote `anticipation`: a antecipação

#### `internal/domain/anticipation/pricing.go`

A Strategy de precificação (Parte 7.4) com a fórmula da Parte 1.5.

```go
package anticipation

import (
	"math"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Pricer é a Strategy de precificação: quanto vale HOJE um valor que vence em N dias.
type Pricer interface {
	PresentValue(net shared.Money, days int, rateMonth shared.Bps) shared.Money
}

// CompoundPricer usa juros compostos pro-rata die:
//
//	valor_presente = valor_futuro / (1 + taxa_mensal) ^ (dias / 30)
//
// Este é o ÚNICO lugar do domínio em que float aparece, porque a potência é fracionária.
// O resultado é arredondado para o centavo imediatamente e volta a ser Money.
type CompoundPricer struct{}

func (CompoundPricer) PresentValue(net shared.Money, days int, rateMonth shared.Bps) shared.Money {
	if days <= 0 || rateMonth == 0 {
		return net
	}
	rate := float64(rateMonth) / 10000
	factor := math.Pow(1+rate, float64(days)/30)
	return shared.Money(math.Round(float64(net) / factor))
}
```

#### `internal/domain/anticipation/anticipation.go`

Simular não muda nada; confirmar muda os recebíveis e gera evento.

```go
package anticipation

import (
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type Status string

const (
	Simulated Status = "SIMULATED" // só cálculo; nada mudou
	Confirmed Status = "CONFIRMED" // recebíveis marcados, dinheiro devido ao EC
)

var (
	ErrNoReceivables  = errors.New("nenhum recebível informado")
	ErrNotAnticipable = errors.New("recebível não está elegível para antecipação")
	ErrWrongMerchant  = errors.New("recebível não pertence ao estabelecimento")
	ErrAlreadyDone    = errors.New("antecipação já confirmada")
)

// Item é o cálculo de um recebível dentro da antecipação.
type Item struct {
	ReceivableID string       `json:"receivable_id"`
	DueDate      time.Time    `json:"due_date"`
	Days         int          `json:"days"`
	Net          shared.Money `json:"net"`           // o que venceria
	PresentValue shared.Money `json:"present_value"` // o que o EC recebe hoje
	Discount     shared.Money `json:"discount"`      // Net - PresentValue
}

// Anticipation é o pedido de antecipação: um conjunto de recebíveis e o preço.
type Anticipation struct {
	id          string
	merchantID  string
	items       []Item
	gross       shared.Money // soma dos Net dos recebíveis
	discount    shared.Money // receita da adquirente
	net         shared.Money // o que o EC recebe
	rateMonth   shared.Bps
	requestedAt time.Time
	status      Status
	version     int
	events      []shared.Event
}

// Simulate precifica sem alterar nada. Todas as regras de elegibilidade (Specification) estão aqui.
func Simulate(merchantID string, receivables []*receivable.Receivable, today time.Time, rateMonth shared.Bps, pricer Pricer) (*Anticipation, error) {
	if len(receivables) == 0 {
		return nil, ErrNoReceivables
	}
	a := &Anticipation{id: shared.NewID("ant"), merchantID: merchantID, rateMonth: rateMonth, requestedAt: today, status: Simulated, version: 1}
	for _, r := range receivables {
		if r.MerchantID() != merchantID {
			return nil, fmt.Errorf("%w: %s", ErrWrongMerchant, r.ID())
		}
		if !r.IsAnticipable(today) {
			return nil, fmt.Errorf("%w: %s (%s, vence %s)", ErrNotAnticipable, r.ID(), r.Status(), r.DueDate().Format("2006-01-02"))
		}
		days := r.DaysUntilDue(today)
		pv := pricer.PresentValue(r.Net(), days, rateMonth)
		a.items = append(a.items, Item{ReceivableID: r.ID(), DueDate: r.DueDate(), Days: days, Net: r.Net(), PresentValue: pv, Discount: r.Net() - pv})
		a.gross += r.Net()
		a.net += pv
	}
	a.discount = a.gross - a.net
	return a, nil
}

// Confirm efetiva: marca cada recebível como antecipado e gera o evento.
// O caso de uso chama isto dentro da Unit of Work, com os recebíveis travados (FOR UPDATE).
func (a *Anticipation) Confirm(receivables []*receivable.Receivable, now time.Time) error {
	if a.status == Confirmed {
		return ErrAlreadyDone
	}
	if len(receivables) != len(a.items) {
		return ErrNotAnticipable
	}
	ids := make([]string, 0, len(receivables))
	for _, r := range receivables {
		if err := r.Anticipate(a.id, now); err != nil {
			return err
		}
		ids = append(ids, r.ID())
	}
	a.status = Confirmed
	a.version++
	a.events = append(a.events, AnticipationConfirmed{
		BaseEvent:      shared.NewBaseEvent("receivable.anticipated", a.id, now),
		AnticipationID: a.id, MerchantID: a.merchantID, Gross: a.gross, Discount: a.discount, Net: a.net, ReceivableIDs: ids,
	})
	return nil
}

func Restore(id, merchantID string, items []Item, gross, discount, net shared.Money, rateMonth shared.Bps, requestedAt time.Time, status Status, version int) *Anticipation {
	return &Anticipation{id: id, merchantID: merchantID, items: items, gross: gross, discount: discount, net: net, rateMonth: rateMonth, requestedAt: requestedAt, status: status, version: version}
}

func (a *Anticipation) PullEvents() []shared.Event { evs := a.events; a.events = nil; return evs }

func (a *Anticipation) ID() string             { return a.id }
func (a *Anticipation) MerchantID() string     { return a.merchantID }
func (a *Anticipation) Items() []Item          { return a.items }
func (a *Anticipation) Gross() shared.Money    { return a.gross }
func (a *Anticipation) Discount() shared.Money { return a.discount }
func (a *Anticipation) Net() shared.Money      { return a.net }
func (a *Anticipation) RateMonth() shared.Bps  { return a.rateMonth }
func (a *Anticipation) RequestedAt() time.Time { return a.requestedAt }
func (a *Anticipation) Status() Status         { return a.status }
func (a *Anticipation) Version() int           { return a.version }

func (a *Anticipation) ReceivableIDs() []string {
	ids := make([]string, len(a.items))
	for i, it := range a.items {
		ids[i] = it.ReceivableID
	}
	return ids
}

type AnticipationConfirmed struct {
	shared.BaseEvent
	AnticipationID string       `json:"anticipation_id"`
	MerchantID     string       `json:"merchant_id"`
	Gross          shared.Money `json:"gross"`
	Discount       shared.Money `json:"discount"`
	Net            shared.Money `json:"net"`
	ReceivableIDs  []string     `json:"receivable_ids"`
}
```

#### `internal/domain/anticipation/ports.go`

```go
package anticipation

import "context"

type Repository interface {
	Create(ctx context.Context, a *Anticipation) error
	FindByID(ctx context.Context, merchantID, id string) (*Anticipation, error)
	ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*Anticipation, error)
}
```

#### `internal/domain/anticipation/pricing_test.go`

Reproduz a tabela da Parte 1.5 centavo a centavo.

```go
package anticipation

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// Reproduz a tabela da Parte 1.5: R$ 96,50 a 2% ao mês.
func TestCompoundPricer(t *testing.T) {
	p := CompoundPricer{}
	cases := []struct {
		days int
		want shared.Money
	}{
		{30, 9461}, {60, 9275}, {90, 9093}, {120, 8915}, {150, 8740}, {180, 8569},
		{210, 8401}, {240, 8236}, {270, 8075}, {300, 7916}, {330, 7761}, {360, 7609},
	}
	var total shared.Money
	for _, c := range cases {
		got := p.PresentValue(9650, c.days, 200)
		if got != c.want {
			t.Errorf("PV(96,50, %d dias) = %d want %d", c.days, got, c.want)
		}
		total += got
	}
	if total != 102051 { // R$ 1.020,51 hoje por R$ 1.158,00 ao longo do ano
		t.Errorf("total = %d", total)
	}
	if p.PresentValue(1000, 0, 200) != 1000 || p.PresentValue(1000, 30, 0) != 1000 {
		t.Error("sem prazo ou sem taxa, não há desconto")
	}
}

func scheduledReceivable(merchantID string, net shared.Money, due time.Time) *receivable.Receivable {
	return receivable.Restore(shared.NewID("rcv"), "tx_1", merchantID, 1, 1, shared.Credit, "VISA",
		net, 0, net, due, receivable.Scheduled, "", "", 1, due, due)
}

func TestSimulateAndConfirm(t *testing.T) {
	today := date(2026, 9, 14)
	rs := []*receivable.Receivable{
		scheduledReceivable("m_1", 9650, date(2026, 10, 14)), // 30 dias
		scheduledReceivable("m_1", 9650, date(2026, 11, 13)), // 60 dias
	}
	a, err := Simulate("m_1", rs, today, 200, CompoundPricer{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Gross() != 19300 || a.Net() != 9461+9275 || a.Discount() != 19300-(9461+9275) || a.Status() != Simulated {
		t.Fatalf("gross=%d net=%d discount=%d", a.Gross(), a.Net(), a.Discount())
	}
	if err := a.Confirm(rs, today); err != nil {
		t.Fatal(err)
	}
	if a.Status() != Confirmed || rs[0].Status() != receivable.Anticipated || rs[0].AnticipationID() != a.ID() {
		t.Fatal("confirmar deve marcar os recebíveis")
	}
	evs := a.PullEvents()
	if len(evs) != 1 || evs[0].EventType() != "receivable.anticipated" {
		t.Fatalf("%+v", evs)
	}
	if err := a.Confirm(rs, today); !errors.Is(err, ErrAlreadyDone) {
		t.Fatal("confirmar duas vezes")
	}
}

func TestSimulateRejects(t *testing.T) {
	today := date(2026, 9, 14)
	if _, err := Simulate("m_1", nil, today, 200, CompoundPricer{}); !errors.Is(err, ErrNoReceivables) {
		t.Error("lista vazia")
	}
	other := scheduledReceivable("m_2", 100, date(2026, 10, 1))
	if _, err := Simulate("m_1", []*receivable.Receivable{other}, today, 200, CompoundPricer{}); !errors.Is(err, ErrWrongMerchant) {
		t.Error("recebível de outro EC")
	}
	past := scheduledReceivable("m_1", 100, date(2026, 9, 14)) // vence hoje: não antecipa
	if _, err := Simulate("m_1", []*receivable.Receivable{past}, today, 200, CompoundPricer{}); !errors.Is(err, ErrNotAnticipable) {
		t.Error("vence hoje")
	}
	done := scheduledReceivable("m_1", 100, date(2026, 10, 1))
	_ = done.Anticipate("ant_x", today)
	if _, err := Simulate("m_1", []*receivable.Receivable{done}, today, 200, CompoundPricer{}); !errors.Is(err, ErrNotAnticipable) {
		t.Error("já antecipado")
	}
}
```

### 1.6 Pacote `settlement`: a liquidação

#### `internal/domain/settlement/settlement.go`

`Build` monta o lote de um EC num dia: separa antecipados, aplica ônus (Parte 2.4) e gera as ordens de pagamento.

```go
package settlement

import (
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type Status string

const (
	Pending   Status = "PENDING"   // montada, ainda não enviada ao banco
	Sent      Status = "SENT"      // ordens enviadas; aguardando retorno
	Confirmed Status = "CONFIRMED" // banco confirmou o crédito
	Failed    Status = "FAILED"    // banco rejeitou (conta inválida etc.)
)

// OrderKind diz para quem vai cada ordem de pagamento.
type OrderKind string

const (
	ToMerchant OrderKind = "TO_MERCHANT" // o normal
	ToCreditor OrderKind = "TO_CREDITOR" // havia ônus registrado: paga o credor
)

// Order é uma ordem de pagamento dentro do lote (vira uma linha do arquivo de liquidação).
type Order struct {
	ID          string               `json:"id"`
	Kind        OrderKind            `json:"kind"`
	Beneficiary string               `json:"beneficiary"`
	BankAccount merchant.BankAccount `json:"bank_account"`
	Amount      shared.Money         `json:"amount"`
}

var (
	ErrNothingToSettle = errors.New("nenhum recebível para liquidar")
	ErrWrongDay        = errors.New("recebível não vence na data da liquidação")
	ErrWrongMerchant   = errors.New("recebível de outro estabelecimento no lote")
	ErrWrongStatus     = errors.New("recebível em estado que não permite liquidação")
)

// Settlement é o lote de liquidação de UM estabelecimento em UMA data.
type Settlement struct {
	id            string
	date          time.Time
	merchantID    string
	total         shared.Money // soma dos recebíveis do lote (líquido)
	toMerchant    shared.Money // o que vai para a conta do EC
	toCreditors   shared.Money // o que vai para credores (ônus)
	retained      shared.Money // recebíveis antecipados: já pagos; o dinheiro fica com a adquirente
	orders        []Order
	receivableIDs []string
	status        Status
	externalRef   string
	failureReason string
	createdAt     time.Time
	updatedAt     time.Time
	version       int
	events        []shared.Event
}

// Build monta o lote: valida, separa antecipados, aplica ônus e gera as ordens.
// É um Template Method em forma de função: o esqueleto é fixo; "pagar" fica no port PaymentGateway.
func Build(date time.Time, m *merchant.Merchant, receivables []*receivable.Receivable, liens []receivable.Lien, now time.Time) (*Settlement, error) {
	if len(receivables) == 0 {
		return nil, ErrNothingToSettle
	}
	day := shared.DateOnly(date)
	s := &Settlement{id: shared.NewID("stl"), date: day, merchantID: m.ID(), status: Pending, createdAt: now, updatedAt: now, version: 1}

	var payable shared.Money
	for _, r := range receivables {
		switch {
		case r.MerchantID() != m.ID():
			return nil, fmt.Errorf("%w: %s", ErrWrongMerchant, r.ID())
		case !r.DueDate().Equal(day):
			return nil, fmt.Errorf("%w: %s vence %s", ErrWrongDay, r.ID(), r.DueDate().Format("2006-01-02"))
		}
		switch r.Status() {
		case receivable.Scheduled:
			payable += r.Net()
		case receivable.Anticipated:
			s.retained += r.Net()
		default:
			return nil, fmt.Errorf("%w: %s está %s", ErrWrongStatus, r.ID(), r.Status())
		}
		s.total += r.Net()
		s.receivableIDs = append(s.receivableIDs, r.ID())
	}

	// Ônus primeiro: o credor tem prioridade sobre o EC, até o limite do que há para pagar.
	for _, l := range liens {
		if payable == 0 {
			break
		}
		amount := l.Amount
		if amount > payable {
			amount = payable
		}
		s.orders = append(s.orders, Order{ID: shared.NewID("ord"), Kind: ToCreditor, Beneficiary: l.CreditorName, BankAccount: l.Creditor, Amount: amount})
		s.toCreditors += amount
		payable -= amount
	}
	if payable > 0 {
		s.orders = append(s.orders, Order{ID: shared.NewID("ord"), Kind: ToMerchant, Beneficiary: m.LegalName(), BankAccount: m.BankAccount(), Amount: payable})
		s.toMerchant = payable
	}
	return s, nil
}

func (s *Settlement) MarkSent(externalRef string, now time.Time) error {
	if s.status != Pending {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Sent)
	}
	s.status, s.externalRef, s.updatedAt = Sent, externalRef, now
	s.version++
	return nil
}

func (s *Settlement) MarkConfirmed(now time.Time) error {
	if s.status != Sent {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Confirmed)
	}
	s.status, s.updatedAt = Confirmed, now
	s.version++
	s.events = append(s.events, SettlementCompleted{
		BaseEvent:    shared.NewBaseEvent("settlement.completed", s.id, now),
		SettlementID: s.id, MerchantID: s.merchantID, Date: s.date, Total: s.total, ToMerchant: s.toMerchant,
		ToCreditors: s.toCreditors, Retained: s.retained, ReceivableIDs: s.receivableIDs,
	})
	return nil
}

func (s *Settlement) MarkFailed(reason string, now time.Time) error {
	if s.status != Sent && s.status != Pending {
		return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, s.status, Failed)
	}
	s.status, s.failureReason, s.updatedAt = Failed, reason, now
	s.version++
	s.events = append(s.events, SettlementFailed{
		BaseEvent:    shared.NewBaseEvent("settlement.failed", s.id, now),
		SettlementID: s.id, MerchantID: s.merchantID, Date: s.date, Reason: reason, ReceivableIDs: s.receivableIDs,
	})
	return nil
}

func Restore(id string, date time.Time, merchantID string, total, toMerchant, toCreditors, retained shared.Money, orders []Order,
	receivableIDs []string, status Status, externalRef, failureReason string, createdAt, updatedAt time.Time, version int) *Settlement {
	return &Settlement{id: id, date: date, merchantID: merchantID, total: total, toMerchant: toMerchant, toCreditors: toCreditors,
		retained: retained, orders: orders, receivableIDs: receivableIDs, status: status, externalRef: externalRef,
		failureReason: failureReason, createdAt: createdAt, updatedAt: updatedAt, version: version}
}

func (s *Settlement) PullEvents() []shared.Event { evs := s.events; s.events = nil; return evs }

func (s *Settlement) ID() string                { return s.id }
func (s *Settlement) Date() time.Time           { return s.date }
func (s *Settlement) MerchantID() string        { return s.merchantID }
func (s *Settlement) Total() shared.Money       { return s.total }
func (s *Settlement) ToMerchant() shared.Money  { return s.toMerchant }
func (s *Settlement) ToCreditors() shared.Money { return s.toCreditors }
func (s *Settlement) Retained() shared.Money    { return s.retained }
func (s *Settlement) Orders() []Order           { return s.orders }
func (s *Settlement) ReceivableIDs() []string   { return s.receivableIDs }
func (s *Settlement) Status() Status            { return s.status }
func (s *Settlement) ExternalRef() string       { return s.externalRef }
func (s *Settlement) FailureReason() string     { return s.failureReason }
func (s *Settlement) CreatedAt() time.Time      { return s.createdAt }
func (s *Settlement) UpdatedAt() time.Time      { return s.updatedAt }
func (s *Settlement) Version() int              { return s.version }

type SettlementCompleted struct {
	shared.BaseEvent
	SettlementID  string       `json:"settlement_id"`
	MerchantID    string       `json:"merchant_id"`
	Date          time.Time    `json:"date"`
	Total         shared.Money `json:"total"`
	ToMerchant    shared.Money `json:"to_merchant"`
	ToCreditors   shared.Money `json:"to_creditors"`
	Retained      shared.Money `json:"retained"`
	ReceivableIDs []string     `json:"receivable_ids"`
}

type SettlementFailed struct {
	shared.BaseEvent
	SettlementID  string    `json:"settlement_id"`
	MerchantID    string    `json:"merchant_id"`
	Date          time.Time `json:"date"`
	Reason        string    `json:"reason"`
	ReceivableIDs []string  `json:"receivable_ids"`
}
```

#### `internal/domain/settlement/ports.go`

```go
package settlement

import (
	"context"
	"time"
)

type Repository interface {
	Create(ctx context.Context, s *Settlement) error
	Update(ctx context.Context, s *Settlement) error
	FindByID(ctx context.Context, merchantID, id string) (*Settlement, error)
	ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*Settlement, error)
	ListByDate(ctx context.Context, day time.Time) ([]*Settlement, error)
}

// PaymentGateway é o port para "mandar o dinheiro": no estudo, um banco simulado
// que grava um arquivo; em produção, a integração com o banco liquidante/câmara.
type PaymentGateway interface {
	// Send envia as ordens e devolve uma referência externa (número do arquivo/lote).
	Send(ctx context.Context, s *Settlement) (externalRef string, err error)
}
```

#### `internal/domain/settlement/settlement_test.go`

```go
package settlement

import (
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func testMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, nil, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, date(2026, 1, 1))
	_ = m.Approve(date(2026, 1, 1))
	return m
}

func rcv(merchantID string, net shared.Money, due time.Time, status receivable.Status) *receivable.Receivable {
	return receivable.Restore(shared.NewID("rcv"), "tx", merchantID, 1, 1, shared.Credit, "VISA", net, 0, net, due, status, "", "", 1, due, due)
}

func TestBuildWithLiensAndAnticipated(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	rs := []*receivable.Receivable{
		rcv(m.ID(), 50000, day, receivable.Scheduled),
		rcv(m.ID(), 30000, day, receivable.Scheduled),
		rcv(m.ID(), 20000, day, receivable.Anticipated), // já pago ao EC na antecipação
	}
	creditor, _ := merchant.NewBankAccount("001", "1234", "99999-9", merchant.Checking)
	liens := []receivable.Lien{{MerchantID: m.ID(), DueDate: day, Amount: 25000, CreditorName: "Banco Credor", Creditor: creditor}}

	s, err := Build(day, m, rs, liens, day)
	if err != nil {
		t.Fatal(err)
	}
	if s.Total() != 100000 || s.Retained() != 20000 || s.ToCreditors() != 25000 || s.ToMerchant() != 55000 {
		t.Fatalf("total=%d retained=%d creditors=%d merchant=%d", s.Total(), s.Retained(), s.ToCreditors(), s.ToMerchant())
	}
	if len(s.Orders()) != 2 || s.Orders()[0].Kind != ToCreditor || s.Orders()[1].Kind != ToMerchant {
		t.Fatalf("orders: %+v", s.Orders())
	}
	if s.Orders()[1].BankAccount != m.BankAccount() {
		t.Fatal("ordem do EC deve ir para o domicílio bancário dele")
	}

	// ônus maior que o disponível: paga só o que há, nada vai para o EC
	big := []receivable.Lien{{MerchantID: m.ID(), DueDate: day, Amount: 999999, CreditorName: "Banco", Creditor: creditor}}
	s2, _ := Build(day, m, rs, big, day)
	if s2.ToCreditors() != 80000 || s2.ToMerchant() != 0 || len(s2.Orders()) != 1 {
		t.Fatalf("creditors=%d merchant=%d", s2.ToCreditors(), s2.ToMerchant())
	}
}

func TestBuildRejects(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	if _, err := Build(day, m, nil, nil, day); !errors.Is(err, ErrNothingToSettle) {
		t.Error("vazio")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv("m_other", 1, day, receivable.Scheduled)}, nil, day); !errors.Is(err, ErrWrongMerchant) {
		t.Error("outro EC")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 1, day.AddDate(0, 0, 1), receivable.Scheduled)}, nil, day); !errors.Is(err, ErrWrongDay) {
		t.Error("outro dia")
	}
	if _, err := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 1, day, receivable.Settled)}, nil, day); !errors.Is(err, ErrWrongStatus) {
		t.Error("já liquidado")
	}
}

func TestLifecycle(t *testing.T) {
	m := testMerchant(t)
	day := date(2026, 10, 13)
	s, _ := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 100, day, receivable.Scheduled)}, nil, day)
	if err := s.MarkConfirmed(day); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("confirmar sem enviar")
	}
	_ = s.MarkSent("ARQ-0001", day)
	if err := s.MarkConfirmed(day); err != nil || s.Status() != Confirmed {
		t.Fatal(err)
	}
	if evs := s.PullEvents(); len(evs) != 1 || evs[0].EventType() != "settlement.completed" {
		t.Fatalf("%+v", evs)
	}
	s2, _ := Build(day, m, []*receivable.Receivable{rcv(m.ID(), 100, day, receivable.Scheduled)}, nil, day)
	_ = s2.MarkSent("ARQ-0002", day)
	if err := s2.MarkFailed("conta inválida", day); err != nil || s2.FailureReason() == "" {
		t.Fatal(err)
	}
}
```

### 1.7 Pacote `chargeback`

#### `internal/domain/chargeback/chargeback.go`

```go
package chargeback

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

type Status string

const (
	Opened   Status = "OPENED"   // emissor contestou; valor retido/estornado do EC
	Defended Status = "DEFENDED" // EC apresentou documentos
	Accepted Status = "ACCEPTED" // EC perdeu: estorno definitivo
	Reversed Status = "REVERSED" // EC ganhou: valor devolvido ao EC
)

var transitions = map[Status][]Status{
	Opened:   {Defended, Accepted, Reversed},
	Defended: {Accepted, Reversed},
}

var (
	ErrInvalidAmount     = errors.New("valor do chargeback maior que o da transação")
	ErrTransactionStatus = errors.New("só transações capturadas ou liquidadas podem ser contestadas")
)

// Chargeback é a contestação de uma compra pelo portador, trazida pelo emissor.
type Chargeback struct {
	id            string
	transactionID string
	merchantID    string
	reasonCode    string // código da bandeira, ex.: "4837" (fraude, Mastercard), "10.4" (Visa)
	amount        shared.Money
	status        Status
	openedAt      time.Time
	deadline      time.Time // prazo para o EC se defender
	resolvedAt    *time.Time
	version       int
	events        []shared.Event
}

const defenseDays = 10

// Open cria a contestação e muda o estado da transação (via tx.Chargeback).
func Open(tx *transaction.Transaction, reasonCode string, amount shared.Money, now time.Time) (*Chargeback, error) {
	if tx.Status() != transaction.Captured && tx.Status() != transaction.Settled {
		return nil, ErrTransactionStatus
	}
	if !amount.IsPositive() || amount > tx.Amount() {
		return nil, ErrInvalidAmount
	}
	if err := tx.Chargeback(reasonCode, now); err != nil {
		return nil, err
	}
	c := &Chargeback{
		id: shared.NewID("cb"), transactionID: tx.ID(), merchantID: tx.MerchantID(), reasonCode: reasonCode,
		amount: amount, status: Opened, openedAt: now, deadline: now.AddDate(0, 0, defenseDays), version: 1,
	}
	c.events = append(c.events, ChargebackOpened{
		BaseEvent:    shared.NewBaseEvent("chargeback.opened", c.id, now),
		ChargebackID: c.id, TransactionID: tx.ID(), MerchantID: tx.MerchantID(), Amount: amount, ReasonCode: reasonCode, Deadline: c.deadline,
	})
	return c, nil
}

func (c *Chargeback) transition(to Status) error {
	for _, allowed := range transitions[c.status] {
		if allowed == to {
			c.status = to
			c.version++
			return nil
		}
	}
	return fmt.Errorf("%w: %s → %s", shared.ErrInvalidTransition, c.status, to)
}

func (c *Chargeback) Defend(now time.Time) error {
	if now.After(c.deadline) {
		return errors.New("prazo de defesa encerrado")
	}
	return c.transition(Defended)
}

func (c *Chargeback) Accept(now time.Time) error  { return c.resolve(Accepted, now) }
func (c *Chargeback) Reverse(now time.Time) error { return c.resolve(Reversed, now) }

func (c *Chargeback) resolve(to Status, now time.Time) error {
	if err := c.transition(to); err != nil {
		return err
	}
	c.resolvedAt = &now
	c.events = append(c.events, ChargebackResolved{
		BaseEvent:    shared.NewBaseEvent("chargeback.resolved", c.id, now),
		ChargebackID: c.id, TransactionID: c.transactionID, MerchantID: c.merchantID, Amount: c.amount, Outcome: string(to),
	})
	return nil
}

func Restore(id, transactionID, merchantID, reasonCode string, amount shared.Money, status Status, openedAt, deadline time.Time, resolvedAt *time.Time, version int) *Chargeback {
	return &Chargeback{id: id, transactionID: transactionID, merchantID: merchantID, reasonCode: reasonCode, amount: amount,
		status: status, openedAt: openedAt, deadline: deadline, resolvedAt: resolvedAt, version: version}
}

func (c *Chargeback) PullEvents() []shared.Event { evs := c.events; c.events = nil; return evs }

func (c *Chargeback) ID() string             { return c.id }
func (c *Chargeback) TransactionID() string  { return c.transactionID }
func (c *Chargeback) MerchantID() string     { return c.merchantID }
func (c *Chargeback) ReasonCode() string     { return c.reasonCode }
func (c *Chargeback) Amount() shared.Money   { return c.amount }
func (c *Chargeback) Status() Status         { return c.status }
func (c *Chargeback) OpenedAt() time.Time    { return c.openedAt }
func (c *Chargeback) Deadline() time.Time    { return c.deadline }
func (c *Chargeback) ResolvedAt() *time.Time { return c.resolvedAt }
func (c *Chargeback) Version() int           { return c.version }

type Repository interface {
	Create(ctx context.Context, c *Chargeback) error
	Update(ctx context.Context, c *Chargeback) error
	FindByID(ctx context.Context, merchantID, id string) (*Chargeback, error)
}

type ChargebackOpened struct {
	shared.BaseEvent
	ChargebackID  string       `json:"chargeback_id"`
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Amount        shared.Money `json:"amount"`
	ReasonCode    string       `json:"reason_code"`
	Deadline      time.Time    `json:"deadline"`
}

type ChargebackResolved struct {
	shared.BaseEvent
	ChargebackID  string       `json:"chargeback_id"`
	TransactionID string       `json:"transaction_id"`
	MerchantID    string       `json:"merchant_id"`
	Amount        shared.Money `json:"amount"`
	Outcome       string       `json:"outcome"`
}
```

### 1.8 Pacote `ledger`: as partidas dobradas

#### `internal/domain/ledger/entry.go`

A contabilidade da Parte 7.6: cada movimento gera lançamentos que somam zero, e o dinheiro do EC nunca se mistura com receita (Parte 2.3).

```go
package ledger

import (
	"context"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Account são as contas contábeis do razão da adquirente.
// Regra de ouro (compliance): o dinheiro do EC (payable_to_merchant) nunca se mistura
// com a receita (revenue_*). Cada movimento gera um par débito/crédito que fecha em zero.
type Account string

const (
	ReceivableFromIssuer Account = "receivable_from_issuer" // o que o emissor nos deve (ativo)
	PayableToMerchant    Account = "payable_to_merchant"    // o que devemos ao EC (passivo)
	RevenueFees          Account = "revenue_fees"           // receita de MDR
	RevenueAnticipation  Account = "revenue_anticipation"   // receita de antecipação
	BankOutgoing         Account = "bank_outgoing"          // dinheiro que saiu da nossa conta
	ChargebackLoss       Account = "chargeback_loss"        // perdas com contestação
)

// Entry é um lançamento. Débito OU crédito, nunca os dois. Tabela append-only.
type Entry struct {
	ID         string
	Account    Account
	MerchantID string
	Debit      shared.Money
	Credit     shared.Money
	RefType    string // "receivable", "settlement", "anticipation", "chargeback"
	RefID      string
	At         time.Time
}

func debit(acc Account, merchantID string, v shared.Money, refType, refID string, at time.Time) Entry {
	return Entry{ID: shared.NewID("le"), Account: acc, MerchantID: merchantID, Debit: v, RefType: refType, RefID: refID, At: at}
}

func credit(acc Account, merchantID string, v shared.Money, refType, refID string, at time.Time) Entry {
	return Entry{ID: shared.NewID("le"), Account: acc, MerchantID: merchantID, Credit: v, RefType: refType, RefID: refID, At: at}
}

// ForScheduled: ao gerar a agenda, reconhecemos o direito contra o emissor (bruto),
// a obrigação com o EC (líquido) e a receita de MDR (taxa).
func ForScheduled(list []*receivable.Receivable, at time.Time) []Entry {
	var out []Entry
	for _, r := range list {
		out = append(out,
			debit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "receivable", r.ID(), at),
			credit(PayableToMerchant, r.MerchantID(), r.Net(), "receivable", r.ID(), at),
			credit(RevenueFees, r.MerchantID(), r.Fee(), "receivable", r.ID(), at),
		)
	}
	return out
}

// ForAnticipation: pagamos hoje o líquido (sai do banco), baixamos a obrigação com o EC
// pelo valor de face e reconhecemos o desconto como receita.
func ForAnticipation(a *anticipation.Anticipation, at time.Time) []Entry {
	return []Entry{
		debit(PayableToMerchant, a.MerchantID(), a.Gross(), "anticipation", a.ID(), at),
		credit(BankOutgoing, a.MerchantID(), a.Net(), "anticipation", a.ID(), at),
		credit(RevenueAnticipation, a.MerchantID(), a.Discount(), "anticipation", a.ID(), at),
	}
}

// ForSettlement: no vencimento, baixamos a obrigação com o EC/credores e registramos a saída do banco.
// Recebíveis antecipados (retained) já foram baixados na antecipação; não geram lançamento aqui.
func ForSettlement(s *settlement.Settlement, at time.Time) []Entry {
	paid := s.ToMerchant() + s.ToCreditors()
	if paid == 0 {
		return nil
	}
	return []Entry{
		debit(PayableToMerchant, s.MerchantID(), paid, "settlement", s.ID(), at),
		credit(BankOutgoing, s.MerchantID(), paid, "settlement", s.ID(), at),
	}
}

// ForChargeback: o recebível não será pago pelo emissor. Se ainda estava na agenda,
// baixamos o direito contra o emissor e a obrigação com o EC (líquido) e a receita (taxa).
func ForChargeback(r *receivable.Receivable, at time.Time) []Entry {
	return []Entry{
		credit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "chargeback", r.ID(), at),
		debit(PayableToMerchant, r.MerchantID(), r.Net(), "chargeback", r.ID(), at),
		debit(RevenueFees, r.MerchantID(), r.Fee(), "chargeback", r.ID(), at),
	}
}

// ForChargebackAnticipated: o EC já recebeu (antecipação) e o emissor não vai pagar.
// O líquido vira perda da adquirente; a taxa reconhecida é revertida.
func ForChargebackAnticipated(r *receivable.Receivable, at time.Time) []Entry {
	return []Entry{
		credit(ReceivableFromIssuer, r.MerchantID(), r.Gross(), "chargeback", r.ID(), at),
		debit(ChargebackLoss, r.MerchantID(), r.Net(), "chargeback", r.ID(), at),
		debit(RevenueFees, r.MerchantID(), r.Fee(), "chargeback", r.ID(), at),
	}
}

// Balanced verifica a invariante das partidas dobradas: Σ débitos == Σ créditos.
func Balanced(entries []Entry) bool {
	var d, c shared.Money
	for _, e := range entries {
		d += e.Debit
		c += e.Credit
	}
	return d == c
}

type Repository interface {
	Append(ctx context.Context, entries []Entry) error
	// Balance devolve créditos − débitos de uma conta (para um EC ou, com "" , geral).
	Balance(ctx context.Context, account Account, merchantID string) (shared.Money, error)
}
```

#### `internal/domain/ledger/entry_test.go`

```go
package ledger

import (
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

func TestEntriesAlwaysBalance(t *testing.T) {
	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	due := at.AddDate(0, 0, 30)
	r1 := receivable.Restore("rcv_1", "tx_1", "m_1", 1, 2, shared.Credit, "VISA", 50000, 1750, 48250, due, receivable.Scheduled, "", "", 1, at, at)
	r2 := receivable.Restore("rcv_2", "tx_1", "m_1", 2, 2, shared.Credit, "VISA", 50000, 1750, 48250, due.AddDate(0, 0, 30), receivable.Scheduled, "", "", 1, at, at)

	sched := ForScheduled([]*receivable.Receivable{r1, r2}, at)
	if len(sched) != 6 || !Balanced(sched) {
		t.Fatalf("scheduled: %d lançamentos, balanced=%v", len(sched), Balanced(sched))
	}

	a, err := anticipation.Simulate("m_1", []*receivable.Receivable{r1, r2}, at, 200, anticipation.CompoundPricer{})
	if err != nil {
		t.Fatal(err)
	}
	ant := ForAnticipation(a, at)
	if !Balanced(ant) {
		t.Fatal("anticipation não fecha")
	}

	cb := ForChargeback(r1, at)
	if !Balanced(cb) {
		t.Fatal("chargeback não fecha")
	}

	// um lançamento solto não fecha
	if Balanced([]Entry{{Debit: 1}}) {
		t.Fatal("deveria detectar desbalanceamento")
	}
}
```

**Compile e teste o domínio inteiro:**

```bash
go vet ./internal/domain/... && go test -cover ./internal/domain/...
```

**Pronto quando:** todos os pacotes passam; `go list -deps ./internal/domain/... | grep -v '^github.com/seu-usuario' | grep -v '^[a-z]*$'` mostra só `github.com/google/uuid` (o domínio não depende de mais nada).

---

## Fase 2 — A camada de aplicação

**Objetivo:** os casos de uso. Cada arquivo orquestra domínio + ports; nenhum contém regra de cálculo (isso ficou no domínio) nem SQL/JSON (isso vai para a infraestrutura). Releia a Parte 6.3.

#### `internal/application/uow.go`

A Unit of Work e o conjunto de repositórios que participam de uma transação de banco (Parte 7.5). A outbox e os eventos processados entram aqui porque precisam estar **na mesma transação** do dado (Parte 12.4).

```go
package application

import (
	"context"

	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/chargeback"
	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// Repositories reúne todos os repositórios que participam de UMA transação de banco.
// A Unit of Work cria uma instância por transação; os casos de uso só enxergam isto.
type Repositories struct {
	Transactions  transaction.Repository
	Receivables   receivable.Repository
	Settlements   settlement.Repository
	Anticipations anticipation.Repository
	Chargebacks   chargeback.Repository
	Ledger        ledger.Repository
	Outbox        OutboxRepository
	Processed     ProcessedEventsRepository
	NSU           transaction.NSUGenerator
}

// UnitOfWork executa fn dentro de uma transação: se fn devolve erro, tudo é desfeito.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(repos Repositories) error) error
}

// OutboxRepository grava eventos na tabela outbox, na MESMA transação do dado.
// Um relay lê a outbox e publica no Kafka depois (padrão Transactional Outbox).
type OutboxRepository interface {
	Append(ctx context.Context, events ...shared.Event) error
}

// ProcessedEventsRepository torna consumidores idempotentes: MarkIfNew devolve false
// se este consumidor já processou este evento (INSERT ... ON CONFLICT DO NOTHING).
type ProcessedEventsRepository interface {
	MarkIfNew(ctx context.Context, consumer, eventID string) (bool, error)
}
```

#### `internal/application/errors.go`

Erro permanente vs. transitório: é o que decide entre DLQ e retry no consumidor Kafka (Parte 12.5).

```go
package application

import "errors"

// PermanentError marca um erro que NÃO vale a pena tentar de novo (mensagem malformada,
// regra de negócio impossível). Consumidores Kafka mandam esses para a DLQ e seguem;
// os demais (banco fora, timeout) são transitórios e o consumidor tenta de novo.
type PermanentError struct{ Err error }

func (e *PermanentError) Error() string { return "erro permanente: " + e.Err.Error() }
func (e *PermanentError) Unwrap() error { return e.Err }

func Permanent(err error) error { return &PermanentError{Err: err} }

func IsPermanent(err error) bool {
	var pe *PermanentError
	return errors.As(err, &pe)
}
```

#### `internal/application/idempotency.go`

O contrato da idempotência (Parte 4.3). Fica na aplicação para o middleware HTTP e o Postgres dependerem dela, e não um do outro.

```go
package application

import "context"

// Idempotência (Parte 4.3): a mesma requisição, repetida, produz a mesma resposta e
// executa uma vez só. A interface fica na camada de aplicação; o Postgres implementa;
// o middleware HTTP usa.

type IdempotencyState int

const (
	IdempotencyNew        IdempotencyState = iota // primeira vez: execute a requisição
	IdempotencyReplay                             // já executada com o mesmo corpo: repita a resposta
	IdempotencyConflict                           // mesma chave, corpo diferente: 409
	IdempotencyInProgress                         // a original ainda está rodando: 409
)

type IdempotencyResult struct {
	State  IdempotencyState
	Status int
	Body   []byte
}

type IdempotencyStore interface {
	// Begin tenta reservar a chave de forma atômica.
	Begin(ctx context.Context, merchantID, key, requestHash string) (IdempotencyResult, error)
	// Complete guarda a resposta para replays.
	Complete(ctx context.Context, merchantID, key string, status int, body []byte) error
	// Abandon libera a chave quando a execução original falhou com 5xx.
	Abandon(ctx context.Context, merchantID, key string) error
}
```

#### `internal/application/merchant/onboard.go`

Credenciamento. Entrada em primitivos (padrão Command, Parte 7.4); o EC nasce em análise.

```go
package merchant

import (
	"context"
	"fmt"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// OnboardCommand é a entrada do caso de uso (padrão Command): só dados primitivos,
// para o handler HTTP não precisar conhecer tipos do domínio.
type OnboardCommand struct {
	Document             string
	LegalName            string
	MCC                  string
	BankCode             string
	Branch               string
	AccountNumber        string
	AccountKind          string
	DebitBps             int
	CreditBps            int
	CreditInstallmentBps map[int]int // parcelas → bps
	AnticipationBpsMonth int
	WebhookURL           string
}

type Onboard struct {
	merchants merchant.Repository
	clock     shared.Clock
}

func NewOnboard(merchants merchant.Repository, clock shared.Clock) *Onboard {
	return &Onboard{merchants: merchants, clock: clock}
}

// Execute credencia o EC. Ele nasce UNDER_REVIEW: a ativação é outro caso de uso,
// com outra permissão (KYC, Parte 2.5).
func (uc *Onboard) Execute(ctx context.Context, cmd OnboardCommand) (*merchant.Merchant, error) {
	doc, err := merchant.NewDocument(cmd.Document)
	if err != nil {
		return nil, err
	}
	bank, err := merchant.NewBankAccount(cmd.BankCode, cmd.Branch, cmd.AccountNumber, merchant.AccountKind(cmd.AccountKind))
	if err != nil {
		return nil, err
	}
	installments := make(map[int]shared.Bps, len(cmd.CreditInstallmentBps))
	for n, bps := range cmd.CreditInstallmentBps {
		installments[n] = shared.Bps(bps)
	}
	plan, err := merchant.NewFeePlan(shared.Bps(cmd.DebitBps), shared.Bps(cmd.CreditBps), installments, shared.Bps(cmd.AnticipationBpsMonth))
	if err != nil {
		return nil, err
	}
	now := uc.clock.Now()
	m, err := merchant.New(doc, cmd.LegalName, cmd.MCC, bank, plan, now)
	if err != nil {
		return nil, err
	}
	if cmd.WebhookURL != "" {
		if err := m.SetWebhook(cmd.WebhookURL, now); err != nil {
			return nil, err
		}
	}
	if err := uc.merchants.Create(ctx, m); err != nil {
		return nil, fmt.Errorf("salvar estabelecimento: %w", err)
	}
	return m, nil
}
```

#### `internal/application/merchant/review.go`

Aprovar, bloquear e configurar. Repare no esqueleto `change` (Template Method).

```go
package merchant

import (
	"context"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Review agrupa as decisões de análise: aprovar e bloquear.
// Quem chama precisa do scope "merchants:approve" (checado no middleware).
type Review struct {
	merchants merchant.Repository
	clock     shared.Clock
}

func NewReview(merchants merchant.Repository, clock shared.Clock) *Review {
	return &Review{merchants: merchants, clock: clock}
}

func (uc *Review) Approve(ctx context.Context, id string) (*merchant.Merchant, error) {
	return uc.change(ctx, id, func(m *merchant.Merchant) error { return m.Approve(uc.clock.Now()) })
}

func (uc *Review) Block(ctx context.Context, id string) (*merchant.Merchant, error) {
	return uc.change(ctx, id, func(m *merchant.Merchant) error { return m.Block(uc.clock.Now()) })
}

// change é o esqueleto comum: carrega, aplica a regra, salva com lock otimista.
func (uc *Review) change(ctx context.Context, id string, apply func(*merchant.Merchant) error) (*merchant.Merchant, error) {
	m, err := uc.merchants.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := apply(m); err != nil {
		return nil, err
	}
	if err := uc.merchants.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// UpdateSettings altera configurações que o próprio EC controla.
type UpdateSettingsCommand struct {
	MerchantID       string
	WebhookURL       *string // nil = não alterar
	AutoAnticipation *bool
}

func (uc *Review) UpdateSettings(ctx context.Context, cmd UpdateSettingsCommand) (*merchant.Merchant, error) {
	return uc.change(ctx, cmd.MerchantID, func(m *merchant.Merchant) error {
		now := uc.clock.Now()
		if cmd.WebhookURL != nil {
			if err := m.SetWebhook(*cmd.WebhookURL, now); err != nil {
				return err
			}
		}
		if cmd.AutoAnticipation != nil {
			m.EnableAutoAnticipation(*cmd.AutoAnticipation, now)
		}
		return nil
	})
}
```

#### `internal/application/transaction/authorize.go`

**O coração da adquirente.** Siga os sete passos do comentário. A Saga de timeout (Parte 7.6) e o `context.WithoutCancel` para o reversal (Parte 3.6) estão aqui.

```go
package transaction

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

type AuthorizeCommand struct {
	MerchantID   string
	Amount       int64
	Product      string
	Installments int
	CardToken    string
}

// Rule é um elo da Chain of Responsibility de pré-autorização (limites, velocidade, fraude).
// Devolve erro para barrar a transação antes de falar com o emissor.
type Rule func(ctx context.Context, m *merchant.Merchant, tx *transaction.Transaction) error

// ErrIssuerUnavailable é o que devolvemos quando o emissor não respondeu a tempo.
var ErrIssuerUnavailable = errors.New("emissor indisponível")

// Authorize é o coração da adquirente.
type Authorize struct {
	merchants     merchant.Repository
	vault         transaction.CardVault
	issuer        transaction.IssuerGateway
	uow           application.UnitOfWork
	clock         shared.Clock
	rules         []Rule
	issuerTimeout time.Duration
}

func NewAuthorize(merchants merchant.Repository, vault transaction.CardVault, issuer transaction.IssuerGateway,
	uow application.UnitOfWork, clock shared.Clock, issuerTimeout time.Duration, rules ...Rule) *Authorize {
	return &Authorize{merchants: merchants, vault: vault, issuer: issuer, uow: uow, clock: clock, rules: rules, issuerTimeout: issuerTimeout}
}

// Execute segue os passos da Parte 15, Fase 3:
//  1. EC ativo?  2. regras do domínio  3. cadeia de pré-autorização
//  4. detokenizar (o PAN existe só aqui, em memória)  5. emissor com timeout
//  6. timeout → Saga: reversal + negada "91"  7. tudo ou nada: transação + outbox
//
// Uma negativa do emissor NÃO é erro: a transação volta com status DENIED.
func (uc *Authorize) Execute(ctx context.Context, cmd AuthorizeCommand) (*transaction.Transaction, error) {
	m, err := uc.merchants.FindByID(ctx, cmd.MerchantID)
	if err != nil {
		return nil, err
	}
	product, err := shared.ParseProduct(cmd.Product)
	if err != nil {
		return nil, err
	}

	card, err := uc.vault.Detokenize(ctx, cmd.CardToken)
	if err != nil {
		return nil, fmt.Errorf("vault: %w", err)
	}
	info, err := transaction.CardInfoFromPAN(cmd.CardToken, string(card.PAN))
	if err != nil {
		return nil, err
	}

	now := uc.clock.Now()
	tx, err := transaction.New(m, shared.Money(cmd.Amount), product, cmd.Installments, info, now)
	if err != nil {
		return nil, err
	}
	for _, rule := range uc.rules {
		if err := rule(ctx, m, tx); err != nil {
			return nil, err
		}
	}

	issuerCtx, cancel := context.WithTimeout(ctx, uc.issuerTimeout)
	resp, issuerErr := uc.issuer.Authorize(issuerCtx, transaction.AuthorizationRequest{
		TransactionID: tx.ID(), PAN: card.PAN, Expiry: card.Expiry, Amount: tx.Amount(),
		Product: tx.Product(), Installments: tx.Installments(), MerchantMCC: m.MCC(),
	})
	cancel()
	card = transaction.CardData{} // o PAN sai da memória assim que não é mais necessário

	switch {
	case errors.Is(issuerErr, context.DeadlineExceeded):
		// Saga: não sabemos se o emissor aprovou. Mandamos desfazer (reversal) e negamos.
		// context.WithoutCancel: o reversal precisa ir mesmo que o cliente HTTP já tenha desistido.
		_ = uc.issuer.Reverse(context.WithoutCancel(ctx), tx.ID())
		resp = transaction.AuthorizationResponse{Approved: false, ResponseCode: "91"}
	case issuerErr != nil:
		return nil, fmt.Errorf("%w: %v", ErrIssuerUnavailable, issuerErr)
	}

	err = uc.uow.Do(ctx, func(repos application.Repositories) error {
		if resp.Approved {
			nsu, err := repos.NSU.Next(ctx)
			if err != nil {
				return err
			}
			if err := tx.Authorize(resp.AuthorizationCode, nsu, uc.clock.Now()); err != nil {
				return err
			}
		} else if err := tx.Deny(resp.ResponseCode, uc.clock.Now()); err != nil {
			return err
		}
		if err := repos.Transactions.Create(ctx, tx); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, tx.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// MaxAmountRule é um exemplo de regra de pré-autorização: teto por transação.
func MaxAmountRule(limit shared.Money) Rule {
	return func(_ context.Context, _ *merchant.Merchant, tx *transaction.Transaction) error {
		if tx.Amount() > limit {
			return fmt.Errorf("%w: acima do limite de %s por transação", ErrLimitExceeded, limit)
		}
		return nil
	}
}

var ErrLimitExceeded = errors.New("limite excedido")
```

#### `internal/application/transaction/lifecycle.go`

Capturar, cancelar e consultar, com `FOR UPDATE` para não capturar duas vezes.

```go
package transaction

import (
	"context"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// Lifecycle agrupa capturar e cancelar: mesmo esqueleto (carrega travado → regra → salva → outbox).
type Lifecycle struct {
	uow   application.UnitOfWork
	clock shared.Clock
}

func NewLifecycle(uow application.UnitOfWork, clock shared.Clock) *Lifecycle {
	return &Lifecycle{uow: uow, clock: clock}
}

func (uc *Lifecycle) Capture(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.change(ctx, merchantID, id, func(tx *transaction.Transaction) error { return tx.Capture(uc.clock.Now()) })
}

func (uc *Lifecycle) Cancel(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.change(ctx, merchantID, id, func(tx *transaction.Transaction) error { return tx.Cancel(uc.clock.Now()) })
}

func (uc *Lifecycle) change(ctx context.Context, merchantID, id string, apply func(*transaction.Transaction) error) (*transaction.Transaction, error) {
	var tx *transaction.Transaction
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		var err error
		// FOR UPDATE: duas capturas simultâneas da mesma transação ficam em fila;
		// a segunda lê CAPTURED e recebe ErrInvalidTransition (409).
		tx, err = repos.Transactions.FindByIDForUpdate(ctx, merchantID, id)
		if err != nil {
			return err
		}
		if err := apply(tx); err != nil {
			return err
		}
		if err := repos.Transactions.Update(ctx, tx); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, tx.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// Get é a consulta. O merchantID vem do token, nunca da URL: filtra na consulta.
type Get struct{ transactions transaction.Repository }

func NewGet(transactions transaction.Repository) *Get { return &Get{transactions: transactions} }

func (uc *Get) Execute(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return uc.transactions.FindByID(ctx, merchantID, id)
}
```

#### `internal/application/transaction/authorize_test.go`

Teste com fakes escritos à mão (Parte 3.7): aprovada, negada, timeout com reversal, emissor fora, regras. Nenhum banco, nenhuma rede.

```go
package transaction

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// ---- fakes: como as interfaces são pequenas, escrevemos à mão (Parte 3.7) ----

type fakeMerchants struct{ m *merchant.Merchant }

func (f fakeMerchants) Create(context.Context, *merchant.Merchant) error { return nil }
func (f fakeMerchants) Update(context.Context, *merchant.Merchant) error { return nil }
func (f fakeMerchants) FindByID(_ context.Context, id string) (*merchant.Merchant, error) {
	if f.m != nil && f.m.ID() == id {
		return f.m, nil
	}
	return nil, shared.ErrNotFound
}

type fakeVault struct{}

func (fakeVault) Detokenize(_ context.Context, token string) (transaction.CardData, error) {
	if token == "tok_visa" {
		return transaction.CardData{PAN: "4111111111111111", Expiry: "12/30"}, nil
	}
	return transaction.CardData{}, transaction.ErrInvalidToken
}

type fakeIssuer struct {
	approve  bool
	code     string
	err      error
	reversed []string
}

func (f *fakeIssuer) Authorize(ctx context.Context, _ transaction.AuthorizationRequest) (transaction.AuthorizationResponse, error) {
	if f.err != nil {
		return transaction.AuthorizationResponse{}, f.err
	}
	if f.approve {
		return transaction.AuthorizationResponse{Approved: true, AuthorizationCode: "123456", ResponseCode: "00"}, nil
	}
	return transaction.AuthorizationResponse{Approved: false, ResponseCode: f.code}, nil
}
func (f *fakeIssuer) Reverse(_ context.Context, id string) error {
	f.reversed = append(f.reversed, id)
	return nil
}

type memTxRepo struct {
	saved map[string]*transaction.Transaction
}

func (r *memTxRepo) Create(_ context.Context, t *transaction.Transaction) error {
	r.saved[t.ID()] = t
	return nil
}
func (r *memTxRepo) Update(_ context.Context, t *transaction.Transaction) error {
	r.saved[t.ID()] = t
	return nil
}
func (r *memTxRepo) FindByID(_ context.Context, merchantID, id string) (*transaction.Transaction, error) {
	t, ok := r.saved[id]
	if !ok || t.MerchantID() != merchantID {
		return nil, shared.ErrNotFound
	}
	return t, nil
}
func (r *memTxRepo) FindByIDForUpdate(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.FindByID(ctx, merchantID, id)
}

type memOutbox struct{ events []shared.Event }

func (o *memOutbox) Append(_ context.Context, evs ...shared.Event) error {
	o.events = append(o.events, evs...)
	return nil
}

type seqNSU struct{ n int }

func (s *seqNSU) Next(context.Context) (string, error) {
	s.n++
	return "NSU" + string(rune('0'+s.n)), nil
}

// fakeUoW não tem banco: só executa fn com os repositórios em memória.
type fakeUoW struct{ repos application.Repositories }

func (u fakeUoW) Do(_ context.Context, fn func(application.Repositories) error) error {
	return fn(u.repos)
}

// ---- fixtures ----

var now = time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)

func activeMerchant(t *testing.T) *merchant.Merchant {
	t.Helper()
	doc, _ := merchant.NewDocument("11.222.333/0001-81")
	bank, _ := merchant.NewBankAccount("341", "0001", "12345-6", merchant.Checking)
	plan, _ := merchant.NewFeePlan(150, 250, map[int]shared.Bps{12: 450}, 200)
	m, _ := merchant.New(doc, "Padaria", "5462", bank, plan, now)
	_ = m.Approve(now)
	return m
}

func setup(t *testing.T, issuer *fakeIssuer, rules ...Rule) (*Authorize, *memTxRepo, *memOutbox, *merchant.Merchant) {
	t.Helper()
	m := activeMerchant(t)
	txRepo := &memTxRepo{saved: map[string]*transaction.Transaction{}}
	outbox := &memOutbox{}
	uow := fakeUoW{repos: application.Repositories{Transactions: txRepo, Outbox: outbox, NSU: &seqNSU{}}}
	uc := NewAuthorize(fakeMerchants{m}, fakeVault{}, issuer, uow, shared.FixedClock{T: now}, 2*time.Second, rules...)
	return uc, txRepo, outbox, m
}

func TestAuthorizeApproved(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: true})
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 120000, Product: "CREDIT", Installments: 12, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Authorized || tx.AuthorizationCode() != "123456" || tx.NSU() == "" {
		t.Fatalf("%+v", tx)
	}
	if tx.Card().Last4 != "1111" || tx.Card().Brand != "VISA" {
		t.Fatal("dados públicos do cartão")
	}
	if len(repo.saved) != 1 || len(outbox.events) != 1 || outbox.events[0].EventType() != "transaction.authorized" {
		t.Fatal("deve salvar transação e evento juntos")
	}
}

func TestAuthorizeDeniedIsNotAnError(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: false, code: "51"})
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Denied || tx.ResponseCode() != "51" {
		t.Fatalf("%+v", tx)
	}
	if len(repo.saved) != 1 || outbox.events[0].EventType() != "transaction.denied" {
		t.Fatal("negada também é persistida (auditoria)")
	}
}

func TestAuthorizeTimeoutTriggersReversal(t *testing.T) {
	issuer := &fakeIssuer{err: context.DeadlineExceeded}
	uc, _, _, m := setup(t, issuer)
	tx, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if err != nil {
		t.Fatal(err)
	}
	if tx.Status() != transaction.Denied || tx.ResponseCode() != "91" {
		t.Fatalf("timeout deve virar negada 91, got %s/%s", tx.Status(), tx.ResponseCode())
	}
	if len(issuer.reversed) != 1 || issuer.reversed[0] != tx.ID() {
		t.Fatal("reversal deve ser enviado")
	}
}

func TestAuthorizeIssuerDownIsError(t *testing.T) {
	uc, repo, _, m := setup(t, &fakeIssuer{err: errors.New("connection refused")})
	_, err := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})
	if !errors.Is(err, ErrIssuerUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if len(repo.saved) != 0 {
		t.Fatal("falha técnica não deve persistir nada")
	}
}

func TestAuthorizeValidationAndRules(t *testing.T) {
	uc, _, _, m := setup(t, &fakeIssuer{approve: true}, MaxAmountRule(100000))
	cases := []struct {
		name string
		cmd  AuthorizeCommand
		want error
	}{
		{"EC inexistente", AuthorizeCommand{MerchantID: "m_x", Amount: 100, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"}, shared.ErrNotFound},
		{"produto inválido", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "PIX", Installments: 1, CardToken: "tok_visa"}, shared.ErrInvalidProduct},
		{"token inválido", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "DEBIT", Installments: 1, CardToken: "tok_x"}, transaction.ErrInvalidToken},
		{"parcelas fora do plano", AuthorizeCommand{MerchantID: m.ID(), Amount: 100, Product: "CREDIT", Installments: 6, CardToken: "tok_visa"}, merchant.ErrInstallmentsNotAllowed},
		{"acima do limite", AuthorizeCommand{MerchantID: m.ID(), Amount: 100001, Product: "CREDIT", Installments: 1, CardToken: "tok_visa"}, ErrLimitExceeded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), c.cmd)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestCaptureAndCancel(t *testing.T) {
	uc, repo, outbox, m := setup(t, &fakeIssuer{approve: true})
	tx, _ := uc.Execute(context.Background(), AuthorizeCommand{MerchantID: m.ID(), Amount: 5000, Product: "DEBIT", Installments: 1, CardToken: "tok_visa"})

	lc := NewLifecycle(fakeUoW{repos: application.Repositories{Transactions: repo, Outbox: outbox}}, shared.FixedClock{T: now})
	if _, err := lc.Capture(context.Background(), "m_other", tx.ID()); !errors.Is(err, shared.ErrNotFound) {
		t.Fatal("outro EC não enxerga a transação")
	}
	got, err := lc.Capture(context.Background(), m.ID(), tx.ID())
	if err != nil || got.Status() != transaction.Captured {
		t.Fatal(err)
	}
	if _, err := lc.Capture(context.Background(), m.ID(), tx.ID()); !errors.Is(err, shared.ErrInvalidTransition) {
		t.Fatal("capturar duas vezes")
	}
	if _, err := lc.Cancel(context.Background(), m.ID(), tx.ID()); err != nil {
		t.Fatal(err)
	}
	if len(outbox.events) != 3 { // authorized, captured, canceled
		t.Fatalf("eventos = %d", len(outbox.events))
	}
}
```

#### `internal/application/receivable/schedule.go`

O consumidor da agenda (Parte 12): idempotente via `processed_events`, transacional via UoW, e registra as URs na registradora.

```go
package receivable

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

const consumerName = "scheduler"

// Scheduler reage a eventos de transação e mantém a agenda:
//   - transaction.captured → gera os recebíveis, o ledger e registra as URs;
//   - transaction.canceled → cancela os recebíveis ainda na agenda.
type Scheduler struct {
	uow       application.UnitOfWork
	merchants merchant.Repository
	registry  receivable.Registry
	calendar  shared.BusinessCalendar
	clock     shared.Clock
	log       *slog.Logger
}

func NewScheduler(uow application.UnitOfWork, merchants merchant.Repository, registry receivable.Registry,
	calendar shared.BusinessCalendar, clock shared.Clock, log *slog.Logger) *Scheduler {
	return &Scheduler{uow: uow, merchants: merchants, registry: registry, calendar: calendar, clock: clock, log: log}
}

// Handle recebe o evento bruto (JSON) e despacha pelo tipo.
// Erros de formato são permanentes (Permanent): não adianta tentar de novo → DLQ.
func (s *Scheduler) Handle(ctx context.Context, eventType string, payload []byte) error {
	switch eventType {
	case "transaction.captured":
		var ev transaction.TransactionCaptured
		if err := json.Unmarshal(payload, &ev); err != nil {
			return application.Permanent(fmt.Errorf("payload inválido: %w", err))
		}
		return s.onCaptured(ctx, ev)
	case "transaction.canceled":
		var ev transaction.TransactionCanceled
		if err := json.Unmarshal(payload, &ev); err != nil {
			return application.Permanent(fmt.Errorf("payload inválido: %w", err))
		}
		return s.onCanceled(ctx, ev)
	}
	return nil // tipos que não nos interessam são ignorados
}

func (s *Scheduler) onCaptured(ctx context.Context, ev transaction.TransactionCaptured) error {
	var created []*receivable.Receivable
	err := s.uow.Do(ctx, func(repos application.Repositories) error {
		isNew, err := repos.Processed.MarkIfNew(ctx, consumerName, ev.EventID())
		if err != nil {
			return err
		}
		if !isNew {
			s.log.InfoContext(ctx, "evento duplicado ignorado", "event_id", ev.EventID())
			return nil
		}
		tx, err := repos.Transactions.FindByID(ctx, ev.MerchantID, ev.AggregateID())
		if err != nil {
			return err
		}
		m, err := s.merchants.FindByID(ctx, ev.MerchantID)
		if err != nil {
			return err
		}
		now := s.clock.Now()
		list, err := receivable.Schedule(tx, m.FeePlan(), s.calendar, now)
		if err != nil {
			return application.Permanent(err) // transação não capturada: regra violada, não é transitório
		}
		if err := repos.Receivables.SaveAll(ctx, list); err != nil {
			return err
		}
		if err := repos.Ledger.Append(ctx, ledger.ForScheduled(list, now)); err != nil {
			return err
		}
		created = list
		return repos.Outbox.Append(ctx, receivable.NewReceivablesScheduled(list, now))
	})
	if err != nil || len(created) == 0 {
		return err
	}
	// Registro na registradora fora da transação de banco: se falhar, o Kafka reentrega
	// o evento; MarkIfNew impede duplicar a agenda, mas o registro precisa ser idempotente também.
	if err := s.registry.Register(ctx, receivable.GroupUnits(created)); err != nil {
		s.log.WarnContext(ctx, "registro de URs falhou; será refeito pelo job de reconciliação", "err", err)
	}
	return nil
}

func (s *Scheduler) onCanceled(ctx context.Context, ev transaction.TransactionCanceled) error {
	return s.uow.Do(ctx, func(repos application.Repositories) error {
		isNew, err := repos.Processed.MarkIfNew(ctx, consumerName, ev.EventID())
		if err != nil || !isNew {
			return err
		}
		list, err := repos.Receivables.ListByTransaction(ctx, ev.AggregateID())
		if err != nil {
			return err
		}
		now := s.clock.Now()
		var changed []*receivable.Receivable
		var entries []ledger.Entry
		for _, r := range list {
			if r.Status() != receivable.Scheduled {
				continue
			}
			if err := r.Cancel(now); err != nil {
				return err
			}
			changed = append(changed, r)
			entries = append(entries, ledger.ForChargeback(r, now)...) // mesmos lançamentos inversos da captura
		}
		if len(changed) == 0 {
			return nil
		}
		if err := repos.Receivables.UpdateAll(ctx, changed); err != nil {
			return err
		}
		return repos.Ledger.Append(ctx, entries)
	})
}
```

#### `internal/application/receivable/query.go`

```go
package receivable

import (
	"context"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/receivable"
)

// Query são as leituras da agenda pelo EC. Só leitura: repositório direto, sem UoW.
type Query struct{ receivables receivable.Repository }

func NewQuery(receivables receivable.Repository) *Query { return &Query{receivables: receivables} }

func (q *Query) List(ctx context.Context, merchantID string, f receivable.ListFilter) ([]*receivable.Receivable, error) {
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	return q.receivables.ListByMerchant(ctx, merchantID, f)
}

func (q *Query) Summary(ctx context.Context, merchantID string, from, to time.Time) ([]receivable.DailySummary, error) {
	return q.receivables.Summary(ctx, merchantID, from, to)
}
```

#### `internal/application/settlement/settle_due.go`

O worker de liquidação da Parte 1.7: dia útil, lock diário, lotes com `SKIP LOCKED`, ônus, banco, ledger, eventos.

```go
package settlement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// DailyLock garante uma única execução por dia mesmo com vários workers (Redis SET NX).
type DailyLock interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key string) error
}

var ErrAlreadyRunning = errors.New("liquidação do dia já está em execução")

// Report é o resumo de uma rodada, para log e métricas.
type Report struct {
	Date        time.Time
	Skipped     string // motivo, se não rodou (fim de semana, feriado)
	Merchants   int
	Settlements int
	Failed      int
	Paid        shared.Money
	Retained    shared.Money
}

type SettleDue struct {
	uow       application.UnitOfWork
	merchants merchant.Repository
	registry  receivable.Registry
	gateway   settlement.PaymentGateway
	calendar  shared.BusinessCalendar
	clock     shared.Clock
	lock      DailyLock
	batchSize int
	log       *slog.Logger
}

func NewSettleDue(uow application.UnitOfWork, merchants merchant.Repository, registry receivable.Registry, gateway settlement.PaymentGateway,
	calendar shared.BusinessCalendar, clock shared.Clock, lock DailyLock, batchSize int, log *slog.Logger) *SettleDue {
	return &SettleDue{uow: uow, merchants: merchants, registry: registry, gateway: gateway, calendar: calendar, clock: clock, lock: lock, batchSize: batchSize, log: log}
}

// Execute liquida tudo que vence em `day`. Passos (Parte 1.7):
//  1. só dia útil;  2. lock diário;  3. em lotes: trava recebíveis (SKIP LOCKED),
//  4. agrupa por EC;  5. consulta ônus;  6. monta o lote;  7. envia ao banco;
//  8. marca SETTLED + ledger + evento — tudo na mesma transação de banco.
//
// Se o banco rejeitar o lote de UM EC, esse lote fica FAILED (auditoria), os recebíveis
// dele continuam SCHEDULED e os demais ECs seguem normalmente.
func (uc *SettleDue) Execute(ctx context.Context, day time.Time) (Report, error) {
	day = shared.DateOnly(day)
	rep := Report{Date: day}
	if !uc.calendar.IsBusinessDay(day) {
		rep.Skipped = "não é dia útil"
		return rep, nil
	}
	key := "settlement:" + day.Format("2006-01-02")
	ok, err := uc.lock.Acquire(ctx, key, 2*time.Hour)
	if err != nil {
		return rep, err
	}
	if !ok {
		return rep, ErrAlreadyRunning
	}
	defer func() { _ = uc.lock.Release(context.WithoutCancel(ctx), key) }()

	for {
		processed, err := uc.batch(ctx, day, &rep)
		if err != nil {
			return rep, err
		}
		if processed == 0 {
			return rep, nil
		}
	}
}

// batch processa até batchSize recebíveis numa transação e devolve quantos pegou.
func (uc *SettleDue) batch(ctx context.Context, day time.Time, rep *Report) (int, error) {
	var count int
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		list, err := repos.Receivables.ListDueOnForUpdate(ctx, day, uc.batchSize)
		if err != nil {
			return err
		}
		count = len(list)
		if count == 0 {
			return nil
		}
		byMerchant := map[string][]*receivable.Receivable{}
		for _, r := range list {
			byMerchant[r.MerchantID()] = append(byMerchant[r.MerchantID()], r)
		}
		now := uc.clock.Now()
		for merchantID, rs := range byMerchant {
			if err := uc.settleMerchant(ctx, repos, merchantID, rs, day, now, rep); err != nil {
				return err
			}
		}
		return nil
	})
	return count, err
}

func (uc *SettleDue) settleMerchant(ctx context.Context, repos application.Repositories, merchantID string,
	rs []*receivable.Receivable, day, now time.Time, rep *Report) error {
	rep.Merchants++
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return fmt.Errorf("EC %s: %w", merchantID, err)
	}
	liens, err := uc.registry.CheckLiens(ctx, merchantID, day) // compliance: nunca pagar sem checar ônus
	if err != nil {
		return fmt.Errorf("consultar ônus de %s: %w", merchantID, err)
	}
	s, err := settlement.Build(day, m, rs, liens, now)
	if err != nil {
		return err
	}
	if err := repos.Settlements.Create(ctx, s); err != nil {
		return err
	}

	ref, sendErr := uc.gateway.Send(ctx, s)
	if sendErr != nil {
		rep.Failed++
		uc.log.ErrorContext(ctx, "banco rejeitou lote", "settlement_id", s.ID(), "merchant_id", merchantID, "err", sendErr)
		if err := s.MarkFailed(sendErr.Error(), now); err != nil {
			return err
		}
		if err := repos.Settlements.Update(ctx, s); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, s.PullEvents()...)
	}

	if err := s.MarkSent(ref, now); err != nil {
		return err
	}
	if err := s.MarkConfirmed(now); err != nil { // o banco simulado confirma na hora; o real confirmaria por arquivo de retorno
		return err
	}
	for _, r := range rs {
		if err := r.Settle(s.ID(), now); err != nil {
			return err
		}
	}
	if err := repos.Receivables.UpdateAll(ctx, rs); err != nil {
		return err
	}
	if err := repos.Settlements.Update(ctx, s); err != nil {
		return err
	}
	if entries := ledger.ForSettlement(s, now); len(entries) > 0 {
		if err := repos.Ledger.Append(ctx, entries); err != nil {
			return err
		}
	}
	rep.Settlements++
	rep.Paid += s.ToMerchant() + s.ToCreditors()
	rep.Retained += s.Retained()
	return repos.Outbox.Append(ctx, s.PullEvents()...)
}
```

#### `internal/application/anticipation/anticipate.go`

Simular e contratar (Parte 1.5), com recebíveis travados e checagem de ônus (Parte 2.4).

```go
package anticipation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

var ErrReceivableLiened = errors.New("há ônus registrado sobre recebíveis desta data; antecipação não permitida")

// Anticipate cobre simular (só cálculo) e contratar (muda estado).
type Anticipate struct {
	uow         application.UnitOfWork
	receivables receivable.Repository // leitura sem transação, para simular
	merchants   merchant.Repository
	registry    receivable.Registry
	pricer      anticipation.Pricer
	clock       shared.Clock
	log         *slog.Logger
}

func NewAnticipate(uow application.UnitOfWork, receivables receivable.Repository, merchants merchant.Repository,
	registry receivable.Registry, pricer anticipation.Pricer, clock shared.Clock, log *slog.Logger) *Anticipate {
	return &Anticipate{uow: uow, receivables: receivables, merchants: merchants, registry: registry, pricer: pricer, clock: clock, log: log}
}

// Simulate não altera nada: devolve bruto, desconto e líquido por recebível.
func (uc *Anticipate) Simulate(ctx context.Context, merchantID string, receivableIDs []string) (*anticipation.Anticipation, error) {
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	rs, err := uc.receivables.FindByIDs(ctx, merchantID, receivableIDs)
	if err != nil {
		return nil, err
	}
	if len(rs) != len(receivableIDs) {
		return nil, fmt.Errorf("%w: algum recebível não existe ou não é seu", shared.ErrNotFound)
	}
	return anticipation.Simulate(merchantID, rs, uc.clock.Now(), m.FeePlan().AnticipationRateMonth(), uc.pricer)
}

// Request contrata. Tudo dentro da UoW com os recebíveis TRAVADOS (FOR UPDATE):
// dois pedidos simultâneos dos mesmos recebíveis → o segundo lê ANTICIPATED e recebe erro.
func (uc *Anticipate) Request(ctx context.Context, merchantID string, receivableIDs []string) (*anticipation.Anticipation, error) {
	m, err := uc.merchants.FindByID(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	var a *anticipation.Anticipation
	err = uc.uow.Do(ctx, func(repos application.Repositories) error {
		rs, err := repos.Receivables.FindForUpdate(ctx, merchantID, receivableIDs)
		if err != nil {
			return err
		}
		if len(rs) != len(receivableIDs) {
			return fmt.Errorf("%w: algum recebível não existe ou não é seu", shared.ErrNotFound)
		}
		now := uc.clock.Now()
		a, err = anticipation.Simulate(merchantID, rs, now, m.FeePlan().AnticipationRateMonth(), uc.pricer)
		if err != nil {
			return err
		}
		if err := uc.ensureNoLiens(ctx, merchantID, rs); err != nil {
			return err
		}
		if err := a.Confirm(rs, now); err != nil {
			return err
		}
		if err := repos.Anticipations.Create(ctx, a); err != nil {
			return err
		}
		if err := repos.Receivables.UpdateAll(ctx, rs); err != nil {
			return err
		}
		if err := repos.Ledger.Append(ctx, ledger.ForAnticipation(a, now)); err != nil {
			return err
		}
		return repos.Outbox.Append(ctx, a.PullEvents()...)
	})
	if err != nil {
		return nil, err
	}
	// A troca de titularidade na registradora acontece após o commit; se falhar, o job
	// de reconciliação refaz a partir do evento receivable.anticipated.
	if err := uc.registry.TransferOwnership(ctx, a.ReceivableIDs(), "ADQUIRENTE"); err != nil {
		uc.log.WarnContext(ctx, "troca de titularidade falhou; reconciliação refará", "anticipation_id", a.ID(), "err", err)
	}
	return a, nil
}

// ensureNoLiens consulta a registradora para cada data de vencimento envolvida.
func (uc *Anticipate) ensureNoLiens(ctx context.Context, merchantID string, rs []*receivable.Receivable) error {
	seen := map[time.Time]bool{}
	for _, r := range rs {
		if seen[r.DueDate()] {
			continue
		}
		seen[r.DueDate()] = true
		liens, err := uc.registry.CheckLiens(ctx, merchantID, r.DueDate())
		if err != nil {
			return fmt.Errorf("consultar ônus: %w", err)
		}
		if len(liens) > 0 {
			return fmt.Errorf("%w (%s)", ErrReceivableLiened, r.DueDate().Format("2006-01-02"))
		}
	}
	return nil
}

// Get e List são leituras.
func (uc *Anticipate) Get(ctx context.Context, merchantID, id string) (*anticipation.Anticipation, error) {
	var a *anticipation.Anticipation
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		var err error
		a, err = repos.Anticipations.FindByID(ctx, merchantID, id)
		return err
	})
	return a, err
}
```

#### `internal/application/chargeback/open.go`

```go
package chargeback

import (
	"context"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/chargeback"
	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type OpenCommand struct {
	MerchantID    string
	TransactionID string
	ReasonCode    string
	Amount        int64
}

// Open simula a bandeira trazendo uma contestação. No mundo real isso chega por arquivo/mensagem
// da bandeira; aqui é um endpoint interno.
type Open struct {
	uow   application.UnitOfWork
	clock shared.Clock
}

func NewOpen(uow application.UnitOfWork, clock shared.Clock) *Open {
	return &Open{uow: uow, clock: clock}
}

// Execute: trava a transação, abre o chargeback, marca recebíveis, lança no ledger, publica eventos.
// Recebível SCHEDULED: só deixa de ser pago. ANTICIPATED: o EC já recebeu → perda para a adquirente
// (na prática, cobrada do EC em repasses futuros; aqui registramos a perda).
func (uc *Open) Execute(ctx context.Context, cmd OpenCommand) (*chargeback.Chargeback, error) {
	var cb *chargeback.Chargeback
	err := uc.uow.Do(ctx, func(repos application.Repositories) error {
		tx, err := repos.Transactions.FindByIDForUpdate(ctx, cmd.MerchantID, cmd.TransactionID)
		if err != nil {
			return err
		}
		now := uc.clock.Now()
		cb, err = chargeback.Open(tx, cmd.ReasonCode, shared.Money(cmd.Amount), now)
		if err != nil {
			return err
		}
		if err := repos.Transactions.Update(ctx, tx); err != nil {
			return err
		}
		if err := repos.Chargebacks.Create(ctx, cb); err != nil {
			return err
		}
		list, err := repos.Receivables.ListByTransaction(ctx, tx.ID())
		if err != nil {
			return err
		}
		var changed []*receivable.Receivable
		var entries []ledger.Entry
		for _, r := range list {
			switch r.Status() {
			case receivable.Scheduled:
				entries = append(entries, ledger.ForChargeback(r, now)...)
			case receivable.Anticipated:
				entries = append(entries, ledger.ForChargebackAnticipated(r, now)...)
			default:
				continue // já liquidado/cancelado: fica como está
			}
			if err := r.Chargeback(now); err != nil {
				return err
			}
			changed = append(changed, r)
		}
		if len(changed) > 0 {
			if err := repos.Receivables.UpdateAll(ctx, changed); err != nil {
				return err
			}
			if err := repos.Ledger.Append(ctx, entries); err != nil {
				return err
			}
		}
		events := append(tx.PullEvents(), cb.PullEvents()...)
		return repos.Outbox.Append(ctx, events...)
	})
	if err != nil {
		return nil, err
	}
	return cb, nil
}
```

#### `internal/application/notification/notify.go`

Webhooks assinados com HMAC (Partes 4.6 e 9.2).

```go
package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Sender entrega um webhook. A infraestrutura implementa com HTTP.
type Sender interface {
	Send(ctx context.Context, url string, body []byte, headers map[string]string) error
}

// Notifier consome eventos e avisa o EC pela URL cadastrada, assinando o corpo com HMAC-SHA256.
// O EC valida a assinatura com o mesmo segredo e usa X-Event-Id para descartar duplicatas.
type Notifier struct {
	merchants merchant.Repository
	sender    Sender
	secret    []byte
	log       *slog.Logger
}

func NewNotifier(merchants merchant.Repository, sender Sender, secret string, log *slog.Logger) *Notifier {
	return &Notifier{merchants: merchants, sender: sender, secret: []byte(secret), log: log}
}

// envelope lê só o que precisamos de qualquer evento: quem é o EC e qual é o id.
type envelope struct {
	EventID    string `json:"event_id"`
	MerchantID string `json:"merchant_id"`
}

func (n *Notifier) Handle(ctx context.Context, eventType string, payload []byte) error {
	var env envelope
	if err := json.Unmarshal(payload, &env); err != nil || env.MerchantID == "" {
		return application.Permanent(fmt.Errorf("evento sem merchant_id: %w", err))
	}
	m, err := n.merchants.FindByID(ctx, env.MerchantID)
	if errors.Is(err, shared.ErrNotFound) {
		return application.Permanent(err)
	}
	if err != nil {
		return err // banco fora: transitório, o consumidor tenta de novo
	}
	if m.WebhookURL() == "" {
		return nil // EC não quer webhooks
	}
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-Event-Id":   env.EventID,
		"X-Event-Type": eventType,
		"X-Signature":  "sha256=" + Sign(n.secret, payload),
	}
	if err := n.sender.Send(ctx, m.WebhookURL(), payload, headers); err != nil {
		return fmt.Errorf("entregar webhook %s ao EC %s: %w", env.EventID, m.ID(), err)
	}
	n.log.InfoContext(ctx, "webhook entregue", "event_id", env.EventID, "event_type", eventType, "merchant_id", m.ID())
	return nil
}

// Sign calcula o HMAC-SHA256 do corpo. O EC recalcula e compara com hmac.Equal (tempo constante).
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// Verify é o que o EC faz do lado dele (fornecido aqui para o cmd/merchant-sim e para testes).
func Verify(secret, body []byte, signature string) bool {
	expected := Sign(secret, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}
```

**Compile e teste:**

```bash
go vet ./internal/application/... && go test ./internal/application/...
```

---

## Fase 3 — Os contratos: OpenAPI e gRPC

**Objetivo:** a API pública descrita antes de existir (design-first, Parte 5) e os contratos internos em Protobuf (Parte 11).

#### `api/openapi.yaml`

```yaml
openapi: 3.1.0
info:
  title: Adquirente API
  version: 1.0.0
  description: |
    API pública da adquirente: credenciamento, tokenização de cartão, autorização, agenda de recebíveis,
    liquidações e antecipação. Todo POST que cria dinheiro ou movimento exige o header `Idempotency-Key`.
    Erros seguem RFC 9457 (Problem Details); decida pelo campo `code`.
servers:
  - url: http://localhost:8080/v1
security:
  - bearerAuth: []

paths:
  /merchants:
    post:
      operationId: createMerchant
      summary: Credencia um estabelecimento (nasce UNDER_REVIEW)
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/CreateMerchantRequest' }
      responses:
        '201': { description: Criado, content: { application/json: { schema: { $ref: '#/components/schemas/Merchant' } } } }
        '400': { $ref: '#/components/responses/BadRequest' }
  /merchants/{id}:
    get:
      operationId: getMerchant
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      responses:
        '200': { description: OK, content: { application/json: { schema: { $ref: '#/components/schemas/Merchant' } } } }
        '403': { $ref: '#/components/responses/Forbidden' }
        '404': { $ref: '#/components/responses/NotFound' }
  /merchants/{id}/approve:
    post:
      operationId: approveMerchant
      summary: Conclui a análise (KYC). Exige scope merchants:approve.
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      responses:
        '200': { description: Ativo, content: { application/json: { schema: { $ref: '#/components/schemas/Merchant' } } } }
        '409': { $ref: '#/components/responses/Conflict' }
  /merchants/{id}/block:
    post:
      operationId: blockMerchant
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      responses:
        '200': { description: Bloqueado, content: { application/json: { schema: { $ref: '#/components/schemas/Merchant' } } } }
  /merchants/{id}/settings:
    patch:
      operationId: updateMerchantSettings
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                webhook_url: { type: string, format: uri, description: Só https; hosts privados são rejeitados }
                auto_anticipation: { type: boolean }
      responses:
        '200': { description: OK, content: { application/json: { schema: { $ref: '#/components/schemas/Merchant' } } } }

  /card-tokens:
    post:
      operationId: tokenizeCard
      summary: Troca o número do cartão por um token (escopo PCI)
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [pan, expiry]
              properties:
                pan: { type: string, example: "4111111111111111" }
                expiry: { type: string, example: "12/30" }
      responses:
        '201':
          description: Token criado
          content:
            application/json:
              schema:
                type: object
                properties:
                  token: { type: string, example: tok_5f1c... }
                  brand: { type: string, example: VISA }
                  bin: { type: string, example: "411111" }
                  last4: { type: string, example: "1111" }

  /transactions:
    post:
      operationId: authorizeTransaction
      summary: Autoriza uma transação de cartão
      parameters: [ { $ref: '#/components/parameters/IdempotencyKey' } ]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/AuthorizeRequest' }
      responses:
        '201':
          description: Transação criada (status AUTHORIZED ou DENIED — negativa não é erro HTTP)
          content: { application/json: { schema: { $ref: '#/components/schemas/Transaction' } } }
        '400': { $ref: '#/components/responses/BadRequest' }
        '409': { $ref: '#/components/responses/Conflict' }
        '422': { $ref: '#/components/responses/BusinessError' }
        '503': { description: Emissor indisponível, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
  /transactions/{id}:
    get:
      operationId: getTransaction
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      responses:
        '200': { description: OK, content: { application/json: { schema: { $ref: '#/components/schemas/Transaction' } } } }
        '404': { $ref: '#/components/responses/NotFound' }
  /transactions/{id}/capture:
    post:
      operationId: captureTransaction
      parameters: [ { $ref: '#/components/parameters/Id' }, { $ref: '#/components/parameters/IdempotencyKey' } ]
      responses:
        '200': { description: Capturada, content: { application/json: { schema: { $ref: '#/components/schemas/Transaction' } } } }
        '409': { $ref: '#/components/responses/Conflict' }
  /transactions/{id}/cancel:
    post:
      operationId: cancelTransaction
      parameters: [ { $ref: '#/components/parameters/Id' }, { $ref: '#/components/parameters/IdempotencyKey' } ]
      responses:
        '200': { description: Cancelada, content: { application/json: { schema: { $ref: '#/components/schemas/Transaction' } } } }
        '409': { $ref: '#/components/responses/Conflict' }

  /receivables:
    get:
      operationId: listReceivables
      parameters:
        - { name: status, in: query, schema: { type: string, enum: [SCHEDULED, ANTICIPATED, SETTLED, CANCELED, CHARGEBACKED] } }
        - { name: from, in: query, schema: { type: string, format: date } }
        - { name: to, in: query, schema: { type: string, format: date } }
        - { name: limit, in: query, schema: { type: integer, default: 100, maximum: 500 } }
      responses:
        '200':
          description: Agenda
          content:
            application/json:
              schema:
                type: object
                properties:
                  data: { type: array, items: { $ref: '#/components/schemas/Receivable' } }
  /receivables/summary:
    get:
      operationId: receivablesSummary
      parameters:
        - { name: from, in: query, schema: { type: string, format: date } }
        - { name: to, in: query, schema: { type: string, format: date } }
      responses:
        '200': { description: Totais por dia e status }
  /settlements:
    get:
      operationId: listSettlements
      responses:
        '200': { description: Liquidações do EC }

  /anticipations/simulate:
    post:
      operationId: simulateAnticipation
      requestBody:
        required: true
        content: { application/json: { schema: { $ref: '#/components/schemas/AnticipationRequest' } } }
      responses:
        '200': { description: Simulação, content: { application/json: { schema: { $ref: '#/components/schemas/Anticipation' } } } }
        '422': { $ref: '#/components/responses/BusinessError' }
  /anticipations:
    post:
      operationId: requestAnticipation
      parameters: [ { $ref: '#/components/parameters/IdempotencyKey' } ]
      requestBody:
        required: true
        content: { application/json: { schema: { $ref: '#/components/schemas/AnticipationRequest' } } }
      responses:
        '201': { description: Contratada, content: { application/json: { schema: { $ref: '#/components/schemas/Anticipation' } } } }
        '409': { $ref: '#/components/responses/Conflict' }
        '422': { $ref: '#/components/responses/BusinessError' }
  /anticipations/{id}:
    get:
      operationId: getAnticipation
      parameters: [ { $ref: '#/components/parameters/Id' } ]
      responses:
        '200': { description: OK, content: { application/json: { schema: { $ref: '#/components/schemas/Anticipation' } } } }

components:
  securitySchemes:
    bearerAuth: { type: http, scheme: bearer, bearerFormat: JWT }
  parameters:
    Id: { name: id, in: path, required: true, schema: { type: string } }
    IdempotencyKey:
      name: Idempotency-Key
      in: header
      required: true
      description: UUID gerado pelo cliente. Repetir a mesma chave com o mesmo corpo devolve a mesma resposta.
      schema: { type: string, format: uuid }
  schemas:
    Problem:
      type: object
      properties:
        type: { type: string }
        title: { type: string }
        status: { type: integer }
        detail: { type: string }
        instance: { type: string }
        code: { type: string, description: Código estável para o cliente decidir }
    CreateMerchantRequest:
      type: object
      required: [document, legal_name, mcc, bank_account, fee_plan]
      properties:
        document: { type: string, example: "11.222.333/0001-81" }
        legal_name: { type: string }
        mcc: { type: string, example: "5462" }
        bank_account:
          type: object
          required: [code, branch, number, kind]
          properties:
            code: { type: string, example: "341" }
            branch: { type: string, example: "0001" }
            number: { type: string, example: "12345-6" }
            kind: { type: string, enum: [CHECKING, SAVINGS, PAYMENT] }
        fee_plan:
          type: object
          properties:
            debit_bps: { type: integer, example: 150 }
            credit_bps: { type: integer, example: 250 }
            credit_installments_bps: { type: object, additionalProperties: { type: integer }, example: { "2": 350, "12": 450 } }
            anticipation_bps_month: { type: integer, example: 200 }
        webhook_url: { type: string, format: uri }
    Merchant:
      type: object
      properties:
        id: { type: string }
        document: { type: string, description: Mascarado }
        legal_name: { type: string }
        mcc: { type: string }
        status: { type: string, enum: [UNDER_REVIEW, ACTIVE, BLOCKED] }
        webhook_url: { type: string }
        auto_anticipation: { type: boolean }
        created_at: { type: string, format: date-time }
    AuthorizeRequest:
      type: object
      required: [amount, product, installments, card_token]
      properties:
        amount: { type: integer, minimum: 1, description: Centavos, example: 120000 }
        product: { type: string, enum: [DEBIT, CREDIT] }
        installments: { type: integer, minimum: 1, maximum: 12 }
        card_token: { type: string, description: Token do vault; nunca o PAN }
    Transaction:
      type: object
      properties:
        id: { type: string }
        merchant_id: { type: string }
        amount: { type: integer }
        amount_formatted: { type: string, example: "R$ 1200,00" }
        product: { type: string }
        installments: { type: integer }
        status: { type: string, enum: [PENDING, AUTHORIZED, DENIED, CAPTURED, CANCELED, SETTLED, CHARGEBACKED] }
        authorization_code: { type: string }
        nsu: { type: string }
        response_code: { type: string, description: "00 aprovada; 51 saldo; 05 não honrar; 91 emissor indisponível" }
        card_brand: { type: string }
        card_last4: { type: string }
        created_at: { type: string, format: date-time }
        captured_at: { type: string, format: date-time }
        canceled_at: { type: string, format: date-time }
    Receivable:
      type: object
      properties:
        id: { type: string }
        transaction_id: { type: string }
        installment_no: { type: integer }
        installments: { type: integer }
        gross_amount: { type: integer }
        fee_amount: { type: integer }
        net_amount: { type: integer }
        due_date: { type: string, format: date }
        status: { type: string }
    AnticipationRequest:
      type: object
      required: [receivable_ids]
      properties:
        receivable_ids: { type: array, items: { type: string }, minItems: 1, maxItems: 500 }
    Anticipation:
      type: object
      properties:
        id: { type: string }
        status: { type: string, enum: [SIMULATED, CONFIRMED] }
        rate_bps_month: { type: integer }
        gross_amount: { type: integer }
        discount_amount: { type: integer }
        net_amount: { type: integer }
        items:
          type: array
          items:
            type: object
            properties:
              receivable_id: { type: string }
              due_date: { type: string, format: date }
              days: { type: integer }
              net_amount: { type: integer }
              present_value: { type: integer }
              discount: { type: integer }
  responses:
    BadRequest: { description: Requisição malformada, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
    Forbidden: { description: Sem permissão, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
    NotFound: { description: Não encontrado, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
    Conflict: { description: Estado ou idempotência em conflito, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
    BusinessError: { description: Regra de negócio impediu, content: { application/problem+json: { schema: { $ref: '#/components/schemas/Problem' } } } }
```

#### `api/proto/issuer/v1/issuer.proto`

```protobuf
syntax = "proto3";

package issuer.v1;

option go_package = "github.com/seu-usuario/adquirente/gen/issuer/v1;issuerv1";

// IssuerService representa o emissor (via bandeira). No estudo é simulado;
// em produção este contrato seria substituído por um adapter ISO 8583.
service IssuerService {
  rpc Authorize(AuthorizeRequest) returns (AuthorizeResponse);
  rpc Reverse(ReverseRequest) returns (ReverseResponse);
}

enum Product {
  PRODUCT_UNSPECIFIED = 0;
  PRODUCT_DEBIT = 1;
  PRODUCT_CREDIT = 2;
}

message AuthorizeRequest {
  string transaction_id = 1;
  string pan = 2;            // em claro só neste canal (mTLS), nunca persistido
  string expiry = 3;         // "MM/YY"
  int64 amount = 4;          // centavos
  string currency = 5;       // "BRL"
  Product product = 6;
  int32 installments = 7;
  string merchant_mcc = 8;
}

message AuthorizeResponse {
  bool approved = 1;
  string authorization_code = 2;
  string response_code = 3;  // "00" aprovada; "51" saldo; "05" não honrar; "91" indisponível
}

message ReverseRequest {
  string transaction_id = 1;
}

message ReverseResponse {
  bool accepted = 1;
}
```

#### `api/proto/vault/v1/vault.proto`

Repare: **não existe campo CVV** neste contrato. Se não existe no contrato, não tem como alguém mandar.

```protobuf
syntax = "proto3";

package vault.v1;

option go_package = "github.com/seu-usuario/adquirente/gen/vault/v1;vaultv1";

// VaultService é o cofre de cartões: o ÚNICO componente que vê o PAN.
// Tokenize guarda cifrado e devolve um token aleatório; Detokenize é restrito ao autorizador.
service VaultService {
  rpc Tokenize(TokenizeRequest) returns (TokenizeResponse);
  rpc Detokenize(DetokenizeRequest) returns (DetokenizeResponse);
}

message TokenizeRequest {
  string pan = 1;
  string expiry = 2;  // "MM/YY"
  // CVV NÃO existe neste contrato de propósito: nunca pode ser armazenado (PCI DSS).
}

message TokenizeResponse {
  string token = 1;
  string brand = 2;
  string bin = 3;
  string last4 = 4;
}

message DetokenizeRequest {
  string token = 1;
}

message DetokenizeResponse {
  string pan = 1;
  string expiry = 2;
}
```

#### `buf.yaml` e `buf.gen.yaml`

```yaml
version: v2
modules:
  - path: api/proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

```yaml
version: v2
plugins:
  - local: protoc-gen-go
    out: gen
    opt: paths=source_relative
  - local: protoc-gen-go-grpc
    out: gen
    opt: paths=source_relative
```

**Gere o código:**

```bash
buf lint && buf generate
ls gen/issuer/v1 gen/vault/v1     # issuer.pb.go issuer_grpc.pb.go vault.pb.go vault_grpc.pb.go
go get google.golang.org/grpc@latest google.golang.org/protobuf@latest
go build ./gen/...
```

A pasta `gen/` está no `.gitignore`: código gerado se regenera no CI.

---

## Fase 4 — Banco de dados

**Objetivo:** migrações e repositórios. Releia a Parte 8. Vamos usar `database/sql` **puro** no núcleo financeiro (para você ver cada SQL, cada lock) e GORM no cadastro (para sentir o que um ORM faz por você e o que ele esconde).

```bash
go get github.com/jackc/pgx/v5@latest gorm.io/gorm@latest gorm.io/driver/postgres@latest
```

### 4.1 Migrações

#### `migrations/000001_merchants.up.sql` e `.down.sql`

```sql
-- Estabelecimentos comerciais (ECs). O plano de taxas fica em JSONB: muda pouco, é lido inteiro.
CREATE TABLE merchants (
    id                 TEXT PRIMARY KEY,
    document           TEXT NOT NULL UNIQUE,          -- CNPJ (só dígitos/letras, validado no domínio)
    legal_name         TEXT NOT NULL,
    mcc                TEXT NOT NULL,
    status             TEXT NOT NULL CHECK (status IN ('UNDER_REVIEW', 'ACTIVE', 'BLOCKED')),
    bank_code          TEXT NOT NULL,
    bank_branch        TEXT NOT NULL,
    bank_account       TEXT NOT NULL,
    bank_account_kind  TEXT NOT NULL,
    fee_plan           JSONB NOT NULL,
    webhook_url        TEXT NOT NULL DEFAULT '',
    auto_anticipation  BOOLEAN NOT NULL DEFAULT FALSE,
    version            INTEGER NOT NULL DEFAULT 1,     -- lock otimista
    created_at         TIMESTAMPTZ NOT NULL,
    updated_at         TIMESTAMPTZ NOT NULL
);

CREATE INDEX merchants_status_idx ON merchants (status);
```

```sql
DROP TABLE IF EXISTS merchants;
```

#### `migrations/000002_transactions.up.sql` e `.down.sql`

Leia a lista de colunas procurando "pan" ou "cvv". Não existe. É assim que se cumpre o PCI (Parte 2.7).

```sql
-- Transações de cartão. Repare: NÃO existe coluna de PAN nem de CVV. Só token, BIN e últimos 4.
CREATE TABLE transactions (
    id                  TEXT PRIMARY KEY,
    merchant_id         TEXT NOT NULL REFERENCES merchants (id),
    amount              BIGINT NOT NULL CHECK (amount > 0),   -- centavos
    product             TEXT NOT NULL CHECK (product IN ('DEBIT', 'CREDIT')),
    installments        INTEGER NOT NULL CHECK (installments BETWEEN 1 AND 12),
    status              TEXT NOT NULL,
    card_token          TEXT NOT NULL,
    card_brand          TEXT NOT NULL,
    card_bin            TEXT NOT NULL,
    card_last4          TEXT NOT NULL,
    authorization_code  TEXT NOT NULL DEFAULT '',
    nsu                 TEXT NOT NULL DEFAULT '',
    response_code       TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    authorized_at       TIMESTAMPTZ,
    captured_at         TIMESTAMPTZ,
    canceled_at         TIMESTAMPTZ,
    version             INTEGER NOT NULL DEFAULT 1
);

CREATE INDEX transactions_merchant_created_idx ON transactions (merchant_id, created_at DESC);
CREATE INDEX transactions_status_idx ON transactions (status);

-- NSU: Número Sequencial Único, gerado pelo banco para nunca repetir mesmo com N réplicas da API.
CREATE SEQUENCE nsu_seq START 1;
```

```sql
DROP SEQUENCE IF EXISTS nsu_seq;
DROP TABLE IF EXISTS transactions;
```

#### `migrations/000003_receivables.up.sql` e `.down.sql`

Os índices são exatamente as consultas que o sistema faz (Parte 8.2).

```sql
-- A agenda de recebíveis: uma linha por parcela.
CREATE TABLE receivables (
    id               TEXT PRIMARY KEY,
    transaction_id   TEXT NOT NULL REFERENCES transactions (id),
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    installment_no   INTEGER NOT NULL,
    installments     INTEGER NOT NULL,
    product          TEXT NOT NULL,
    brand            TEXT NOT NULL,
    gross_amount     BIGINT NOT NULL,
    fee_amount       BIGINT NOT NULL,
    net_amount       BIGINT NOT NULL,
    due_date         DATE NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('SCHEDULED', 'ANTICIPATED', 'SETTLED', 'CANCELED', 'CHARGEBACKED')),
    settlement_id    TEXT NOT NULL DEFAULT '',
    anticipation_id  TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL
);

-- As consultas que o sistema faz de verdade:
CREATE INDEX receivables_due_status_idx      ON receivables (due_date, status);            -- worker de liquidação
CREATE INDEX receivables_merchant_due_idx    ON receivables (merchant_id, due_date);       -- agenda do EC
CREATE INDEX receivables_transaction_idx     ON receivables (transaction_id);              -- cancelamento/chargeback
```

```sql
DROP TABLE IF EXISTS receivables;
```

#### `migrations/000004_settlements_anticipations_chargebacks.up.sql` e `.down.sql`

```sql
-- Lotes de liquidação (um por EC por dia). Ordens e ids em JSONB: são lidos sempre inteiros.
CREATE TABLE settlements (
    id               TEXT PRIMARY KEY,
    settlement_date  DATE NOT NULL,
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    total_amount     BIGINT NOT NULL,
    to_merchant      BIGINT NOT NULL,
    to_creditors     BIGINT NOT NULL,
    retained         BIGINT NOT NULL,
    orders           JSONB NOT NULL,
    receivable_ids   JSONB NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('PENDING', 'SENT', 'CONFIRMED', 'FAILED')),
    external_ref     TEXT NOT NULL DEFAULT '',
    failure_reason   TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    created_at       TIMESTAMPTZ NOT NULL,
    updated_at       TIMESTAMPTZ NOT NULL
);
CREATE INDEX settlements_date_idx     ON settlements (settlement_date);
CREATE INDEX settlements_merchant_idx ON settlements (merchant_id, settlement_date DESC);

CREATE TABLE anticipations (
    id               TEXT PRIMARY KEY,
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    items            JSONB NOT NULL,
    gross_amount     BIGINT NOT NULL,
    discount_amount  BIGINT NOT NULL,
    net_amount       BIGINT NOT NULL,
    rate_bps_month   INTEGER NOT NULL,
    requested_at     TIMESTAMPTZ NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('SIMULATED', 'CONFIRMED')),
    version          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX anticipations_merchant_idx ON anticipations (merchant_id, requested_at DESC);

CREATE TABLE chargebacks (
    id               TEXT PRIMARY KEY,
    transaction_id   TEXT NOT NULL REFERENCES transactions (id),
    merchant_id      TEXT NOT NULL REFERENCES merchants (id),
    reason_code      TEXT NOT NULL,
    amount           BIGINT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('OPENED', 'DEFENDED', 'ACCEPTED', 'REVERSED')),
    opened_at        TIMESTAMPTZ NOT NULL,
    deadline         TIMESTAMPTZ NOT NULL,
    resolved_at      TIMESTAMPTZ,
    version          INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX chargebacks_transaction_idx ON chargebacks (transaction_id);
```

```sql
DROP TABLE IF EXISTS chargebacks;
DROP TABLE IF EXISTS anticipations;
DROP TABLE IF EXISTS settlements;
```

#### `migrations/000005_ledger_outbox_processed.up.sql` e `.down.sql`

Três tabelas append-only: o razão, a outbox e os eventos processados.

```sql
-- Razão contábil: APPEND-ONLY. Nunca UPDATE, nunca DELETE. Débito OU crédito por linha.
CREATE TABLE ledger_entries (
    id              TEXT PRIMARY KEY,
    account         TEXT NOT NULL,
    merchant_id     TEXT NOT NULL,
    debit           BIGINT NOT NULL DEFAULT 0 CHECK (debit >= 0),
    credit          BIGINT NOT NULL DEFAULT 0 CHECK (credit >= 0),
    reference_type  TEXT NOT NULL,
    reference_id    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    CHECK (debit = 0 OR credit = 0)
);
CREATE INDEX ledger_account_merchant_idx ON ledger_entries (account, merchant_id);
CREATE INDEX ledger_reference_idx        ON ledger_entries (reference_type, reference_id);

-- Outbox: eventos gravados na MESMA transação do dado; o relay publica no Kafka depois.
CREATE TABLE outbox (
    id            BIGSERIAL PRIMARY KEY,           -- ordem de publicação
    event_id      TEXT NOT NULL UNIQUE,
    aggregate_id  TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    payload       JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at  TIMESTAMPTZ
);
CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

-- Consumidores idempotentes: (consumidor, evento) só processa uma vez.
CREATE TABLE processed_events (
    consumer      TEXT NOT NULL,
    event_id      TEXT NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (consumer, event_id)
);
```

```sql
DROP TABLE IF EXISTS processed_events;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS ledger_entries;
```

#### `migrations/000006_idempotency_vault.up.sql` e `.down.sql`

```sql
-- Idempotência: uma chave por EC. A resposta é guardada para ser repetida.
CREATE TABLE idempotency_keys (
    merchant_id      TEXT NOT NULL,
    idempotency_key  TEXT NOT NULL,
    request_hash     TEXT NOT NULL,                  -- SHA-256 do corpo: mesmo key + corpo diferente = 409
    response_status  INTEGER,                        -- NULL enquanto a requisição original ainda roda
    response_body    BYTEA,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (merchant_id, idempotency_key)
);
CREATE INDEX idempotency_expires_idx ON idempotency_keys (expires_at);

-- Cofre de cartões. É a ÚNICA tabela com PAN, e mesmo assim cifrado (AES-256-GCM).
-- Em produção vive em outro banco, em outra rede, acessível só pelo serviço vault.
CREATE TABLE card_vault (
    token              TEXT PRIMARY KEY,
    pan_ciphertext     BYTEA NOT NULL,
    expiry_ciphertext  BYTEA NOT NULL,
    brand              TEXT NOT NULL,
    bin                TEXT NOT NULL,
    last4              TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```sql
DROP TABLE IF EXISTS card_vault;
DROP TABLE IF EXISTS idempotency_keys;
```

```bash
docker compose up -d postgres && sleep 5 && make migrate-up
```

### 4.2 Repositórios com `database/sql`

#### `internal/infrastructure/postgres/db.go`

Conexão com pool e espera pelo banco. `DBTX` é a interface que permite o mesmo repositório rodar com ou sem transação.

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registra o driver "pgx" no database/sql
)

// DBTX é o que *sql.DB e *sql.Tx têm em comum. Os repositórios recebem DBTX:
// fora de transação usam o pool; dentro da Unit of Work usam a transação.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Connect abre o pool e espera o banco responder (útil no Docker Compose, onde a API
// pode subir antes do Postgres).
func Connect(ctx context.Context, url string) (*sql.DB, error) {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return nil, fmt.Errorf("abrir conexão: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	for attempt := 1; ; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		if attempt >= 10 {
			_ = db.Close()
			return nil, fmt.Errorf("banco não respondeu após %d tentativas: %w", attempt, err)
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
}
```

#### `internal/infrastructure/postgres/uow.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/seu-usuario/adquirente/internal/application"
)

// UnitOfWork implementa application.UnitOfWork com uma transação SQL.
type UnitOfWork struct{ db *sql.DB }

func NewUnitOfWork(db *sql.DB) *UnitOfWork { return &UnitOfWork{db: db} }

func (u *UnitOfWork) Do(ctx context.Context, fn func(repos application.Repositories) error) error {
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("iniciar transação: %w", err)
	}
	// Rollback depois de Commit é inofensivo (devolve ErrTxDone). Garante o "nada" do "tudo ou nada".
	defer func() { _ = tx.Rollback() }()

	if err := fn(NewRepositories(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("confirmar transação: %w", err)
	}
	return nil
}

// NewRepositories monta todos os repositórios sobre a mesma conexão/transação.
// Também é usado com *sql.DB para leituras fora de transação.
func NewRepositories(db DBTX) application.Repositories {
	return application.Repositories{
		Transactions:  NewTransactionRepository(db),
		Receivables:   NewReceivableRepository(db),
		Settlements:   NewSettlementRepository(db),
		Anticipations: NewAnticipationRepository(db),
		Chargebacks:   NewChargebackRepository(db),
		Ledger:        NewLedgerRepository(db),
		Outbox:        NewOutboxRepository(db),
		Processed:     NewProcessedEventsRepository(db),
		NSU:           NewNSUGenerator(db),
	}
}
```

#### `internal/infrastructure/postgres/transaction_repository.go`

Repare no `FOR UPDATE` e na verificação de versão no `UPDATE` (Parte 8.3). E no `NSUGenerator` com `SEQUENCE`.

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

type TransactionRepository struct{ db DBTX }

func NewTransactionRepository(db DBTX) *TransactionRepository { return &TransactionRepository{db: db} }

// Garante em tempo de compilação que implementamos o port.
var _ transaction.Repository = (*TransactionRepository)(nil)

const txColumns = `id, merchant_id, amount, product, installments, status, card_token, card_brand, card_bin, card_last4,
	authorization_code, nsu, response_code, created_at, authorized_at, captured_at, canceled_at, version`

func (r *TransactionRepository) Create(ctx context.Context, t *transaction.Transaction) error {
	c := t.Card()
	_, err := r.db.ExecContext(ctx, `INSERT INTO transactions (`+txColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		t.ID(), t.MerchantID(), int64(t.Amount()), string(t.Product()), t.Installments(), string(t.Status()),
		c.Token, c.Brand, c.BIN, c.Last4, t.AuthorizationCode(), t.NSU(), t.ResponseCode(),
		t.CreatedAt(), t.AuthorizedAt(), t.CapturedAt(), t.CanceledAt(), t.Version())
	if err != nil {
		return fmt.Errorf("inserir transação: %w", err)
	}
	return nil
}

func (r *TransactionRepository) FindByID(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.findOne(ctx, `SELECT `+txColumns+` FROM transactions WHERE id = $1 AND merchant_id = $2`, id, merchantID)
}

// FindByIDForUpdate trava a linha até o fim da transação (lock pessimista).
func (r *TransactionRepository) FindByIDForUpdate(ctx context.Context, merchantID, id string) (*transaction.Transaction, error) {
	return r.findOne(ctx, `SELECT `+txColumns+` FROM transactions WHERE id = $1 AND merchant_id = $2 FOR UPDATE`, id, merchantID)
}

// Update grava com verificação de versão: a entidade incrementa version a cada transição;
// se o banco já tem uma versão igual ou maior, alguém alterou antes → ErrConcurrentUpdate.
func (r *TransactionRepository) Update(ctx context.Context, t *transaction.Transaction) error {
	res, err := r.db.ExecContext(ctx, `UPDATE transactions SET status = $2, authorization_code = $3, nsu = $4, response_code = $5,
		authorized_at = $6, captured_at = $7, canceled_at = $8, version = $9
		WHERE id = $1 AND version < $9`,
		t.ID(), string(t.Status()), t.AuthorizationCode(), t.NSU(), t.ResponseCode(),
		t.AuthorizedAt(), t.CapturedAt(), t.CanceledAt(), t.Version())
	if err != nil {
		return fmt.Errorf("atualizar transação: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *TransactionRepository) findOne(ctx context.Context, query string, args ...any) (*transaction.Transaction, error) {
	var (
		id, merchantID, product, status, token, brand, bin, last4, authCode, nsu, respCode string
		amount                                                                             int64
		installments, version                                                              int
		createdAt                                                                          time.Time
		authorizedAt, capturedAt, canceledAt                                               sql.NullTime
	)
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&id, &merchantID, &amount, &product, &installments, &status,
		&token, &brand, &bin, &last4, &authCode, &nsu, &respCode, &createdAt, &authorizedAt, &capturedAt, &canceledAt, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar transação: %w", err)
	}
	return transaction.Restore(id, merchantID, shared.Money(amount), shared.Product(product), installments,
		transaction.CardInfo{Token: token, Brand: brand, BIN: bin, Last4: last4},
		transaction.Status(status), authCode, nsu, respCode, createdAt,
		nullTime(authorizedAt), nullTime(capturedAt), nullTime(canceledAt), version), nil
}

func nullTime(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// NSUGenerator usa uma SEQUENCE do banco: único mesmo com várias réplicas da API.
type NSUGenerator struct{ db DBTX }

func NewNSUGenerator(db DBTX) *NSUGenerator { return &NSUGenerator{db: db} }

func (g *NSUGenerator) Next(ctx context.Context) (string, error) {
	var n int64
	if err := g.db.QueryRowContext(ctx, `SELECT nextval('nsu_seq')`).Scan(&n); err != nil {
		return "", fmt.Errorf("gerar NSU: %w", err)
	}
	return fmt.Sprintf("%012d", n), nil
}
```

#### `internal/infrastructure/postgres/receivable_repository.go`

`FOR UPDATE SKIP LOCKED` para o worker; `ORDER BY id FOR UPDATE` para evitar deadlock; filtros sempre com parâmetros (Parte 9.5).

```go
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type ReceivableRepository struct{ db DBTX }

func NewReceivableRepository(db DBTX) *ReceivableRepository { return &ReceivableRepository{db: db} }

var _ receivable.Repository = (*ReceivableRepository)(nil)

const rcvColumns = `id, transaction_id, merchant_id, installment_no, installments, product, brand, gross_amount, fee_amount,
	net_amount, due_date, status, settlement_id, anticipation_id, version, created_at, updated_at`

func (r *ReceivableRepository) SaveAll(ctx context.Context, list []*receivable.Receivable) error {
	for _, x := range list {
		_, err := r.db.ExecContext(ctx, `INSERT INTO receivables (`+rcvColumns+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
			x.ID(), x.TransactionID(), x.MerchantID(), x.InstallmentNo(), x.Installments(), string(x.Product()), x.Brand(),
			int64(x.Gross()), int64(x.Fee()), int64(x.Net()), x.DueDate(), string(x.Status()), x.SettlementID(), x.AnticipationID(),
			x.Version(), x.CreatedAt(), x.UpdatedAt())
		if err != nil {
			return fmt.Errorf("inserir recebível %s: %w", x.ID(), err)
		}
	}
	return nil
}

func (r *ReceivableRepository) ListByMerchant(ctx context.Context, merchantID string, f receivable.ListFilter) ([]*receivable.Receivable, error) {
	// SQL montado com PARÂMETROS ($n), nunca com concatenação de valores: sem SQL injection.
	var where []string
	args := []any{merchantID}
	where = append(where, "merchant_id = $1")
	if f.Status != "" {
		args = append(args, string(f.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if !f.From.IsZero() {
		args = append(args, shared.DateOnly(f.From))
		where = append(where, fmt.Sprintf("due_date >= $%d", len(args)))
	}
	if !f.To.IsZero() {
		args = append(args, shared.DateOnly(f.To))
		where = append(where, fmt.Sprintf("due_date <= $%d", len(args)))
	}
	args = append(args, f.Limit)
	query := `SELECT ` + rcvColumns + ` FROM receivables WHERE ` + strings.Join(where, " AND ") +
		fmt.Sprintf(" ORDER BY due_date, installment_no LIMIT $%d", len(args))
	return r.list(ctx, query, args...)
}

func (r *ReceivableRepository) ListByTransaction(ctx context.Context, transactionID string) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE transaction_id = $1 ORDER BY installment_no FOR UPDATE`, transactionID)
}

// ListDueOnForUpdate: SKIP LOCKED faz vários workers pegarem linhas diferentes sem esperar um pelo outro.
func (r *ReceivableRepository) ListDueOnForUpdate(ctx context.Context, day time.Time, limit int) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables
		WHERE due_date = $1 AND status IN ('SCHEDULED', 'ANTICIPATED')
		ORDER BY merchant_id, id LIMIT $2 FOR UPDATE SKIP LOCKED`, shared.DateOnly(day), limit)
}

func (r *ReceivableRepository) FindByIDs(ctx context.Context, merchantID string, ids []string) ([]*receivable.Receivable, error) {
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE merchant_id = $1 AND id = ANY($2) ORDER BY due_date, id`, merchantID, ids)
}

func (r *ReceivableRepository) FindForUpdate(ctx context.Context, merchantID string, ids []string) ([]*receivable.Receivable, error) {
	// ORDER BY id: travar sempre na mesma ordem evita deadlock entre dois pedidos concorrentes.
	return r.list(ctx, `SELECT `+rcvColumns+` FROM receivables WHERE merchant_id = $1 AND id = ANY($2) ORDER BY id FOR UPDATE`, merchantID, ids)
}

func (r *ReceivableRepository) UpdateAll(ctx context.Context, list []*receivable.Receivable) error {
	for _, x := range list {
		res, err := r.db.ExecContext(ctx, `UPDATE receivables SET status = $2, settlement_id = $3, anticipation_id = $4, version = $5, updated_at = $6
			WHERE id = $1 AND version < $5`,
			x.ID(), string(x.Status()), x.SettlementID(), x.AnticipationID(), x.Version(), x.UpdatedAt())
		if err != nil {
			return fmt.Errorf("atualizar recebível %s: %w", x.ID(), err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return shared.ErrConcurrentUpdate
		}
	}
	return nil
}

func (r *ReceivableRepository) Summary(ctx context.Context, merchantID string, from, to time.Time) ([]receivable.DailySummary, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT due_date, status, COUNT(*), COALESCE(SUM(net_amount), 0)
		FROM receivables WHERE merchant_id = $1 AND due_date BETWEEN $2 AND $3
		GROUP BY due_date, status ORDER BY due_date, status`, merchantID, shared.DateOnly(from), shared.DateOnly(to))
	if err != nil {
		return nil, fmt.Errorf("resumo da agenda: %w", err)
	}
	defer rows.Close()
	var out []receivable.DailySummary
	for rows.Next() {
		var s receivable.DailySummary
		var status string
		var net int64
		if err := rows.Scan(&s.DueDate, &status, &s.Count, &net); err != nil {
			return nil, err
		}
		s.Status, s.Net = receivable.Status(status), shared.Money(net)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ReceivableRepository) list(ctx context.Context, query string, args ...any) ([]*receivable.Receivable, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar recebíveis: %w", err)
	}
	defer rows.Close()
	var out []*receivable.Receivable
	for rows.Next() {
		x, err := scanReceivable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func scanReceivable(rows *sql.Rows) (*receivable.Receivable, error) {
	var (
		id, txID, merchantID, product, brand, status, settlementID, anticipationID string
		installmentNo, installments, version                                       int
		gross, fee, net                                                            int64
		dueDate, createdAt, updatedAt                                              time.Time
	)
	if err := rows.Scan(&id, &txID, &merchantID, &installmentNo, &installments, &product, &brand, &gross, &fee, &net,
		&dueDate, &status, &settlementID, &anticipationID, &version, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("ler recebível: %w", err)
	}
	return receivable.Restore(id, txID, merchantID, installmentNo, installments, shared.Product(product), brand,
		shared.Money(gross), shared.Money(fee), shared.Money(net), shared.DateOnly(dueDate), receivable.Status(status),
		settlementID, anticipationID, version, createdAt, updatedAt), nil
}
```

#### `internal/infrastructure/postgres/settlement_repository.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type SettlementRepository struct{ db DBTX }

func NewSettlementRepository(db DBTX) *SettlementRepository { return &SettlementRepository{db: db} }

var _ settlement.Repository = (*SettlementRepository)(nil)

const stlColumns = `id, settlement_date, merchant_id, total_amount, to_merchant, to_creditors, retained, orders, receivable_ids,
	status, external_ref, failure_reason, version, created_at, updated_at`

func (r *SettlementRepository) Create(ctx context.Context, s *settlement.Settlement) error {
	orders, _ := json.Marshal(s.Orders())
	ids, _ := json.Marshal(s.ReceivableIDs())
	_, err := r.db.ExecContext(ctx, `INSERT INTO settlements (`+stlColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID(), s.Date(), s.MerchantID(), int64(s.Total()), int64(s.ToMerchant()), int64(s.ToCreditors()), int64(s.Retained()),
		orders, ids, string(s.Status()), s.ExternalRef(), s.FailureReason(), s.Version(), s.CreatedAt(), s.UpdatedAt())
	if err != nil {
		return fmt.Errorf("inserir liquidação: %w", err)
	}
	return nil
}

func (r *SettlementRepository) Update(ctx context.Context, s *settlement.Settlement) error {
	res, err := r.db.ExecContext(ctx, `UPDATE settlements SET status = $2, external_ref = $3, failure_reason = $4, version = $5, updated_at = $6
		WHERE id = $1 AND version < $5`, s.ID(), string(s.Status()), s.ExternalRef(), s.FailureReason(), s.Version(), s.UpdatedAt())
	if err != nil {
		return fmt.Errorf("atualizar liquidação: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *SettlementRepository) FindByID(ctx context.Context, merchantID, id string) (*settlement.Settlement, error) {
	list, err := r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE id = $1 AND merchant_id = $2`, id, merchantID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, shared.ErrNotFound
	}
	return list[0], nil
}

func (r *SettlementRepository) ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*settlement.Settlement, error) {
	return r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE merchant_id = $1 ORDER BY settlement_date DESC LIMIT $2`, merchantID, limit)
}

func (r *SettlementRepository) ListByDate(ctx context.Context, day time.Time) ([]*settlement.Settlement, error) {
	return r.list(ctx, `SELECT `+stlColumns+` FROM settlements WHERE settlement_date = $1 ORDER BY merchant_id`, shared.DateOnly(day))
}

func (r *SettlementRepository) list(ctx context.Context, query string, args ...any) ([]*settlement.Settlement, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar liquidações: %w", err)
	}
	defer rows.Close()
	var out []*settlement.Settlement
	for rows.Next() {
		var (
			id, merchantID, status, externalRef, failureReason string
			total, toMerchant, toCreditors, retained           int64
			ordersJSON, idsJSON                                []byte
			version                                            int
			date, createdAt, updatedAt                         time.Time
		)
		if err := rows.Scan(&id, &date, &merchantID, &total, &toMerchant, &toCreditors, &retained, &ordersJSON, &idsJSON,
			&status, &externalRef, &failureReason, &version, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("ler liquidação: %w", err)
		}
		var orders []settlement.Order
		var ids []string
		if err := json.Unmarshal(ordersJSON, &orders); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(idsJSON, &ids); err != nil {
			return nil, err
		}
		out = append(out, settlement.Restore(id, shared.DateOnly(date), merchantID, shared.Money(total), shared.Money(toMerchant),
			shared.Money(toCreditors), shared.Money(retained), orders, ids, settlement.Status(status), externalRef, failureReason,
			createdAt, updatedAt, version))
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return out, nil
}
```

#### `internal/infrastructure/postgres/anticipation_repository.go`

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type AnticipationRepository struct{ db DBTX }

func NewAnticipationRepository(db DBTX) *AnticipationRepository {
	return &AnticipationRepository{db: db}
}

var _ anticipation.Repository = (*AnticipationRepository)(nil)

const antColumns = `id, merchant_id, items, gross_amount, discount_amount, net_amount, rate_bps_month, requested_at, status, version`

func (r *AnticipationRepository) Create(ctx context.Context, a *anticipation.Anticipation) error {
	items, _ := json.Marshal(a.Items())
	_, err := r.db.ExecContext(ctx, `INSERT INTO anticipations (`+antColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		a.ID(), a.MerchantID(), items, int64(a.Gross()), int64(a.Discount()), int64(a.Net()), int(a.RateMonth()), a.RequestedAt(), string(a.Status()), a.Version())
	if err != nil {
		return fmt.Errorf("inserir antecipação: %w", err)
	}
	return nil
}

func (r *AnticipationRepository) FindByID(ctx context.Context, merchantID, id string) (*anticipation.Anticipation, error) {
	list, err := r.list(ctx, `SELECT `+antColumns+` FROM anticipations WHERE id = $1 AND merchant_id = $2`, id, merchantID)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, shared.ErrNotFound
	}
	return list[0], nil
}

func (r *AnticipationRepository) ListByMerchant(ctx context.Context, merchantID string, limit int) ([]*anticipation.Anticipation, error) {
	return r.list(ctx, `SELECT `+antColumns+` FROM anticipations WHERE merchant_id = $1 ORDER BY requested_at DESC LIMIT $2`, merchantID, limit)
}

func (r *AnticipationRepository) list(ctx context.Context, query string, args ...any) ([]*anticipation.Anticipation, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listar antecipações: %w", err)
	}
	defer rows.Close()
	var out []*anticipation.Anticipation
	for rows.Next() {
		var (
			id, merchantID, status string
			itemsJSON              []byte
			gross, discount, net   int64
			rate, version          int
			requestedAt            time.Time
		)
		if err := rows.Scan(&id, &merchantID, &itemsJSON, &gross, &discount, &net, &rate, &requestedAt, &status, &version); err != nil {
			return nil, fmt.Errorf("ler antecipação: %w", err)
		}
		var items []anticipation.Item
		if err := json.Unmarshal(itemsJSON, &items); err != nil {
			return nil, err
		}
		out = append(out, anticipation.Restore(id, merchantID, items, shared.Money(gross), shared.Money(discount), shared.Money(net),
			shared.Bps(rate), requestedAt, anticipation.Status(status), version))
	}
	return out, rows.Err()
}
```

#### `internal/infrastructure/postgres/chargeback_repository.go`

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/chargeback"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type ChargebackRepository struct{ db DBTX }

func NewChargebackRepository(db DBTX) *ChargebackRepository { return &ChargebackRepository{db: db} }

var _ chargeback.Repository = (*ChargebackRepository)(nil)

func (r *ChargebackRepository) Create(ctx context.Context, c *chargeback.Chargeback) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO chargebacks (id, transaction_id, merchant_id, reason_code, amount, status, opened_at, deadline, resolved_at, version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		c.ID(), c.TransactionID(), c.MerchantID(), c.ReasonCode(), int64(c.Amount()), string(c.Status()), c.OpenedAt(), c.Deadline(), c.ResolvedAt(), c.Version())
	if err != nil {
		return fmt.Errorf("inserir chargeback: %w", err)
	}
	return nil
}

func (r *ChargebackRepository) Update(ctx context.Context, c *chargeback.Chargeback) error {
	res, err := r.db.ExecContext(ctx, `UPDATE chargebacks SET status = $2, resolved_at = $3, version = $4 WHERE id = $1 AND version < $4`,
		c.ID(), string(c.Status()), c.ResolvedAt(), c.Version())
	if err != nil {
		return fmt.Errorf("atualizar chargeback: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func (r *ChargebackRepository) FindByID(ctx context.Context, merchantID, id string) (*chargeback.Chargeback, error) {
	var (
		cid, txID, mid, reason, status string
		amount                         int64
		openedAt, deadline             time.Time
		resolvedAt                     sql.NullTime
		version                        int
	)
	err := r.db.QueryRowContext(ctx, `SELECT id, transaction_id, merchant_id, reason_code, amount, status, opened_at, deadline, resolved_at, version
		FROM chargebacks WHERE id = $1 AND merchant_id = $2`, id, merchantID).
		Scan(&cid, &txID, &mid, &reason, &amount, &status, &openedAt, &deadline, &resolvedAt, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar chargeback: %w", err)
	}
	return chargeback.Restore(cid, txID, mid, reason, shared.Money(amount), chargeback.Status(status), openedAt, deadline, nullTime(resolvedAt), version), nil
}
```

#### `internal/infrastructure/postgres/ledger_repository.go`

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/seu-usuario/adquirente/internal/domain/ledger"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type LedgerRepository struct{ db DBTX }

func NewLedgerRepository(db DBTX) *LedgerRepository { return &LedgerRepository{db: db} }

var _ ledger.Repository = (*LedgerRepository)(nil)

// Append recusa lotes que não fecham em zero: a invariante contábil vale também na borda.
func (r *LedgerRepository) Append(ctx context.Context, entries []ledger.Entry) error {
	if !ledger.Balanced(entries) {
		return fmt.Errorf("lote de lançamentos desbalanceado")
	}
	for _, e := range entries {
		_, err := r.db.ExecContext(ctx, `INSERT INTO ledger_entries (id, account, merchant_id, debit, credit, reference_type, reference_id, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			e.ID, string(e.Account), e.MerchantID, int64(e.Debit), int64(e.Credit), e.RefType, e.RefID, e.At)
		if err != nil {
			return fmt.Errorf("inserir lançamento: %w", err)
		}
	}
	return nil
}

// Balance: créditos − débitos. Para payable_to_merchant, é "quanto devemos ao EC agora".
func (r *LedgerRepository) Balance(ctx context.Context, account ledger.Account, merchantID string) (shared.Money, error) {
	var v int64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(credit) - SUM(debit), 0) FROM ledger_entries
		WHERE account = $1 AND ($2 = '' OR merchant_id = $2)`, string(account), merchantID).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("saldo contábil: %w", err)
	}
	return shared.Money(v), nil
}
```

#### `internal/infrastructure/postgres/outbox_repository.go`

Outbox e eventos processados (Parte 12.4 e 12.5).

```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

type OutboxRepository struct{ db DBTX }

func NewOutboxRepository(db DBTX) *OutboxRepository { return &OutboxRepository{db: db} }

var _ application.OutboxRepository = (*OutboxRepository)(nil)

// Append serializa cada evento em JSON. Como todo evento embute shared.BaseEvent com tags json,
// o payload sai com event_id, event_type, occurred_at, aggregate_id, version e os campos próprios.
func (r *OutboxRepository) Append(ctx context.Context, events ...shared.Event) error {
	for _, ev := range events {
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("serializar evento %s: %w", ev.EventType(), err)
		}
		_, err = r.db.ExecContext(ctx, `INSERT INTO outbox (event_id, aggregate_id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4, $5)`, ev.EventID(), ev.AggregateID(), ev.EventType(), payload, ev.OccurredAt())
		if err != nil {
			return fmt.Errorf("gravar na outbox: %w", err)
		}
	}
	return nil
}

type ProcessedEventsRepository struct{ db DBTX }

func NewProcessedEventsRepository(db DBTX) *ProcessedEventsRepository {
	return &ProcessedEventsRepository{db: db}
}

var _ application.ProcessedEventsRepository = (*ProcessedEventsRepository)(nil)

// MarkIfNew: ON CONFLICT DO NOTHING → 0 linhas afetadas significa "já processado".
func (r *ProcessedEventsRepository) MarkIfNew(ctx context.Context, consumer, eventID string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO processed_events (consumer, event_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, consumer, eventID)
	if err != nil {
		return false, fmt.Errorf("marcar evento processado: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}
```

#### `internal/infrastructure/postgres/idempotency_store.go`

O `INSERT ... ON CONFLICT DO NOTHING` é o que torna a idempotência atômica.

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/seu-usuario/adquirente/internal/application"
)

type IdempotencyStore struct {
	db  *sql.DB
	ttl time.Duration
}

var _ application.IdempotencyStore = (*IdempotencyStore)(nil)

func NewIdempotencyStore(db *sql.DB, ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{db: db, ttl: ttl}
}

// Begin tenta reservar a chave. O INSERT ... ON CONFLICT DO NOTHING é atômico: de dois pedidos
// simultâneos, só um consegue inserir; o outro cai no SELECT e vê "em andamento".
func (s *IdempotencyStore) Begin(ctx context.Context, merchantID, key, requestHash string) (application.IdempotencyResult, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO idempotency_keys (merchant_id, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, merchantID, key, requestHash, time.Now().Add(s.ttl))
	if err != nil {
		return application.IdempotencyResult{}, fmt.Errorf("reservar chave de idempotência: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 1 {
		return application.IdempotencyResult{State: application.IdempotencyNew}, nil
	}

	var storedHash string
	var status sql.NullInt64
	var body []byte
	err = s.db.QueryRowContext(ctx, `SELECT request_hash, response_status, response_body FROM idempotency_keys
		WHERE merchant_id = $1 AND idempotency_key = $2`, merchantID, key).Scan(&storedHash, &status, &body)
	if errors.Is(err, sql.ErrNoRows) { // a chave foi abandonada entre o INSERT e o SELECT: peça para tentar de novo
		return application.IdempotencyResult{State: application.IdempotencyInProgress}, nil
	}
	if err != nil {
		return application.IdempotencyResult{}, fmt.Errorf("ler chave de idempotência: %w", err)
	}
	switch {
	case storedHash != requestHash:
		return application.IdempotencyResult{State: application.IdempotencyConflict}, nil
	case !status.Valid:
		return application.IdempotencyResult{State: application.IdempotencyInProgress}, nil
	}
	return application.IdempotencyResult{State: application.IdempotencyReplay, Status: int(status.Int64), Body: body}, nil
}

func (s *IdempotencyStore) Complete(ctx context.Context, merchantID, key string, status int, body []byte) error {
	_, err := s.db.ExecContext(ctx, `UPDATE idempotency_keys SET response_status = $3, response_body = $4
		WHERE merchant_id = $1 AND idempotency_key = $2`, merchantID, key, status, body)
	return err
}

func (s *IdempotencyStore) Abandon(ctx context.Context, merchantID, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE merchant_id = $1 AND idempotency_key = $2 AND response_status IS NULL`, merchantID, key)
	return err
}
```

#### `internal/infrastructure/postgres/card_vault_store.go`

Só o binário `vault` importa isto.

```go
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// CardRecord é o que o cofre guarda: PAN e validade CIFRADOS, mais os dados públicos.
type CardRecord struct {
	Token             string
	PANCiphertext     []byte
	ExpiryCiphertext  []byte
	Brand, BIN, Last4 string
}

// CardVaultStore só é usado pelo serviço vault (Parte 9.6). Nenhum outro binário importa isto.
type CardVaultStore struct{ db *sql.DB }

func NewCardVaultStore(db *sql.DB) *CardVaultStore { return &CardVaultStore{db: db} }

func (s *CardVaultStore) Save(ctx context.Context, rec CardRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO card_vault (token, pan_ciphertext, expiry_ciphertext, brand, bin, last4)
		VALUES ($1,$2,$3,$4,$5,$6)`, rec.Token, rec.PANCiphertext, rec.ExpiryCiphertext, rec.Brand, rec.BIN, rec.Last4)
	if err != nil {
		return fmt.Errorf("gravar cartão no cofre: %w", err)
	}
	return nil
}

func (s *CardVaultStore) Find(ctx context.Context, token string) (CardRecord, error) {
	var rec CardRecord
	err := s.db.QueryRowContext(ctx, `SELECT token, pan_ciphertext, expiry_ciphertext, brand, bin, last4 FROM card_vault WHERE token = $1`, token).
		Scan(&rec.Token, &rec.PANCiphertext, &rec.ExpiryCiphertext, &rec.Brand, &rec.BIN, &rec.Last4)
	if errors.Is(err, sql.ErrNoRows) {
		return CardRecord{}, shared.ErrNotFound
	}
	if err != nil {
		return CardRecord{}, fmt.Errorf("ler cartão do cofre: %w", err)
	}
	return rec, nil
}
```

### 4.3 Repositório com GORM

#### `internal/infrastructure/gormrepo/merchant_repository.go`

Compare com o repositório de transações: menos SQL, mais mágica. Note como o `Model` nunca sai do pacote e como o lock otimista é feito no `Where`.

```go
package gormrepo

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Este pacote mostra um ORM (GORM) no cadastro, para você comparar com o SQL explícito do núcleo.
// Regra: o Model é um detalhe de infraestrutura. Ele NUNCA sai deste pacote; o domínio só vê merchant.Merchant.

// feePlanJSON é como o plano de taxas vai para a coluna JSONB.
type feePlanJSON struct {
	Debit              int         `json:"debit_bps"`
	CreditOneShot      int         `json:"credit_bps"`
	CreditInstallments map[int]int `json:"credit_installments_bps"`
	AnticipationMonth  int         `json:"anticipation_bps_month"`
}

// Scan/Value ensinam o GORM a ler/gravar o JSONB.
func (f *feePlanJSON) Scan(src any) error {
	b, ok := src.([]byte)
	if !ok {
		if s, ok := src.(string); ok {
			b = []byte(s)
		} else {
			return fmt.Errorf("fee_plan: tipo inesperado %T", src)
		}
	}
	return json.Unmarshal(b, f)
}

func (f feePlanJSON) Value() (driver.Value, error) { return json.Marshal(f) }

type MerchantModel struct {
	ID               string      `gorm:"primaryKey"`
	Document         string      `gorm:"uniqueIndex;not null"`
	LegalName        string      `gorm:"not null"`
	MCC              string      `gorm:"column:mcc;not null"`
	Status           string      `gorm:"not null;index"`
	BankCode         string      `gorm:"not null"`
	BankBranch       string      `gorm:"not null"`
	BankAccount      string      `gorm:"not null"`
	BankAccountKind  string      `gorm:"not null"`
	FeePlan          feePlanJSON `gorm:"type:jsonb;not null"`
	WebhookURL       string      `gorm:"column:webhook_url;not null"`
	AutoAnticipation bool        `gorm:"not null"`
	Version          int         `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (MerchantModel) TableName() string { return "merchants" }

type MerchantRepository struct{ db *gorm.DB }

// New abre o GORM em cima do MESMO *sql.DB do resto do sistema (um pool só).
func New(sqlDB *sql.DB) (*MerchantRepository, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("abrir gorm: %w", err)
	}
	return &MerchantRepository{db: db}, nil
}

var _ merchant.Repository = (*MerchantRepository)(nil)

func (r *MerchantRepository) Create(ctx context.Context, m *merchant.Merchant) error {
	if err := r.db.WithContext(ctx).Create(toModel(m)).Error; err != nil {
		return fmt.Errorf("inserir estabelecimento: %w", err)
	}
	return nil
}

func (r *MerchantRepository) FindByID(ctx context.Context, id string) (*merchant.Merchant, error) {
	var model MerchantModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar estabelecimento: %w", err)
	}
	return toDomain(model)
}

// Update com lock otimista: só grava se a versão no banco for menor que a da entidade.
func (r *MerchantRepository) Update(ctx context.Context, m *merchant.Merchant) error {
	model := toModel(m)
	res := r.db.WithContext(ctx).Model(&MerchantModel{}).
		Where("id = ? AND version < ?", model.ID, model.Version).
		Select("*").Omit("id", "created_at").Updates(model)
	if res.Error != nil {
		return fmt.Errorf("atualizar estabelecimento: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return shared.ErrConcurrentUpdate
	}
	return nil
}

func toModel(m *merchant.Merchant) *MerchantModel {
	plan := m.FeePlan()
	inst := map[int]int{}
	for n, bps := range plan.CreditInstallments() {
		inst[n] = int(bps)
	}
	b := m.BankAccount()
	return &MerchantModel{
		ID: m.ID(), Document: m.Document().String(), LegalName: m.LegalName(), MCC: m.MCC(), Status: string(m.Status()),
		BankCode: b.BankCode, BankBranch: b.Branch, BankAccount: b.Number, BankAccountKind: string(b.Kind),
		FeePlan:    feePlanJSON{Debit: int(plan.Debit()), CreditOneShot: int(plan.CreditOneShot()), CreditInstallments: inst, AnticipationMonth: int(plan.AnticipationRateMonth())},
		WebhookURL: m.WebhookURL(), AutoAnticipation: m.AutoAnticipation(), Version: m.Version(), CreatedAt: m.CreatedAt(), UpdatedAt: m.UpdatedAt(),
	}
}

func toDomain(model MerchantModel) (*merchant.Merchant, error) {
	inst := map[int]shared.Bps{}
	for n, bps := range model.FeePlan.CreditInstallments {
		inst[n] = shared.Bps(bps)
	}
	plan, err := merchant.NewFeePlan(shared.Bps(model.FeePlan.Debit), shared.Bps(model.FeePlan.CreditOneShot), inst, shared.Bps(model.FeePlan.AnticipationMonth))
	if err != nil {
		return nil, err
	}
	bank := merchant.BankAccount{BankCode: model.BankCode, Branch: model.BankBranch, Number: model.BankAccount, Kind: merchant.AccountKind(model.BankAccountKind)}
	return merchant.Restore(model.ID, merchant.Document(model.Document), model.LegalName, model.MCC, merchant.Status(model.Status), bank, plan,
		model.WebhookURL, model.AutoAnticipation, model.Version, model.CreatedAt, model.UpdatedAt), nil
}
```

**Compile:**

```bash
go build ./internal/...
```

---

## Fase 5 — O resto da infraestrutura

```bash
go get github.com/segmentio/kafka-go@latest github.com/redis/go-redis/v9@latest github.com/sony/gobreaker/v2@latest \
  github.com/prometheus/client_golang@latest go.opentelemetry.io/otel@latest go.opentelemetry.io/otel/sdk@latest \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest \
  go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@latest \
  go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@latest \
  github.com/gin-gonic/gin@latest github.com/golang-jwt/jwt/v5@latest
```

(Se o download falhar por rede, tente `GOPROXY=direct go get ...`.)

#### `internal/infrastructure/config/config.go`

```go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config reúne tudo que vem do ambiente. Regra: NENHUM segredo no código; em dev via .env,
// em produção via Secrets do Kubernetes / gerenciador de segredos.
type Config struct {
	Env         string // dev | prod
	ServiceName string
	LogLevel    string

	HTTPAddr string
	GRPCAddr string

	DatabaseURL  string
	RedisAddr    string
	KafkaBrokers []string

	IssuerAddr    string
	VaultAddr     string
	IssuerTimeout time.Duration
	MaxTxAmount   int64 // teto por transação, em centavos

	JWTPublicKeyFile  string
	JWTPrivateKeyFile string // só o auth-sim
	JWTIssuer         string
	JWTAudience       string

	TLSCertFile string
	TLSKeyFile  string
	TLSCAFile   string

	VaultKeyBase64 string // só o vault

	OTLPEndpoint string

	SettlementOutputDir string
	RegistryLiensFile   string
	SettlementDate      string // YYYY-MM-DD; vazio = hoje

	WebhookSecret string

	IssuerSimLatencyMs int
}

func Load() Config {
	return Config{
		Env:         getenv("APP_ENV", "dev"),
		ServiceName: getenv("SERVICE_NAME", "adquirente"),
		LogLevel:    getenv("LOG_LEVEL", "info"),

		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
		GRPCAddr: getenv("GRPC_ADDR", ":9090"),

		DatabaseURL:  getenv("DATABASE_URL", "postgres://adq:adq@localhost:5432/adquirente?sslmode=disable"),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: strings.Split(getenv("KAFKA_BROKERS", "localhost:9092"), ","),

		IssuerAddr:    getenv("ISSUER_ADDR", "localhost:9091"),
		VaultAddr:     getenv("VAULT_ADDR", "localhost:9092"),
		IssuerTimeout: duration("ISSUER_TIMEOUT", 2*time.Second),
		MaxTxAmount:   integer("MAX_TX_AMOUNT", 5_000_000), // R$ 50.000,00

		JWTPublicKeyFile:  getenv("JWT_PUBLIC_KEY_FILE", "certs/jwt-public.pem"),
		JWTPrivateKeyFile: getenv("JWT_PRIVATE_KEY_FILE", "certs/jwt-private.pem"),
		JWTIssuer:         getenv("JWT_ISSUER", "https://auth.adquirente.local"),
		JWTAudience:       getenv("JWT_AUDIENCE", "adquirente-api"),

		TLSCertFile: os.Getenv("TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("TLS_KEY_FILE"),
		TLSCAFile:   os.Getenv("TLS_CA_FILE"),

		VaultKeyBase64: os.Getenv("VAULT_KEY_BASE64"),

		OTLPEndpoint: os.Getenv("OTLP_ENDPOINT"), // ex.: otel-collector:4317; vazio desliga o tracing

		SettlementOutputDir: getenv("SETTLEMENT_OUTPUT_DIR", "./out/settlements"),
		RegistryLiensFile:   os.Getenv("REGISTRY_LIENS_FILE"),
		SettlementDate:      os.Getenv("SETTLEMENT_DATE"),

		WebhookSecret: getenv("WEBHOOK_SECRET", "dev-secret-change-me"),

		IssuerSimLatencyMs: int(integer("ISSUER_SIM_LATENCY_MS", 50)),
	}
}

func (c Config) IsProd() bool { return c.Env == "prod" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func integer(key string, def int64) int64 {
	if v, err := strconv.ParseInt(os.Getenv(key), 10, 64); err == nil {
		return v
	}
	return def
}

func duration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(key)); err == nil {
		return v
	}
	return def
}
```

#### `internal/infrastructure/observability/logging.go`

Log JSON com `trace_id` em toda linha (Parte 13.2).

```go
package observability

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// NewLogger cria o logger JSON e o define como padrão.
// Cada linha ganha trace_id e span_id (quando há um span no contexto): é o que liga log ↔ trace.
func NewLogger(level, service string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	logger := slog.New(&traceHandler{Handler: base}).With("service", service)
	slog.SetDefault(logger)
	return logger
}

type traceHandler struct{ slog.Handler }

func (h *traceHandler) Handle(ctx context.Context, rec slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		rec.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, rec)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}
```

#### `internal/infrastructure/observability/otel.go`

```go
package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// SetupTracing liga o OpenTelemetry: spans vão para o Collector via OTLP/gRPC.
// Devolve uma função de shutdown que envia o que ficou no buffer (chame no graceful shutdown).
// Endpoint vazio = tracing desligado (útil em testes).
func SetupTracing(ctx context.Context, serviceName, endpoint string) (func(context.Context) error, error) {
	// A propagação (header traceparent) é configurada mesmo sem exporter: os ids continuam fluindo.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("criar exporter OTLP: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))), // 100% em estudo; em prod, amostre
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
```

#### `internal/infrastructure/observability/metrics.go`

```go
package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics são as métricas do negócio e do transporte. Labels de baixa cardinalidade só:
// NUNCA merchant_id ou transaction_id como label.
type Metrics struct {
	registry *prometheus.Registry

	HTTPRequests   *prometheus.CounterVec   // route, method, status
	HTTPDuration   *prometheus.HistogramVec // route, method
	Authorizations *prometheus.CounterVec   // product, result (approved|denied|error)
	IssuerLatency  prometheus.Histogram
	SettledAmount  prometheus.Counter // centavos liquidados (soma)
	OutboxLag      prometheus.Gauge
}

func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	m := &Metrics{
		registry:     reg,
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "http_requests_total", Help: "Requisições HTTP"}, []string{"route", "method", "status"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "Duração das requisições",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5}}, []string{"route", "method"}),
		Authorizations: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "authorizations_total", Help: "Autorizações por produto e resultado"}, []string{"product", "result"}),
		IssuerLatency: prometheus.NewHistogram(prometheus.HistogramOpts{Name: "issuer_authorize_duration_seconds", Help: "Tempo de resposta do emissor",
			Buckets: []float64{.05, .1, .25, .5, 1, 2, 5}}),
		SettledAmount: prometheus.NewCounter(prometheus.CounterOpts{Name: "settled_amount_cents_total", Help: "Total liquidado (centavos)"}),
		OutboxLag:     prometheus.NewGauge(prometheus.GaugeOpts{Name: "outbox_pending_events", Help: "Eventos ainda não publicados"}),
	}
	reg.MustRegister(m.HTTPRequests, m.HTTPDuration, m.Authorizations, m.IssuerLatency, m.SettledAmount, m.OutboxLag)
	return m
}

// Handler é o que o Prometheus raspa em GET /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
```

#### `internal/infrastructure/cryptoutil/aesgcm.go`

AES-256-GCM (Parte 9.2). O único código de criptografia do projeto, e ele só chama a biblioteca padrão.

```go
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// Cipher cifra e decifra com AES-256-GCM (confidencialidade + integridade).
// A chave vem de um gerenciador de segredos; nunca do código.
type Cipher struct{ aead cipher.AEAD }

// NewAESGCM recebe a chave em base64 (32 bytes = AES-256).
// Gere uma em desenvolvimento com: openssl rand -base64 32
func NewAESGCM(base64Key string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("chave inválida (esperado base64): %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("a chave precisa ter 32 bytes (AES-256)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt devolve nonce || ciphertext. O nonce é aleatório e NUNCA se repete com a mesma chave.
func (c *Cipher) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, plain, nil), nil
}

func (c *Cipher) Decrypt(data []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(data) < n {
		return nil, errors.New("dado cifrado curto demais")
	}
	plain, err := c.aead.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return nil, errors.New("falha ao decifrar: dado corrompido ou chave errada")
	}
	return plain, nil
}
```

#### `internal/infrastructure/cryptoutil/aesgcm_test.go`

```go
package cryptoutil

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestAESGCMRoundTrip(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))
	c, err := NewAESGCM(key)
	if err != nil {
		t.Fatal(err)
	}
	pan := []byte("4111111111111111")
	ct1, _ := c.Encrypt(pan)
	ct2, _ := c.Encrypt(pan)
	if bytes.Equal(ct1, ct2) {
		t.Fatal("nonce aleatório: cifrar duas vezes deve dar resultados diferentes")
	}
	if bytes.Contains(ct1, pan) {
		t.Fatal("o PAN não pode aparecer em claro no ciphertext")
	}
	got, err := c.Decrypt(ct1)
	if err != nil || !bytes.Equal(got, pan) {
		t.Fatalf("decrypt = %q, %v", got, err)
	}
	ct1[len(ct1)-1] ^= 0xFF // adultera um byte
	if _, err := c.Decrypt(ct1); err == nil {
		t.Fatal("GCM deve detectar adulteração")
	}
	if _, err := NewAESGCM(base64.StdEncoding.EncodeToString([]byte("curta"))); err == nil {
		t.Fatal("chave curta deve falhar")
	}
}
```

#### `internal/infrastructure/grpcclient/tls.go`

mTLS (Parte 9.3 e 10.7).

```go
package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// TLSFiles são os caminhos dos certificados para mTLS. Vazios = sem TLS (só em desenvolvimento local).
type TLSFiles struct {
	CertFile string // certificado deste serviço
	KeyFile  string // chave privada deste serviço
	CAFile   string // autoridade que assina os certificados internos
}

// LoadClientTLS monta a configuração de cliente mTLS: apresenta nosso certificado e só aceita
// servidores assinados pela nossa CA.
func LoadClientTLS(f TLSFiles) (*tls.Config, error) {
	if f.CertFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(f.CertFile, f.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("carregar certificado do cliente: %w", err)
	}
	ca, err := os.ReadFile(f.CAFile)
	if err != nil {
		return nil, fmt.Errorf("ler CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, errors.New("CA inválida")
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, RootCAs: pool, MinVersion: tls.VersionTLS13}, nil
}

// LoadServerTLS: o servidor exige certificado do cliente (RequireAndVerifyClientCert).
func LoadServerTLS(f TLSFiles) (*tls.Config, error) {
	cfg, err := LoadClientTLS(f)
	if err != nil || cfg == nil {
		return cfg, err
	}
	cfg.ClientCAs = cfg.RootCAs
	cfg.ClientAuth = tls.RequireAndVerifyClientCert
	return cfg, nil
}

// DialCredentials devolve a opção de credencial para grpc.NewClient.
func DialCredentials(cfg *tls.Config) grpc.DialOption {
	if cfg == nil {
		return grpc.WithTransportCredentials(insecure.NewCredentials())
	}
	return grpc.WithTransportCredentials(credentials.NewTLS(cfg))
}
```

#### `internal/infrastructure/grpcclient/issuer.go`

O Adapter do emissor com Circuit Breaker (Parte 7.6) e a tradução de erros gRPC.

```go
package grpcclient

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	issuerv1 "github.com/seu-usuario/adquirente/gen/issuer/v1"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// IssuerClient é o Adapter: traduz o port do domínio (transaction.IssuerGateway) para o
// contrato gRPC do emissor, e envolve as chamadas num Circuit Breaker.
type IssuerClient struct {
	conn    *grpc.ClientConn
	client  issuerv1.IssuerServiceClient
	breaker *gobreaker.CircuitBreaker[*issuerv1.AuthorizeResponse]
}

var _ transaction.IssuerGateway = (*IssuerClient)(nil)

func NewIssuerClient(addr string, tlsCfg *tls.Config) (*IssuerClient, error) {
	conn, err := grpc.NewClient(addr, DialCredentials(tlsCfg), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, fmt.Errorf("conectar ao emissor: %w", err)
	}
	// Disjuntor: abre após 5 falhas consecutivas; fica aberto 10s; depois testa (meio-aberto).
	// Negativas (Approved=false) NÃO contam como falha: só erro técnico.
	settings := gobreaker.Settings{
		Name:        "issuer",
		Timeout:     10 * time.Second,
		ReadyToTrip: func(c gobreaker.Counts) bool { return c.ConsecutiveFailures >= 5 },
	}
	return &IssuerClient{conn: conn, client: issuerv1.NewIssuerServiceClient(conn), breaker: gobreaker.NewCircuitBreaker[*issuerv1.AuthorizeResponse](settings)}, nil
}

func (c *IssuerClient) Authorize(ctx context.Context, req transaction.AuthorizationRequest) (transaction.AuthorizationResponse, error) {
	product := issuerv1.Product_PRODUCT_CREDIT
	if req.Product == shared.Debit {
		product = issuerv1.Product_PRODUCT_DEBIT
	}
	resp, err := c.breaker.Execute(func() (*issuerv1.AuthorizeResponse, error) {
		return c.client.Authorize(ctx, &issuerv1.AuthorizeRequest{
			TransactionId: req.TransactionID, Pan: string(req.PAN), Expiry: req.Expiry, Amount: int64(req.Amount),
			Currency: "BRL", Product: product, Installments: int32(req.Installments), MerchantMcc: req.MerchantMCC,
		})
	})
	if err != nil {
		return transaction.AuthorizationResponse{}, translate(err)
	}
	return transaction.AuthorizationResponse{Approved: resp.Approved, AuthorizationCode: resp.AuthorizationCode, ResponseCode: resp.ResponseCode}, nil
}

func (c *IssuerClient) Reverse(ctx context.Context, transactionID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.Reverse(ctx, &issuerv1.ReverseRequest{TransactionId: transactionID})
	return translate(err)
}

func (c *IssuerClient) Close() error { return c.conn.Close() }

// translate converte erros gRPC nos erros que o caso de uso sabe tratar:
// DeadlineExceeded do gRPC vira context.DeadlineExceeded, para o `errors.Is` do Authorize funcionar.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
		return fmt.Errorf("circuit breaker aberto: %w", err)
	}
	switch status.Code(err) {
	case codes.DeadlineExceeded:
		return fmt.Errorf("%w: %v", context.DeadlineExceeded, err)
	case codes.Unavailable:
		return fmt.Errorf("emissor indisponível: %w", err)
	}
	return err
}
```

#### `internal/infrastructure/grpcclient/vault.go`

```go
package grpcclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/seu-usuario/adquirente/gen/vault/v1"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// VaultClient fala com o cofre de cartões. Implementa transaction.CardVault (Detokenize)
// e oferece Tokenize para o endpoint de tokenização da API.
type VaultClient struct {
	conn   *grpc.ClientConn
	client vaultv1.VaultServiceClient
}

var _ transaction.CardVault = (*VaultClient)(nil)

func NewVaultClient(addr string, tlsCfg *tls.Config) (*VaultClient, error) {
	conn, err := grpc.NewClient(addr, DialCredentials(tlsCfg), grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, fmt.Errorf("conectar ao vault: %w", err)
	}
	return &VaultClient{conn: conn, client: vaultv1.NewVaultServiceClient(conn)}, nil
}

func (c *VaultClient) Detokenize(ctx context.Context, token string) (transaction.CardData, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.client.Detokenize(ctx, &vaultv1.DetokenizeRequest{Token: token})
	if status.Code(err) == codes.NotFound {
		return transaction.CardData{}, transaction.ErrInvalidToken
	}
	if err != nil {
		return transaction.CardData{}, err
	}
	return transaction.CardData{PAN: transaction.PAN(resp.Pan), Expiry: resp.Expiry}, nil
}

// TokenizedCard é o resultado público da tokenização.
type TokenizedCard struct {
	Token, Brand, BIN, Last4 string
}

func (c *VaultClient) Tokenize(ctx context.Context, pan, expiry string) (TokenizedCard, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.client.Tokenize(ctx, &vaultv1.TokenizeRequest{Pan: pan, Expiry: expiry})
	if status.Code(err) == codes.InvalidArgument {
		return TokenizedCard{}, transaction.ErrInvalidPAN
	}
	if err != nil {
		return TokenizedCard{}, fmt.Errorf("%w: %v", shared.ErrNotFound, err)
	}
	return TokenizedCard{Token: resp.Token, Brand: resp.Brand, BIN: resp.Bin, Last4: resp.Last4}, nil
}

func (c *VaultClient) Close() error { return c.conn.Close() }
```

#### `internal/infrastructure/kafka/publisher.go`

```go
package kafka

import (
	"context"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"
)

// Publisher escreve no Kafka. Um Writer serve para todos os tópicos (o tópico vai na mensagem).
type Publisher struct{ w *kafkago.Writer }

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{w: &kafkago.Writer{
		Addr:                   kafkago.TCP(brokers...),
		Balancer:               &kafkago.Hash{},    // mesma chave → mesma partição → ordem por EC
		RequiredAcks:           kafkago.RequireAll, // acks=all: só confirma quando todas as réplicas gravaram
		AllowAutoTopicCreation: true,               // conveniência de desenvolvimento; em produção, tópicos são criados por IaC
	}}
}

func (p *Publisher) Publish(ctx context.Context, topic, key string, value []byte, headers map[string]string) error {
	msg := kafkago.Message{Topic: topic, Key: []byte(key), Value: value}
	for k, v := range headers {
		msg.Headers = append(msg.Headers, kafkago.Header{Key: k, Value: []byte(v)})
	}
	if err := p.w.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publicar em %s: %w", topic, err)
	}
	return nil
}

func (p *Publisher) Close() error { return p.w.Close() }
```

#### `internal/infrastructure/kafka/relay.go`

A segunda metade da outbox (Parte 12.4).

```go
package kafka

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Relay é a segunda metade do padrão Transactional Outbox (Parte 7.6 / 12.4):
// lê eventos ainda não publicados da tabela outbox e os manda para o Kafka.
// Pode rodar em várias réplicas ao mesmo tempo: o FOR UPDATE SKIP LOCKED evita disputa.
type Relay struct {
	db       *sql.DB
	pub      *Publisher
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewRelay(db *sql.DB, pub *Publisher, log *slog.Logger, interval time.Duration, batch int) *Relay {
	return &Relay{db: db, pub: pub, log: log, interval: interval, batch: batch}
}

// Run publica em loop até o contexto ser cancelado.
func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		n, err := r.publishBatch(ctx)
		if err != nil {
			r.log.ErrorContext(ctx, "relay da outbox falhou", "err", err)
		}
		if n == r.batch { // ainda há fila: não espera o ticker
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *Relay) publishBatch(ctx context.Context) (int, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT id, event_id, aggregate_id, event_type, payload FROM outbox
		WHERE published_at IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`, r.batch)
	if err != nil {
		return 0, err
	}
	type row struct {
		id                              int64
		eventID, aggregateID, eventType string
		payload                         []byte
	}
	var pending []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.id, &x.eventID, &x.aggregateID, &x.eventType, &x.payload); err != nil {
			rows.Close()
			return 0, err
		}
		pending = append(pending, x)
	}
	rows.Close()
	if len(pending) == 0 {
		return 0, nil
	}

	var published []int64
	for _, x := range pending {
		// tópico = tipo do evento ("transaction.captured"); chave = id do agregado (ordem por transação/EC)
		err := r.pub.Publish(ctx, x.eventType, x.aggregateID, x.payload, map[string]string{
			"event_id": x.eventID, "event_type": x.eventType,
		})
		if err != nil {
			r.log.WarnContext(ctx, "falha ao publicar; ficará para a próxima rodada", "event_id", x.eventID, "err", err)
			break // mantém a ordem: não pula eventos
		}
		published = append(published, x.id)
	}
	if len(published) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET published_at = now() WHERE id = ANY($1)`, published); err != nil {
			return 0, fmt.Errorf("marcar publicados: %w", err)
		}
	}
	return len(published), tx.Commit()
}
```

#### `internal/infrastructure/kafka/consumer.go`

At-least-once, retry com backoff, DLQ (Parte 12.5).

```go
package kafka

import (
	"context"
	"errors"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/seu-usuario/adquirente/internal/application"
)

// HandlerFunc é o que cada consumidor implementa: recebe o tipo e o JSON do evento.
type HandlerFunc func(ctx context.Context, eventType string, payload []byte) error

// Consumer lê de um consumer group, chama o handler e só confirma o offset DEPOIS de processar
// (at-least-once). Erros transitórios: tenta de novo com backoff. Erros permanentes ou esgotadas
// as tentativas: manda para a DLQ (<tópico>.dlq) e segue, para não travar a partição.
type Consumer struct {
	reader      *kafkago.Reader
	dlq         *kafkago.Writer
	handler     HandlerFunc
	log         *slog.Logger
	maxAttempts int
}

func NewConsumer(brokers []string, groupID string, topics []string, handler HandlerFunc, log *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:     brokers,
			GroupID:     groupID,
			GroupTopics: topics,
			MinBytes:    1,
			MaxBytes:    10e6,
			MaxWait:     500 * time.Millisecond,
		}),
		dlq:         &kafkago.Writer{Addr: kafkago.TCP(brokers...), AllowAutoTopicCreation: true, RequiredAcks: kafkago.RequireAll},
		handler:     handler,
		log:         log,
		maxAttempts: 5,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	defer c.reader.Close()
	defer c.dlq.Close()
	for {
		msg, err := c.reader.FetchMessage(ctx) // não confirma ainda
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		c.process(ctx, msg)
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.log.ErrorContext(ctx, "falha ao confirmar offset", "err", err)
		}
	}
}

func (c *Consumer) process(ctx context.Context, msg kafkago.Message) {
	eventType := header(msg, "event_type")
	if eventType == "" {
		eventType = msg.Topic
	}
	eventID := header(msg, "event_id")
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		lastErr = c.handler(ctx, eventType, msg.Value)
		if lastErr == nil {
			return
		}
		if application.IsPermanent(lastErr) {
			break
		}
		backoff := time.Duration(200*(1<<attempt)) * time.Millisecond // 400ms, 800ms, 1.6s, 3.2s, 6.4s
		c.log.WarnContext(ctx, "erro transitório; tentando de novo", "event_id", eventID, "attempt", attempt, "err", lastErr)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
	c.log.ErrorContext(ctx, "evento enviado para a DLQ", "event_id", eventID, "topic", msg.Topic, "err", lastErr)
	dead := kafkago.Message{Topic: msg.Topic + ".dlq", Key: msg.Key, Value: msg.Value, Headers: msg.Headers}
	dead.Headers = append(dead.Headers, kafkago.Header{Key: "error", Value: []byte(lastErr.Error())})
	if err := c.dlq.WriteMessages(ctx, dead); err != nil {
		c.log.ErrorContext(ctx, "falha ao gravar na DLQ", "err", err)
	}
}

func header(msg kafkago.Message, key string) string {
	for _, h := range msg.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}
```

#### `internal/infrastructure/redis/lock.go`

Lock distribuído para "uma liquidação por dia" (Parte 8.10).

```go
package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Lock é um lock distribuído simples: SET key token NX EX ttl.
// Usado para garantir UMA liquidação por dia mesmo com vários workers.
type Lock struct {
	client *goredis.Client
	tokens map[string]string
}

func NewLock(addr string) *Lock {
	return &Lock{client: goredis.NewClient(&goredis.Options{Addr: addr}), tokens: map[string]string{}}
}

func (l *Lock) Acquire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return false, err
	}
	token := hex.EncodeToString(buf)
	ok, err := l.client.SetNX(ctx, key, token, ttl).Result() // NX: só se não existir. Atômico.
	if err != nil {
		return false, err
	}
	if ok {
		l.tokens[key] = token
	}
	return ok, nil
}

// releaseScript só apaga se o valor ainda for o NOSSO token (não solta o lock de outro processo).
var releaseScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0`)

func (l *Lock) Release(ctx context.Context, key string) error {
	token, ok := l.tokens[key]
	if !ok {
		return nil
	}
	delete(l.tokens, key)
	return releaseScript.Run(ctx, l.client, []string{key}, token).Err()
}

func (l *Lock) Ping(ctx context.Context) error { return l.client.Ping(ctx).Err() }
func (l *Lock) Close() error                   { return l.client.Close() }
```

#### `internal/infrastructure/registry/fake.go`

A registradora simulada. O arquivo de ônus é como você testa a regra de compliance mais importante da liquidação.

```go
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
)

// Fake simula a registradora de recebíveis (CERC/TAG/B3/Núclea) em memória.
// Ônus são carregados de um arquivo JSON para você testar a regra "não pagar ao EC se há ônus".
//
// Formato do arquivo:
//
//	[{"merchant_id":"m_...","due_date":"2026-10-13","amount":25000,
//	  "creditor_name":"Banco Credor","bank_code":"001","branch":"1234","number":"99999-9"}]
type Fake struct {
	mu     sync.Mutex
	liens  []receivable.Lien
	units  []receivable.Unit
	owners map[string]string // receivable_id → dono
}

var _ receivable.Registry = (*Fake)(nil)

func NewFake(liensFile string) (*Fake, error) {
	f := &Fake{owners: map[string]string{}}
	if liensFile == "" {
		return f, nil
	}
	raw, err := os.ReadFile(liensFile)
	if err != nil {
		return nil, fmt.Errorf("ler arquivo de ônus: %w", err)
	}
	var rows []struct {
		MerchantID   string `json:"merchant_id"`
		DueDate      string `json:"due_date"`
		Amount       int64  `json:"amount"`
		CreditorName string `json:"creditor_name"`
		BankCode     string `json:"bank_code"`
		Branch       string `json:"branch"`
		Number       string `json:"number"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("arquivo de ônus inválido: %w", err)
	}
	for _, r := range rows {
		due, err := time.Parse("2006-01-02", r.DueDate)
		if err != nil {
			return nil, fmt.Errorf("data inválida %q: %w", r.DueDate, err)
		}
		acc, err := merchant.NewBankAccount(r.BankCode, r.Branch, r.Number, merchant.Checking)
		if err != nil {
			return nil, err
		}
		f.liens = append(f.liens, receivable.Lien{MerchantID: r.MerchantID, DueDate: shared.DateOnly(due), Amount: shared.Money(r.Amount), CreditorName: r.CreditorName, Creditor: acc})
	}
	return f, nil
}

func (f *Fake) Register(_ context.Context, units []receivable.Unit) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.units = append(f.units, units...)
	return nil
}

func (f *Fake) CheckLiens(_ context.Context, merchantID string, dueDate time.Time) ([]receivable.Lien, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []receivable.Lien
	for _, l := range f.liens {
		if l.MerchantID == merchantID && l.DueDate.Equal(shared.DateOnly(dueDate)) {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *Fake) TransferOwnership(_ context.Context, receivableIDs []string, newOwner string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range receivableIDs {
		f.owners[id] = newOwner
	}
	return nil
}
```

#### `internal/infrastructure/bank/file_gateway.go`

```go
package bank

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seu-usuario/adquirente/internal/domain/settlement"
)

// FileGateway simula o banco liquidante: grava cada lote como um arquivo JSON
// (o análogo do arquivo CNAB/remessa) e devolve o nome do arquivo como referência.
//
// Gancho de teste: qualquer ordem cuja conta termine em "-0" é rejeitada com "conta inválida",
// para você exercitar o caminho de falha da liquidação.
type FileGateway struct{ dir string }

var _ settlement.PaymentGateway = (*FileGateway)(nil)

func NewFileGateway(dir string) (*FileGateway, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("criar diretório de liquidação: %w", err)
	}
	return &FileGateway{dir: dir}, nil
}

func (g *FileGateway) Send(_ context.Context, s *settlement.Settlement) (string, error) {
	for _, o := range s.Orders() {
		if strings.HasSuffix(o.BankAccount.Number, "-0") {
			return "", errors.New("conta inválida: " + o.BankAccount.Number)
		}
	}
	name := fmt.Sprintf("settlement-%s-%s.json", s.Date().Format("20060102"), s.ID())
	payload := map[string]any{
		"settlement_id": s.ID(),
		"date":          s.Date().Format("2006-01-02"),
		"merchant_id":   s.MerchantID(),
		"orders":        s.Orders(),
		"total":         s.ToMerchant() + s.ToCreditors(),
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	if err := os.WriteFile(filepath.Join(g.dir, name), data, 0o640); err != nil {
		return "", fmt.Errorf("gravar arquivo de liquidação: %w", err)
	}
	return name, nil
}
```

#### `internal/infrastructure/webhook/sender.go`

```go
package webhook

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/seu-usuario/adquirente/internal/application/notification"
)

// HTTPSender entrega webhooks com timeout curto e SEM seguir redirecionamentos
// (um redirect poderia levar o corpo assinado para um host que não validamos: SSRF).
type HTTPSender struct{ client *http.Client }

var _ notification.Sender = (*HTTPSender)(nil)

func NewHTTPSender() *HTTPSender {
	return &HTTPSender{client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirecionamento não permitido") },
	}}
}

func (s *HTTPSender) Send(ctx context.Context, url string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("EC respondeu %d", resp.StatusCode)
	}
	return nil
}
```

**Compile e teste:**

```bash
go build ./internal/... && go test ./internal/infrastructure/...
```

---

## Fase 6 — Handlers: HTTP (Gin) e gRPC

### 6.1 Servidores gRPC

#### `internal/handler/grpc/issuer_server.go`

O emissor simulado. Leia as regras no comentário: são os "botões" que você vai apertar na Fase 8 para testar cada caminho.

```go
package grpc

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	issuerv1 "github.com/seu-usuario/adquirente/gen/issuer/v1"
)

// IssuerServer é o emissor SIMULADO. Regras propositalmente simples e previsíveis,
// para você testar cada caminho do autorizador:
//   - PAN terminado em "0000"  → negada "05" (não honrar)
//   - valor > R$ 5.000,00       → negada "51" (saldo/limite insuficiente)
//   - PAN terminado em "9999"  → demora 5s (para testar timeout + reversal)
//   - qualquer outro            → aprovada "00" com código de 6 dígitos
type IssuerServer struct {
	issuerv1.UnimplementedIssuerServiceServer
	latency time.Duration
}

func NewIssuerServer(latency time.Duration) *IssuerServer { return &IssuerServer{latency: latency} }

func (s *IssuerServer) Authorize(ctx context.Context, req *issuerv1.AuthorizeRequest) (*issuerv1.AuthorizeResponse, error) {
	delay := s.latency
	if strings.HasSuffix(req.Pan, "9999") {
		delay = 5 * time.Second
	}
	select {
	case <-ctx.Done(): // o cliente desistiu (deadline): não adianta responder
		return nil, ctx.Err()
	case <-time.After(delay):
	}
	switch {
	case strings.HasSuffix(req.Pan, "0000"):
		return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "05"}, nil
	case req.Amount > 500_000:
		return &issuerv1.AuthorizeResponse{Approved: false, ResponseCode: "51"}, nil
	}
	return &issuerv1.AuthorizeResponse{Approved: true, AuthorizationCode: randomDigits(6), ResponseCode: "00"}, nil
}

func (s *IssuerServer) Reverse(_ context.Context, _ *issuerv1.ReverseRequest) (*issuerv1.ReverseResponse, error) {
	return &issuerv1.ReverseResponse{Accepted: true}, nil
}

func randomDigits(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		d, _ := rand.Int(rand.Reader, big.NewInt(10))
		b.WriteString(fmt.Sprint(d.Int64()))
	}
	return b.String()
}
```

#### `internal/handler/grpc/vault_server.go`

O cofre. Note os logs: token e `last4`, nunca o PAN.

```go
package grpc

import (
	"context"
	"errors"
	"log/slog"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	vaultv1 "github.com/seu-usuario/adquirente/gen/vault/v1"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
	"github.com/seu-usuario/adquirente/internal/infrastructure/cryptoutil"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
)

// CardStore é o que o vault precisa da persistência (interface pequena, fácil de trocar).
type CardStore interface {
	Save(ctx context.Context, rec postgres.CardRecord) error
	Find(ctx context.Context, token string) (postgres.CardRecord, error)
}

// VaultServer é o cofre. Único lugar do sistema que vê PAN em claro e o guarda (cifrado).
type VaultServer struct {
	vaultv1.UnimplementedVaultServiceServer
	store  CardStore
	cipher *cryptoutil.Cipher
	log    *slog.Logger
}

func NewVaultServer(store CardStore, cipher *cryptoutil.Cipher, log *slog.Logger) *VaultServer {
	return &VaultServer{store: store, cipher: cipher, log: log}
}

var expiryRe = regexp.MustCompile(`^(0[1-9]|1[0-2])/\d{2}$`)

func (s *VaultServer) Tokenize(ctx context.Context, req *vaultv1.TokenizeRequest) (*vaultv1.TokenizeResponse, error) {
	if !transaction.Luhn(req.Pan) || !expiryRe.MatchString(req.Expiry) {
		return nil, status.Error(codes.InvalidArgument, "cartão inválido")
	}
	token := shared.NewID("tok")
	info, err := transaction.CardInfoFromPAN(token, req.Pan)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "cartão inválido")
	}
	panCT, err := s.cipher.Encrypt([]byte(req.Pan))
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao cifrar")
	}
	expCT, err := s.cipher.Encrypt([]byte(req.Expiry))
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao cifrar")
	}
	if err := s.store.Save(ctx, postgres.CardRecord{Token: token, PANCiphertext: panCT, ExpiryCiphertext: expCT, Brand: info.Brand, BIN: info.BIN, Last4: info.Last4}); err != nil {
		s.log.ErrorContext(ctx, "gravar no cofre", "err", err)
		return nil, status.Error(codes.Internal, "falha ao guardar")
	}
	// Repare: o log só tem token e last4. Nunca o PAN.
	s.log.InfoContext(ctx, "cartão tokenizado", "token", token, "brand", info.Brand, "last4", info.Last4)
	return &vaultv1.TokenizeResponse{Token: token, Brand: info.Brand, Bin: info.BIN, Last4: info.Last4}, nil
}

func (s *VaultServer) Detokenize(ctx context.Context, req *vaultv1.DetokenizeRequest) (*vaultv1.DetokenizeResponse, error) {
	rec, err := s.store.Find(ctx, req.Token)
	if errors.Is(err, shared.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "token desconhecido")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao ler")
	}
	pan, err := s.cipher.Decrypt(rec.PANCiphertext)
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao decifrar")
	}
	exp, err := s.cipher.Decrypt(rec.ExpiryCiphertext)
	if err != nil {
		return nil, status.Error(codes.Internal, "falha ao decifrar")
	}
	// Auditoria (PCI): todo acesso a dado de cartão fica registrado, com quem pediu (mTLS identifica o chamador).
	s.log.InfoContext(ctx, "cartão detokenizado", "token", req.Token, "last4", rec.Last4)
	return &vaultv1.DetokenizeResponse{Pan: string(pan), Expiry: string(exp)}, nil
}
```

### 6.2 HTTP

#### `internal/handler/http/problem.go`

Problem Details (Parte 4.2) e a tabela que traduz cada erro do domínio num status HTTP. Um lugar só.

```go
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	appant "github.com/seu-usuario/adquirente/internal/application/anticipation"
	apptx "github.com/seu-usuario/adquirente/internal/application/transaction"
	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/chargeback"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// Problem é o formato de erro da RFC 9457 (Problem Details). O cliente decide pelo `code`.
type Problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
	Code     string `json:"code"`
}

func problem(c *gin.Context, status int, code, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, Problem{
		Type: "https://api.adquirente.com/errors/" + code, Title: http.StatusText(status),
		Status: status, Detail: detail, Instance: c.Request.URL.Path, Code: code,
	})
}

// errorMap traduz erros do domínio/aplicação em (status, code). Um lugar só; os handlers só chamam respondError.
var errorMap = []struct {
	err    error
	status int
	code   string
}{
	{shared.ErrNotFound, http.StatusNotFound, "NOT_FOUND"},
	{shared.ErrInvalidTransition, http.StatusConflict, "INVALID_TRANSITION"},
	{shared.ErrConcurrentUpdate, http.StatusConflict, "CONCURRENT_UPDATE"},
	{anticipation.ErrAlreadyDone, http.StatusConflict, "ALREADY_DONE"},

	{shared.ErrInvalidAmount, http.StatusBadRequest, "INVALID_AMOUNT"},
	{shared.ErrInvalidProduct, http.StatusBadRequest, "INVALID_PRODUCT"},
	{merchant.ErrInvalidDocument, http.StatusBadRequest, "INVALID_DOCUMENT"},
	{merchant.ErrInvalidBankAccount, http.StatusBadRequest, "INVALID_BANK_ACCOUNT"},
	{merchant.ErrInvalidLegalName, http.StatusBadRequest, "INVALID_LEGAL_NAME"},
	{merchant.ErrInvalidMCC, http.StatusBadRequest, "INVALID_MCC"},
	{merchant.ErrInvalidFeePlan, http.StatusBadRequest, "INVALID_FEE_PLAN"},
	{merchant.ErrInvalidWebhook, http.StatusBadRequest, "INVALID_WEBHOOK_URL"},
	{transaction.ErrInvalidPAN, http.StatusBadRequest, "INVALID_CARD"},
	{transaction.ErrInvalidToken, http.StatusBadRequest, "INVALID_CARD_TOKEN"},
	{transaction.ErrDebitInstallments, http.StatusBadRequest, "DEBIT_CANNOT_INSTALL"},
	{transaction.ErrMaxInstallments, http.StatusBadRequest, "INVALID_INSTALLMENTS"},
	{anticipation.ErrNoReceivables, http.StatusBadRequest, "NO_RECEIVABLES"},

	{merchant.ErrMerchantInactive, http.StatusUnprocessableEntity, "MERCHANT_INACTIVE"},
	{merchant.ErrInstallmentsNotAllowed, http.StatusUnprocessableEntity, "INSTALLMENTS_NOT_ALLOWED"},
	{apptx.ErrLimitExceeded, http.StatusUnprocessableEntity, "LIMIT_EXCEEDED"},
	{anticipation.ErrNotAnticipable, http.StatusUnprocessableEntity, "RECEIVABLE_NOT_ANTICIPABLE"},
	{anticipation.ErrWrongMerchant, http.StatusForbidden, "FORBIDDEN"},
	{appant.ErrReceivableLiened, http.StatusUnprocessableEntity, "RECEIVABLE_LIENED"},
	{chargeback.ErrInvalidAmount, http.StatusUnprocessableEntity, "INVALID_CHARGEBACK_AMOUNT"},
	{chargeback.ErrTransactionStatus, http.StatusUnprocessableEntity, "TRANSACTION_NOT_DISPUTABLE"},

	{apptx.ErrIssuerUnavailable, http.StatusServiceUnavailable, "ISSUER_UNAVAILABLE"},
	{context.DeadlineExceeded, http.StatusGatewayTimeout, "UPSTREAM_TIMEOUT"},
}

func respondError(c *gin.Context, err error) {
	for _, m := range errorMap {
		if errors.Is(err, m.err) {
			problem(c, m.status, m.code, err.Error())
			return
		}
	}
	// Erro inesperado: loga com detalhe, responde sem detalhe (não vaza internals).
	slog.ErrorContext(c.Request.Context(), "erro inesperado", "err", err, "path", c.Request.URL.Path)
	problem(c, http.StatusInternalServerError, "INTERNAL_ERROR", "erro interno do servidor")
}
```

#### `internal/handler/http/dto.go`

```go
package http

import (
	"time"

	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/chargeback"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
)

// DTOs: structs "burras" só para transporte. A validação de FORMATO fica nas tags `binding`;
// a validação de NEGÓCIO fica no domínio. As respostas nunca expõem PAN.

// ---------- requests ----------

type CreateMerchantRequest struct {
	Document  string `json:"document" binding:"required"`
	LegalName string `json:"legal_name" binding:"required"`
	MCC       string `json:"mcc" binding:"required,len=4"`
	Bank      struct {
		Code   string `json:"code" binding:"required,len=3"`
		Branch string `json:"branch" binding:"required"`
		Number string `json:"number" binding:"required"`
		Kind   string `json:"kind" binding:"required,oneof=CHECKING SAVINGS PAYMENT"`
	} `json:"bank_account" binding:"required"`
	FeePlan struct {
		DebitBps             int         `json:"debit_bps" binding:"min=0,max=10000"`
		CreditBps            int         `json:"credit_bps" binding:"min=0,max=10000"`
		CreditInstallmentBps map[int]int `json:"credit_installments_bps"`
		AnticipationBpsMonth int         `json:"anticipation_bps_month" binding:"min=0,max=10000"`
	} `json:"fee_plan" binding:"required"`
	WebhookURL string `json:"webhook_url"`
}

type UpdateMerchantSettingsRequest struct {
	WebhookURL       *string `json:"webhook_url"`
	AutoAnticipation *bool   `json:"auto_anticipation"`
}

type TokenizeCardRequest struct {
	PAN    string `json:"pan" binding:"required,numeric,min=13,max=19"`
	Expiry string `json:"expiry" binding:"required,len=5"`
	// Sem campo CVV, de propósito: o CVV nunca chega ao nosso sistema.
}

type AuthorizeRequest struct {
	Amount       int64  `json:"amount" binding:"required,gt=0"`
	Product      string `json:"product" binding:"required,oneof=DEBIT CREDIT"`
	Installments int    `json:"installments" binding:"required,min=1,max=12"`
	CardToken    string `json:"card_token" binding:"required"`
}

type AnticipationRequest struct {
	ReceivableIDs []string `json:"receivable_ids" binding:"required,min=1,max=500"`
}

type OpenChargebackRequest struct {
	MerchantID    string `json:"merchant_id" binding:"required"`
	TransactionID string `json:"transaction_id" binding:"required"`
	ReasonCode    string `json:"reason_code" binding:"required"`
	Amount        int64  `json:"amount" binding:"required,gt=0"`
}

// ---------- responses ----------

type MerchantResponse struct {
	ID               string    `json:"id"`
	Document         string    `json:"document"` // mascarado
	LegalName        string    `json:"legal_name"`
	MCC              string    `json:"mcc"`
	Status           string    `json:"status"`
	WebhookURL       string    `json:"webhook_url,omitempty"`
	AutoAnticipation bool      `json:"auto_anticipation"`
	CreatedAt        time.Time `json:"created_at"`
}

func toMerchantResponse(m *merchant.Merchant) MerchantResponse {
	return MerchantResponse{ID: m.ID(), Document: m.Document().Masked(), LegalName: m.LegalName(), MCC: m.MCC(),
		Status: string(m.Status()), WebhookURL: m.WebhookURL(), AutoAnticipation: m.AutoAnticipation(), CreatedAt: m.CreatedAt()}
}

type CardTokenResponse struct {
	Token string `json:"token"`
	Brand string `json:"brand"`
	BIN   string `json:"bin"`
	Last4 string `json:"last4"`
}

type TransactionResponse struct {
	ID                string     `json:"id"`
	MerchantID        string     `json:"merchant_id"`
	Amount            int64      `json:"amount"`
	AmountFormatted   string     `json:"amount_formatted"`
	Product           string     `json:"product"`
	Installments      int        `json:"installments"`
	Status            string     `json:"status"`
	AuthorizationCode string     `json:"authorization_code,omitempty"`
	NSU               string     `json:"nsu,omitempty"`
	ResponseCode      string     `json:"response_code,omitempty"`
	CardBrand         string     `json:"card_brand"`
	CardLast4         string     `json:"card_last4"`
	CreatedAt         time.Time  `json:"created_at"`
	CapturedAt        *time.Time `json:"captured_at,omitempty"`
	CanceledAt        *time.Time `json:"canceled_at,omitempty"`
}

func toTransactionResponse(t *transaction.Transaction) TransactionResponse {
	return TransactionResponse{ID: t.ID(), MerchantID: t.MerchantID(), Amount: int64(t.Amount()), AmountFormatted: t.Amount().String(),
		Product: string(t.Product()), Installments: t.Installments(), Status: string(t.Status()), AuthorizationCode: t.AuthorizationCode(),
		NSU: t.NSU(), ResponseCode: t.ResponseCode(), CardBrand: t.Card().Brand, CardLast4: t.Card().Last4,
		CreatedAt: t.CreatedAt(), CapturedAt: t.CapturedAt(), CanceledAt: t.CanceledAt()}
}

type ReceivableResponse struct {
	ID             string `json:"id"`
	TransactionID  string `json:"transaction_id"`
	InstallmentNo  int    `json:"installment_no"`
	Installments   int    `json:"installments"`
	Product        string `json:"product"`
	Brand          string `json:"brand"`
	Gross          int64  `json:"gross_amount"`
	Fee            int64  `json:"fee_amount"`
	Net            int64  `json:"net_amount"`
	NetFormatted   string `json:"net_formatted"`
	DueDate        string `json:"due_date"`
	Status         string `json:"status"`
	SettlementID   string `json:"settlement_id,omitempty"`
	AnticipationID string `json:"anticipation_id,omitempty"`
}

func toReceivableResponse(r *receivable.Receivable) ReceivableResponse {
	return ReceivableResponse{ID: r.ID(), TransactionID: r.TransactionID(), InstallmentNo: r.InstallmentNo(), Installments: r.Installments(),
		Product: string(r.Product()), Brand: r.Brand(), Gross: int64(r.Gross()), Fee: int64(r.Fee()), Net: int64(r.Net()), NetFormatted: r.Net().String(),
		DueDate: r.DueDate().Format("2006-01-02"), Status: string(r.Status()), SettlementID: r.SettlementID(), AnticipationID: r.AnticipationID()}
}

func toReceivableResponses(list []*receivable.Receivable) []ReceivableResponse {
	out := make([]ReceivableResponse, 0, len(list))
	for _, r := range list {
		out = append(out, toReceivableResponse(r))
	}
	return out
}

type DailySummaryResponse struct {
	DueDate string `json:"due_date"`
	Status  string `json:"status"`
	Count   int    `json:"count"`
	Net     int64  `json:"net_amount"`
}

func toSummaryResponses(list []receivable.DailySummary) []DailySummaryResponse {
	out := make([]DailySummaryResponse, 0, len(list))
	for _, s := range list {
		out = append(out, DailySummaryResponse{DueDate: s.DueDate.Format("2006-01-02"), Status: string(s.Status), Count: s.Count, Net: int64(s.Net)})
	}
	return out
}

type AnticipationItemResponse struct {
	ReceivableID string `json:"receivable_id"`
	DueDate      string `json:"due_date"`
	Days         int    `json:"days"`
	Net          int64  `json:"net_amount"`
	PresentValue int64  `json:"present_value"`
	Discount     int64  `json:"discount"`
}

type AnticipationResponse struct {
	ID           string                     `json:"id"`
	MerchantID   string                     `json:"merchant_id"`
	Status       string                     `json:"status"`
	RateBpsMonth int                        `json:"rate_bps_month"`
	Gross        int64                      `json:"gross_amount"`
	Discount     int64                      `json:"discount_amount"`
	Net          int64                      `json:"net_amount"`
	NetFormatted string                     `json:"net_formatted"`
	RequestedAt  time.Time                  `json:"requested_at"`
	Items        []AnticipationItemResponse `json:"items"`
}

func toAnticipationResponse(a *anticipation.Anticipation) AnticipationResponse {
	items := make([]AnticipationItemResponse, 0, len(a.Items()))
	for _, it := range a.Items() {
		items = append(items, AnticipationItemResponse{ReceivableID: it.ReceivableID, DueDate: it.DueDate.Format("2006-01-02"), Days: it.Days,
			Net: int64(it.Net), PresentValue: int64(it.PresentValue), Discount: int64(it.Discount)})
	}
	return AnticipationResponse{ID: a.ID(), MerchantID: a.MerchantID(), Status: string(a.Status()), RateBpsMonth: int(a.RateMonth()),
		Gross: int64(a.Gross()), Discount: int64(a.Discount()), Net: int64(a.Net()), NetFormatted: a.Net().String(), RequestedAt: a.RequestedAt(), Items: items}
}

type SettlementResponse struct {
	ID          string    `json:"id"`
	Date        string    `json:"date"`
	Total       int64     `json:"total_amount"`
	ToMerchant  int64     `json:"to_merchant"`
	ToCreditors int64     `json:"to_creditors"`
	Retained    int64     `json:"retained"`
	Status      string    `json:"status"`
	ExternalRef string    `json:"external_ref,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func toSettlementResponses(list []*settlement.Settlement) []SettlementResponse {
	out := make([]SettlementResponse, 0, len(list))
	for _, s := range list {
		out = append(out, SettlementResponse{ID: s.ID(), Date: s.Date().Format("2006-01-02"), Total: int64(s.Total()), ToMerchant: int64(s.ToMerchant()),
			ToCreditors: int64(s.ToCreditors()), Retained: int64(s.Retained()), Status: string(s.Status()), ExternalRef: s.ExternalRef(), CreatedAt: s.CreatedAt()})
	}
	return out
}

type ChargebackResponse struct {
	ID            string    `json:"id"`
	TransactionID string    `json:"transaction_id"`
	MerchantID    string    `json:"merchant_id"`
	ReasonCode    string    `json:"reason_code"`
	Amount        int64     `json:"amount"`
	Status        string    `json:"status"`
	OpenedAt      time.Time `json:"opened_at"`
	Deadline      time.Time `json:"deadline"`
}

func toChargebackResponse(c *chargeback.Chargeback) ChargebackResponse {
	return ChargebackResponse{ID: c.ID(), TransactionID: c.TransactionID(), MerchantID: c.MerchantID(), ReasonCode: c.ReasonCode(),
		Amount: int64(c.Amount()), Status: string(c.Status()), OpenedAt: c.OpenedAt(), Deadline: c.Deadline()}
}
```

#### `internal/handler/http/middleware/request_id.go`

```go
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDKey = "request_id"

// RequestID aceita o X-Request-Id do cliente (para ele correlacionar do lado dele) ou gera um.
// O id volta no header da resposta e vai para todos os logs da requisição.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-Id")
		if id == "" || len(id) > 128 {
			id = uuid.NewString()
		}
		c.Set(RequestIDKey, id)
		c.Header("X-Request-Id", id)
		c.Next()
	}
}
```

#### `internal/handler/http/middleware/logger.go`

```go
package middleware

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
)

// Logger escreve UMA linha estruturada por requisição, depois que ela termina.
// Nada do corpo é logado (pode conter dados de cartão).
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", status,
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", c.GetString(RequestIDKey),
			"client_ip", c.ClientIP(),
		}
		if p, ok := c.Get(PrincipalKey); ok {
			attrs = append(attrs, "client_id", p.(Principal).ClientID, "merchant_id", p.(Principal).MerchantID)
		}
		switch {
		case status >= 500:
			log.ErrorContext(c.Request.Context(), "request", attrs...)
		case status >= 400:
			log.WarnContext(c.Request.Context(), "request", attrs...)
		default:
			log.InfoContext(c.Request.Context(), "request", attrs...)
		}
	}
}

// Metrics alimenta as métricas RED (rate, errors, duration) por rota.
func Metrics(m *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched" // evita cardinalidade explodir com URLs inválidas
		}
		m.HTTPRequests.WithLabelValues(route, c.Request.Method, strconv.Itoa(c.Writer.Status())).Inc()
		m.HTTPDuration.WithLabelValues(route, c.Request.Method).Observe(time.Since(start).Seconds())
	}
}
```

#### `internal/handler/http/middleware/auth.go`

JWT RS256 com algoritmo fixo, emissor, audiência e expiração (Parte 10.3); scopes (Parte 10.5).

```go
package middleware

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const PrincipalKey = "principal"

// Principal é quem está chamando. MerchantID vazio = token de operador interno (painel/suporte).
type Principal struct {
	ClientID   string
	MerchantID string
	Scopes     map[string]bool
}

func (p Principal) Has(scope string) bool { return p.Scopes[scope] }

// Claims são os campos do JWT. Só identificadores e permissões: JWT não é cifrado.
type Claims struct {
	jwt.RegisteredClaims
	MerchantID string `json:"merchant_id,omitempty"`
	Scope      string `json:"scope"`
}

func LoadPublicKey(path string) (*rsa.PublicKey, error) {
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ler chave pública: %w", err)
	}
	return jwt.ParseRSAPublicKeyFromPEM(pem)
}

// Authenticate valida o Bearer token: assinatura RS256, emissor, audiência e expiração.
// Fixar o algoritmo (WithValidMethods) fecha o ataque clássico de trocar "alg" por "none".
func Authenticate(pub *rsa.PublicKey, issuer, audience string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" || raw == c.GetHeader("Authorization") {
			unauthorized(c, "token ausente")
			return
		}
		claims := &Claims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return pub, nil },
			jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
			jwt.WithIssuer(issuer),
			jwt.WithAudience(audience),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !tok.Valid {
			unauthorized(c, "token inválido")
			return
		}
		scopes := map[string]bool{}
		for _, s := range strings.Fields(claims.Scope) {
			scopes[s] = true
		}
		c.Set(PrincipalKey, Principal{ClientID: claims.Subject, MerchantID: claims.MerchantID, Scopes: scopes})
		c.Next()
	}
}

// Authorize exige um scope. 403 (e não 401): sabemos quem é, mas não pode.
func Authorize(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !PrincipalFrom(c).Has(scope) {
			c.Header("Content-Type", "application/problem+json")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"status": 403, "title": "Forbidden", "code": "FORBIDDEN",
				"detail": "escopo insuficiente: " + scope, "instance": c.Request.URL.Path})
		}
	}
}

func PrincipalFrom(c *gin.Context) Principal {
	if p, ok := c.Get(PrincipalKey); ok {
		return p.(Principal)
	}
	return Principal{Scopes: map[string]bool{}}
}

func unauthorized(c *gin.Context, detail string) {
	c.Header("WWW-Authenticate", `Bearer realm="adquirente"`)
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"status": 401, "title": "Unauthorized", "code": "UNAUTHENTICATED",
		"detail": detail, "instance": c.Request.URL.Path})
}
```

#### `internal/handler/http/middleware/idempotency.go`

```go
package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/seu-usuario/adquirente/internal/application"
)

// Idempotency implementa a Parte 4.3. Exige o header Idempotency-Key em toda rota onde é aplicado.
//
//	primeira vez            → executa e guarda (status, corpo)
//	repetida, mesmo corpo   → devolve a resposta guardada, sem executar
//	repetida, corpo difere  → 409 IDEMPOTENCY_KEY_REUSED
//	original ainda rodando  → 409 REQUEST_IN_PROGRESS
//	original falhou com 5xx → chave liberada; o cliente pode tentar de novo
func Idempotency(store application.IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("Idempotency-Key")
		if key == "" || len(key) > 128 {
			abortProblem(c, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "envie o header Idempotency-Key (um UUID por operação)")
			return
		}
		p := PrincipalFrom(c)
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
		if err != nil {
			abortProblem(c, http.StatusBadRequest, "INVALID_BODY", "não foi possível ler o corpo")
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body)) // devolve o corpo para o handler ler

		sum := sha256.Sum256(append([]byte(c.Request.Method+" "+c.Request.URL.Path+"\n"), body...))
		hash := hex.EncodeToString(sum[:])

		res, err := store.Begin(c.Request.Context(), p.MerchantID, key, hash)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "idempotência indisponível", "err", err)
			abortProblem(c, http.StatusServiceUnavailable, "IDEMPOTENCY_UNAVAILABLE", "tente novamente")
			return
		}
		switch res.State {
		case application.IdempotencyReplay:
			c.Header("Idempotent-Replayed", "true")
			c.Data(res.Status, "application/json", res.Body)
			c.Abort()
			return
		case application.IdempotencyConflict:
			abortProblem(c, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "esta chave já foi usada com um corpo diferente")
			return
		case application.IdempotencyInProgress:
			abortProblem(c, http.StatusConflict, "REQUEST_IN_PROGRESS", "a requisição original ainda está em processamento")
			return
		}

		w := &bodyCapture{ResponseWriter: c.Writer, buf: &bytes.Buffer{}}
		c.Writer = w
		c.Next()

		status := c.Writer.Status()
		if status >= 500 {
			_ = store.Abandon(c.Request.Context(), p.MerchantID, key)
			return
		}
		if err := store.Complete(c.Request.Context(), p.MerchantID, key, status, w.buf.Bytes()); err != nil {
			slog.ErrorContext(c.Request.Context(), "guardar resposta idempotente", "err", err)
		}
	}
}

// bodyCapture copia o que o handler escreve, para guardarmos a resposta.
type bodyCapture struct {
	gin.ResponseWriter
	buf *bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCapture) WriteString(s string) (int, error) {
	w.buf.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func abortProblem(c *gin.Context, status int, code, detail string) {
	c.Header("Content-Type", "application/problem+json")
	c.AbortWithStatusJSON(status, gin.H{"status": status, "title": http.StatusText(status), "code": code, "detail": detail, "instance": c.Request.URL.Path})
}
```

#### `internal/handler/http/merchants.go`

Repare em `ownedMerchantID` e `requireMerchant`: a regra de posse da Parte 10.5.

```go
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appmerchant "github.com/seu-usuario/adquirente/internal/application/merchant"
	"github.com/seu-usuario/adquirente/internal/domain/merchant"
	"github.com/seu-usuario/adquirente/internal/handler/http/middleware"
)

type MerchantHandler struct {
	onboard   *appmerchant.Onboard
	review    *appmerchant.Review
	merchants merchant.Repository
}

func NewMerchantHandler(onboard *appmerchant.Onboard, review *appmerchant.Review, merchants merchant.Repository) *MerchantHandler {
	return &MerchantHandler{onboard: onboard, review: review, merchants: merchants}
}

// POST /v1/merchants  (scope merchants:write)
func (h *MerchantHandler) Create(c *gin.Context) {
	var req CreateMerchantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	m, err := h.onboard.Execute(c.Request.Context(), appmerchant.OnboardCommand{
		Document: req.Document, LegalName: req.LegalName, MCC: req.MCC,
		BankCode: req.Bank.Code, Branch: req.Bank.Branch, AccountNumber: req.Bank.Number, AccountKind: req.Bank.Kind,
		DebitBps: req.FeePlan.DebitBps, CreditBps: req.FeePlan.CreditBps, CreditInstallmentBps: req.FeePlan.CreditInstallmentBps,
		AnticipationBpsMonth: req.FeePlan.AnticipationBpsMonth, WebhookURL: req.WebhookURL,
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Location", "/v1/merchants/"+m.ID())
	c.JSON(http.StatusCreated, toMerchantResponse(m))
}

// GET /v1/merchants/:id  (scope merchants:read; o EC só vê a si mesmo)
func (h *MerchantHandler) Get(c *gin.Context) {
	id, ok := ownedMerchantID(c, c.Param("id"))
	if !ok {
		return
	}
	m, err := h.merchants.FindByID(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

// POST /v1/merchants/:id/approve  (scope merchants:approve — analista de KYC)
func (h *MerchantHandler) Approve(c *gin.Context) {
	m, err := h.review.Approve(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

// POST /v1/merchants/:id/block  (scope merchants:approve)
func (h *MerchantHandler) Block(c *gin.Context) {
	m, err := h.review.Block(c.Request.Context(), c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

// PATCH /v1/merchants/:id/settings  (scope merchants:write; o EC só altera a si mesmo)
func (h *MerchantHandler) UpdateSettings(c *gin.Context) {
	id, ok := ownedMerchantID(c, c.Param("id"))
	if !ok {
		return
	}
	var req UpdateMerchantSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	m, err := h.review.UpdateSettings(c.Request.Context(), appmerchant.UpdateSettingsCommand{MerchantID: id, WebhookURL: req.WebhookURL, AutoAnticipation: req.AutoAnticipation})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toMerchantResponse(m))
}

// ownedMerchantID aplica a regra de POSSE: um token de EC só pode agir sobre o próprio EC.
// Um token de operador (sem merchant_id) pode agir sobre qualquer id.
func ownedMerchantID(c *gin.Context, urlID string) (string, bool) {
	p := middleware.PrincipalFrom(c)
	if p.MerchantID != "" && p.MerchantID != urlID {
		problem(c, http.StatusForbidden, "FORBIDDEN", "você não pode acessar outro estabelecimento")
		return "", false
	}
	return urlID, true
}

// requireMerchant é para rotas que só fazem sentido para um EC (transações, agenda, antecipação).
func requireMerchant(c *gin.Context) (string, bool) {
	p := middleware.PrincipalFrom(c)
	if p.MerchantID == "" {
		problem(c, http.StatusForbidden, "FORBIDDEN", "esta operação exige credencial de estabelecimento")
		return "", false
	}
	return p.MerchantID, true
}
```

#### `internal/handler/http/transactions.go`

```go
package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	apptx "github.com/seu-usuario/adquirente/internal/application/transaction"
	"github.com/seu-usuario/adquirente/internal/domain/transaction"
	"github.com/seu-usuario/adquirente/internal/infrastructure/grpcclient"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
)

type TransactionHandler struct {
	authorize *apptx.Authorize
	lifecycle *apptx.Lifecycle
	get       *apptx.Get
	vault     *grpcclient.VaultClient
	metrics   *observability.Metrics
}

func NewTransactionHandler(authorize *apptx.Authorize, lifecycle *apptx.Lifecycle, get *apptx.Get, vault *grpcclient.VaultClient, metrics *observability.Metrics) *TransactionHandler {
	return &TransactionHandler{authorize: authorize, lifecycle: lifecycle, get: get, vault: vault, metrics: metrics}
}

// POST /v1/card-tokens — troca PAN por token. Esta rota está no escopo PCI: em produção ela
// vive num host isolado servido pelo próprio vault; aqui passa pela API por simplicidade.
func (h *TransactionHandler) Tokenize(c *gin.Context) {
	var req TokenizeCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", "cartão inválido") // nunca ecoa o corpo
		return
	}
	card, err := h.vault.Tokenize(c.Request.Context(), req.PAN, req.Expiry)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, CardTokenResponse{Token: card.Token, Brand: card.Brand, BIN: card.BIN, Last4: card.Last4})
}

// POST /v1/transactions  (Idempotency-Key obrigatório)
func (h *TransactionHandler) Authorize(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AuthorizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	tx, err := h.authorize.Execute(c.Request.Context(), apptx.AuthorizeCommand{
		MerchantID: merchantID, Amount: req.Amount, Product: req.Product, Installments: req.Installments, CardToken: req.CardToken,
	})
	if err != nil {
		h.metrics.Authorizations.WithLabelValues(req.Product, "error").Inc()
		respondError(c, err)
		return
	}
	h.metrics.Authorizations.WithLabelValues(req.Product, strings.ToLower(string(tx.Status()))).Inc()
	// Negada também é 201: o recurso "transação" foi criado, com status DENIED e response_code.
	c.Header("Location", "/v1/transactions/"+tx.ID())
	c.JSON(http.StatusCreated, toTransactionResponse(tx))
}

// GET /v1/transactions/:id
func (h *TransactionHandler) Get(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	tx, err := h.get.Execute(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTransactionResponse(tx))
}

// POST /v1/transactions/:id/capture
func (h *TransactionHandler) Capture(c *gin.Context) { h.change(c, h.lifecycle.Capture) }

// POST /v1/transactions/:id/cancel
func (h *TransactionHandler) Cancel(c *gin.Context) { h.change(c, h.lifecycle.Cancel) }

// change é o esqueleto comum de capturar/cancelar: posse → caso de uso → resposta.
func (h *TransactionHandler) change(c *gin.Context, fn func(ctx context.Context, merchantID, id string) (*transaction.Transaction, error)) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	tx, err := fn(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toTransactionResponse(tx))
}
```

#### `internal/handler/http/receivables.go`

```go
package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	apprcv "github.com/seu-usuario/adquirente/internal/application/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/settlement"
)

type ReceivableHandler struct {
	query       *apprcv.Query
	settlements settlement.Repository
}

func NewReceivableHandler(query *apprcv.Query, settlements settlement.Repository) *ReceivableHandler {
	return &ReceivableHandler{query: query, settlements: settlements}
}

// GET /v1/receivables?status=SCHEDULED&from=2026-10-01&to=2026-10-31&limit=100
func (h *ReceivableHandler) List(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	f := receivable.ListFilter{Status: receivable.Status(c.Query("status"))}
	var err error
	if f.From, err = parseDate(c.Query("from")); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "from deve ser YYYY-MM-DD")
		return
	}
	if f.To, err = parseDate(c.Query("to")); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "to deve ser YYYY-MM-DD")
		return
	}
	f.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "100"))
	list, err := h.query.List(c.Request.Context(), merchantID, f)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toReceivableResponses(list)})
}

// GET /v1/receivables/summary?from=...&to=...
func (h *ReceivableHandler) Summary(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	from, err1 := parseDate(c.DefaultQuery("from", time.Now().Format("2006-01-02")))
	to, err2 := parseDate(c.DefaultQuery("to", time.Now().AddDate(0, 3, 0).Format("2006-01-02")))
	if err1 != nil || err2 != nil {
		problem(c, http.StatusBadRequest, "INVALID_DATE", "from/to devem ser YYYY-MM-DD")
		return
	}
	list, err := h.query.Summary(c.Request.Context(), merchantID, from, to)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toSummaryResponses(list)})
}

// GET /v1/settlements
func (h *ReceivableHandler) ListSettlements(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	list, err := h.settlements.ListByMerchant(c.Request.Context(), merchantID, 100)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toSettlementResponses(list)})
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", s)
}
```

#### `internal/handler/http/anticipations.go`

```go
package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	appant "github.com/seu-usuario/adquirente/internal/application/anticipation"
	appcb "github.com/seu-usuario/adquirente/internal/application/chargeback"
)

type AnticipationHandler struct{ uc *appant.Anticipate }

func NewAnticipationHandler(uc *appant.Anticipate) *AnticipationHandler {
	return &AnticipationHandler{uc: uc}
}

// POST /v1/anticipations/simulate — não muda nada
func (h *AnticipationHandler) Simulate(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AnticipationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.uc.Simulate(c.Request.Context(), merchantID, req.ReceivableIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnticipationResponse(a))
}

// POST /v1/anticipations — contrata (Idempotency-Key obrigatório)
func (h *AnticipationHandler) Create(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	var req AnticipationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	a, err := h.uc.Request(c.Request.Context(), merchantID, req.ReceivableIDs)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Location", "/v1/anticipations/"+a.ID())
	c.JSON(http.StatusCreated, toAnticipationResponse(a))
}

// GET /v1/anticipations/:id
func (h *AnticipationHandler) Get(c *gin.Context) {
	merchantID, ok := requireMerchant(c)
	if !ok {
		return
	}
	a, err := h.uc.Get(c.Request.Context(), merchantID, c.Param("id"))
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, toAnticipationResponse(a))
}

// ChargebackHandler é a rota INTERNA que simula a bandeira trazendo uma contestação.
type ChargebackHandler struct{ open *appcb.Open }

func NewChargebackHandler(open *appcb.Open) *ChargebackHandler { return &ChargebackHandler{open: open} }

// POST /internal/chargebacks  (scope chargebacks:write — só operador)
func (h *ChargebackHandler) Open(c *gin.Context) {
	var req OpenChargebackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		problem(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	cb, err := h.open.Execute(c.Request.Context(), appcb.OpenCommand{MerchantID: req.MerchantID, TransactionID: req.TransactionID, ReasonCode: req.ReasonCode, Amount: req.Amount})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toChargebackResponse(cb))
}
```

#### `internal/handler/http/router.go`

A tabela de rotas: quem exige qual scope, onde entra a idempotência, o que fica fora da autenticação.

```go
package http

import (
	"context"
	"crypto/rsa"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"github.com/seu-usuario/adquirente/internal/application"
	"github.com/seu-usuario/adquirente/internal/handler/http/middleware"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
)

// Deps é tudo que o router precisa. O main monta e injeta.
type Deps struct {
	Log          *slog.Logger
	Metrics      *observability.Metrics
	PublicKey    *rsa.PublicKey
	JWTIssuer    string
	JWTAudience  string
	Idempotency  application.IdempotencyStore
	Ready        func(ctx context.Context) error // checa banco etc.
	Merchants    *MerchantHandler
	Transactions *TransactionHandler
	Receivables  *ReceivableHandler
	Anticipation *AnticipationHandler
	Chargebacks  *ChargebackHandler
	Prod         bool
}

func NewRouter(d Deps) *gin.Engine {
	if d.Prod {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), otelgin.Middleware("api"), middleware.Metrics(d.Metrics), middleware.Logger(d.Log))

	// Sem autenticação: saúde e métricas (em produção, /metrics fica só na rede interna).
	r.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		if err := d.Ready(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/metrics", gin.WrapH(d.Metrics.Handler()))
	if !d.Prod {
		r.StaticFile("/openapi.yaml", "api/openapi.yaml")
	}

	auth := middleware.Authenticate(d.PublicKey, d.JWTIssuer, d.JWTAudience)
	v1 := r.Group("/v1", auth)

	merchants := v1.Group("/merchants")
	merchants.POST("", middleware.Authorize("merchants:write"), d.Merchants.Create)
	merchants.GET("/:id", middleware.Authorize("merchants:read"), d.Merchants.Get)
	merchants.PATCH("/:id/settings", middleware.Authorize("merchants:write"), d.Merchants.UpdateSettings)
	merchants.POST("/:id/approve", middleware.Authorize("merchants:approve"), d.Merchants.Approve)
	merchants.POST("/:id/block", middleware.Authorize("merchants:approve"), d.Merchants.Block)

	v1.POST("/card-tokens", middleware.Authorize("transactions:write"), d.Transactions.Tokenize)

	tx := v1.Group("/transactions", middleware.Authorize("transactions:write"))
	tx.POST("", middleware.Idempotency(d.Idempotency), d.Transactions.Authorize)
	tx.POST("/:id/capture", middleware.Idempotency(d.Idempotency), d.Transactions.Capture)
	tx.POST("/:id/cancel", middleware.Idempotency(d.Idempotency), d.Transactions.Cancel)
	v1.GET("/transactions/:id", middleware.Authorize("transactions:read"), d.Transactions.Get)

	rcv := v1.Group("/receivables", middleware.Authorize("receivables:read"))
	rcv.GET("", d.Receivables.List)
	rcv.GET("/summary", d.Receivables.Summary)
	v1.GET("/settlements", middleware.Authorize("receivables:read"), d.Receivables.ListSettlements)

	ant := v1.Group("/anticipations", middleware.Authorize("anticipations:write"))
	ant.POST("/simulate", d.Anticipation.Simulate)
	ant.POST("", middleware.Idempotency(d.Idempotency), d.Anticipation.Create)
	ant.GET("/:id", d.Anticipation.Get)

	internal := r.Group("/internal", auth, middleware.Authorize("chargebacks:write"))
	internal.POST("/chargebacks", d.Chargebacks.Open)

	return r
}
```

```bash
go build ./...
```

---

## Fase 7 — Os binários

Cada `main.go` só monta as peças (o "composition root") e cuida do ciclo de vida: sinais, graceful shutdown (Parte 4.5), tracing.

#### `cmd/api/main.go`

```go
// A API pública da adquirente: credenciamento, tokenização, autorização, agenda, antecipação.
// O main só MONTA as peças (injeção de dependência) e cuida do ciclo de vida do processo.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	appant "github.com/seu-usuario/adquirente/internal/application/anticipation"
	appcb "github.com/seu-usuario/adquirente/internal/application/chargeback"
	appmerchant "github.com/seu-usuario/adquirente/internal/application/merchant"
	apprcv "github.com/seu-usuario/adquirente/internal/application/receivable"
	apptx "github.com/seu-usuario/adquirente/internal/application/transaction"
	"github.com/seu-usuario/adquirente/internal/domain/anticipation"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	httphandler "github.com/seu-usuario/adquirente/internal/handler/http"
	"github.com/seu-usuario/adquirente/internal/handler/http/middleware"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/gormrepo"
	"github.com/seu-usuario/adquirente/internal/infrastructure/grpcclient"
	"github.com/seu-usuario/adquirente/internal/infrastructure/kafka"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
	"github.com/seu-usuario/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "api")

	// ctx é cancelado no SIGTERM (Kubernetes) ou Ctrl+C: é o gatilho do graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "api", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()

	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")

	tlsCfg, err := grpcclient.LoadClientTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	issuer, err := grpcclient.NewIssuerClient(cfg.IssuerAddr, tlsCfg)
	must(log, err, "emissor")
	defer issuer.Close()
	vault, err := grpcclient.NewVaultClient(cfg.VaultAddr, tlsCfg)
	must(log, err, "vault")
	defer vault.Close()

	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")

	publicKey, err := middleware.LoadPublicKey(cfg.JWTPublicKeyFile)
	must(log, err, "chave pública JWT")

	// ---- montagem (Composition Root) ----
	clock := shared.RealClock{}
	uow := postgres.NewUnitOfWork(db)
	read := postgres.NewRepositories(db) // leituras fora de transação
	metrics := observability.NewMetrics()

	authorize := apptx.NewAuthorize(merchants, vault, issuer, uow, clock, cfg.IssuerTimeout,
		apptx.MaxAmountRule(shared.Money(cfg.MaxTxAmount)))
	lifecycle := apptx.NewLifecycle(uow, clock)
	anticipate := appant.NewAnticipate(uow, read.Receivables, merchants, reg, anticipation.CompoundPricer{}, clock, log)

	router := httphandler.NewRouter(httphandler.Deps{
		Log: log, Metrics: metrics, PublicKey: publicKey, JWTIssuer: cfg.JWTIssuer, JWTAudience: cfg.JWTAudience,
		Idempotency:  postgres.NewIdempotencyStore(db, 24*time.Hour),
		Ready:        db.PingContext,
		Merchants:    httphandler.NewMerchantHandler(appmerchant.NewOnboard(merchants, clock), appmerchant.NewReview(merchants, clock), merchants),
		Transactions: httphandler.NewTransactionHandler(authorize, lifecycle, apptx.NewGet(read.Transactions), vault, metrics),
		Receivables:  httphandler.NewReceivableHandler(apprcv.NewQuery(read.Receivables), read.Settlements),
		Anticipation: httphandler.NewAnticipationHandler(anticipate),
		Chargebacks:  httphandler.NewChargebackHandler(appcb.NewOpen(uow, clock)),
		Prod:         cfg.IsProd(),
	})

	// Relay da outbox: publica no Kafka o que os casos de uso gravaram. Roda junto com a API.
	publisher := kafka.NewPublisher(cfg.KafkaBrokers)
	defer publisher.Close()
	go kafka.NewRelay(db, publisher, log, 500*time.Millisecond, 100).Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Info("API ouvindo", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor HTTP caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("desligando: aguardando requisições em andamento")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	_ = shutdownTracing(shutdownCtx)
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error("falha na inicialização", "component", what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/auth-sim/main.go`

```go
// auth-sim é um servidor OAuth 2.0 mínimo (client credentials) para DESENVOLVIMENTO.
// Emite JWT RS256 com merchant_id e scopes, e publica a chave pública em JWKS.
// Em produção use um provedor pronto (Keycloak, Auth0, Cognito).
package main

import (
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
)

// client é uma credencial cadastrada. Arquivo certs/clients.json:
//
//	{ "client_padaria": {"secret": "s3cr3t", "merchant_id": "m_...", "scope": "transactions:write transactions:read receivables:read anticipations:write merchants:read merchants:write"},
//	  "operador":       {"secret": "op-s3cr3t", "scope": "merchants:read merchants:write merchants:approve chargebacks:write"} }
type client struct {
	Secret     string `json:"secret"`
	MerchantID string `json:"merchant_id"`
	Scope      string `json:"scope"`
}

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "auth-sim")

	keyPEM, err := os.ReadFile(cfg.JWTPrivateKeyFile)
	must(log, err, "ler chave privada")
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(keyPEM)
	must(log, err, "chave privada inválida")

	raw, err := os.ReadFile(getenv("AUTH_CLIENTS_FILE", "certs/clients.json"))
	must(log, err, "ler clients.json")
	var clients map[string]client
	must(log, json.Unmarshal(raw, &clients), "clients.json inválido")

	r := gin.New()
	r.Use(gin.Recovery())

	r.POST("/oauth/token", func(c *gin.Context) {
		if c.PostForm("grant_type") != "client_credentials" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported_grant_type"})
			return
		}
		id, secret := c.PostForm("client_id"), c.PostForm("client_secret")
		cl, ok := clients[id]
		// Comparação em tempo constante: não vaza, pelo tempo de resposta, quantos caracteres acertou.
		if !ok || subtle.ConstantTimeCompare([]byte(cl.Secret), []byte(secret)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_client"})
			return
		}
		now := time.Now()
		claims := jwt.MapClaims{
			"iss": cfg.JWTIssuer, "aud": cfg.JWTAudience, "sub": id,
			"iat": now.Unix(), "exp": now.Add(15 * time.Minute).Unix(), // curto: token vazado vale pouco
			"scope": cl.Scope,
		}
		if cl.MerchantID != "" {
			claims["merchant_id"] = cl.MerchantID
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "dev-key-1"
		signed, err := tok.SignedString(priv)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server_error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"access_token": signed, "token_type": "Bearer", "expires_in": 900, "scope": cl.Scope})
	})

	// JWKS: a chave pública em formato padrão, para quem quiser validar tokens sem arquivo PEM.
	r.GET("/.well-known/jwks.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"keys": []gin.H{jwk(&priv.PublicKey, "dev-key-1")}})
	})

	log.Info("auth-sim ouvindo", "addr", cfg.HTTPAddr)
	must(log, (&http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 5 * time.Second}).ListenAndServe(), "servidor")
}

func jwk(pub *rsa.PublicKey, kid string) gin.H {
	return gin.H{
		"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/issuer-sim/main.go`

```go
// issuer-sim é o emissor/bandeira simulado: um servidor gRPC com regras previsíveis.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	issuerv1 "github.com/seu-usuario/adquirente/gen/issuer/v1"
	grpchandler "github.com/seu-usuario/adquirente/internal/handler/grpc"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/grpcclient"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "issuer-sim")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "issuer-sim", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	opts := []grpc.ServerOption{grpc.StatsHandler(otelgrpc.NewServerHandler())}
	tlsCfg, err := grpcclient.LoadServerTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	if tlsCfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg))) // mTLS: só clientes com certificado da nossa CA
	}

	srv := grpc.NewServer(opts...)
	issuerv1.RegisterIssuerServiceServer(srv, grpchandler.NewIssuerServer(time.Duration(cfg.IssuerSimLatencyMs)*time.Millisecond))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	must(log, err, "listen")
	go func() {
		log.Info("issuer-sim ouvindo", "addr", cfg.GRPCAddr, "latency_ms", cfg.IssuerSimLatencyMs, "mtls", tlsCfg != nil)
		if err := srv.Serve(lis); err != nil {
			log.Error("gRPC caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.GracefulStop()
	_ = shutdownTracing(context.Background())
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/vault/main.go`

```go
// vault é o cofre de cartões: o único serviço que vê e guarda (cifrado) o PAN.
// Em produção: rede isolada (NetworkPolicy), banco próprio, mTLS obrigatório, chave no KMS.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	vaultv1 "github.com/seu-usuario/adquirente/gen/vault/v1"
	grpchandler "github.com/seu-usuario/adquirente/internal/handler/grpc"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/cryptoutil"
	"github.com/seu-usuario/adquirente/internal/infrastructure/grpcclient"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "vault")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "vault", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	cipher, err := cryptoutil.NewAESGCM(cfg.VaultKeyBase64)
	must(log, err, "VAULT_KEY_BASE64 (gere com: openssl rand -base64 32)")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()

	opts := []grpc.ServerOption{grpc.StatsHandler(otelgrpc.NewServerHandler())}
	tlsCfg, err := grpcclient.LoadServerTLS(grpcclient.TLSFiles{CertFile: cfg.TLSCertFile, KeyFile: cfg.TLSKeyFile, CAFile: cfg.TLSCAFile})
	must(log, err, "tls")
	if tlsCfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	} else if cfg.IsProd() {
		log.Error("vault em produção exige mTLS")
		os.Exit(1)
	}

	srv := grpc.NewServer(opts...)
	vaultv1.RegisterVaultServiceServer(srv, grpchandler.NewVaultServer(postgres.NewCardVaultStore(db), cipher, log))

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	must(log, err, "listen")
	go func() {
		log.Info("vault ouvindo", "addr", cfg.GRPCAddr, "mtls", tlsCfg != nil)
		if err := srv.Serve(lis); err != nil {
			log.Error("gRPC caiu", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	srv.GracefulStop()
	_ = shutdownTracing(context.Background())
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/scheduler/main.go`

```go
// scheduler consome eventos de transação e mantém a agenda de recebíveis.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	apprcv "github.com/seu-usuario/adquirente/internal/application/receivable"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/gormrepo"
	"github.com/seu-usuario/adquirente/internal/infrastructure/kafka"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
	"github.com/seu-usuario/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "scheduler")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "scheduler", cfg.OTLPEndpoint)
	must(log, err, "tracing")

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")
	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")

	uc := apprcv.NewScheduler(postgres.NewUnitOfWork(db), merchants, reg, shared.BrazilCalendar{}, shared.RealClock{}, log)

	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "scheduler",
		[]string{"transaction.captured", "transaction.canceled"}, uc.Handle, log)
	log.Info("scheduler consumindo", "brokers", cfg.KafkaBrokers)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer parou", "err", err)
	}
	_ = shutdownTracing(context.Background())
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/settlement/main.go`

```go
// settlement roda UMA liquidação (a de hoje, ou a data em SETTLEMENT_DATE) e sai.
// Em Kubernetes vira um CronJob às 06:00 de dia útil.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	appstl "github.com/seu-usuario/adquirente/internal/application/settlement"
	"github.com/seu-usuario/adquirente/internal/domain/shared"
	"github.com/seu-usuario/adquirente/internal/infrastructure/bank"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/gormrepo"
	"github.com/seu-usuario/adquirente/internal/infrastructure/kafka"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
	"github.com/seu-usuario/adquirente/internal/infrastructure/redis"
	"github.com/seu-usuario/adquirente/internal/infrastructure/registry"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "settlement")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.SetupTracing(ctx, "settlement", cfg.OTLPEndpoint)
	must(log, err, "tracing")
	defer func() { _ = shutdownTracing(context.Background()) }()

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")
	reg, err := registry.NewFake(cfg.RegistryLiensFile)
	must(log, err, "registradora")
	gateway, err := bank.NewFileGateway(cfg.SettlementOutputDir)
	must(log, err, "banco liquidante")
	lock := redis.NewLock(cfg.RedisAddr)
	must(log, lock.Ping(ctx), "redis")
	defer lock.Close()

	day := time.Now().UTC()
	if cfg.SettlementDate != "" {
		day, err = time.Parse("2006-01-02", cfg.SettlementDate)
		must(log, err, "SETTLEMENT_DATE deve ser YYYY-MM-DD")
	}

	uc := appstl.NewSettleDue(postgres.NewUnitOfWork(db), merchants, reg, gateway, shared.BrazilCalendar{}, shared.RealClock{}, lock, 500, log)
	rep, err := uc.Execute(ctx, day)
	if err != nil {
		log.Error("liquidação falhou", "date", day.Format("2006-01-02"), "err", err)
		os.Exit(1)
	}
	log.Info("liquidação concluída", "date", rep.Date.Format("2006-01-02"), "skipped", rep.Skipped, "merchants", rep.Merchants,
		"settlements", rep.Settlements, "failed", rep.Failed, "paid", rep.Paid.String(), "retained", rep.Retained.String())

	// Publica os eventos gerados (settlement.completed/failed) antes de sair: o worker não tem relay permanente.
	pub := kafka.NewPublisher(cfg.KafkaBrokers)
	defer pub.Close()
	relay := kafka.NewRelay(db, pub, log, time.Second, 100)
	relayCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	relay.Run(relayCtx)
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `cmd/notifier/main.go`

```go
// notifier consome todos os eventos relevantes ao EC e entrega webhooks assinados.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/seu-usuario/adquirente/internal/application/notification"
	"github.com/seu-usuario/adquirente/internal/infrastructure/config"
	"github.com/seu-usuario/adquirente/internal/infrastructure/gormrepo"
	"github.com/seu-usuario/adquirente/internal/infrastructure/kafka"
	"github.com/seu-usuario/adquirente/internal/infrastructure/observability"
	"github.com/seu-usuario/adquirente/internal/infrastructure/postgres"
	"github.com/seu-usuario/adquirente/internal/infrastructure/webhook"
)

func main() {
	cfg := config.Load()
	log := observability.NewLogger(cfg.LogLevel, "notifier")
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Connect(ctx, cfg.DatabaseURL)
	must(log, err, "banco")
	defer db.Close()
	merchants, err := gormrepo.New(db)
	must(log, err, "gorm")

	uc := notification.NewNotifier(merchants, webhook.NewHTTPSender(), cfg.WebhookSecret, log)
	topics := []string{
		"transaction.authorized", "transaction.captured", "transaction.canceled", "transaction.chargebacked",
		"receivable.scheduled", "receivable.anticipated", "settlement.completed", "settlement.failed", "chargeback.opened",
	}
	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "notifier", topics, uc.Handle, log)
	log.Info("notifier consumindo", "topics", topics)
	if err := consumer.Run(ctx); err != nil {
		log.Error("consumer parou", "err", err)
	}
}

func must(log *slog.Logger, err error, what string) {
	if err != nil {
		log.Error(what, "err", err)
		os.Exit(1)
	}
}
```

#### `deploy/docker/Dockerfile`

```dockerfile
# Um Dockerfile para todos os binários: docker build --build-arg SERVICE=api -f deploy/docker/Dockerfile .
# Etapa 1: compila num container com Go. Etapa 2: imagem final mínima, sem shell, sem root.

FROM golang:1.27 AS builder
ARG SERVICE
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /out/app /app
COPY api/openapi.yaml /api/openapi.yaml
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/app"]
```

**Compile tudo:**

```bash
go mod tidy && go vet ./... && go test -race ./... && go build ./...
```

---

## Fase 8 — Rodando o fluxo completo

Agora você opera a adquirente de ponta a ponta. Os comandos usam `curl`, `jq` e `uuidgen`; no Windows (Git Bash) instale o `jq` e, se faltar `uuidgen`, use `python -c "import uuid;print(uuid.uuid4())"`.

```bash
make certs && cat certs/vault.env >> .env
export $(grep VAULT_KEY_BASE64 .env)
docker compose up -d --build
docker compose logs -f api            # espere "API ouvindo"
```

**1. Token de operador** (quem credencia e aprova):

```bash
OP=$(curl -s -X POST localhost:8082/oauth/token -d grant_type=client_credentials -d client_id=operador -d client_secret=op-s3cr3t | jq -r .access_token)
```

**2. Credenciar o EC:**

```bash
curl -s -X POST localhost:8080/v1/merchants -H "Authorization: Bearer $OP" -H 'Content-Type: application/json' -d '{
  "document":"11.222.333/0001-81","legal_name":"Padaria Pão Quente LTDA","mcc":"5462",
  "bank_account":{"code":"341","branch":"0001","number":"12345-6","kind":"CHECKING"},
  "fee_plan":{"debit_bps":150,"credit_bps":250,"credit_installments_bps":{"2":350,"3":350,"6":350,"12":450},"anticipation_bps_month":200}
}' | tee /tmp/m.json; MID=$(jq -r .id /tmp/m.json)
```

O status vem `UNDER_REVIEW`. Tente autorizar com ele e você recebe `422 MERCHANT_INACTIVE`: é o KYC da Parte 2.5 funcionando.

**3. Aprovar** (scope `merchants:approve`):

```bash
curl -s -X POST localhost:8080/v1/merchants/$MID/approve -H "Authorization: Bearer $OP" | jq .status
```

**4. Dar ao EC uma credencial:** edite `certs/clients.json`, troque `SUBSTITUA_PELO_ID_DO_EC` pelo `$MID` e reinicie o `auth-sim` (`docker compose restart auth-sim`). Depois:

```bash
TK=$(curl -s -X POST localhost:8082/oauth/token -d grant_type=client_credentials -d client_id=client_padaria -d client_secret=padaria-s3cr3t | jq -r .access_token)
```

**5. Tokenizar um cartão** (o PAN só passa aqui):

```bash
TOKEN=$(curl -s -X POST localhost:8080/v1/card-tokens -H "Authorization: Bearer $TK" -H 'Content-Type: application/json' \
  -d '{"pan":"4111111111111111","expiry":"12/30"}' | jq -r .token)
```

**6. Autorizar R$ 1.200,00 em 12x** (com `Idempotency-Key`):

```bash
KEY=$(uuidgen)
curl -s -X POST localhost:8080/v1/transactions -H "Authorization: Bearer $TK" -H "Idempotency-Key: $KEY" -H 'Content-Type: application/json' \
  -d "{\"amount\":120000,\"product\":\"CREDIT\",\"installments\":12,\"card_token\":\"$TOKEN\"}" | tee /tmp/tx.json
TX=$(jq -r .id /tmp/tx.json)
```

Repita o mesmo `curl` com a mesma `$KEY`: volta a **mesma** resposta e o header `Idempotent-Replayed: true`. Troque o valor mantendo a chave: `409 IDEMPOTENCY_KEY_REUSED`.

**7. Capturar:**

```bash
curl -s -X POST localhost:8080/v1/transactions/$TX/capture -H "Authorization: Bearer $TK" -H "Idempotency-Key: $(uuidgen)" | jq .status
```

**8. Ver a agenda** (o `scheduler` consumiu `transaction.captured` do Kafka e gerou 12 recebíveis):

```bash
curl -s "localhost:8080/v1/receivables?status=SCHEDULED" -H "Authorization: Bearer $TK" | jq '.data[] | {installment_no, due_date, net_amount}'
```

Compare com a tabela da Parte 1.4. Abra o Kafka UI (`localhost:8081`) e veja os tópicos `transaction.authorized`, `transaction.captured`, `receivable.scheduled`. Abra o Jaeger (`localhost:16686`) e veja o trace da autorização: API → vault → issuer-sim → Postgres.

**9. Antecipar as 3 primeiras parcelas:**

```bash
IDS=$(curl -s "localhost:8080/v1/receivables?status=SCHEDULED&limit=3" -H "Authorization: Bearer $TK" | jq -c '[.data[].id]')
curl -s -X POST localhost:8080/v1/anticipations/simulate -H "Authorization: Bearer $TK" -H 'Content-Type: application/json' -d "{\"receivable_ids\":$IDS}" | jq '{gross_amount,discount_amount,net_amount}'
curl -s -X POST localhost:8080/v1/anticipations -H "Authorization: Bearer $TK" -H "Idempotency-Key: $(uuidgen)" -H 'Content-Type: application/json' -d "{\"receivable_ids\":$IDS}" | jq .status
```

**10. Liquidar** a data da primeira parcela (veja em `due_date`):

```bash
make settle DATE=2026-10-14      # ajuste para a data da sua agenda
ls out/settlements/               # o "arquivo de liquidação"
curl -s localhost:8080/v1/settlements -H "Authorization: Bearer $TK" | jq .
```

Rode de novo com a mesma data: nada é pago em dobro (o lock e os estados garantem).

**11. Chargeback** (rota interna, token de operador):

```bash
curl -s -X POST localhost:8080/internal/chargebacks -H "Authorization: Bearer $OP" -H 'Content-Type: application/json' \
  -d "{\"merchant_id\":\"$MID\",\"transaction_id\":\"$TX\",\"reason_code\":\"4837\",\"amount\":120000}" | jq .
```

**Os "botões" do emissor simulado** para testar cada caminho:

| Cartão | Resultado |
|---|---|
| `4111111111111111` | aprovada |
| `4111111111110000` | negada `05` (não honrar) |
| qualquer, valor > R$ 5.000,00 | negada `51` (limite) |
| `4111111111119999` | emissor demora 5s → timeout de 2s → **reversal** + negada `91` |

Para testar **ônus**, coloque em `certs/liens.json` uma entrada para o `$MID` na data de uma parcela e rode a liquidação: parte do valor vai para o credor (veja no arquivo gerado).

**Pronto quando:** você consegue explicar, olhando o Jaeger e o Kafka UI, o caminho de cada real desde a maquininha até a conta do lojista.

---

## Fase 9 — Kubernetes

**Objetivo:** tudo rodando num cluster, com deploy sem queda. Releia a Parte 14.

#### `deploy/k8s/base/kustomization.yaml`

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: adquirente
resources:
  - namespace.yaml
  - config.yaml
  - api.yaml
  - vault.yaml
  - issuer-sim.yaml
  - scheduler.yaml
  - notifier.yaml
  - settlement-cronjob.yaml
  - ingress.yaml
```

#### `deploy/k8s/base/namespace.yaml`

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: adquirente
  labels:
    # Pod Security Standards: nenhum pod privilegiado neste namespace
    pod-security.kubernetes.io/enforce: restricted
```

#### `deploy/k8s/base/config.yaml`

```yaml
# Configuração NÃO sensível. Os segredos (DATABASE_URL, VAULT_KEY_BASE64, WEBHOOK_SECRET) vêm de um Secret
# criado fora do git — em produção, pelo External Secrets Operator lendo do AWS Secrets Manager (Parte 16).
apiVersion: v1
kind: ConfigMap
metadata:
  name: adquirente-config
data:
  APP_ENV: prod
  LOG_LEVEL: info
  KAFKA_BROKERS: kafka-bootstrap:9092
  REDIS_ADDR: redis:6379
  ISSUER_ADDR: issuer-sim:9090
  VAULT_ADDR: vault:9090
  OTLP_ENDPOINT: otel-collector.observability:4317
  JWT_ISSUER: https://auth.adquirente.com
  JWT_AUDIENCE: adquirente-api
  JWT_PUBLIC_KEY_FILE: /etc/adquirente/jwt-public.pem
  TLS_CERT_FILE: /etc/tls/tls.crt
  TLS_KEY_FILE: /etc/tls/tls.key
  TLS_CA_FILE: /etc/tls/ca.crt
---
# Exemplo de Secret (valores fictícios, base64). NUNCA commite um Secret real.
apiVersion: v1
kind: Secret
metadata:
  name: adquirente-secrets
type: Opaque
stringData:
  DATABASE_URL: postgres://user:senha@rds-host:5432/adquirente?sslmode=verify-full
  WEBHOOK_SECRET: troque-me
  VAULT_KEY_BASE64: troque-me
```

#### `deploy/k8s/base/api.yaml`

Deployment sem queda, probes, HPA, PDB, `topologySpreadConstraints`, securityContext restrito.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels: { app: api }
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate: { maxUnavailable: 0, maxSurge: 1 }   # deploy sem queda
  selector:
    matchLabels: { app: api }
  template:
    metadata:
      labels: { app: api }
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: api                              # identidade própria (IRSA na AWS)
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        seccompProfile: { type: RuntimeDefault }
      topologySpreadConstraints:                           # espalha réplicas por zona
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector: { matchLabels: { app: api } }
      containers:
        - name: api
          image: ghcr.io/seu-usuario/adquirente-api:1.0.0   # tag imutável, nunca :latest
          ports: [{ name: http, containerPort: 8080 }]
          envFrom:
            - configMapRef: { name: adquirente-config }
            - secretRef: { name: adquirente-secrets }
          env:
            - { name: HTTP_ADDR, value: ":8080" }
            - { name: SERVICE_NAME, value: api }
          volumeMounts:
            - { name: jwt-public, mountPath: /etc/adquirente, readOnly: true }
            - { name: tls, mountPath: /etc/tls, readOnly: true }
          resources:
            requests: { cpu: 250m, memory: 128Mi }
            limits:   { cpu: "1",  memory: 512Mi }
          readinessProbe:
            httpGet: { path: /health/ready, port: http }
            periodSeconds: 5
            failureThreshold: 3
          livenessProbe:
            httpGet: { path: /health/live, port: http }
            periodSeconds: 10
            failureThreshold: 3
          lifecycle:
            preStop:
              exec: { command: ["/bin/sh", "-c", "sleep 5"] }   # distroless não tem sh: em prod use sleep via init ou remova
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: ["ALL"] }
      terminationGracePeriodSeconds: 30
      volumes:
        - name: jwt-public
          configMap: { name: jwt-public-key }
        - name: tls
          secret: { secretName: api-mtls }                 # emitido pelo cert-manager
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: api
---
apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  selector: { app: api }
  ports: [{ name: http, port: 80, targetPort: http }]
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api
spec:
  scaleTargetRef: { apiVersion: apps/v1, kind: Deployment, name: api }
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource: { name: cpu, target: { type: Utilization, averageUtilization: 60 } }
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: api
spec:
  minAvailable: 2
  selector:
    matchLabels: { app: api }
```

#### `deploy/k8s/base/vault.yaml`

A `NetworkPolicy` aqui é a regra PCI mais importante do cluster.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vault
  labels: { app: vault }
spec:
  replicas: 2
  selector:
    matchLabels: { app: vault }
  template:
    metadata:
      labels: { app: vault }
    spec:
      serviceAccountName: vault
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        seccompProfile: { type: RuntimeDefault }
      containers:
        - name: vault
          image: ghcr.io/seu-usuario/adquirente-vault:1.0.0
          ports: [{ name: grpc, containerPort: 9090 }]
          envFrom:
            - configMapRef: { name: adquirente-config }
            - secretRef: { name: adquirente-secrets }
          env:
            - { name: GRPC_ADDR, value: ":9090" }
          volumeMounts:
            - { name: tls, mountPath: /etc/tls, readOnly: true }
          resources:
            requests: { cpu: 100m, memory: 64Mi }
            limits:   { cpu: 500m, memory: 256Mi }
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: ["ALL"] }
      volumes:
        - name: tls
          secret: { secretName: vault-mtls }
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: vault
---
apiVersion: v1
kind: Service
metadata:
  name: vault
spec:
  selector: { app: vault }
  ports: [{ name: grpc, port: 9090, targetPort: grpc }]
---
# A regra mais importante do PCI no cluster: SÓ a API fala com o vault. Nada mais.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vault-ingress
spec:
  podSelector:
    matchLabels: { app: vault }
  policyTypes: [Ingress, Egress]
  ingress:
    - from:
        - podSelector: { matchLabels: { app: api } }
      ports: [{ protocol: TCP, port: 9090 }]
  egress:
    - to:                                   # só o banco do vault e o DNS
        - ipBlock: { cidr: 10.0.0.0/8 }
      ports:
        - { protocol: TCP, port: 5432 }
        - { protocol: UDP, port: 53 }
        - { protocol: TCP, port: 53 }
```

#### `deploy/k8s/base/issuer-sim.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: issuer-sim
  labels: { app: issuer-sim }
spec:
  replicas: 1
  selector:
    matchLabels: { app: issuer-sim }
  template:
    metadata:
      labels: { app: issuer-sim }
    spec:
      securityContext: { runAsNonRoot: true, seccompProfile: { type: RuntimeDefault } }
      containers:
        - name: issuer-sim
          image: ghcr.io/seu-usuario/adquirente-issuer-sim:1.0.0
          ports: [{ name: grpc, containerPort: 9090 }]
          envFrom: [{ configMapRef: { name: adquirente-config } }]
          env:
            - { name: GRPC_ADDR, value: ":9090" }
            - { name: ISSUER_SIM_LATENCY_MS, value: "50" }
          volumeMounts: [{ name: tls, mountPath: /etc/tls, readOnly: true }]
          resources:
            requests: { cpu: 50m, memory: 32Mi }
            limits:   { cpu: 200m, memory: 128Mi }
          securityContext: { allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: { drop: ["ALL"] } }
      volumes:
        - name: tls
          secret: { secretName: issuer-sim-mtls }
---
apiVersion: v1
kind: Service
metadata:
  name: issuer-sim
spec:
  selector: { app: issuer-sim }
  ports: [{ name: grpc, port: 9090, targetPort: grpc }]
```

#### `deploy/k8s/base/scheduler.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scheduler
  labels: { app: scheduler }
spec:
  replicas: 2                       # 2 instâncias no mesmo consumer group dividem as partições
  selector:
    matchLabels: { app: scheduler }
  template:
    metadata:
      labels: { app: scheduler }
    spec:
      serviceAccountName: scheduler
      automountServiceAccountToken: false
      securityContext: { runAsNonRoot: true, seccompProfile: { type: RuntimeDefault } }
      containers:
        - name: scheduler
          image: ghcr.io/seu-usuario/adquirente-scheduler:1.0.0
          envFrom:
            - configMapRef: { name: adquirente-config }
            - secretRef: { name: adquirente-secrets }
          resources:
            requests: { cpu: 100m, memory: 64Mi }
            limits:   { cpu: 500m, memory: 256Mi }
          securityContext: { allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: { drop: ["ALL"] } }
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: scheduler
```

#### `deploy/k8s/base/notifier.yaml`

O único pod com saída para a internet, e mesmo assim só porta 443 e nunca para redes privadas.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notifier
  labels: { app: notifier }
spec:
  replicas: 2
  selector:
    matchLabels: { app: notifier }
  template:
    metadata:
      labels: { app: notifier }
    spec:
      serviceAccountName: notifier
      automountServiceAccountToken: false
      securityContext: { runAsNonRoot: true, seccompProfile: { type: RuntimeDefault } }
      containers:
        - name: notifier
          image: ghcr.io/seu-usuario/adquirente-notifier:1.0.0
          envFrom:
            - configMapRef: { name: adquirente-config }
            - secretRef: { name: adquirente-secrets }
          resources:
            requests: { cpu: 50m, memory: 64Mi }
            limits:   { cpu: 300m, memory: 256Mi }
          securityContext: { allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: { drop: ["ALL"] } }
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: notifier
---
# O notifier é o ÚNICO pod que fala com a internet (webhooks dos ECs). Egress liberado só para ele.
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: notifier-egress
spec:
  podSelector:
    matchLabels: { app: notifier }
  policyTypes: [Egress]
  egress:
    - to: [{ ipBlock: { cidr: 0.0.0.0/0, except: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.169.254/32"] } }]
      ports: [{ protocol: TCP, port: 443 }]
    - to: [{ ipBlock: { cidr: 10.0.0.0/8 } }]      # banco, kafka, dns internos
```

#### `deploy/k8s/base/settlement-cronjob.yaml`

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: settlement
spec:
  schedule: "0 9 * * 1-5"            # 09:00 UTC = 06:00 em Brasília, segunda a sexta
  timeZone: "America/Sao_Paulo"      # K8s ≥ 1.27: então pode usar "0 6 * * 1-5" diretamente
  concurrencyPolicy: Forbid          # NUNCA duas liquidações ao mesmo tempo (o lock no Redis é a 2ª barreira)
  successfulJobsHistoryLimit: 7
  failedJobsHistoryLimit: 7
  jobTemplate:
    spec:
      backoffLimit: 2
      activeDeadlineSeconds: 3600
      template:
        spec:
          restartPolicy: Never
          serviceAccountName: settlement
          automountServiceAccountToken: false
          securityContext: { runAsNonRoot: true, seccompProfile: { type: RuntimeDefault } }
          containers:
            - name: settlement
              image: ghcr.io/seu-usuario/adquirente-settlement:1.0.0
              envFrom:
                - configMapRef: { name: adquirente-config }
                - secretRef: { name: adquirente-secrets }
              env:
                - { name: SETTLEMENT_OUTPUT_DIR, value: /out }
              volumeMounts: [{ name: out, mountPath: /out }]
              resources:
                requests: { cpu: 250m, memory: 128Mi }
                limits:   { cpu: "1",  memory: 512Mi }
              securityContext: { allowPrivilegeEscalation: false, readOnlyRootFilesystem: true, capabilities: { drop: ["ALL"] } }
          volumes:
            - name: out
              emptyDir: {}            # em produção: os arquivos vão para o S3 (Parte 16), não para disco local
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: settlement
```

#### `deploy/k8s/base/ingress.yaml`

```yaml
# Na AWS, este Ingress vira um Application Load Balancer via AWS Load Balancer Controller,
# com TLS do ACM e o WAF criado pelo Terraform (Parte 16). As anotações fazem essa ligação.
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-policy: ELBSecurityPolicy-TLS13-1-2-2021-06
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:sa-east-1:123456789012:certificate/SUBSTITUA
    alb.ingress.kubernetes.io/wafv2-acl-arn: arn:aws:wafv2:sa-east-1:123456789012:regional/webacl/adquirente-api/SUBSTITUA
    alb.ingress.kubernetes.io/healthcheck-path: /health/ready
    alb.ingress.kubernetes.io/load-balancer-attributes: access_logs.s3.enabled=true,access_logs.s3.bucket=adquirente-prod-alb-logs
spec:
  rules:
    - host: api.adquirente.com
      http:
        paths:
          - path: /v1
            pathType: Prefix
            backend:
              service: { name: api, port: { name: http } }
          - path: /health
            pathType: Prefix
            backend:
              service: { name: api, port: { name: http } }
          # /metrics e /internal NÃO estão expostos: só pela rede interna.
```

**Localmente, com kind:**

```bash
kind create cluster --name adquirente
for s in api vault issuer-sim scheduler settlement notifier; do
  docker build --build-arg SERVICE=$s -t ghcr.io/seu-usuario/adquirente-$s:1.0.0 -f deploy/docker/Dockerfile .
  kind load docker-image ghcr.io/seu-usuario/adquirente-$s:1.0.0 --name adquirente
done
kubectl create namespace adquirente
kubectl -n adquirente create configmap jwt-public-key --from-file=jwt-public.pem=certs/jwt-public.pem
kubectl apply -k deploy/k8s/base
kubectl -n adquirente get pods -w
```

Postgres, Redis e Kafka no kind: use os charts Helm do Bitnami ou os operadores (CloudNativePG, Strimzi); em produção eles ficam **fora** do cluster (RDS, ElastiCache, MSK: Parte 16). Os Secrets `*-mtls` vêm do cert-manager (`helm install cert-manager ...` + um `Issuer` interno).

**Teste de deploy sem queda:** rode um `k6`/`hey` contra a API e, no meio, `kubectl -n adquirente rollout restart deploy/api`. Zero erros.

---

## Fase 10 — Endurecimento e compliance

Feche o checklist da Parte 2.8 apontando para o que você construiu:

| Exigência | Onde está |
|---|---|
| Nunca guardar CVV; PAN só tokenizado | `vault.proto` sem CVV; `card_vault` cifrado; nenhuma outra tabela com PAN |
| Mascarar PAN em toda saída | `transaction.MaskPAN`, tipo `PAN` com `LogValue`; DTOs só com `card_last4` |
| Trilha de auditoria | `ledger_entries`, `outbox`, `processed_events` append-only; logs do vault |
| Verificar ônus antes de liquidar | `settle_due.go` chama `registry.CheckLiens` antes de `settlement.Build` |
| Segregar dinheiro do EC da receita | contas `payable_to_merchant` vs `revenue_*` no ledger |
| Credenciamento com análise | `UNDER_REVIEW` + `POST /merchants/:id/approve` com scope próprio |
| Idempotência em todo POST financeiro | middleware + `idempotency_keys` |
| TLS em tudo | mTLS nos gRPC (`tls.go`), Ingress TLS 1.3, `rds.force_ssl` (Parte 16) |
| Segredos fora do código | `config.go` lê do ambiente; Secrets/External Secrets no K8s |
| Menor privilégio de rede | `NetworkPolicy` do vault e do notifier |
| Registrar acesso a dados sensíveis | logs do `Detokenize` com token/last4 |
| Dependências auditadas | `make lint` (`govulncheck`); scan de imagem no ECR (Parte 16) |

O que ficou como exercício (e é o que você faria numa adquirente real em seguida): tabela `webhook_deliveries` com retries persistentes; job de reconciliação registradora ↔ agenda; anonimização LGPD após o prazo legal; marcar a transação como `SETTLED` quando o último recebível liquidar; antecipação automática no `scheduler`; painel interno com OIDC.

---

# PARTE 16 — A nuvem: AWS com Terraform, segurança em primeiro lugar

Até aqui a adquirente roda no seu computador (Docker Compose) ou num cluster local (kind). Uma adquirente de verdade roda em **produção**: várias máquinas, em prédios diferentes, com banco replicado, backups, firewall, auditoria de tudo e alguém sendo acordado quando algo quebra. Esta parte constrói essa infraestrutura na **AWS**, escrita como código com **Terraform**, e explica cada decisão.

**Todo o Terraform desta parte passou em `terraform fmt` e `terraform validate` com o provider AWS 6.x.** Ele não foi aplicado numa conta real (isso custa dinheiro e exige uma conta sua); a seção 16.19 diz exatamente como aplicar e a 16.22 quanto custa.

## 16.1 O que é "nuvem" e o que é a AWS

Nuvem é alugar computação, armazenamento e rede de alguém que opera datacenters gigantes, pagando pelo uso, e configurando tudo por API em vez de comprar servidor. A **AWS** (Amazon Web Services) é a maior. Vocabulário mínimo:

- **Região**: um conjunto de datacenters numa área geográfica. `sa-east-1` é São Paulo. Dados de pagamento brasileiros ficam aqui (LGPD e boa prática de residência de dados).
- **Zona de disponibilidade (AZ)**: um ou mais datacenters independentes dentro da região (energia, refrigeração e rede próprios). `sa-east-1a`, `1b`, `1c`. Espalhar em 3 AZs significa que um datacenter pode pegar fogo e a adquirente continua.
- **Serviço gerenciado**: a AWS opera o software para você. Em vez de instalar PostgreSQL numa máquina e cuidar de backup, réplica e patch, você usa o **RDS**. Em vez de operar Kafka, **MSK**. Em vez de Kubernetes na unha, **EKS**. Custa mais que a máquina crua, mas custa muito menos que a equipe para operar aquilo com a qualidade que pagamento exige.
- **IAM** (*Identity and Access Management*): quem (pessoa, serviço, pod) pode fazer o quê em qual recurso. É o coração da segurança na AWS.
- **VPC** (*Virtual Private Cloud*): sua rede privada dentro da AWS, com sub-redes, rotas e firewalls.

**Responsabilidade compartilhada.** A AWS é responsável pela segurança **da** nuvem (o datacenter, o hardware, o hipervisor, o software do serviço gerenciado). Você é responsável pela segurança **na** nuvem: quem tem acesso, o que está exposto, se os dados estão cifrados, se os logs estão ligados, se a aplicação tem bugs. Nenhum recurso desta parte existe por acaso: cada um cobre um pedaço da **sua** metade.

## 16.2 Infraestrutura como código e Terraform

**Infraestrutura como código (IaC)** é descrever servidores, redes e bancos em arquivos de texto, versionados no git, e deixar uma ferramenta criar e alterar os recursos. Vantagens que importam para uma adquirente:

- **Reprodutível**: o ambiente de homologação é igual ao de produção porque saiu do mesmo código.
- **Auditável**: cada mudança é um commit, revisado por outra pessoa, com histórico. O Banco Central e o auditor PCI perguntam "quem mudou a regra de firewall e quando?"; a resposta é `git log`.
- **Revisável antes de acontecer**: o Terraform mostra o **plano** (o que vai criar, alterar, destruir) antes de fazer.
- **Sem cliques**: ninguém configura produção pelo console web, na mão, sem registro.

**Terraform** é a ferramenta de IaC mais usada, multi-nuvem. Conceitos:

| Conceito | O que é |
|---|---|
| **Provider** | O plugin que fala com uma nuvem (`hashicorp/aws`). Você fixa a versão (`~> 6.0`). |
| **Resource** | Uma coisa a criar: `resource "aws_s3_bucket" "trail" { ... }`. Tipo, nome interno, argumentos. |
| **Data source** | Uma coisa que já existe e você só lê: `data "aws_route53_zone" "this"`. |
| **Variable / Output** | Entradas e saídas de um módulo. |
| **Module** | Uma pasta de `.tf` reutilizável: `module "network" { source = "../../modules/network" ... }`. |
| **State** | Um arquivo JSON onde o Terraform anota o que criou e os ids reais. **É sensível** (contém tudo, às vezes segredos) e **crítico** (se perder, o Terraform não sabe mais o que é dele). Por isso fica num bucket S3 cifrado, versionado, com lock. |
| **`terraform init`** | Baixa providers e módulos; configura o backend do state. |
| **`terraform plan`** | Compara código × state × realidade e mostra o que faria. Leia sempre. |
| **`terraform apply`** | Executa o plano. |
| **`terraform destroy`** | Apaga tudo. Em produção, protegido por `prevent_destroy` e `deletion_protection`. |
| **`terraform fmt` / `validate`** | Formata e checa sintaxe/tipos sem tocar na nuvem. |

Um `resource` por dentro:

```hcl
resource "aws_s3_bucket" "settlement" {      # tipo do recurso e um nome só para o Terraform
  bucket              = "adquirente-prod-settlement-files-123456789012"   # argumentos
  object_lock_enabled = true
}

resource "aws_s3_bucket_versioning" "settlement" {
  bucket = aws_s3_bucket.settlement.id       # referência: cria a dependência automaticamente
  versioning_configuration { status = "Enabled" }
}
```

O Terraform monta o grafo de dependências pelas referências e cria na ordem certa.

## 16.3 A arquitetura que vamos criar

```
                                internet
                                    │
                         ┌──────────▼──────────┐
                         │  Route 53 (DNS)     │  api.adquirente.com
                         │  ACM (certificado)  │
                         │  WAF (firewall web) │
                         └──────────┬──────────┘
 ┌──────────────────────────────────┼─────────────────── VPC 10.40.0.0/16 (sa-east-1) ──────┐
 │  subnets PÚBLICAS (a,b,c)        ▼                                                       │
 │   ┌────────────────────── Application Load Balancer (TLS 1.3) ──────────────────┐        │
 │   │ NAT gw a │ NAT gw b │ NAT gw c │   (saída para a internet, só dos pods)      │        │
 │   └──────────┴──────────┴──────────┴──────────────────┬───────────────────────────┘        │
 │  subnets PRIVADAS (a,b,c)                              ▼                                   │
 │   ┌─────────────────────────── EKS (nós Bottlerocket/Graviton) ──────────────────┐        │
 │   │  api ×3   scheduler ×2   notifier ×2   vault ×2   issuer-sim   settlement    │        │
 │   │  (NetworkPolicy: só api → vault; só notifier → internet)                      │        │
 │   └─────┬───────────────┬───────────────────────────────────┬──────────────────────┘        │
 │         │               │                                   │                               │
 │   ┌─────▼─────┐   ┌─────▼──────────┐              ┌─────────▼──────────┐                    │
 │   │  MSK      │   │ VPC endpoints  │              │  ECR (imagens)      │                    │
 │   │  Kafka ×3 │   │ S3 ECR KMS     │              │  scan, imutável     │                    │
 │   └───────────┘   │ Secrets Logs   │              └────────────────────┘                    │
 │                   └────────────────┘                                                         │
 │  subnets ISOLADAS (a,b,c)  — SEM rota para a internet                                        │
 │   ┌───────────────────────┐  ┌───────────────────────┐  ┌───────────────────┐               │
 │   │ RDS PostgreSQL "core" │  │ RDS PostgreSQL "vault"│  │ ElastiCache Redis │               │
 │   │ Multi-AZ, KMS, TLS    │  │ Multi-AZ, KMS, TLS    │  │ Multi-AZ, TLS,auth│               │
 │   └───────────────────────┘  └───────────────────────┘  └───────────────────┘               │
 └──────────────────────────────────────────────────────────────────────────────────────────────┘
   Em volta de tudo: KMS (chaves), CloudTrail (auditoria), GuardDuty (ameaças), Config (conformidade),
   Security Hub (painel), Secrets Manager (segredos), CloudWatch (logs, alarmes), S3 Object Lock (liquidação)
```

Mapa componente do projeto → serviço AWS:

| No seu computador (Compose) | Na AWS | Módulo Terraform |
|---|---|---|
| `postgres` | RDS PostgreSQL Multi-AZ (um para o core, outro para o vault) | `database` |
| `redis` | ElastiCache Redis (Multi-AZ, TLS, senha) | `cache` |
| `kafka` | MSK (3 brokers, TLS, IAM) | `kafka` |
| imagens Docker locais | ECR (scan, tags imutáveis) | `registry` |
| kind | EKS (API privada, Bottlerocket, Pod Identity) | `eks` |
| `out/settlements/*.json` | S3 com Object Lock (5 anos) | `storage` |
| `certs/*.pem`, `.env` | KMS + Secrets Manager + External Secrets | `security`, `database`, `cache` |
| `localhost:8080` | Route 53 + ACM + WAF + ALB | `edge` |
| Jaeger/Prometheus/Grafana | continuam no cluster (Helm) + CloudWatch para a infra | `observability` |
| ninguém olhando | CloudTrail, GuardDuty, Config, Security Hub, alarmes, orçamento | `security`, `observability` |

## 16.4 Princípios de segurança na nuvem (o que guia cada linha do Terraform)

1. **Menor privilégio, sempre.** Cada pod, cada pessoa, cada serviço só tem a permissão exata. Nunca `*`. O worker de liquidação pode `s3:PutObject` num bucket; não pode listar, ler nem apagar.
2. **Identidade em vez de segredo.** Pods assumem uma role IAM (Pod Identity) em vez de carregar access keys. A aplicação conecta no banco com token IAM ou lê a senha do Secrets Manager em runtime; nunca há senha em variável de ambiente no git.
3. **Rede em camadas.** Pública só para o balanceador. Privada para os pods. Isolada (sem rota para fora) para dados. Security groups referenciam **outros security groups**, não CIDRs: "o banco aceita conexão dos nós do EKS", e ponto.
4. **Tudo cifrado, com as nossas chaves.** Em repouso: KMS em RDS, EBS, S3, MSK, ElastiCache, logs, Secrets do Kubernetes. Em trânsito: TLS obrigatório (`rds.force_ssl`, MSK `TLS`, Redis `transit_encryption`, ALB TLS 1.3, mTLS entre pods). Chaves separadas por finalidade, com rotação automática.
5. **Tudo logado, e o log é imutável.** CloudTrail (API da AWS), VPC Flow Logs (rede), logs do EKS (API do Kubernetes), logs do ALB e do WAF, pgaudit no Postgres. Logs de auditoria em S3 com **Object Lock**: nem o administrador apaga.
6. **Detecção e resposta.** GuardDuty (ameaças), Config (desvios de conformidade), Security Hub (painel, benchmark PCI), alarmes (uso do root, partições offline, disco cheio) que acordam alguém.
7. **Imutabilidade.** Tags de imagem imutáveis no ECR; SO dos nós imutável (Bottlerocket); infraestrutura só muda via código revisado.
8. **Separação.** Cofre de cartões em banco separado, acessível só pelo serviço vault (NetworkPolicy + security group). Ambientes (dev/homolog/prod) em **contas AWS separadas** (16.23).
9. **Sem porta aberta desnecessária.** Nenhum SSH: acesso a nós via SSM Session Manager (auditado). API do EKS privada. Brokers e bancos sem IP público. IMDSv2 obrigatório nos nós.
10. **Backup testado e recuperável.** RDS com PITR de 35 dias e snapshot final; S3 versionado; state do Terraform versionado. "Backup que nunca foi restaurado não é backup."

## 16.5 Antes do Terraform: preparar a conta (manual, uma vez)

Algumas coisas precisam ser feitas no console, por uma pessoa, antes de qualquer automação:

1. **Root**: ative MFA com chave física (FIDO2/YubiKey), **apague** qualquer access key do root, guarde a senha num cofre da empresa. O root não é usado no dia a dia; o alarme `root-usage` da 16.18 dispara se for.
2. **IAM Identity Center (SSO)**: pessoas entram por SSO com MFA, assumindo roles temporárias. Nenhum usuário IAM com senha para humanos.
3. **Organização multi-conta** (16.23): pelo menos `security` (logs), `prod`, `staging`, `dev`. Este guia mostra uma conta; o desenho escala para várias.
4. **Região**: fixe `sa-east-1`. Opcional: desative regiões que você não usa (menos superfície).
5. **Orçamento e alerta de custo**: antes de criar qualquer recurso (o módulo `observability` também cria um).
6. **Uma identidade para o Terraform**: em CI, uma role assumida via OIDC pelo GitHub Actions (16.21), sem chaves estáticas. Localmente, sua sessão SSO (`aws sso login`).

## 16.6 A estrutura dos arquivos

```
deploy/terraform/
├── bootstrap/main.tf            # cria o bucket do state (roda uma vez, state local)
├── envs/prod/                   # o ambiente: compõe os módulos
│   ├── versions.tf              # versões, backend S3, provider, tags padrão
│   ├── variables.tf
│   ├── main.tf                  # chama os módulos e liga saídas a entradas
│   ├── outputs.tf
│   └── prod.tfvars              # valores deste ambiente
└── modules/
    ├── security/main.tf         # KMS, CloudTrail, GuardDuty, Config, Security Hub, travas de conta
    ├── network/main.tf          # VPC, 3×3 subnets, NAT, rotas, endpoints, flow logs
    ├── storage/main.tf          # S3: arquivos de liquidação (Object Lock) e logs de acesso
    ├── registry/main.tf         # ECR por serviço
    ├── eks/main.tf              # cluster, nós, addons, Pod Identity, roles mínimas
    ├── database/main.tf         # RDS PostgreSQL core + vault
    ├── cache/main.tf            # ElastiCache Redis
    ├── kafka/main.tf            # MSK
    ├── edge/main.tf             # ACM, WAF, DNS
    └── observability/main.tf    # SNS, alarmes, orçamento
```

Cada módulo tem entradas (`variable`), recursos e saídas (`output`) num único `main.tf`, para você ler de cima a baixo. Em projetos maiores, separe em `variables.tf`, `outputs.tf`, etc.

## 16.7 Bootstrap: onde o state mora

#### `deploy/terraform/bootstrap/main.tf`

Leia os comentários: cada bloco protege o state de um jeito (cifra, versão, sem público, só TLS, sem destruir).

```hcl
# BOOTSTRAP: cria onde o Terraform guarda o próprio estado (o "state").
# Roda UMA vez, com credenciais de administrador, com state local. Depois disso, todo o resto usa o backend S3.
#
# Por que isso importa para segurança: o state contém tudo sobre a infraestrutura (inclusive segredos que
# passam por ele). Ele precisa estar cifrado, versionado, sem acesso público e com lock contra escrita concorrente.

terraform {
  required_version = ">= 1.10"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = var.region
  default_tags {
    tags = { Project = "adquirente", ManagedBy = "terraform", Component = "bootstrap" }
  }
}

variable "region" {
  type    = string
  default = "sa-east-1" # São Paulo: dados de pagamento brasileiros ficam no Brasil
}

variable "state_bucket_name" {
  type        = string
  description = "Nome globalmente único do bucket do state, ex.: adquirente-prod-tfstate-123456789012"
}

# Chave KMS própria para o state: quem pode ler o state é quem pode usar esta chave.
resource "aws_kms_key" "tfstate" {
  description             = "Cifra o Terraform state"
  enable_key_rotation     = true # a AWS troca o material da chave todo ano, sem você fazer nada
  deletion_window_in_days = 30
}

resource "aws_kms_alias" "tfstate" {
  name          = "alias/adquirente-tfstate"
  target_key_id = aws_kms_key.tfstate.key_id
}

resource "aws_s3_bucket" "tfstate" {
  bucket = var.state_bucket_name
  lifecycle {
    prevent_destroy = true # um terraform destroy acidental não pode apagar o state
  }
}

# Versionamento: todo apply gera uma versão nova; dá para voltar se alguém corromper o state.
resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.tfstate.arn
    }
    bucket_key_enabled = true
  }
}

# Bloqueio TOTAL de acesso público. Vale para qualquer bucket deste projeto.
resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Política: recusa qualquer acesso que não seja por TLS.
data "aws_iam_policy_document" "tfstate" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.tfstate.arn, "${aws_s3_bucket.tfstate.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  policy = data.aws_iam_policy_document.tfstate.json
}

output "state_bucket" { value = aws_s3_bucket.tfstate.bucket }
output "state_kms_key_arn" { value = aws_kms_key.tfstate.arn }
```

```bash
cd deploy/terraform/bootstrap
terraform init
terraform apply -var state_bucket_name=adquirente-prod-tfstate-$(aws sts get-caller-identity --query Account --output text)
```

Anote o nome do bucket: ele vai no `backend "s3"` do ambiente.

## 16.8 O ambiente `prod`

#### `deploy/terraform/envs/prod/versions.tf`

Fixa versões (para o `plan` de hoje ser igual ao de amanhã), configura o backend S3 com lock nativo (`use_lockfile`, Terraform ≥ 1.10) e aplica **tags em todo recurso**: `Project`, `Environment`, `ManagedBy`, `DataClass=payments`. Tags são como você acha, cobra e aplica políticas depois.

```hcl
terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }

  # O state fica no bucket criado pelo bootstrap. use_lockfile (Terraform >= 1.10) usa o próprio S3
  # para o lock: dois applies ao mesmo tempo não se atropelam.
  backend "s3" {
    bucket       = "adquirente-prod-tfstate-SUBSTITUA"
    key          = "envs/prod/terraform.tfstate"
    region       = "sa-east-1"
    encrypt      = true
    kms_key_id   = "alias/adquirente-tfstate"
    use_lockfile = true
  }
}

provider "aws" {
  region = var.region

  # Tags em TUDO: custo por componente, dono, ambiente. Auditoria e FinOps agradecem.
  default_tags {
    tags = {
      Project     = "adquirente"
      Environment = var.environment
      ManagedBy   = "terraform"
      DataClass   = "payments" # sinaliza dados de pagamento para políticas de compliance
    }
  }
}
```

#### `deploy/terraform/envs/prod/variables.tf`

```hcl
variable "region" {
  type    = string
  default = "sa-east-1"
}

variable "environment" {
  type    = string
  default = "prod"
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
}

variable "domain_name" {
  type        = string
  description = "Domínio da API, ex.: api.adquirente.com (a zona Route53 já deve existir)"
}

variable "route53_zone_name" {
  type        = string
  description = "Nome da zona hospedada, ex.: adquirente.com"
}

variable "alert_email" {
  type        = string
  description = "E-mail que recebe os alarmes (confirme a inscrição do SNS)"
}

variable "eks_version" {
  type    = string
  default = "1.31"
}

variable "monthly_budget_usd" {
  type    = number
  default = 3000
}
```

#### `deploy/terraform/envs/prod/main.tf`

A composição. Repare como as saídas de um módulo viram entradas de outro (`module.security.kms_data_arn` → `database`), e como o EKS recebe **só os ARNs dos segredos** que pode ler.

```hcl
# O ambiente de produção é a COMPOSIÇÃO dos módulos. Cada módulo é uma peça com uma responsabilidade;
# aqui só ligamos as saídas de um às entradas do outro.

locals {
  name = "adquirente-${var.environment}"
  azs  = ["${var.region}a", "${var.region}b", "${var.region}c"] # 3 zonas: uma cai, duas continuam
}

module "security" {
  source = "../../modules/security"
  name   = local.name
}

module "network" {
  source           = "../../modules/network"
  name             = local.name
  cidr             = var.vpc_cidr
  azs              = local.azs
  flow_log_kms_arn = module.security.kms_logs_arn
}

module "storage" {
  source      = "../../modules/storage"
  name        = local.name
  kms_key_arn = module.security.kms_data_arn
}

module "registry" {
  source      = "../../modules/registry"
  name        = local.name
  services    = ["api", "vault", "issuer-sim", "scheduler", "settlement", "notifier"]
  kms_key_arn = module.security.kms_data_arn
}

module "eks" {
  source             = "../../modules/eks"
  name               = local.name
  cluster_version    = var.eks_version
  vpc_id             = module.network.vpc_id
  private_subnet_ids = module.network.private_subnet_ids
  kms_secrets_arn    = module.security.kms_eks_arn
  log_kms_arn        = module.security.kms_logs_arn
  secrets_arns       = [module.database.secret_arn["core"], module.database.secret_arn["vault"], module.cache.auth_secret_arn]
  settlement_bucket  = module.storage.settlement_bucket_arn
}

module "database" {
  source              = "../../modules/database"
  name                = local.name
  vpc_id              = module.network.vpc_id
  subnet_ids          = module.network.isolated_subnet_ids
  allowed_sg_ids      = [module.eks.node_security_group_id]
  kms_key_arn         = module.security.kms_data_arn
  performance_kms_arn = module.security.kms_data_arn
  instances = {
    core  = { instance_class = "db.r6g.large", allocated_storage = 100 }
    vault = { instance_class = "db.t4g.medium", allocated_storage = 20 } # banco separado para o cofre de cartões
  }
}

module "cache" {
  source         = "../../modules/cache"
  name           = local.name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.isolated_subnet_ids
  allowed_sg_ids = [module.eks.node_security_group_id]
  kms_key_arn    = module.security.kms_data_arn
}

module "kafka" {
  source         = "../../modules/kafka"
  name           = local.name
  vpc_id         = module.network.vpc_id
  subnet_ids     = module.network.private_subnet_ids
  allowed_sg_ids = [module.eks.node_security_group_id]
  kms_key_arn    = module.security.kms_data_arn
  log_kms_arn    = module.security.kms_logs_arn
}

module "edge" {
  source            = "../../modules/edge"
  name              = local.name
  domain_name       = var.domain_name
  route53_zone_name = var.route53_zone_name
  log_kms_arn       = module.security.kms_logs_arn
}

module "observability" {
  source               = "../../modules/observability"
  name                 = local.name
  alert_email          = var.alert_email
  kms_key_arn          = module.security.kms_logs_arn
  db_identifiers       = module.database.identifiers
  msk_cluster_name     = module.kafka.cluster_name
  redis_group_id       = module.cache.replication_group_id
  monthly_budget_usd   = var.monthly_budget_usd
  cloudtrail_log_group = module.security.trail_log_group
}
```

#### `deploy/terraform/envs/prod/outputs.tf`

```hcl
output "eks_cluster_name" { value = module.eks.cluster_name }
output "ecr_repositories" { value = module.registry.repository_urls }
output "rds_endpoints" { value = module.database.endpoints }
output "redis_endpoint" { value = module.cache.primary_endpoint }
output "msk_bootstrap_brokers_sasl_iam" { value = module.kafka.bootstrap_brokers_sasl_iam }
output "acm_certificate_arn" { value = module.edge.certificate_arn }
output "waf_web_acl_arn" { value = module.edge.web_acl_arn }
output "settlement_bucket" { value = module.storage.settlement_bucket_name }
output "alerts_topic_arn" { value = module.observability.alerts_topic_arn }
```

#### `deploy/terraform/envs/prod/prod.tfvars`

```hcl
region             = "sa-east-1"
environment        = "prod"
vpc_cidr           = "10.40.0.0/16"
domain_name        = "api.adquirente.com"
route53_zone_name  = "adquirente.com"
alert_email        = "sre@adquirente.com"
eks_version        = "1.31"
monthly_budget_usd = 3000
```

## 16.9 Módulo `security`: a base

O que cada bloco faz e **por que**:

- **Três chaves KMS** (`data`, `eks`, `logs`) com `enable_key_rotation`. Uma chave por finalidade limita o estrago se uma política de chave for mal configurada e deixa o `kms:Decrypt` auditável por contexto. A chave de logs tem uma política que permite aos serviços de log usá-la.
- **`aws_ebs_encryption_by_default`**: qualquer disco novo na conta nasce cifrado, mesmo que alguém esqueça.
- **`aws_s3_account_public_access_block`**: trava de conta inteira: nenhum bucket pode virar público.
- **Política de senha** para os raros usuários IAM (o certo é SSO).
- **CloudTrail** multi-região, com validação de integridade (hash encadeado) e KMS, gravando num bucket com **Object Lock em modo COMPLIANCE por 1 ano** (o PCI exige logs de auditoria por 12 meses; em COMPLIANCE nem o root apaga). O mesmo trail vai para o CloudWatch Logs, para alarmes em tempo real.
- **GuardDuty**: analisa CloudTrail, Flow Logs e DNS procurando padrões de ataque (credencial usada de um país estranho, instância minerando cripto, exfiltração).
- **AWS Config** + regras gerenciadas: "todo RDS está cifrado?", "algum RDS ficou público?", "o root tem access key?". Se a resposta muda, você sabe.
- **Security Hub** com os padrões *AWS Foundational* e **PCI DSS**: um painel que agrega tudo e mostra o que falta para o benchmark.
- **IAM Access Analyzer**: lista o que está acessível de fora da conta.

#### `deploy/terraform/modules/security/main.tf`

```hcl
# MÓDULO SECURITY: a base de segurança da conta. Chaves KMS, trilha de auditoria, detecção de ameaças,
# avaliação contínua de conformidade e travas de conta.

variable "name" { type = string }

data "aws_caller_identity" "current" {}
data "aws_region" "current" {}

# ---------- KMS: uma chave por finalidade (separação de responsabilidades) ----------
# Se a chave de logs vazar, os dados continuam protegidos; e o acesso a cada chave é auditado separadamente.

resource "aws_kms_key" "data" {
  description             = "${var.name}: dados (RDS, S3, MSK, ElastiCache)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}
resource "aws_kms_alias" "data" {
  name          = "alias/${var.name}-data"
  target_key_id = aws_kms_key.data.key_id
}

resource "aws_kms_key" "eks" {
  description             = "${var.name}: Secrets do Kubernetes (envelope encryption no etcd)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
}
resource "aws_kms_alias" "eks" {
  name          = "alias/${var.name}-eks"
  target_key_id = aws_kms_key.eks.key_id
}

# A chave de logs precisa permitir que o serviço CloudWatch Logs a use.
data "aws_iam_policy_document" "kms_logs" {
  statement {
    sid       = "Root"
    actions   = ["kms:*"]
    resources = ["*"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
  }
  statement {
    sid       = "CloudWatchLogs"
    actions   = ["kms:Encrypt*", "kms:Decrypt*", "kms:ReEncrypt*", "kms:GenerateDataKey*", "kms:Describe*"]
    resources = ["*"]
    principals {
      type        = "Service"
      identifiers = ["logs.${data.aws_region.current.region}.amazonaws.com", "cloudtrail.amazonaws.com", "delivery.logs.amazonaws.com"]
    }
  }
}

resource "aws_kms_key" "logs" {
  description             = "${var.name}: logs (CloudWatch, CloudTrail, Flow Logs, WAF)"
  enable_key_rotation     = true
  deletion_window_in_days = 30
  policy                  = data.aws_iam_policy_document.kms_logs.json
}
resource "aws_kms_alias" "logs" {
  name          = "alias/${var.name}-logs"
  target_key_id = aws_kms_key.logs.key_id
}

# ---------- Travas de conta ----------

# Todo volume EBS novo nasce cifrado, mesmo que alguém esqueça.
resource "aws_ebs_encryption_by_default" "this" { enabled = true }

# Nenhum bucket da conta pode virar público, mesmo que alguém tente.
resource "aws_s3_account_public_access_block" "this" {
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Senhas de usuários IAM (os poucos que existirem; o certo é SSO): fortes e rotacionadas.
resource "aws_iam_account_password_policy" "this" {
  minimum_password_length        = 14
  require_lowercase_characters   = true
  require_uppercase_characters   = true
  require_numbers                = true
  require_symbols                = true
  max_password_age               = 90
  password_reuse_prevention      = 24
  allow_users_to_change_password = true
}

# ---------- CloudTrail: QUEM fez O QUÊ, QUANDO, DE ONDE, em todas as regiões ----------

resource "aws_s3_bucket" "trail" {
  bucket        = "${var.name}-cloudtrail-${data.aws_caller_identity.current.account_id}"
  force_destroy = false
}

resource "aws_s3_bucket_versioning" "trail" {
  bucket = aws_s3_bucket.trail.id
  versioning_configuration { status = "Enabled" }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "trail" {
  bucket = aws_s3_bucket.trail.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = aws_kms_key.logs.arn
    }
  }
}

resource "aws_s3_bucket_public_access_block" "trail" {
  bucket                  = aws_s3_bucket.trail.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Object Lock em modo COMPLIANCE: nem o root apaga um log de auditoria antes do prazo.
resource "aws_s3_bucket_object_lock_configuration" "trail" {
  bucket = aws_s3_bucket.trail.id
  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 1 # PCI DSS: logs de auditoria por pelo menos 1 ano
    }
  }
  depends_on = [aws_s3_bucket_versioning.trail]
}

data "aws_iam_policy_document" "trail_bucket" {
  statement {
    sid       = "AWSCloudTrailAclCheck"
    actions   = ["s3:GetBucketAcl"]
    resources = [aws_s3_bucket.trail.arn]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
  }
  statement {
    sid       = "AWSCloudTrailWrite"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.trail.arn}/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "Service"
      identifiers = ["cloudtrail.amazonaws.com"]
    }
    condition {
      test     = "StringEquals"
      variable = "s3:x-amz-acl"
      values   = ["bucket-owner-full-control"]
    }
  }
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.trail.arn, "${aws_s3_bucket.trail.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "trail" {
  bucket = aws_s3_bucket.trail.id
  policy = data.aws_iam_policy_document.trail_bucket.json
}

# Além do S3 (arquivo imutável), o CloudTrail também envia para o CloudWatch Logs: é o que permite
# alarmes em tempo quase real ("alguém usou o root", "alguém desligou o GuardDuty").
resource "aws_cloudwatch_log_group" "trail" {
  name              = "/aws/cloudtrail/${var.name}"
  retention_in_days = 400
  kms_key_id        = aws_kms_key.logs.arn
}

resource "aws_iam_role" "trail" {
  name = "${var.name}-cloudtrail-logs"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "cloudtrail.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy" "trail" {
  role = aws_iam_role.trail.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents"]
      Resource = "${aws_cloudwatch_log_group.trail.arn}:*"
    }]
  })
}

resource "aws_cloudtrail" "this" {
  name                          = var.name
  s3_bucket_name                = aws_s3_bucket.trail.id
  cloud_watch_logs_group_arn    = "${aws_cloudwatch_log_group.trail.arn}:*"
  cloud_watch_logs_role_arn     = aws_iam_role.trail.arn
  is_multi_region_trail         = true # um atacante que cria recursos em outra região também aparece
  include_global_service_events = true # IAM, STS, Route53
  enable_log_file_validation    = true # hash encadeado: prova que ninguém alterou os logs
  kms_key_id                    = aws_kms_key.logs.arn
  depends_on                    = [aws_s3_bucket_policy.trail]
}

# ---------- GuardDuty: detecção de ameaças (credencial vazada, mineração, exfiltração) ----------
resource "aws_guardduty_detector" "this" {
  enable = true
}

# ---------- AWS Config: registra o estado de cada recurso e avalia regras continuamente ----------
resource "aws_iam_role" "config" {
  name = "${var.name}-config"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "config.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "config" {
  role       = aws_iam_role.config.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWS_ConfigRole"
}

resource "aws_config_configuration_recorder" "this" {
  name     = var.name
  role_arn = aws_iam_role.config.arn
  recording_group {
    all_supported                 = true
    include_global_resource_types = true
  }
}

resource "aws_config_delivery_channel" "this" {
  name           = var.name
  s3_bucket_name = aws_s3_bucket.trail.id
  s3_key_prefix  = "config"
  depends_on     = [aws_config_configuration_recorder.this]
}

resource "aws_config_configuration_recorder_status" "this" {
  name       = aws_config_configuration_recorder.this.name
  is_enabled = true
  depends_on = [aws_config_delivery_channel.this]
}

# Regras gerenciadas: cada uma é uma pergunta que a AWS faz o tempo todo aos seus recursos.
locals {
  config_rules = {
    "rds-storage-encrypted"                  = "RDS_STORAGE_ENCRYPTED"
    "rds-instance-public-access-check"       = "RDS_INSTANCE_PUBLIC_ACCESS_CHECK"
    "s3-bucket-public-read-prohibited"       = "S3_BUCKET_PUBLIC_READ_PROHIBITED"
    "s3-bucket-server-side-encryption"       = "S3_BUCKET_SERVER_SIDE_ENCRYPTION_ENABLED"
    "encrypted-volumes"                      = "ENCRYPTED_VOLUMES"
    "iam-root-access-key-check"              = "IAM_ROOT_ACCESS_KEY_CHECK"
    "root-account-mfa-enabled"               = "ROOT_ACCOUNT_MFA_ENABLED"
    "cloud-trail-encryption-enabled"         = "CLOUD_TRAIL_ENCRYPTION_ENABLED"
    "eks-endpoint-no-public-access"          = "EKS_ENDPOINT_NO_PUBLIC_ACCESS"
    "elasticache-repl-grp-encrypted-at-rest" = "ELASTICACHE_REPL_GRP_ENCRYPTED_AT_REST"
  }
}

resource "aws_config_config_rule" "managed" {
  for_each = local.config_rules
  name     = each.key
  source {
    owner             = "AWS"
    source_identifier = each.value
  }
  depends_on = [aws_config_configuration_recorder_status.this]
}

# ---------- Security Hub: agrega GuardDuty, Config, Inspector num painel e compara com benchmarks ----------
resource "aws_securityhub_account" "this" {}

resource "aws_securityhub_standards_subscription" "aws_foundational" {
  standards_arn = "arn:aws:securityhub:${data.aws_region.current.region}::standards/aws-foundational-security-best-practices/v/1.0.0"
  depends_on    = [aws_securityhub_account.this]
}

resource "aws_securityhub_standards_subscription" "pci" {
  standards_arn = "arn:aws:securityhub:${data.aws_region.current.region}::standards/pci-dss/v/3.2.1"
  depends_on    = [aws_securityhub_account.this]
}

# ---------- IAM Access Analyzer: quem de FORA da conta consegue acessar o quê ----------
resource "aws_accessanalyzer_analyzer" "this" {
  analyzer_name = var.name
  type          = "ACCOUNT"
}

output "kms_data_arn" { value = aws_kms_key.data.arn }
output "kms_eks_arn" { value = aws_kms_key.eks.arn }
output "kms_logs_arn" { value = aws_kms_key.logs.arn }
output "trail_bucket" { value = aws_s3_bucket.trail.bucket }
output "trail_log_group" { value = aws_cloudwatch_log_group.trail.name }
```

## 16.10 Módulo `network`: a VPC em três camadas

Por que três camadas e não uma: a **isolada** não tem rota para `0.0.0.0/0`, nem via NAT. Um banco comprometido não consegue "ligar para fora" para vazar dados; um pod comprometido não consegue alcançar o banco por outro caminho que não o security group. É defesa em profundidade aplicada à rede.

Detalhes que importam:

- **`map_public_ip_on_launch = false`** até na pública: IP público só para quem for explicitamente configurado (o ALB).
- **Um NAT por AZ**: a queda de uma zona não derruba a saída das outras. (Custa três NATs; em dev, use um.)
- **Security group default sem regras**: ninguém cai no default por acidente.
- **VPC Endpoints** para S3, ECR, KMS, Secrets Manager, Logs, STS: o tráfego para esses serviços nunca sai da rede da AWS (menos superfície, menos custo de NAT, e a política de endpoint pode restringir a quais buckets se fala).
- **Flow Logs** de todo tráfego, aceito e rejeitado, por 1 ano, cifrados: é com isso que o GuardDuty e você investigam um incidente ("de onde veio a conexão às 03:12?").
- As tags `kubernetes.io/role/elb` e `internal-elb` são como o AWS Load Balancer Controller descobre em quais subnets criar o ALB.

#### `deploy/terraform/modules/network/main.tf`

```hcl
# MÓDULO NETWORK: a VPC em três camadas, por zona de disponibilidade.
#
#   public   → só o load balancer e os NAT gateways. Nada nosso roda aqui.
#   private  → os nós do EKS (pods). Saem para a internet só pelo NAT (para webhooks e imagens).
#   isolated → banco de dados e cache. SEM rota para a internet, nem via NAT. Se um pod for comprometido,
#              o banco continua alcançável só de dentro; se o banco for comprometido, não consegue "ligar para fora".

variable "name" { type = string }
variable "cidr" { type = string }
variable "azs" { type = list(string) }
variable "flow_log_kms_arn" { type = string }

resource "aws_vpc" "this" {
  cidr_block           = var.cidr
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = var.name }
}

# O security group DEFAULT da VPC fica sem nenhuma regra: ninguém usa "o default" por acidente.
resource "aws_default_security_group" "this" {
  vpc_id = aws_vpc.this.id
  tags   = { Name = "${var.name}-default-DO-NOT-USE" }
}

# /16 dividido: cada AZ recebe um /20 público, um /20 privado e um /20 isolado (4096 IPs cada).
resource "aws_subnet" "public" {
  count                   = length(var.azs)
  vpc_id                  = aws_vpc.this.id
  availability_zone       = var.azs[count.index]
  cidr_block              = cidrsubnet(var.cidr, 4, count.index)
  map_public_ip_on_launch = false # nem na pública ninguém ganha IP público automático
  tags = {
    Name                     = "${var.name}-public-${var.azs[count.index]}"
    "kubernetes.io/role/elb" = "1" # o AWS Load Balancer Controller acha as subnets do ALB por esta tag
  }
}

resource "aws_subnet" "private" {
  count             = length(var.azs)
  vpc_id            = aws_vpc.this.id
  availability_zone = var.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr, 4, count.index + 4)
  tags = {
    Name                              = "${var.name}-private-${var.azs[count.index]}"
    "kubernetes.io/role/internal-elb" = "1"
  }
}

resource "aws_subnet" "isolated" {
  count             = length(var.azs)
  vpc_id            = aws_vpc.this.id
  availability_zone = var.azs[count.index]
  cidr_block        = cidrsubnet(var.cidr, 4, count.index + 8)
  tags              = { Name = "${var.name}-isolated-${var.azs[count.index]}" }
}

# ---------- saída para a internet ----------
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = { Name = var.name }
}

# Um NAT por AZ: a queda de uma zona não derruba a saída das outras.
resource "aws_eip" "nat" {
  count  = length(var.azs)
  domain = "vpc"
  tags   = { Name = "${var.name}-nat-${var.azs[count.index]}" }
}

resource "aws_nat_gateway" "this" {
  count         = length(var.azs)
  allocation_id = aws_eip.nat[count.index].id
  subnet_id     = aws_subnet.public[count.index].id
  tags          = { Name = "${var.name}-${var.azs[count.index]}" }
  depends_on    = [aws_internet_gateway.this]
}

# ---------- rotas ----------
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }
  tags = { Name = "${var.name}-public" }
}

resource "aws_route_table_association" "public" {
  count          = length(var.azs)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_route_table" "private" {
  count  = length(var.azs)
  vpc_id = aws_vpc.this.id
  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.this[count.index].id
  }
  tags = { Name = "${var.name}-private-${var.azs[count.index]}" }
}

resource "aws_route_table_association" "private" {
  count          = length(var.azs)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private[count.index].id
}

# Isolada: SEM rota default. Só fala dentro da VPC (e com os endpoints abaixo).
resource "aws_route_table" "isolated" {
  vpc_id = aws_vpc.this.id
  tags   = { Name = "${var.name}-isolated" }
}

resource "aws_route_table_association" "isolated" {
  count          = length(var.azs)
  subnet_id      = aws_subnet.isolated[count.index].id
  route_table_id = aws_route_table.isolated.id
}

# ---------- VPC Endpoints: falar com serviços AWS SEM sair para a internet ----------
# Menos tráfego pelo NAT (custo) e menos superfície: o tráfego para S3/ECR/KMS/Secrets nunca sai da rede da AWS.

resource "aws_vpc_endpoint" "s3" {
  vpc_id            = aws_vpc.this.id
  service_name      = "com.amazonaws.${data.aws_region.current.region}.s3"
  vpc_endpoint_type = "Gateway"
  route_table_ids   = concat(aws_route_table.private[*].id, [aws_route_table.isolated.id])
  tags              = { Name = "${var.name}-s3" }
}

resource "aws_security_group" "endpoints" {
  name        = "${var.name}-vpc-endpoints"
  description = "HTTPS dos pods para os endpoints privados"
  vpc_id      = aws_vpc.this.id
  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = [var.cidr]
  }
  tags = { Name = "${var.name}-vpc-endpoints" }
}

locals {
  interface_endpoints = ["ecr.api", "ecr.dkr", "secretsmanager", "kms", "logs", "sts", "monitoring", "xray"]
}

resource "aws_vpc_endpoint" "interface" {
  for_each            = toset(local.interface_endpoints)
  vpc_id              = aws_vpc.this.id
  service_name        = "com.amazonaws.${data.aws_region.current.region}.${each.value}"
  vpc_endpoint_type   = "Interface"
  subnet_ids          = aws_subnet.private[*].id
  security_group_ids  = [aws_security_group.endpoints.id]
  private_dns_enabled = true
  tags                = { Name = "${var.name}-${each.value}" }
}

# ---------- VPC Flow Logs: registro de TODO tráfego (aceito e rejeitado) ----------
resource "aws_cloudwatch_log_group" "flow" {
  name              = "/aws/vpc/${var.name}/flow-logs"
  retention_in_days = 365
  kms_key_id        = var.flow_log_kms_arn
}

resource "aws_iam_role" "flow" {
  name = "${var.name}-flow-logs"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "vpc-flow-logs.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy" "flow" {
  role = aws_iam_role.flow.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["logs:CreateLogStream", "logs:PutLogEvents", "logs:DescribeLogGroups", "logs:DescribeLogStreams"]
      Resource = "${aws_cloudwatch_log_group.flow.arn}:*"
    }]
  })
}

resource "aws_flow_log" "this" {
  vpc_id          = aws_vpc.this.id
  traffic_type    = "ALL"
  iam_role_arn    = aws_iam_role.flow.arn
  log_destination = aws_cloudwatch_log_group.flow.arn
}

data "aws_region" "current" {}

output "vpc_id" { value = aws_vpc.this.id }
output "vpc_cidr" { value = aws_vpc.this.cidr_block }
output "public_subnet_ids" { value = aws_subnet.public[*].id }
output "private_subnet_ids" { value = aws_subnet.private[*].id }
output "isolated_subnet_ids" { value = aws_subnet.isolated[*].id }
```

## 16.11 Módulo `storage`: S3 com Object Lock

O arquivo de liquidação é um registro financeiro: prova do que foi pago a quem. A regulação exige guardá-lo por anos e ninguém pode alterá-lo. **Object Lock em modo COMPLIANCE por 5 anos** faz exatamente isso: o objeto não pode ser apagado nem sobrescrito por ninguém, nem pela AWS, até o prazo. Mais: versionamento, KMS com a chave `data`, bloqueio de público, política que recusa TLS inválido e uploads sem a nossa chave, logs de acesso (quem leu qual arquivo) e transição para Glacier após 90 dias (barato, ainda imutável).

O bucket de logs do ALB usa `AES256` (SSE-S3) porque o serviço de ALB não escreve com KMS: limitação documentada.

#### `deploy/terraform/modules/storage/main.tf`

```hcl
# MÓDULO STORAGE: buckets S3. Um para os arquivos de liquidação (registro financeiro, imutável),
# um para logs de acesso do ALB. Todo bucket: cifrado com KMS, versionado, sem acesso público, só TLS.

variable "name" { type = string }
variable "kms_key_arn" { type = string }

data "aws_caller_identity" "current" {}
data "aws_elb_service_account" "this" {}

# ---------- arquivos de liquidação ----------
resource "aws_s3_bucket" "settlement" {
  bucket              = "${var.name}-settlement-files-${data.aws_caller_identity.current.account_id}"
  object_lock_enabled = true # precisa ser decidido na criação; não dá para ligar depois
}

resource "aws_s3_bucket_versioning" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  versioning_configuration { status = "Enabled" }
}

# COMPLIANCE + 5 anos: obrigação regulatória de guardar o histórico. Ninguém apaga antes, nem o root.
resource "aws_s3_bucket_object_lock_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    default_retention {
      mode  = "COMPLIANCE"
      years = 5
    }
  }
  depends_on = [aws_s3_bucket_versioning.settlement]
}

resource "aws_s3_bucket_server_side_encryption_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm     = "aws:kms"
      kms_master_key_id = var.kms_key_arn
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_public_access_block" "settlement" {
  bucket                  = aws_s3_bucket.settlement.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Depois de 90 dias o arquivo raramente é lido: vai para uma classe mais barata, sem perder o Object Lock.
resource "aws_s3_bucket_lifecycle_configuration" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  rule {
    id     = "archive"
    status = "Enabled"
    filter {}
    transition {
      days          = 90
      storage_class = "GLACIER_IR"
    }
    noncurrent_version_transition {
      noncurrent_days = 30
      storage_class   = "GLACIER_IR"
    }
  }
}

# Registro de quem acessou cada arquivo (PCI: auditar acesso a dados financeiros).
resource "aws_s3_bucket_logging" "settlement" {
  bucket        = aws_s3_bucket.settlement.id
  target_bucket = aws_s3_bucket.access_logs.id
  target_prefix = "settlement/"
}

data "aws_iam_policy_document" "settlement" {
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.settlement.arn, "${aws_s3_bucket.settlement.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
  # Recusa uploads que não peçam cifragem com a NOSSA chave.
  statement {
    sid     = "DenyWrongEncryption"
    effect  = "Deny"
    actions = ["s3:PutObject"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = ["${aws_s3_bucket.settlement.arn}/*"]
    condition {
      test     = "StringNotEquals"
      variable = "s3:x-amz-server-side-encryption-aws-kms-key-id"
      values   = [var.kms_key_arn]
    }
  }
}

resource "aws_s3_bucket_policy" "settlement" {
  bucket = aws_s3_bucket.settlement.id
  policy = data.aws_iam_policy_document.settlement.json
}

# ---------- logs de acesso (S3 e ALB) ----------
resource "aws_s3_bucket" "access_logs" {
  bucket = "${var.name}-access-logs-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_versioning" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  versioning_configuration { status = "Enabled" }
}

# Logs do ALB só aceitam SSE-S3 (AES256), não KMS: limitação do serviço.
resource "aws_s3_bucket_server_side_encryption_configuration" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  rule {
    apply_server_side_encryption_by_default { sse_algorithm = "AES256" }
  }
}

resource "aws_s3_bucket_public_access_block" "access_logs" {
  bucket                  = aws_s3_bucket.access_logs.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_lifecycle_configuration" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  rule {
    id     = "expire"
    status = "Enabled"
    filter {}
    expiration { days = 400 } # PCI pede 1 ano; damos folga
  }
}

data "aws_iam_policy_document" "access_logs" {
  # O serviço de ALB da região escreve os logs de acesso aqui.
  statement {
    sid       = "ALBLogDelivery"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.access_logs.arn}/alb/AWSLogs/${data.aws_caller_identity.current.account_id}/*"]
    principals {
      type        = "AWS"
      identifiers = [data.aws_elb_service_account.this.arn]
    }
  }
  statement {
    sid       = "S3ServerAccessLogs"
    actions   = ["s3:PutObject"]
    resources = ["${aws_s3_bucket.access_logs.arn}/*"]
    principals {
      type        = "Service"
      identifiers = ["logging.s3.amazonaws.com"]
    }
  }
  statement {
    sid     = "DenyInsecureTransport"
    effect  = "Deny"
    actions = ["s3:*"]
    principals {
      type        = "*"
      identifiers = ["*"]
    }
    resources = [aws_s3_bucket.access_logs.arn, "${aws_s3_bucket.access_logs.arn}/*"]
    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }
}

resource "aws_s3_bucket_policy" "access_logs" {
  bucket = aws_s3_bucket.access_logs.id
  policy = data.aws_iam_policy_document.access_logs.json
}

output "settlement_bucket_name" { value = aws_s3_bucket.settlement.bucket }
output "settlement_bucket_arn" { value = aws_s3_bucket.settlement.arn }
output "access_logs_bucket_name" { value = aws_s3_bucket.access_logs.bucket }
```

## 16.12 Módulo `registry`: ECR

Imagens de container são a **cadeia de suprimentos** do software. Três proteções: **tags imutáveis** (ninguém troca o conteúdo de `1.4.2` por baixo do deploy), **scan no push e contínuo** (CVEs novas em imagens antigas aparecem no Inspector) e **KMS**. A política de ciclo de vida apaga o que não se usa: imagem velha é superfície de ataque.

#### `deploy/terraform/modules/registry/main.tf`

```hcl
# MÓDULO REGISTRY: um repositório ECR por serviço.
# Tags imutáveis (ninguém sobrescreve "1.4.2" com outra imagem), scan de vulnerabilidades no push,
# cifragem com KMS e limpeza de imagens antigas.

variable "name" { type = string }
variable "services" { type = list(string) }
variable "kms_key_arn" { type = string }

resource "aws_ecr_repository" "this" {
  for_each             = toset(var.services)
  name                 = "${var.name}/${each.value}"
  image_tag_mutability = "IMMUTABLE"

  image_scanning_configuration {
    scan_on_push = true
  }

  encryption_configuration {
    encryption_type = "KMS"
    kms_key         = var.kms_key_arn
  }
}

# Mantém as 30 últimas imagens; apaga o resto. Imagem não usada é superfície de ataque e custo.
resource "aws_ecr_lifecycle_policy" "this" {
  for_each   = aws_ecr_repository.this
  repository = each.value.name
  policy = jsonencode({
    rules = [{
      rulePriority = 1
      description  = "manter as 30 mais recentes"
      selection = {
        tagStatus   = "any"
        countType   = "imageCountMoreThan"
        countNumber = 30
      }
      action = { type = "expire" }
    }]
  })
}

# Scan contínuo (não só no push): CVEs novas em imagens antigas aparecem no Inspector.
resource "aws_ecr_registry_scanning_configuration" "this" {
  scan_type = "ENHANCED"
  rule {
    scan_frequency = "CONTINUOUS_SCAN"
    repository_filter {
      filter      = "${var.name}/*"
      filter_type = "WILDCARD"
    }
  }
}

output "repository_urls" { value = { for k, r in aws_ecr_repository.this : k => r.repository_url } }
```

## 16.13 Módulo `eks`: o cluster

As decisões, uma a uma:

- **`endpoint_public_access = false`**: a API do Kubernetes não existe na internet. `kubectl` só de dentro da VPC (VPN, bastion via SSM, ou runner de CI na VPC). Elimina a classe inteira de ataques "achei um kube-apiserver aberto".
- **`encryption_config` com KMS**: os `Secret`s do Kubernetes ficam no etcd; o EKS já cifra o disco, mas aqui envelopamos com a **nossa** chave: quem não pode usar a chave não lê o segredo nem com acesso ao etcd.
- **`authentication_mode = "API"`**: acesso ao cluster gerido por *access entries* (IAM), auditável, em vez do ConfigMap `aws-auth` editável à mão.
- **Logs do control plane**: `api`, `audit`, `authenticator`... Quem chamou o quê na API do Kubernetes, por 1 ano.
- **Nós em subnets privadas**, **Bottlerocket** (SO mínimo, imutável, feito para containers), **Graviton** (arm64: mais barato; Go compila nativamente), **IMDSv2 obrigatório** com `hop_limit = 1` (um pod comprometido não rouba as credenciais do nó via SSRF: o ataque mais comum em nuvem), disco cifrado, **SSM em vez de SSH** (sem porta 22; sessão auditada).
- **Pod Identity**: cada ServiceAccount recebe uma role IAM mínima. `external-secrets` só lê os segredos listados; `settlement` só grava no bucket; os clientes Kafka só falam com o cluster e tópicos do projeto.

#### `deploy/terraform/modules/eks/main.tf`

```hcl
# MÓDULO EKS: o cluster Kubernetes gerenciado.
# Decisões de segurança: API do cluster SÓ privada; Secrets cifrados com nossa chave KMS; logs de auditoria
# ligados; nós em subnets privadas com IMDSv2 obrigatório; identidade por pod (Pod Identity) em vez de
# credenciais no nó; acesso ao cluster por IAM (access entries), não por ConfigMap aws-auth.

variable "name" { type = string }
variable "cluster_version" { type = string }
variable "vpc_id" { type = string }
variable "private_subnet_ids" { type = list(string) }
variable "kms_secrets_arn" { type = string }
variable "log_kms_arn" { type = string }
variable "secrets_arns" { type = list(string) }
variable "settlement_bucket" { type = string }

data "aws_caller_identity" "current" {}

# ---------- papel do control plane ----------
resource "aws_iam_role" "cluster" {
  name = "${var.name}-eks-cluster"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "eks.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

# Logs do control plane (api, audit, authenticator...): quem chamou o quê na API do Kubernetes.
resource "aws_cloudwatch_log_group" "cluster" {
  name              = "/aws/eks/${var.name}/cluster"
  retention_in_days = 365
  kms_key_id        = var.log_kms_arn
}

resource "aws_eks_cluster" "this" {
  name     = var.name
  version  = var.cluster_version
  role_arn = aws_iam_role.cluster.arn

  vpc_config {
    subnet_ids              = var.private_subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = false # kubectl só de dentro da VPC (VPN/bastion/SSM). A API do cluster não existe na internet.
  }

  # Secrets do Kubernetes ficam no etcd; aqui garantimos que estão cifrados com a NOSSA chave.
  encryption_config {
    provider { key_arn = var.kms_secrets_arn }
    resources = ["secrets"]
  }

  access_config {
    authentication_mode                         = "API" # acesso gerido por access entries (IAM), auditável
    bootstrap_cluster_creator_admin_permissions = true
  }

  enabled_cluster_log_types = ["api", "audit", "authenticator", "controllerManager", "scheduler"]

  depends_on = [aws_iam_role_policy_attachment.cluster, aws_cloudwatch_log_group.cluster]
}

# ---------- nós ----------
resource "aws_iam_role" "node" {
  name = "${var.name}-eks-node"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "ec2.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "node" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore", # acesso ao nó via SSM Session Manager: sem SSH, sem porta 22
  ])
  role       = aws_iam_role.node.name
  policy_arn = each.value
}

# Launch template para exigir IMDSv2: fecha o ataque clássico de roubar credenciais do nó via SSRF.
resource "aws_launch_template" "node" {
  name_prefix = "${var.name}-node-"
  metadata_options {
    http_tokens                 = "required"
    http_put_response_hop_limit = 1 # pods não alcançam o metadata do nó
    http_endpoint               = "enabled"
  }
  block_device_mappings {
    device_name = "/dev/xvda"
    ebs {
      volume_size = 50
      volume_type = "gp3"
      encrypted   = true
    }
  }
  tag_specifications {
    resource_type = "instance"
    tags          = { Name = "${var.name}-node" }
  }
}

resource "aws_eks_node_group" "general" {
  cluster_name    = aws_eks_cluster.this.name
  node_group_name = "general"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.private_subnet_ids
  instance_types  = ["m6g.large"]         # Graviton: mais barato; nossas imagens são Go, compilam para arm64
  ami_type        = "BOTTLEROCKET_ARM_64" # SO mínimo, imutável, feito para containers
  capacity_type   = "ON_DEMAND"

  scaling_config {
    desired_size = 3
    min_size     = 3
    max_size     = 12
  }

  update_config { max_unavailable = 1 }

  launch_template {
    id      = aws_launch_template.node.id
    version = aws_launch_template.node.latest_version
  }

  depends_on = [aws_iam_role_policy_attachment.node]
}

# ---------- addons ----------
resource "aws_eks_addon" "this" {
  for_each = toset(["vpc-cni", "coredns", "kube-proxy", "eks-pod-identity-agent"])

  cluster_name                = aws_eks_cluster.this.name
  addon_name                  = each.value
  resolve_conflicts_on_update = "OVERWRITE"
  depends_on                  = [aws_eks_node_group.general]
}

# ---------- identidade por pod ----------
# Um pod recebe UMA role IAM, com UMA política mínima. Nada de credenciais em variável de ambiente.

data "aws_iam_policy_document" "pod_identity_trust" {
  statement {
    actions = ["sts:AssumeRole", "sts:TagSession"]
    principals {
      type        = "Service"
      identifiers = ["pods.eks.amazonaws.com"]
    }
  }
}

# External Secrets Operator: lê do Secrets Manager e cria Secrets no cluster. Só os ARNs listados.
resource "aws_iam_role" "external_secrets" {
  name               = "${var.name}-external-secrets"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "external_secrets" {
  role = aws_iam_role.external_secrets.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
      Resource = var.secrets_arns
    }]
  })
}

resource "aws_eks_pod_identity_association" "external_secrets" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "external-secrets"
  service_account = "external-secrets"
  role_arn        = aws_iam_role.external_secrets.arn
  depends_on      = [aws_eks_addon.this]
}

# Worker de liquidação: só escreve no bucket de liquidação (e nada mais).
resource "aws_iam_role" "settlement" {
  name               = "${var.name}-settlement"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "settlement" {
  role = aws_iam_role.settlement.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:PutObject"]
      Resource = "${var.settlement_bucket}/*"
    }]
  })
}

resource "aws_eks_pod_identity_association" "settlement" {
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "adquirente"
  service_account = "settlement"
  role_arn        = aws_iam_role.settlement.arn
  depends_on      = [aws_eks_addon.this]
}

# Serviços que falam com o MSK via IAM: permissão de conectar e usar tópicos com prefixo do projeto.
resource "aws_iam_role" "kafka_client" {
  name               = "${var.name}-kafka-client"
  assume_role_policy = data.aws_iam_policy_document.pod_identity_trust.json
}

resource "aws_iam_role_policy" "kafka_client" {
  role = aws_iam_role.kafka_client.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:Connect", "kafka-cluster:DescribeCluster"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:cluster/${var.name}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:*Topic*", "kafka-cluster:WriteData", "kafka-cluster:ReadData"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:topic/${var.name}/*"
      },
      {
        Effect   = "Allow"
        Action   = ["kafka-cluster:AlterGroup", "kafka-cluster:DescribeGroup"]
        Resource = "arn:aws:kafka:*:${data.aws_caller_identity.current.account_id}:group/${var.name}/*"
      }
    ]
  })
}

resource "aws_eks_pod_identity_association" "kafka_client" {
  for_each        = toset(["api", "scheduler", "notifier", "settlement"])
  cluster_name    = aws_eks_cluster.this.name
  namespace       = "adquirente"
  service_account = each.value
  role_arn        = aws_iam_role.kafka_client.arn
  depends_on      = [aws_eks_addon.this]
}

output "cluster_name" { value = aws_eks_cluster.this.name }
output "cluster_endpoint" { value = aws_eks_cluster.this.endpoint }
output "node_security_group_id" { value = aws_eks_cluster.this.vpc_config[0].cluster_security_group_id }
output "oidc_issuer" { value = aws_eks_cluster.this.identity[0].oidc[0].issuer }
```

## 16.14 Módulo `database`: RDS PostgreSQL

Duas instâncias, `core` e `vault`, pelo mesmo módulo (`for_each`). Cada uma:

- **Multi-AZ**: réplica síncrona em outra zona, failover automático em ~1 min. Sem isso, um problema no datacenter para a adquirente.
- **Subnets isoladas**, **`publicly_accessible = false`**, security group que só aceita **o security group dos nós do EKS** na 5432.
- **`rds.force_ssl = 1`**: conexão sem TLS é recusada, mesmo de dentro da VPC. Na aplicação, `sslmode=verify-full`.
- **`manage_master_user_password`**: a AWS gera a senha, guarda no Secrets Manager (cifrada com nossa chave) e **rotaciona sozinha**. Nenhum humano conhece a senha; a aplicação lê o segredo em runtime (via External Secrets).
- **IAM authentication**: alternativa à senha: token de 15 minutos emitido pela role do pod.
- **pgaudit**: quem executou qual DDL/escrita, no log; **`log_min_duration_statement`** para consultas lentas; logs exportados ao CloudWatch.
- **Backups por 35 dias com PITR**, janela fora do horário de liquidação, **`deletion_protection`**, **snapshot final** obrigatório.
- **Performance Insights** e **Enhanced Monitoring** para saber por que está lento antes do cliente reclamar.

#### `deploy/terraform/modules/database/main.tf`

```hcl
# MÓDULO DATABASE: PostgreSQL gerenciado (RDS). Uma instância por entrada em `instances`:
# "core" (o sistema) e "vault" (o cofre de cartões), SEPARADAS. Se o core vazar, o cofre não vai junto.
#
# Cada instância: Multi-AZ (réplica síncrona em outra zona, failover automático), cifrada com KMS,
# sem IP público, só aceita TLS, senha do master gerida pela AWS (Secrets Manager, rotação automática),
# backups por 35 dias com point-in-time recovery, proteção contra deleção, logs e Performance Insights.

variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "performance_kms_arn" { type = string }
variable "instances" {
  type = map(object({
    instance_class    = string
    allocated_storage = number
  }))
}

resource "aws_db_subnet_group" "this" {
  name       = var.name
  subnet_ids = var.subnet_ids # subnets ISOLADAS: sem rota para a internet
}

# Security group: só a porta 5432, só a partir dos nós do EKS. Nenhum CIDR, nenhum 0.0.0.0/0.
resource "aws_security_group" "db" {
  name        = "${var.name}-rds"
  description = "PostgreSQL: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-rds" }
}

resource "aws_vpc_security_group_ingress_rule" "db" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = each.value
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
}

# Parâmetros do Postgres com foco em segurança e auditoria.
resource "aws_db_parameter_group" "this" {
  name   = "${var.name}-pg16"
  family = "postgres16"

  parameter { # recusa conexão sem TLS, mesmo de dentro da VPC
    name  = "rds.force_ssl"
    value = "1"
  }
  parameter { # pgaudit: quem executou qual comando (DDL e escritas)
    name         = "shared_preload_libraries"
    value        = "pgaudit"
    apply_method = "pending-reboot"
  }
  parameter {
    name  = "pgaudit.log"
    value = "ddl,write,role"
  }
  parameter { # consultas lentas (> 500 ms) no log: observabilidade
    name  = "log_min_duration_statement"
    value = "500"
  }
  parameter {
    name  = "log_connections"
    value = "1"
  }
  parameter {
    name  = "log_disconnections"
    value = "1"
  }
}

# Papel para o Enhanced Monitoring (métricas do SO a cada 60s).
resource "aws_iam_role" "monitoring" {
  name = "${var.name}-rds-monitoring"
  assume_role_policy = jsonencode({
    Version   = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = "sts:AssumeRole", Principal = { Service = "monitoring.rds.amazonaws.com" } }]
  })
}

resource "aws_iam_role_policy_attachment" "monitoring" {
  role       = aws_iam_role.monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}

resource "aws_db_instance" "this" {
  for_each = var.instances

  identifier     = "${var.name}-${each.key}"
  engine         = "postgres"
  engine_version = "16"
  instance_class = each.value.instance_class

  # armazenamento: gp3 cifrado, cresce sozinho até o teto
  allocated_storage     = each.value.allocated_storage
  max_allocated_storage = each.value.allocated_storage * 5
  storage_type          = "gp3"
  storage_encrypted     = true
  kms_key_id            = var.kms_key_arn

  db_name  = "adquirente"
  username = "adq_admin"
  # A AWS cria e ROTACIONA a senha no Secrets Manager. Ninguém conhece a senha; a aplicação lê o secret.
  manage_master_user_password   = true
  master_user_secret_kms_key_id = var.kms_key_arn

  # Autenticação por IAM: a aplicação pode se conectar com token IAM de 15 min em vez de senha.
  iam_database_authentication_enabled = true

  # rede
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  multi_az               = true
  parameter_group_name   = aws_db_parameter_group.this.name

  # backups e proteção
  backup_retention_period   = 35            # PITR: voltar para qualquer segundo dos últimos 35 dias
  backup_window             = "03:00-04:00" # UTC (00:00-01:00 Brasília): fora da liquidação
  maintenance_window        = "sun:04:00-sun:05:00"
  copy_tags_to_snapshot     = true
  deletion_protection       = true
  skip_final_snapshot       = false
  final_snapshot_identifier = "${var.name}-${each.key}-final"
  delete_automated_backups  = false

  # observabilidade
  performance_insights_enabled          = true
  performance_insights_kms_key_id       = var.performance_kms_arn
  performance_insights_retention_period = 7
  monitoring_interval                   = 60
  monitoring_role_arn                   = aws_iam_role.monitoring.arn
  enabled_cloudwatch_logs_exports       = ["postgresql", "upgrade"]

  # atualizações
  auto_minor_version_upgrade  = true
  allow_major_version_upgrade = false
  apply_immediately           = false

  tags = { Component = each.key }
}

output "identifiers" { value = { for k, db in aws_db_instance.this : k => db.identifier } }
output "endpoints" { value = { for k, db in aws_db_instance.this : k => db.endpoint } }
output "secret_arn" { value = { for k, db in aws_db_instance.this : k => db.master_user_secret[0].secret_arn } }
output "security_group_id" { value = aws_security_group.db.id }
```

## 16.15 Módulo `cache`: ElastiCache Redis

Redis guarda o lock diário da liquidação. Multi-AZ com failover, **cifrado em repouso e em trânsito**, **senha** (`auth_token`) gerada pelo Terraform e guardada no Secrets Manager, acesso só dos nós do EKS. A senha aparece no state (por isso o state é cifrado e restrito); em maturidade maior, gere a senha fora do Terraform ou use IAM auth do ElastiCache.

#### `deploy/terraform/modules/cache/main.tf`

```hcl
# MÓDULO CACHE: Redis gerenciado (ElastiCache), usado para o lock diário da liquidação e cache.
# Multi-AZ com failover; cifrado em repouso (KMS) e em trânsito (TLS); exige senha (AUTH token)
# guardada no Secrets Manager; só acessível pelos nós do EKS.

variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }

resource "aws_elasticache_subnet_group" "this" {
  name       = var.name
  subnet_ids = var.subnet_ids
}

resource "aws_security_group" "redis" {
  name        = "${var.name}-redis"
  description = "Redis: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-redis" }
}

resource "aws_vpc_security_group_ingress_rule" "redis" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.redis.id
  referenced_security_group_id = each.value
  from_port                    = 6379
  to_port                      = 6379
  ip_protocol                  = "tcp"
}

# Senha forte gerada pelo Terraform e guardada SÓ no Secrets Manager (o state também a contém: por isso
# o state é cifrado e restrito).
resource "random_password" "auth" {
  length           = 64
  special          = true
  override_special = "!&#$^<>-" # caracteres aceitos pelo ElastiCache
}

resource "aws_secretsmanager_secret" "auth" {
  name       = "${var.name}/redis/auth-token"
  kms_key_id = var.kms_key_arn
}

resource "aws_secretsmanager_secret_version" "auth" {
  secret_id     = aws_secretsmanager_secret.auth.id
  secret_string = random_password.auth.result
}

resource "aws_elasticache_replication_group" "this" {
  replication_group_id = var.name
  description          = "Redis da adquirente (locks e cache)"
  engine               = "redis"
  engine_version       = "7.1"
  node_type            = "cache.t4g.small"
  port                 = 6379

  num_cache_clusters         = 2 # primário + réplica em outra AZ
  multi_az_enabled           = true
  automatic_failover_enabled = true

  subnet_group_name  = aws_elasticache_subnet_group.this.name
  security_group_ids = [aws_security_group.redis.id]

  at_rest_encryption_enabled = true
  kms_key_id                 = var.kms_key_arn
  transit_encryption_enabled = true
  auth_token                 = random_password.auth.result
  auth_token_update_strategy = "ROTATE"

  snapshot_retention_limit   = 7
  snapshot_window            = "02:00-03:00"
  maintenance_window         = "sun:05:00-sun:06:00"
  apply_immediately          = false
  auto_minor_version_upgrade = true
}

output "primary_endpoint" { value = aws_elasticache_replication_group.this.primary_endpoint_address }
output "replication_group_id" { value = aws_elasticache_replication_group.this.id }
output "auth_secret_arn" { value = aws_secretsmanager_secret.auth.arn }
```

## 16.16 Módulo `kafka`: MSK

Três brokers (um por AZ), **TLS obrigatório**, **autenticação IAM** (cada pod usa sua role; nada de usuário/senha), **sem acesso público**, KMS em repouso, logs dos brokers no CloudWatch, métricas via Prometheus (JMX exporter). A configuração do broker prioriza **durabilidade**: replicação 3, `min.insync.replicas=2` (um evento só é confirmado quando está em 2 brokers), `unclean.leader.election=false` (nunca eleger um líder que pode ter perdido dados) e **`auto.create.topics.enable=false`** (tópicos são criados por IaC ou por um job controlado; não por um bug de nome de tópico).

Na aplicação, o `kafka-go` precisa do mecanismo SASL/IAM (`github.com/aws/aws-msk-iam-sasl-signer-go`) em vez da conexão em claro do Compose; é uma troca no `Publisher`/`Consumer` da Fase 5, e só lá.

#### `deploy/terraform/modules/kafka/main.tf`

```hcl
# MÓDULO KAFKA: Amazon MSK (Kafka gerenciado). 3 brokers, um por zona.
# Cifrado em repouso (KMS) e em trânsito (TLS obrigatório); autenticação por IAM (nada de senha em
# arquivo); tópicos NÃO são criados automaticamente (IaC decide); replicação 3 com mínimo 2 em sincronia.

variable "name" { type = string }
variable "vpc_id" { type = string }
variable "subnet_ids" { type = list(string) }
variable "allowed_sg_ids" { type = list(string) }
variable "kms_key_arn" { type = string }
variable "log_kms_arn" { type = string }

resource "aws_security_group" "kafka" {
  name        = "${var.name}-msk"
  description = "Kafka: apenas a partir do cluster EKS"
  vpc_id      = var.vpc_id
  tags        = { Name = "${var.name}-msk" }
}

resource "aws_vpc_security_group_ingress_rule" "kafka_iam" {
  for_each                     = toset(var.allowed_sg_ids)
  security_group_id            = aws_security_group.kafka.id
  referenced_security_group_id = each.value
  from_port                    = 9098 # porta do SASL/IAM com TLS
  to_port                      = 9098
  ip_protocol                  = "tcp"
}

# Configuração do broker: durabilidade de dados de pagamento acima de tudo.
resource "aws_msk_configuration" "this" {
  name              = var.name
  kafka_versions    = ["3.6.0"]
  server_properties = <<-PROPERTIES
    auto.create.topics.enable=false
    default.replication.factor=3
    min.insync.replicas=2
    unclean.leader.election.enable=false
    num.partitions=6
    log.retention.hours=168
  PROPERTIES
}

resource "aws_cloudwatch_log_group" "kafka" {
  name              = "/aws/msk/${var.name}"
  retention_in_days = 90
  kms_key_id        = var.log_kms_arn
}

resource "aws_msk_cluster" "this" {
  cluster_name           = var.name
  kafka_version          = "3.6.0"
  number_of_broker_nodes = 3

  broker_node_group_info {
    instance_type   = "kafka.m7g.large"
    client_subnets  = var.subnet_ids
    security_groups = [aws_security_group.kafka.id]
    storage_info {
      ebs_storage_info { volume_size = 200 }
    }
    connectivity_info {
      public_access { type = "DISABLED" } # brokers nunca na internet
    }
  }

  configuration_info {
    arn      = aws_msk_configuration.this.arn
    revision = aws_msk_configuration.this.latest_revision
  }

  encryption_info {
    encryption_at_rest_kms_key_arn = var.kms_key_arn
    encryption_in_transit {
      client_broker = "TLS"
      in_cluster    = true
    }
  }

  client_authentication {
    sasl { iam = true } # cada pod autentica com sua role (Pod Identity); nada de usuário/senha
    unauthenticated = false
  }

  enhanced_monitoring = "PER_TOPIC_PER_PARTITION"

  logging_info {
    broker_logs {
      cloudwatch_logs {
        enabled   = true
        log_group = aws_cloudwatch_log_group.kafka.name
      }
    }
  }

  open_monitoring {
    prometheus {
      jmx_exporter { enabled_in_broker = true }
      node_exporter { enabled_in_broker = true }
    }
  }
}

output "cluster_name" { value = aws_msk_cluster.this.cluster_name }
output "cluster_arn" { value = aws_msk_cluster.this.arn }
output "bootstrap_brokers_sasl_iam" { value = aws_msk_cluster.this.bootstrap_brokers_sasl_iam }
```

## 16.17 Módulo `edge`: a borda pública

- **ACM**: certificado TLS gratuito, validado por DNS, **renovado automaticamente**. Certificado vencido derrubando a API é um clássico que aqui não acontece.
- **WAF**: um firewall de aplicação na frente do ALB, com regras gerenciadas pela AWS (injeção, inputs maliciosos conhecidos, IPs de reputação ruim) e um **rate limit por IP** (2.000 requisições / 5 min). Os logs do WAF vão para o CloudWatch com o header `Authorization` **redigido**: token não pode aparecer em log.
- O **ALB** em si nasce do `Ingress` da Fase 9 (o AWS Load Balancer Controller lê as anotações com o ARN do certificado e do WAF).

#### `deploy/terraform/modules/edge/main.tf`

```hcl
# MÓDULO EDGE: a borda pública. Certificado TLS (ACM, renovação automática), WAF (bloqueia ataques
# conhecidos e limita taxa) e DNS. O ALB em si é criado pelo AWS Load Balancer Controller a partir do
# Ingress do Kubernetes; aqui criamos o que o Ingress referencia por ARN.

variable "name" { type = string }
variable "domain_name" { type = string }
variable "route53_zone_name" { type = string }
variable "log_kms_arn" { type = string }

data "aws_route53_zone" "this" {
  name = var.route53_zone_name
}

# ---------- certificado TLS ----------
resource "aws_acm_certificate" "api" {
  domain_name       = var.domain_name
  validation_method = "DNS"
  lifecycle { create_before_destroy = true }
}

# A validação por DNS prova que somos dono do domínio; a ACM renova sozinha antes de expirar.
resource "aws_route53_record" "validation" {
  for_each = {
    for dvo in aws_acm_certificate.api.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }
  zone_id = data.aws_route53_zone.this.zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 60
  records = [each.value.record]
}

resource "aws_acm_certificate_validation" "api" {
  certificate_arn         = aws_acm_certificate.api.arn
  validation_record_fqdns = [for r in aws_route53_record.validation : r.fqdn]
}

# ---------- WAF ----------
resource "aws_wafv2_web_acl" "api" {
  name  = "${var.name}-api"
  scope = "REGIONAL" # para ALB (CLOUDFRONT seria para CDN)

  default_action {
    allow {}
  }

  # Regras gerenciadas pela AWS: assinaturas de ataques atualizadas por eles.
  dynamic "rule" {
    for_each = {
      AWSManagedRulesCommonRuleSet          = 10
      AWSManagedRulesKnownBadInputsRuleSet  = 20
      AWSManagedRulesSQLiRuleSet            = 30
      AWSManagedRulesAmazonIpReputationList = 40
    }
    content {
      name     = rule.key
      priority = rule.value
      override_action {
        none {}
      }
      statement {
        managed_rule_group_statement {
          name        = rule.key
          vendor_name = "AWS"
        }
      }
      visibility_config {
        cloudwatch_metrics_enabled = true
        metric_name                = rule.key
        sampled_requests_enabled   = true
      }
    }
  }

  # Rate limit: mais de 2.000 requisições em 5 min do mesmo IP → bloqueia. Freia abuso e credential stuffing.
  rule {
    name     = "rate-limit-per-ip"
    priority = 50
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit              = 2000
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "rate-limit-per-ip"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "${var.name}-api"
    sampled_requests_enabled   = true
  }
}

# Logs do WAF: o nome do log group PRECISA começar com "aws-waf-logs-".
resource "aws_cloudwatch_log_group" "waf" {
  name              = "aws-waf-logs-${var.name}"
  retention_in_days = 365
  kms_key_id        = var.log_kms_arn
}

resource "aws_wafv2_web_acl_logging_configuration" "api" {
  resource_arn            = aws_wafv2_web_acl.api.arn
  log_destination_configs = [aws_cloudwatch_log_group.waf.arn]

  # Não grava o header Authorization nos logs: token não pode aparecer em log.
  redacted_fields {
    single_header { name = "authorization" }
  }
}

output "certificate_arn" { value = aws_acm_certificate_validation.api.certificate_arn }
output "web_acl_arn" { value = aws_wafv2_web_acl.api.arn }
output "zone_id" { value = data.aws_route53_zone.this.zone_id }
```

## 16.18 Módulo `observability`: quem acorda de noite

A aplicação já emite logs, métricas e traces (Parte 13). Este módulo cobre o que **ela** não vê: a saúde da infraestrutura. Um tópico SNS cifrado recebe todos os alarmes (e-mail aqui; Slack/PagerDuty na vida real): CPU e disco do RDS, conexões, partições offline e disco do Kafka, memória do Redis, **uso do root** (via filtro sobre o log do CloudTrail) e um **orçamento mensal** que avisa a 80% da previsão.

#### `deploy/terraform/modules/observability/main.tf`

```hcl
# MÓDULO OBSERVABILITY: alarmes que acordam alguém, para o que a aplicação não enxerga sozinha
# (a saúde do banco, do Kafka, do Redis) e controle de custo.

variable "name" { type = string }
variable "alert_email" { type = string }
variable "kms_key_arn" { type = string }
variable "db_identifiers" { type = map(string) }
variable "msk_cluster_name" { type = string }
variable "redis_group_id" { type = string }
variable "monthly_budget_usd" { type = number }
variable "cloudtrail_log_group" { type = string }

# Tópico SNS cifrado: todo alarme publica aqui; e-mail, Slack, PagerDuty se inscrevem.
resource "aws_sns_topic" "alerts" {
  name              = "${var.name}-alerts"
  kms_master_key_id = var.kms_key_arn
}

resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# ---------- RDS ----------
resource "aws_cloudwatch_metric_alarm" "rds_cpu" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-cpu-high"
  namespace           = "AWS/RDS"
  metric_name         = "CPUUtilization"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 3
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
  ok_actions          = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "rds_storage" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-free-storage-low"
  namespace           = "AWS/RDS"
  metric_name         = "FreeStorageSpace"
  statistic           = "Minimum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 10 * 1024 * 1024 * 1024 # 10 GiB
  comparison_operator = "LessThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "rds_connections" {
  for_each            = var.db_identifiers
  alarm_name          = "${each.value}-connections-high"
  namespace           = "AWS/RDS"
  metric_name         = "DatabaseConnections"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 5
  threshold           = 400
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { DBInstanceIdentifier = each.value }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- MSK ----------
resource "aws_cloudwatch_metric_alarm" "msk_offline_partitions" {
  alarm_name          = "${var.name}-msk-offline-partitions"
  namespace           = "AWS/Kafka"
  metric_name         = "OfflinePartitionsCount"
  statistic           = "Maximum"
  period              = 60
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { "Cluster Name" = var.msk_cluster_name }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "msk_disk" {
  alarm_name          = "${var.name}-msk-disk-high"
  namespace           = "AWS/Kafka"
  metric_name         = "KafkaDataLogsDiskUsed"
  statistic           = "Maximum"
  period              = 300
  evaluation_periods  = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { "Cluster Name" = var.msk_cluster_name }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- Redis ----------
resource "aws_cloudwatch_metric_alarm" "redis_memory" {
  alarm_name          = "${var.name}-redis-memory-high"
  namespace           = "AWS/ElastiCache"
  metric_name         = "DatabaseMemoryUsagePercentage"
  statistic           = "Average"
  period              = 300
  evaluation_periods  = 2
  threshold           = 80
  comparison_operator = "GreaterThanThreshold"
  dimensions          = { ReplicationGroupId = var.redis_group_id }
  alarm_actions       = [aws_sns_topic.alerts.arn]
}

# ---------- segurança operacional ----------
# Uso do usuário root = incidente. Alarme via métrica de log do CloudTrail.
resource "aws_cloudwatch_log_metric_filter" "root_usage" {
  name           = "${var.name}-root-usage"
  log_group_name = var.cloudtrail_log_group
  pattern        = "{ $.userIdentity.type = \"Root\" && $.userIdentity.invokedBy NOT EXISTS && $.eventType != \"AwsServiceEvent\" }"
  metric_transformation {
    name      = "RootUsage"
    namespace = var.name
    value     = "1"
  }
}

resource "aws_cloudwatch_metric_alarm" "root_usage" {
  alarm_name          = "${var.name}-root-usage"
  namespace           = var.name
  metric_name         = "RootUsage"
  statistic           = "Sum"
  period              = 300
  evaluation_periods  = 1
  threshold           = 0
  comparison_operator = "GreaterThanThreshold"
  alarm_actions       = [aws_sns_topic.alerts.arn]
  treat_missing_data  = "notBreaching"
}

# ---------- custo ----------
resource "aws_budgets_budget" "monthly" {
  name         = "${var.name}-monthly"
  budget_type  = "COST"
  limit_amount = tostring(var.monthly_budget_usd)
  limit_unit   = "USD"
  time_unit    = "MONTHLY"

  notification {
    comparison_operator        = "GREATER_THAN"
    threshold                  = 80
    threshold_type             = "PERCENTAGE"
    notification_type          = "FORECASTED"
    subscriber_email_addresses = [var.alert_email]
  }
}

output "alerts_topic_arn" { value = aws_sns_topic.alerts.arn }
```

## 16.19 Aplicando, passo a passo

```bash
# 0. credenciais: sessão SSO (nunca access keys em arquivo)
aws sso login --profile adquirente-prod
export AWS_PROFILE=adquirente-prod

# 1. bootstrap (uma vez) — seção 16.7

# 2. o ambiente
cd deploy/terraform/envs/prod
#    edite versions.tf com o nome do bucket do state; edite prod.tfvars
terraform init
terraform plan -var-file=prod.tfvars -out=plan.tfplan      # LEIA o plano inteiro
terraform apply plan.tfplan                                # 25–40 min (EKS, RDS Multi-AZ e MSK demoram)

# 3. acesso ao cluster: a API é privada. Opções: VPN (Client VPN), um bastion mínimo acessado por SSM,
#    ou rodar o kubectl de um runner dentro da VPC. Exemplo com SSM port-forward via um bastion:
aws eks update-kubeconfig --name adquirente-prod
kubectl get nodes

# 4. controladores no cluster (Helm): AWS Load Balancer Controller (cria o ALB do Ingress),
#    External Secrets Operator (Secrets Manager → Secret), cert-manager (mTLS interno),
#    metrics-server (HPA), kube-prometheus-stack + Tempo/Jaeger (observabilidade)
helm repo add eks https://aws.github.io/eks-charts
helm install aws-load-balancer-controller eks/aws-load-balancer-controller -n kube-system \
  --set clusterName=adquirente-prod --set serviceAccount.name=aws-load-balancer-controller
helm repo add external-secrets https://charts.external-secrets.io
helm install external-secrets external-secrets/external-secrets -n external-secrets --create-namespace
#    (o Pod Identity de cada um vem do módulo eks; o do ALB controller você adiciona seguindo o mesmo padrão)

# 5. imagens
aws ecr get-login-password | docker login --username AWS --password-stdin <account>.dkr.ecr.sa-east-1.amazonaws.com
for s in api vault issuer-sim scheduler settlement notifier; do
  docker buildx build --platform linux/arm64 --build-arg SERVICE=$s \
    -t <account>.dkr.ecr.sa-east-1.amazonaws.com/adquirente-prod/$s:1.0.0 -f deploy/docker/Dockerfile --push .
done

# 6. a aplicação: um overlay Kustomize "prod" que troca as imagens pelas do ECR, o ConfigMap pelos
#    endpoints do terraform output, e substitui o Secret estático por ExternalSecrets
kubectl apply -k deploy/k8s/overlays/prod
```

Um `ExternalSecret` (o que substitui o `Secret` da Fase 9):

```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata: { name: adquirente-secrets, namespace: adquirente }
spec:
  refreshInterval: 1h                      # rotação do RDS chega ao pod em até 1h
  secretStoreRef: { name: aws-secrets-manager, kind: ClusterSecretStore }
  target: { name: adquirente-secrets }
  data:
    - secretKey: DB_PASSWORD
      remoteRef: { key: rds!db-XXXX, property: password }   # o secret que o RDS criou
    - secretKey: REDIS_AUTH
      remoteRef: { key: adquirente-prod/redis/auth-token }
```

## 16.20 O que muda na aplicação para rodar na AWS

Quase nada no código; quase tudo em configuração (é o prêmio da Clean Architecture):

| Item | Compose | AWS |
|---|---|---|
| `DATABASE_URL` | `sslmode=disable` | `sslmode=verify-full sslrootcert=/etc/rds/global-bundle.pem`, senha do Secrets Manager ou token IAM |
| Kafka | `kafka:19092` em claro | brokers SASL/IAM na 9098 (`bootstrap_brokers_sasl_iam`), mecanismo IAM no `kafka-go` |
| Redis | sem TLS/senha | `rediss://` com `auth_token` |
| Arquivo de liquidação | disco local | `s3://.../settlement-files/` (um `S3Gateway` implementando `settlement.PaymentGateway`) |
| Chave do vault | `.env` | Secrets Manager (ou, para PCI de verdade, um HSM: CloudHSM/KMS com chave de dados) |
| JWT | `auth-sim` | provedor OIDC (Cognito/Keycloak); a API lê o JWKS |
| mTLS | opcional | obrigatório (cert-manager emite; `TLS_*_FILE` montados do Secret) |
| Tracing | Jaeger | OTel Collector → Tempo/X-Ray |

Só o adapter muda; domínio e casos de uso não sabem que estão na AWS.

## 16.21 CI/CD para infraestrutura e aplicação

- **Sem chaves estáticas**: o GitHub Actions assume uma role via **OIDC** (`aws-actions/configure-aws-credentials` com `role-to-assume`); a *trust policy* da role só aceita o repositório e a branch certos.
- **Infra**: em PR, `terraform fmt -check`, `validate`, **`tfsec`/`checkov`/`trivy config`** (policy-as-code: "há bucket sem cifragem?", "há SG com 0.0.0.0/0?") e `terraform plan` postado como comentário. No merge em `main`, `apply` com aprovação manual (environment protection). **Drift detection**: um `plan` agendado diariamente; diferença = alguém mexeu no console = alerta.
- **Aplicação**: `go vet`, `golangci-lint`, `govulncheck`, testes com `-race`, `buf lint`/`breaking`, build multi-arch, **assinatura da imagem** (cosign) e scan; deploy em staging automático; em prod, canário (Argo Rollouts) observando a taxa de aprovação e o p99 antes de promover.
- **Segredos no CI**: nenhum. O CI só tem a role OIDC; os segredos da aplicação vivem no Secrets Manager e chegam ao pod pelo External Secrets.

## 16.22 Quanto custa (ordem de grandeza, sa-east-1, on-demand)

| Item | Mensal aproximado (USD) |
|---|---|
| EKS control plane | ~73 |
| 3 × m6g.large | ~250 |
| 3 × NAT gateway + tráfego | ~130+ |
| RDS core db.r6g.large Multi-AZ + 100 GB | ~600 |
| RDS vault db.t4g.medium Multi-AZ + 20 GB | ~150 |
| ElastiCache 2 × cache.t4g.small | ~60 |
| MSK 3 × kafka.m7g.large + 600 GB | ~700 |
| ALB, WAF, Route 53, ACM | ~60 |
| CloudTrail, Config, GuardDuty, Security Hub, logs | ~80–200 (cresce com volume) |
| **Total** | **~2.100–2.300** |

Para um ambiente de **estudo/dev**: uma AZ, um NAT, `db.t4g.micro` single-AZ, MSK Serverless ou 2 × `kafka.t3.small`, e `terraform destroy` ao fim do dia. Cai para ~300–400. **Nunca** faça isso em produção de pagamento: Multi-AZ e 3 brokers são requisito de disponibilidade, não luxo.

## 16.23 O que ainda falta para uma adquirente real (e por que não está aqui)

Este guia constrói o núcleo seguro de **uma** conta. Uma adquirente licenciada precisa ir além:

- **AWS Organizations + SCPs**: contas separadas (`security`, `log-archive`, `prod`, `staging`, `dev`, `sandbox`) e políticas de controle de serviço que impedem, por exemplo, criar recursos fora de `sa-east-1` ou desligar o CloudTrail, mesmo para administradores.
- **Rede de gerência**: Client VPN ou Transit Gateway para o acesso privado ao EKS; bastion **sem** IP público, acessado só por SSM.
- **Shield Advanced** (anti-DDoS) e ajuste fino do WAF com regras próprias (padrões de fraude no `POST /transactions`).
- **Macie** (procura dados pessoais/cartão em S3 onde não deveriam estar) e **Inspector** (vulnerabilidades em nós e imagens).
- **AWS Backup** com cofre imutável e **cópia cross-region** dos snapshots do RDS: plano de recuperação de desastre com RPO/RTO definidos e **ensaiado**.
- **Gerência de chaves PCI**: para o cofre de cartões em produção, chaves em **HSM** (CloudHSM) ou KMS com *key policy* restrita a uma role única, com *dual control* para rotação; separação de funções entre quem opera o vault e quem opera o resto.
- **Pentest anual e ASV scan** (exigência PCI), **runbooks** de resposta a incidente (chave vazada → revogar, rotacionar, comunicar BCB/ANPD/bandeira), e treino.
- **Segregação de dados**: o banco `vault` numa **conta AWS separada** com peering restrito, não só numa instância separada.

Cada um desses é um módulo Terraform a mais no mesmo desenho.

## 16.24 Checklist: exigência → recurso

| Exigência (Parte 2) | Onde está no Terraform |
|---|---|
| Dados de cartão cifrados, chaves gerenciadas | KMS `data` com rotação; RDS `vault` com `storage_encrypted`; `card_vault` só acessível pelo serviço vault |
| TLS em trânsito, inclusive interno | `rds.force_ssl`, MSK `TLS`, Redis `transit_encryption`, ALB TLS 1.3, mTLS via cert-manager |
| Logs de auditoria imutáveis por ≥ 1 ano | CloudTrail em S3 com Object Lock COMPLIANCE (1 ano), validação de integridade, Flow Logs (365 dias), logs do EKS |
| Registro financeiro por 5 anos, inalterável | bucket de liquidação com Object Lock COMPLIANCE (5 anos), versionado, logs de acesso |
| Menor privilégio | Pod Identity com políticas por serviço; SGs referenciando SGs; endpoints privados; nada público |
| Detecção e resposta | GuardDuty, Config + regras, Security Hub (PCI), alarmes SNS, alarme de root |
| Segredos fora do código, rotacionados | Secrets Manager (RDS gerenciado com rotação), External Secrets, sem chaves no CI (OIDC) |
| Disponibilidade | 3 AZs, Multi-AZ no RDS e Redis, 3 brokers, HPA + PDB, NAT por AZ |
| Backup e recuperação | RDS PITR 35 dias + snapshot final + `deletion_protection`; S3 versionado; state versionado |
| Residência de dados (LGPD) | tudo em `sa-east-1`; tag `DataClass=payments` para políticas |
| Superfície mínima | API do EKS privada, sem SSH, IMDSv2, Bottlerocket, distroless, tags imutáveis, scan contínuo |

---

# Glossário

| Termo | Significado |
|---|---|
| **Adquirente / credenciadora** | Empresa que credencia lojistas, processa transações de cartão e paga o lojista |
| **Agenda de recebíveis** | Lista de tudo que um EC tem a receber, por data |
| **Antecipação (ARV)** | Receber hoje, com desconto, o que venceria no futuro |
| **Arranjo de pagamento** | Conjunto de regras de uma bandeira/produto (ex.: Visa crédito) |
| **Autorização** | Pedido ao emissor para reservar o valor no limite/saldo do portador |
| **Bandeira** | Visa, Mastercard, Elo...; instituidor do arranjo |
| **BIN** | Primeiros 6–8 dígitos do cartão; identifica emissor e bandeira |
| **bps (basis point)** | 0,01%. 250 bps = 2,50% |
| **Captura** | Confirmação de que a venda aconteceu; só transações capturadas geram recebíveis |
| **Chargeback** | Contestação da compra pelo portador, com estorno ao EC |
| **Circuit breaker** | Padrão que para de chamar uma dependência que está falhando |
| **Clean Architecture** | Organização em camadas onde dependências apontam para o domínio |
| **Compliance** | Cumprimento de leis, normas do regulador e regras contratuais |
| **Conciliação** | Conferência de que autorizado = capturado = pago = repassado |
| **Consumer group** | Instâncias de um consumidor Kafka que dividem as partições |
| **CVV/CVC** | Código de segurança do cartão; nunca pode ser armazenado |
| **D+n** | n dias (corridos ou úteis) após a data da captura |
| **DLQ** | *Dead-letter queue*: tópico para mensagens que falharam permanentemente |
| **Domicílio bancário** | Conta em que o EC recebe |
| **DTO** | Struct só para transporte de dados (JSON, mensagem) |
| **EC** | Estabelecimento comercial; o lojista |
| **Emissor** | Banco que emitiu o cartão ao portador |
| **Evento de domínio** | Fato que aconteceu, publicado para quem quiser reagir |
| **gRPC** | Framework de chamadas remotas com contrato `.proto` e transporte binário |
| **HMAC** | Assinatura com chave secreta compartilhada; usada em webhooks |
| **HPA** | *Horizontal Pod Autoscaler*: escala réplicas no Kubernetes |
| **Idempotência** | Repetir a operação produz o mesmo resultado; essencial em pagamentos |
| **Intercâmbio** | Parte do MDR que vai para o emissor |
| **IP** | Instituição de pagamento (Lei 12.865/2013) |
| **ISO 8583** | Formato de mensagem entre maquininha, adquirente, bandeira e emissor |
| **JWT** | Token assinado com identidade e permissões |
| **KYC** | *Know Your Customer*: conhecer o cliente antes de credenciar |
| **Ledger** | Livro-razão com lançamentos de partidas dobradas |
| **Liquidação** | Pagamento efetivo ao EC (ou entre instituições) |
| **MCC** | Código de categoria do comerciante |
| **MDR** | Taxa que o EC paga sobre cada venda |
| **mTLS** | TLS em que cliente e servidor apresentam certificado |
| **NSU** | Número Sequencial Único da transação |
| **Núclea (ex-CIP)** | Câmara que centraliza a liquidação de cartões no Brasil |
| **Observabilidade** | Capacidade de entender o sistema por logs, métricas e traces |
| **Ônus** | Garantia dada sobre um recebível a um credor |
| **OpenAPI / Swagger** | Formato e ferramentas para descrever APIs REST |
| **Outbox** | Tabela onde eventos são gravados na mesma transação do dado, para publicação posterior |
| **PAN** | Número do cartão |
| **PCI DSS** | Padrão de segurança das bandeiras para dados de cartão |
| **PLD/FT** | Prevenção à lavagem de dinheiro e ao financiamento do terrorismo |
| **Port / Adapter** | Interface definida pelo núcleo / implementação concreta |
| **Portador** | Dono do cartão |
| **Registradora** | Entidade que registra as URs (CERC, TAG, B3, Núclea) |
| **Reversal** | Mensagem que desfaz uma autorização (ex.: após timeout) |
| **Saga** | Operação distribuída com compensações |
| **Scope** | Permissão granular carregada no token |
| **Subadquirente** | Empresa que revende o serviço da adquirente para lojistas menores |
| **Tokenização** | Trocar o PAN por um token sem valor fora do sistema |
| **Trace / span** | Caminho de uma requisição / um passo dele |
| **Unit of Work** | Tudo ou nada numa transação de banco |
| **UR** | Unidade de Recebível: EC + arranjo + adquirente + data de liquidação |
| **Webhook** | Chamada HTTP que a adquirente faz ao EC para avisar de um evento |

**Termos de nuvem (Parte 16)**

| Termo | Significado |
|---|---|
| **AZ (zona de disponibilidade)** | Datacenter(s) independentes dentro de uma região; espalhar em 3 dá tolerância a falha de prédio |
| **CloudTrail** | Registro de toda chamada à API da AWS: quem fez o quê, quando, de onde |
| **Config / Security Hub / GuardDuty** | Conformidade contínua / painel agregado com benchmarks (PCI) / detecção de ameaças |
| **EKS / RDS / MSK / ElastiCache / ECR** | Kubernetes / PostgreSQL / Kafka / Redis / registro de imagens, todos gerenciados pela AWS |
| **IaC** | Infraestrutura como código: recursos descritos em arquivos versionados e aplicados por ferramenta |
| **IAM / role / policy** | Identidades e permissões; uma role é uma identidade assumível; a policy diz o que ela pode |
| **IMDSv2** | Versão do serviço de metadados da instância que exige token; fecha o roubo de credenciais via SSRF |
| **KMS** | Serviço de chaves de cifragem; "envelope encryption" com rotação e auditoria de uso |
| **Object Lock (COMPLIANCE)** | Objeto S3 que ninguém apaga nem altera até o prazo, nem o root |
| **Pod Identity / IRSA** | Um pod do EKS assume uma role IAM própria, sem chaves |
| **Provider / resource / module / state** | Vocabulário do Terraform: plugin da nuvem / uma coisa criada / pasta reutilizável / registro do que existe |
| **Security group** | Firewall com estado por recurso; aqui, sempre referenciando outro SG, nunca `0.0.0.0/0` |
| **Subnet pública / privada / isolada** | Com rota para a internet / só saída via NAT / sem rota alguma para fora |
| **VPC / VPC endpoint** | Sua rede privada na AWS / caminho privado para um serviço AWS sem passar pela internet |
| **WAF** | Firewall de aplicação web: bloqueia ataques conhecidos e limita taxa antes de chegar à API |

---

# Referências

**Regulação (leia na fonte; verifique a versão vigente)**
- Lei 12.865/2013 — arranjos e instituições de pagamento: `planalto.gov.br`
- Resoluções BCB 80/2021 e 81/2021 — IPs e arranjos: `bcb.gov.br/estabilidadefinanceira/arranjos_pagamento`
- Resolução CMN 4.734/2019 e Circular BCB 3.952/2019 — registro de recebíveis
- Resolução BCB 85/2021 — segurança cibernética das IPs
- Circular BCB 3.978/2020 — PLD/FT
- Lei 13.709/2018 — LGPD
- PCI DSS v4.0: `pcisecuritystandards.org`
- Núclea (SLC): `nuclea.com.br`; registradoras: `cerc.inf.br`, `tag.com.vc`, `b3.com.br`

**Go**
- Tour e documentação: `go.dev/tour`, `go.dev/doc`
- Effective Go e Go Code Review Comments (estilo oficial)
- Livro: *Learning Go* (Jon Bodner); *100 Go Mistakes* (Teiva Harsanyi)
- Bibliotecas citadas: `gin-gonic/gin`, `oapi-codegen`, `sqlc`, `gorm.io`, `golang-migrate`, `golang-jwt/jwt`, `grpc-go`, `bufbuild/buf`, `segmentio/kafka-go`, `twmb/franz-go`, `sony/gobreaker`, `golang.org/x/sync/errgroup`, `testcontainers-go`, `moov-io/iso8583`

**Arquitetura e padrões**
- *Clean Architecture* (Robert C. Martin); *Domain-Driven Design* (Eric Evans); *Implementing DDD* (Vaughn Vernon)
- *Design Patterns* (Gamma et al.); `refactoring.guru/design-patterns` (versão visual, com Go)
- *Patterns of Enterprise Application Architecture* (Martin Fowler): Repository, Unit of Work
- `microservices.io/patterns`: Outbox, Saga, Idempotent Consumer, Circuit Breaker

**Dados, mensageria, observabilidade, infra**
- PostgreSQL docs (transações e isolamento): `postgresql.org/docs/current/transaction-iso.html`
- *Designing Data-Intensive Applications* (Martin Kleppmann): o melhor livro sobre consistência, replicação e mensageria
- Kafka docs: `kafka.apache.org/documentation`
- OpenTelemetry: `opentelemetry.io/docs/languages/go`
- Prometheus: `prometheus.io/docs`; Google SRE Book (SLOs): `sre.google/books`
- Kubernetes: `kubernetes.io/docs/concepts`; *Kubernetes Up & Running*
- OWASP Top 10: `owasp.org/Top10`; OWASP API Security Top 10

**Nuvem e Terraform**
- Terraform: `developer.hashicorp.com/terraform/docs`; provider AWS: `registry.terraform.io/providers/hashicorp/aws`
- AWS Well-Architected Framework (pilar de segurança): `docs.aws.amazon.com/wellarchitected`
- AWS Security Reference Architecture (multi-conta): `docs.aws.amazon.com/prescriptive-guidance/latest/security-reference-architecture`
- EKS Best Practices Guide (segurança): `docs.aws.amazon.com/eks/latest/best-practices/security.html`
- PCI DSS on AWS (compliance guide): `aws.amazon.com/compliance/pci-dss-level-1-faqs`
- Policy-as-code: `tfsec`, `checkov`, `trivy config`

**Especificações**
- HTTP: RFC 9110; Problem Details: RFC 9457; JWT: RFC 7519; OAuth 2.0: RFC 6749; W3C Trace Context; OpenAPI 3.1; Protocol Buffers Language Guide

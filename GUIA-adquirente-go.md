# Guia completo: construindo uma ADQUIRENTE do zero com Go

Este guia parte do zero. Ele não assume que você sabe o que é uma adquirente, nem que conhece Go, REST, gRPC, Kafka, Kubernetes, banco de dados, segurança, design patterns ou as regras do Banco Central. Cada conceito é explicado antes de ser usado, com analogias, exemplos numéricos e código.

O guia tem duas metades:

- **Partes 1 a 14 — os conceitos.** Primeiro o negócio (o que uma adquirente faz e por quê), depois a regulação, depois cada tecnologia. Cada parte termina com um bloco **"No nosso projeto"** dizendo exatamente onde aquele conceito entra na adquirente que você vai construir.
- **Parte 15 — o roteiro.** Um passo a passo em fases, do `go mod init` até o deploy em Kubernetes. Cada fase diz o que construir, quais conceitos usa, quais arquivos criar e como saber que está pronto.

**Pré-requisito sugerido:** o guia `simple_bank/GUIA-api-banco-go.md`. Lá você construiu uma API bancária com Go, Gin, PostgreSQL, DDD e Clean Architecture. Este guia usa a mesma organização de pastas (`domain`, `application`, `handler`, `infrastructure`) e assume que você já rodou aquele projeto. Se não rodou, tudo bem: a Parte 3 e a Parte 6 revisam o necessário, só que mais rápido.

**Sobre o código deste guia.** Os trechos das Partes 1 a 14 são exemplos curtos, escritos para explicar uma ideia por vez. A Parte 15 traz o esqueleto do projeto real. Você vai escrever e compilar o código na sua máquina fase a fase; é assim que se aprende. Quando um trecho for um esboço (e não um arquivo pronto), o texto avisa.

**Sobre as normas do Banco Central.** Regulação muda. Os números de resoluções e circulares citados aqui estavam corretos quando o guia foi escrito, mas antes de tomar qualquer decisão de negócio consulte o site do BCB (`bcb.gov.br`) e um advogado especializado. Este guia ensina engenharia; ele não substitui assessoria jurídica.

Dica de leitura: não tente decorar tudo. Leia a Parte 1 com calma (é a mais importante, porque todo o resto existe para servir a ela), passe o olho nas Partes 2 a 14, comece a Parte 15 e volte às partes de conceito quando o roteiro pedir.

---

## O que vamos construir

Uma **adquirente simulada**: o sistema que fica entre a maquininha do comerciante e o banco emissor do cartão, autoriza a compra, guarda a "agenda" do que o comerciante tem a receber, liquida (paga) esse valor na data certa e permite antecipar recebíveis.

Como não temos acesso à Visa, à Mastercard, ao Banco Central nem a um banco de verdade, vamos **simular** as pontas externas (emissor, bandeira, câmara de liquidação, registradora). Tudo o que fica no meio, que é a adquirente de fato, será real: regras de negócio, banco de dados, APIs, eventos, segurança, observabilidade e deploy.

| Capacidade | Como | Partes do guia |
|---|---|---|
| Credenciar um comerciante (EC) com plano de taxas e conta para receber | API REST (`POST /merchants`) | 4, 8, 15 |
| Autorizar uma transação de cartão (débito, crédito à vista, parcelado) | API REST (`POST /transactions`) + emissor simulado via gRPC | 4, 11, 15 |
| Capturar / cancelar uma transação | API REST | 15 |
| Gerar a agenda de recebíveis (o que o EC vai receber e quando) | Regras de domínio + evento Kafka | 1, 12, 15 |
| Liquidar (pagar) os recebíveis no vencimento | Worker que consome eventos e gera o "arquivo de liquidação" | 1, 12, 15 |
| Antecipar recebíveis com desconto | API REST + cálculo financeiro | 1, 15 |
| Tratar chargeback (contestação) | API REST + ajuste na agenda | 1, 15 |
| Avisar o comerciante (webhook assinado) | Consumidor Kafka + HMAC | 9, 12 |
| Autenticar e autorizar quem chama a API | JWT / OAuth2 client credentials + scopes | 10 |
| Documentar a API | OpenAPI / Swagger | 5 |
| Enxergar o que está acontecendo em produção | Logs estruturados, métricas Prometheus, tracing OpenTelemetry | 13 |
| Rodar tudo em produção | Docker + Kubernetes | 14 |

Tecnologias: **Go**, **Gin**, **PostgreSQL**, **GORM** e **sqlc** (para você comparar ORM com SQL explícito), **gRPC + Protocol Buffers**, **Kafka**, **Redis**, **OpenTelemetry + Jaeger**, **Prometheus + Grafana**, **Docker**, **Kubernetes**.

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
| 15 | O roteiro | Mão na massa, fase a fase |
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
| 12 | R$ 96,50 | 360 | 96,50 / 1,02¹² | R$ 76,10 |

Somando as 12, o EC recebe hoje cerca de **R$ 1.020,63** em vez de R$ 1.158,00 ao longo do ano. A diferença (R$ 137,37) é receita da adquirente, que vai receber os R$ 1.158,00 do emissor normalmente.

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

Use esta tabela quando estiver escrevendo o projeto. Cada linha é um requisito que você vai implementar em alguma fase da Parte 15.

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
- `oapi-codegen` gera tipos e interface do servidor Gin; os handlers implementam a interface.
- `/docs` expõe o Swagger UI (só em ambientes internos; em produção, a doc fica num portal).
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
│   ├── merchant/        merchant.go, fee_plan.go, bank_account.go, document.go, repository.go
│   ├── transaction/     transaction.go (máquina de estados), events.go, repository.go, issuer_gateway.go
│   ├── receivable/      receivable.go, schedule.go (D+n, dias úteis), repository.go, registry_port.go
│   ├── settlement/      settlement.go, repository.go
│   ├── anticipation/    anticipation.go, pricing.go (valor presente), repository.go
│   ├── chargeback/
│   └── shared/          money.go, clock.go, event.go, errors.go
├── application/
│   ├── merchant/        onboard.go, approve.go
│   ├── transaction/     authorize.go, capture.go, cancel.go, refund.go
│   ├── settlement/      settle_due.go, reconcile.go
│   ├── anticipation/    simulate.go, request.go
│   └── uow.go
├── infrastructure/
│   ├── postgres/        repositórios (GORM e/ou database/sql), uow.go, migrations
│   ├── kafka/           publisher.go, consumer.go, outbox_relay.go
│   ├── grpcclient/      issuer.go
│   ├── redis/           idempotency.go, ratelimit.go
│   └── registry/        fake_registry.go
└── handler/
    ├── http/            gin: router.go, middlewares/, merchants.go, transactions.go, dto.go, problem.go
    └── grpc/            issuer-sim server
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

Decisão do guia, que também é a mais comum em fintechs Go: **sqlc ou `database/sql` para o núcleo** (transações, recebíveis, liquidação, ledger) e **GORM para o cadastro** (merchants, usuários, planos de taxa). Assim você aprende os dois e sente a diferença.

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

- PostgreSQL com migrações golang-migrate; `sqlc` no núcleo financeiro; GORM no cadastro.
- Dinheiro em `BIGINT` centavos; taxas em bps.
- `SELECT ... FOR UPDATE` para autorizar/capturar; `FOR UPDATE SKIP LOCKED` no worker de liquidação; lock otimista no cadastro.
- Tabelas `outbox`, `processed_events`, `idempotency_keys`, `ledger_entries` desde a primeira fase.
- Redis para idempotência (com fallback na tabela), rate limit e cache.

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
- Consumidores: `scheduler` (gera agenda), `notifier` (webhooks), `registry-sync` (registra URs), `ledger` (partidas dobradas), `reconciliation`.

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

# PARTE 15 — O roteiro: construindo a adquirente fase a fase

Cada fase tem: **objetivo**, **conceitos** (com a parte do guia para reler), **o que construir**, **como testar** e **critério de pronto**. Faça uma fase por vez, na ordem. Commit ao fim de cada uma. Não pule para Kubernetes antes de o domínio estar sólido: a ordem existe para você sentir cada problema antes de conhecer a solução.

Estimativa honesta: uma pessoa estudando com dedicação leva de 2 a 4 meses para passar por tudo. O que importa é a Fase 4 funcionar bem; dali para a frente é incremento.

---

## Fase 0 — Ambiente e esqueleto

**Objetivo:** projeto criado, ferramentas instaladas, Docker Compose com Postgres/Redis/Kafka/Jaeger subindo, um `GET /health` respondendo.

**Conceitos:** Go e layout (Parte 3), containers (Parte 14.1).

**Construir:**

```
adquirente/
├── cmd/api/main.go
├── internal/{domain,application,infrastructure,handler}/   (vazios, com doc.go)
├── api/openapi.yaml            (só /health por enquanto)
├── deploy/docker/api.Dockerfile
├── docker-compose.yml          (postgres, redis, kafka, kafka-ui, jaeger, prometheus, grafana, api)
├── migrations/
├── Makefile                    (run, test, lint, migrate-up, compose-up, generate)
├── .golangci.yml
├── .env.example
└── go.mod
```

Ferramentas: `go`, `docker`, `golangci-lint`, `migrate`, `sqlc`, `buf`, `oapi-codegen`, `kind`, `kubectl`.

**Testar:** `make compose-up` sobe tudo; `curl localhost:8080/health` responde `{"status":"ok"}`; Jaeger abre em `localhost:16686`; Kafka UI em `localhost:8081`.

**Pronto quando:** tudo sobe com um comando, e `make lint test` passa (mesmo sem testes ainda).

---

## Fase 1 — O domínio (sem banco, sem HTTP)

**Objetivo:** todas as regras de negócio escritas e testadas, em Go puro. Esta é a fase mais importante e a que mais vale a pena caprichar.

**Conceitos:** o negócio inteiro (Parte 1), SOLID/DDD (Parte 6), padrões Factory, State, Strategy, Specification (Parte 7), dinheiro em centavos (Parte 8.5).

**Construir**, em `internal/domain/`:

`shared/money.go` — `Money int64`; `Bps int`; `ApplyBps(gross Money, rate Bps) Money`; `Split(total Money, n int) []Money` (divide em n parcelas, diferença de centavos na primeira); `String()`.

`shared/clock.go` — `type Clock interface{ Now() time.Time }`; `RealClock`; `FixedClock` para testes.

`shared/calendar.go` — `type BusinessCalendar interface{ NextBusinessDay(t time.Time) time.Time; AddBusinessDays(t time.Time, n int) time.Time }`; implementação com fins de semana + lista de feriados nacionais.

`shared/event.go` — `type DomainEvent interface{ EventID() string; EventType() string; OccurredAt() time.Time; AggregateID() string }`.

`merchant/` — `Document` (CNPJ com validação de dígitos), `BankAccount`, `FeePlan` (mapa produto+parcelas → MDR bps; `AnticipationRateBpsMonth`), `Merchant` com estados `UNDER_REVIEW → ACTIVE → BLOCKED` e métodos `Approve()`, `Block()`, `IsActive()`. Erros: `ErrInvalidDocument`, `ErrMerchantInactive`.

`transaction/` — `Product` (DEBIT, CREDIT), `CardInfo{Token, Brand, BIN, Last4}` (nunca PAN), `Transaction` com a máquina de estados da Parte 7.4 e métodos `Authorize(code, nsu)`, `Deny(reasonCode)`, `Capture(now)`, `Cancel(now)`, `Chargeback(now)`. Regras: débito só 1 parcela; parcelas dentro do permitido pelo plano; valor > 0. Eventos: `TransactionAuthorized`, `TransactionDenied`, `TransactionCaptured`, `TransactionCanceled`. Port `IssuerGateway`.

`receivable/` — `Receivable{ID, TransactionID, MerchantID, InstallmentNo, Gross, Fee, Net, DueDate, Status}` com estados `SCHEDULED → ANTICIPATED | SETTLED | CANCELED | CHARGEBACKED`. Serviço de domínio `Schedule(tx, feePlan, calendar) ([]*Receivable, error)`:

```go
// Regras: débito → 1 recebível em D+1 útil.
// crédito n parcelas → n recebíveis em D+30·k, ajustados para o próximo dia útil.
// fee = ApplyBps(gross, mdr); net = gross - fee; parcelas via Split.
```

Port `ReceivablesRegistry` (`Register`, `CheckLiens`). `ReceivableUnit` (agrupamento por EC + arranjo + data).

`anticipation/` — `Pricer` (Strategy) com `PresentValue(net Money, days int, rateBpsMonth Bps) Money` (juros compostos pro-rata die); `Anticipation` com `Simulate(receivables, today, rate)` e `Confirm()`; Specification `IsAnticipable(today)`.

`settlement/` — `Settlement{ID, Date, MerchantID, BankAccount, Total, Status, ExternalRef}`; `Build(receivables []*Receivable, liens []Lien) ([]*Settlement, error)` que agrupa e redireciona para credor quando há ônus.

`chargeback/` — `Chargeback{TransactionID, ReasonCode, Amount, Status, Deadline}`; abrir, defender, resolver.

`ledger/` — `Entry{Account, MerchantID, Debit, Credit, RefType, RefID}`; funções que geram os pares para captura, liquidação, antecipação e chargeback; invariante: soma de débitos == soma de créditos.

**Testar:** table-driven em tudo (Parte 3.7). Casos obrigatórios: `Split(1000, 3)` = `[334, 333, 333]`; D+30 caindo no sábado vai para segunda; parcela 12 de uma venda em 31/01; antecipação de 12 parcelas bate com a tabela da Parte 1.5 (tolerância de 1 centavo); transição inválida retorna `ErrInvalidTransition`; ledger sempre fecha em zero. `go test -cover ./internal/domain/...` ≥ 95%.

**Pronto quando:** o domínio compila sem importar nada além da biblioteca padrão, e você consegue explicar cada regra da Parte 1 apontando para o teste que a prova.

---

## Fase 2 — Credenciamento: a primeira API completa

**Objetivo:** `POST /v1/merchants`, `GET /v1/merchants/{id}`, `POST /v1/merchants/{id}/approve`, `PUT /v1/merchants/{id}/fee-plan`, com Postgres (GORM), autenticação JWT, OpenAPI e observabilidade básica.

**Conceitos:** REST e Gin (Parte 4), OpenAPI/oapi-codegen (Parte 5), GORM e migrações (Parte 8), JWT/scopes (Parte 10), slog/OTel (Parte 13), KYC (Parte 2.5).

**Construir:**
- Migrações `000001_merchants`, `000002_merchant_bank_accounts`, `000003_fee_plans`, `000004_audit_log`.
- `application/merchant/onboard.go`, `approve.go`, `update_fee_plan.go` (um caso de uso por arquivo, com `Command` de entrada).
- `infrastructure/postgres/gorm/merchant_repository.go` com lock otimista.
- `handler/http/`: `router.go`, `problem.go`, middlewares `RequestID`, `Logger`, `Recovery`, `Authenticate`, `Authorize`, `Audit`; handlers gerados a partir do `openapi.yaml`.
- `cmd/auth-sim`: emite JWT RS256 com `merchant_id` e scopes; publica JWKS.
- OTel + slog + Prometheus ligados no `main`.

**Testar:** testes de handler com `httptest` e caso de uso com repositório fake; teste de integração do repositório com testcontainers; fluxo manual pelo Swagger UI em `/docs`: criar EC (fica `UNDER_REVIEW`), aprovar com token que tem `merchants:approve`, tentar aprovar com token sem o scope (403).

**Pronto quando:** você abre o Jaeger e vê o trace de um `POST /v1/merchants` com o span do Postgres dentro; o log JSON tem `trace_id`; `GET /metrics` mostra `http_requests_total`.

---

## Fase 3 — Autorização: o coração

**Objetivo:** `POST /v1/transactions` autoriza uma transação de cartão chamando o vault e o emissor simulado via gRPC, com idempotência, timeout e reversal.

**Conceitos:** ciclo da transação (Parte 1.2), idempotência (Parte 4.3), gRPC (Parte 11), tokenização e mascaramento (Parte 9.6), Saga/Circuit Breaker/Retry (Parte 7.6), `context` e *late response* (Parte 3.6).

**Construir:**
- `api/proto/vault/v1`, `api/proto/issuer/v1`; `buf generate`.
- `cmd/vault`: gRPC `Tokenize(pan, expiry) → token`, `Detokenize(token) → pan`; AES-256-GCM; tabela própria `card_vault(token, ciphertext, nonce, brand, bin, last4)`; **nenhuma outra tabela do sistema pode ter PAN**.
- `cmd/issuer-sim`: regras da Parte 11.4 + latência artificial configurável (para testar timeout) + modo "responde depois de 3s" (para testar late response).
- Migração `000005_transactions`, `000006_idempotency_keys`, `000007_outbox`.
- `application/transaction/authorize.go`:

```go
func (uc *AuthorizeUseCase) Execute(ctx context.Context, cmd AuthorizeCommand) (*transaction.Transaction, error) {
    m, err := uc.merchants.FindActive(ctx, cmd.MerchantID)            // 1. EC ativo?
    if err != nil { return nil, err }
    tx, err := transaction.New(m, cmd.Amount, cmd.Product, cmd.Installments, cmd.Card, uc.clock.Now())
    if err != nil { return nil, err }                                  // 2. regras de domínio
    if err := uc.preAuth.Run(ctx, m, tx); err != nil { return nil, err } // 3. chain: limites, velocidade
    pan, err := uc.vault.Detokenize(ctx, cmd.Card.Token)               // 4. só aqui o PAN existe, em memória
    if err != nil { return nil, err }
    resp, err := uc.issuer.Authorize(ctx, buildIssuerRequest(tx, pan)) // 5. emissor, com timeout 2s + breaker
    pan = ""                                                           //    e some
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        _ = uc.issuer.Reverse(context.WithoutCancel(ctx), tx.ID())     // 6. Saga: compensa
        tx.Deny("91")                                                  //    "emissor indisponível"
    case err != nil:
        return nil, fmt.Errorf("issuer: %w", err)
    case resp.Approved:
        tx.Authorize(resp.Code, uc.nsu.Next())
    default:
        tx.Deny(resp.ResponseCode)
    }
    err = uc.uow.Do(ctx, func(r Repositories) error {                  // 7. tudo ou nada
        if err := r.Transactions.Save(ctx, tx); err != nil { return err }
        return r.Outbox.Append(ctx, tx.PullEvents()...)
    })
    return tx, err
}
```

- Middleware `Idempotency` (Redis `SET NX` + tabela); rota `POST /v1/transactions`, `POST /v1/transactions/{id}/capture`, `POST /v1/transactions/{id}/cancel`, `GET /v1/transactions/{id}`.
- Adapter `grpcclient.Issuer` com `gobreaker` e métricas (Decorator).
- Relay da outbox (goroutine) publicando em `transaction.*` no Kafka (só publica; ninguém consome ainda).

**Testar:** unitários do caso de uso com fakes de vault/emissor (aprovada, negada, timeout → reversal chamado); teste de idempotência: dois `POST` iguais em paralelo (`errgroup`) → uma transação, duas respostas iguais; mesmo `Idempotency-Key` com corpo diferente → 409; capturar transação negada → 409 com `INVALID_TRANSITION`; `grep` no log procurando 16 dígitos seguidos → zero ocorrências.

**Pronto quando:** no Jaeger, um trace de autorização mostra API → vault → issuer-sim → Postgres → (outbox); no Kafka UI aparece `transaction.captured` com `card_last4` e sem PAN.

---

## Fase 4 — Agenda de recebíveis por eventos

**Objetivo:** o consumidor `scheduler` gera os recebíveis a partir de `transaction.captured`; o EC consulta sua agenda; o ledger registra.

**Conceitos:** agenda (Parte 1.4 e 1.6), Kafka e consumidor idempotente (Parte 12), Observer/Outbox (Parte 7), CQRS leve (Parte 7.6).

**Construir:**
- Migrações `000008_receivables`, `000009_processed_events`, `000010_ledger_entries`, `000011_receivables_daily_summary`.
- `cmd/scheduler`: consumidor de `transaction.captured` e `transaction.canceled`; usa `receivable.Schedule`; grava recebíveis + lançamentos do ledger (`payable_to_merchant` / `revenue_fees`) + resumo diário na mesma transação; publica `receivable.scheduled`.
- `GET /v1/merchants/{id}/receivables?status=&from=&to=` (paginado por cursor) e `GET /v1/merchants/{id}/receivables/summary` (lê o resumo).
- `cmd/registry-sim` (gRPC) e consumidor `registry-sync` que registra URs a partir de `receivable.scheduled`.
- DLQ + alerta.

**Testar:** capture uma venda de R$ 1.200 em 12x e confira a agenda contra a tabela da Parte 1.4; derrube o `scheduler`, capture 10 vendas, suba de novo: as 10 agendas aparecem (nada perdido); reprocesse o tópico do início (reset de offset): nenhum recebível duplicado (`processed_events` funcionando); soma do ledger é zero.

**Pronto quando:** o trace de uma captura continua no `scheduler` como span filho (propagação pelo header Kafka), e `kafka_consumer_lag` aparece no Grafana.

---

## Fase 5 — Liquidação

**Objetivo:** o worker diário paga o que vence hoje, respeitando ônus, gerando o arquivo de liquidação e conciliando.

**Conceitos:** liquidação e conciliação (Parte 1.7 e 1.8), registro/ônus (Parte 2.4), `FOR UPDATE SKIP LOCKED` (Parte 8.3), `errgroup` (Parte 3.6), Template Method (Parte 7.4), CronJob (Parte 14).

**Construir:**
- Migração `000012_settlements`, `000013_settlement_orders`.
- `application/settlement/settle_due.go`: lista recebíveis `SCHEDULED` com `due_date = hoje` (sqlc, `SKIP LOCKED`, em lotes por EC), chama `registry.CheckLiens`, monta `Settlement`s com `settlement.Build`, gera ordens de pagamento (arquivo CNAB simplificado ou JSON), marca `SETTLED`, lança no ledger (`payable_to_merchant` ↓, `bank_outgoing` ↑), publica `settlement.completed`. Lock distribuído no Redis para "uma execução por dia".
- `cmd/settlement`: binário que roda uma vez e sai (para virar CronJob); flag `--date` para reprocessar um dia.
- `cmd/bank-sim`: recebe o arquivo e devolve um "retorno" com sucesso/falha por ordem; `application/settlement/reconcile.go` lê o retorno e trata falhas (conta inválida → recebível volta para `SCHEDULED` com pendência e alerta).
- `GET /v1/merchants/{id}/settlements`.

**Testar:** cenário com 3 ECs, um com ônus parcial na registradora simulada: o valor com ônus vai para o credor, o resto para o EC; rodar o worker duas vezes no mesmo dia: a segunda não paga nada; matar o worker no meio (kill -9) e rodar de novo: nada pago em dobro, nada perdido (graças ao `SKIP LOCKED` + estados); conciliação aponta a ordem que o `bank-sim` rejeitou.

**Pronto quando:** `make settle DATE=2026-10-13` liquida e o Grafana mostra "valor liquidado hoje" igual ao "previsto para hoje" do resumo.

---

## Fase 6 — Antecipação

**Objetivo:** o EC simula e contrata antecipação; os recebíveis mudam de dono.

**Conceitos:** antecipação (Parte 1.5), Strategy/Specification (Parte 7), lock pessimista (Parte 8.3).

**Construir:**
- Migração `000014_anticipations`.
- `POST /v1/anticipations/simulate` (não muda nada; devolve bruto, desconto, líquido, por recebível) e `POST /v1/anticipations` (com `Idempotency-Key`): dentro da UoW, `SELECT ... FOR UPDATE` nos recebíveis, checa `IsAnticipable`, checa ônus na registradora, marca `ANTICIPATED`, cria `Anticipation`, ledger (`payable_to_merchant` ↓ pelo líquido... na verdade: `payable_to_merchant` ↓ pelo bruto, `revenue_anticipation` ↑ pelo desconto, `bank_outgoing` ↑ pelo líquido), publica `receivable.anticipated` (o `registry-sync` registra a troca de titularidade).
- Antecipação **automática** opcional: flag no EC; o `scheduler` já cria o recebível como antecipado e o liquida em D+1.
- Ajuste no worker de liquidação: recebível `ANTICIPATED` no vencimento **não** paga o EC (o dinheiro fica com a adquirente); ledger reflete.

**Testar:** simulação bate com a tabela da Parte 1.5; duas contratações simultâneas dos mesmos recebíveis → uma passa, a outra recebe 409; antecipar recebível com ônus → 422 `RECEIVABLE_LIENED`; no vencimento, o worker não paga o EC pelo recebível antecipado.

---

## Fase 7 — Chargeback, estorno, webhooks e monitoramento PLD

**Objetivo:** fechar o ciclo de vida e avisar o EC de tudo.

**Conceitos:** chargeback (Parte 1.8), webhooks/HMAC (Partes 4.6 e 9.2), PLD (Parte 2.5).

**Construir:**
- `POST /v1/transactions/{id}/refund` (estorno após liquidação: recebível negativo ou débito no próximo repasse) e `POST /internal/chargebacks` (simula a bandeira abrindo contestação): `Transaction.Chargeback()`, recebíveis `SCHEDULED` viram `CHARGEBACKED`; se já `SETTLED`/`ANTICIPATED`, gera débito na agenda futura; ledger; eventos.
- `cmd/notifier`: consome `transaction.*`, `settlement.*`, `chargeback.*`; envia webhook assinado (HMAC-SHA256, `X-Signature`, `X-Event-Id`) para a URL do EC; retries com backoff; tabela `webhook_deliveries`; validação anti-SSRF no cadastro da URL.
- Job `pld-monitor` (CronJob a cada hora): regras simples sobre `transactions` (valor médio, picos, horário); grava `alerts`.

**Testar:** chargeback de venda 12x com 3 parcelas já liquidadas: 9 canceladas + débito de 3 no próximo repasse; webhook: um receptor de teste (`cmd/merchant-sim`) valida a assinatura e rejeita corpo adulterado; desligue o receptor, gere eventos, ligue: entregas chegam com retries.

---

## Fase 8 — Observabilidade completa e hardening

**Objetivo:** dashboards, alertas, SLO, auditoria, segurança de dependências.

**Conceitos:** Parte 13 inteira, Parte 9.7.

**Construir:** dashboard Grafana da Parte 13.6 (exporte o JSON para `deploy/grafana/`); regras de alerta da Parte 13.5; `govulncheck` e Trivy no CI; `audit_log` completo; runbooks em `docs/runbooks/`; teste de carga com `k6` (meta: p99 < 500ms na autorização com emissor simulado a 50ms, 200 req/s numa máquina de dev).

**Pronto quando:** você derruba o `issuer-sim` e, em menos de 1 minuto, o alerta "taxa de aprovação caiu" dispara, o breaker abre, a API responde 503 rápido, e o dashboard mostra tudo isso.

---

## Fase 9 — Kubernetes

**Objetivo:** tudo rodando num cluster kind, com deploy sem queda.

**Conceitos:** Parte 14 inteira.

**Construir:** Dockerfiles `distroless` por binário; `deploy/k8s/base` + `overlays/dev`; Secrets e ConfigMaps; probes; HPA na API; CronJobs (settlement, pld-monitor); NetworkPolicy no vault; Ingress com TLS (cert autoassinado em dev); OTel Collector, Prometheus, Grafana, Jaeger no cluster; `make k8s-up`.

**Testar:** `kubectl rollout restart deploy/api` durante um `k6` de autorizações: zero erros (graceful shutdown + `maxUnavailable: 0`); `kubectl delete pod` de um `scheduler`: outro assume a partição, lag volta a zero; CronJob de liquidação rodou às 06:00 e `concurrencyPolicy: Forbid` impediu o segundo.

---

## Fase 10 — Compliance e fechamento

**Objetivo:** revisar o checklist da Parte 2.8 item a item e documentar.

**Construir:** política de retenção e job de anonimização (LGPD); revisão PCI (escopo = só o vault; PAN nunca em log/evento/API); mTLS entre serviços (cert-manager ou mesh); relatório "o que este sistema faz para cada exigência" em `docs/compliance.md`; `docs/architecture.md` com o diagrama C4 dos serviços e os fluxos de autorização, agenda, liquidação e antecipação.

**Pronto quando:** cada linha do checklist da Parte 2.8 aponta para um arquivo, um teste ou um documento.

---

## Depois disso

Se quiser continuar: Pix (arranjo instantâneo, liquidação via SPI), split de pagamento (marketplaces), painel interno com OIDC e RBAC, antifraude com score, integração real com uma registradora em sandbox (a CERC e a TAG têm ambientes de homologação), ISO 8583 com uma biblioteca Go (`moov-io/iso8583`) para sentir o protocolo real, e multi-tenancy (uma plataforma que serve várias subadquirentes).

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

**Especificações**
- HTTP: RFC 9110; Problem Details: RFC 9457; JWT: RFC 7519; OAuth 2.0: RFC 6749; W3C Trace Context; OpenAPI 3.1; Protocol Buffers Language Guide

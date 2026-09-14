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

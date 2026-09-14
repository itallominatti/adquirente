package shared

import "errors"

var (
	ErrNotFound          = errors.New("registro não encontrado")
	ErrInvalidAmount     = errors.New("o valor deve ser maior que zero")
	ErrInvalidTransition = errors.New("transição de estado inválida")
	ErrConcurrentUpdate  = errors.New("o registro foi alterado por outra operação; tente novamente")
)

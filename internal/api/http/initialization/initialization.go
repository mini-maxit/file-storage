package initialization

import "github.com/mini-maxit/file-storage/internal/config"

type Initialization struct {
}

func NewInitialization(cfg *config.Config) *Initialization {
	return &Initialization{}
}

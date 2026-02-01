package league

import (
	"wagr/src/internal/fantasy/sleeper"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db *pgxpool.Pool
	sleeperClient *sleeper.Client
}

func NewService(db *pgxpool.Pool, sleeperClient *sleeper.Client) *Service {
	return &Service{
		db: db,
		sleeperClient: sleeperClient,
	}
}



package repository

import (
	"context"
	"database/sql"
	"log"

	"github.com/stilln0thing/GoKart/services/user-service/internal/domain"
)

type postgresRepo struct {
	db *sql.DB
}

func NewPostgresRepo(db *sql.DB) UserRepository {
	return &postgresRepo{db: db}
}

func (r *postgresRepo) CreateUser(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (id, username, email, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)`
	_, err := r.db.ExecContext(ctx, query, user.ID, user.Username, user.Email, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return err
	}
	log.Println("User created successfully")
	return nil
}

func (r *postgresRepo) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT id, username, email, created_at, updated_at FROM users WHERE email = $1`
	row := r.db.QueryRowContext(ctx, query, email)

	var user domain.User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No user found
		}
		return nil, err
	}
	return &user, nil
}
	
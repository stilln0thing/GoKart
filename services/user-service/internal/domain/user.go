package domain

import (
	"time"
	"errors"
)

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u *User) Validate() error {
	if u.Username == "" {
		return errors.New("Username cannot be empty")
	}
	if u.Email == "" {
		return errors.New("Email cannot be empty")
	}
	return nil
}
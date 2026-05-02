package domain

import "time"

type Account struct {
	ID               string
	AccountNumber    string
	AccountType      string
	BankCode         string
	Balance          float64
	HeldBalance      float64
	AvailableBalance float64
	Currency         string
	Status           string
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

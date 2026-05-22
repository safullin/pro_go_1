package model

import "time"

const (
	OrderStatusNew        = "NEW"
	OrderStatusProcessing = "PROCESSING"
	OrderStatusInvalid    = "INVALID"
	OrderStatusProcessed  = "PROCESSED"
)

type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

type Order struct {
	UploadedAt time.Time
	Accrual    *float64
	Number     string
	Status     string
	UserID     int64
}

type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Withdrawal struct {
	ProcessedAt time.Time
	Order       string
	Sum         float64
}

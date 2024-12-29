package models

import (
	"github.com/jinzhu/gorm"
	"time"
)

type Token struct {
	gorm.Model
	ExpiresAt time.Time
	TokenHash string
	CompanyID string

	State      string
	GrantToken string
	ConsentId  string

	AccessToken  string
	RefreshToken string

	Payments []Payment
}

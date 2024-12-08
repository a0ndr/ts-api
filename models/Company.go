package models

type Company struct {
	ID          string `gorm:"primaryKey;size:8"`
	CompanyName string
	Enabled     bool `gorm:"index"`

	Tokens []Token
}

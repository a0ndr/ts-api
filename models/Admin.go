package models

import "gorm.io/gorm"

type Admin struct {
	gorm.Model

	FullName string
	Email    string

	PasswordHash string
	Role         string // ro, admin
}

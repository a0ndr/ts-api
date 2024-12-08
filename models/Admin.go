package models

import "gorm.io/gorm"

type Admin struct {
	gorm.Model

	Username string
	FullName string
	Email    string

	PasswordHash string
	Roles        []*Role `gorm:"many2many:admin_roles;"`
}

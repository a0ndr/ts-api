package models

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	Name     string
	Policies []Policy
	Admins   []*Admin `gorm:"many2many:role_admins;"`
}

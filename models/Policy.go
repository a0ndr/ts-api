package models

import "gorm.io/gorm"

type PolicyNode struct {
	Group     string `json:"group"`
	Action    string `json:"action"`
	Permitted bool   `json:"permitted"`
}

type Policy struct {
	gorm.Model
	Name        string
	PolicyNodes []PolicyNode
}

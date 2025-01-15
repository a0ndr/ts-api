package kernel

import (
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/matthewhartstonge/argon2"
	"log"
)

func (art *AppRuntime) Seed() {
	if art.DatabaseClient.Find(&models.Admin{}).RowsAffected == 0 {
		argon := argon2.DefaultConfig()
		password, _ := argon.HashEncoded([]byte("P@ssw0rd123"))
		admin := &models.Admin{
			FullName:     "Default Admin",
			Email:        "admin@example.com",
			PasswordHash: string(password),
			Role:         "admin",
		}
		art.DatabaseClient.Create(admin)
		log.Printf(" * Created new admin: admin@example.com:P@ssw0rd123")
	}
}

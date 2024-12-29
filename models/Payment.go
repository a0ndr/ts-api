package models

import "gorm.io/gorm"

//goland:noinspection ALL
const (
	PTYPE_SEPA         = "sepa-credit-transfers"
	PTYPE_SEPA_INSTANT = "instant-sepa-credit-transfers"
	PTYPE_CROSS_BORDER = "cross-border-credit-transfers"
)

type Payment struct {
	gorm.Model

	TokenID uint

	Amount       string
	Currency     string
	DebtorIban   string
	CreditorIban string
	CreditorName string
	Note         string

	Type               string
	InstantPaymentFlow string
	ChargeBearer       string

	PaymentID       string
	AuthorizationID string
}

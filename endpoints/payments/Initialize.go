package payments

import (
	"bytes"
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/assert"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"io"
	"log"
	"net/http"
	"slices"
)

type InitPaymentDto struct {
	Amount       string `json:"amount" binding:"required"`
	Currency     string `json:"currency" binding:"required"` //e.g. EUR
	DebtorIban   string `json:"debtorIban" binding:"required"`
	CreditorIban string `json:"creditorIban" binding:"required"` //e.g. SK6909000000001234567890
	CreditorName string `json:"creditorName" binding:"required"`
	Note         string `json:"note" binding:"required"` // remittanceInformationUnstructured
	Type         string `json:"type" binding:"required"` // refer to TransTypeValues

	// for instant-sepa-credit-transfers
	// refer to InstantPaymentFlowValues
	InstantPaymentFlow string `json:"instantPaymentFlow"`

	ChargeBearer string `json:"chargeBearer"` // refer to ChargeBearerValues
}

type PaymentInfo struct {
	PaymentId       string `json:"paymentId"`
	AuthorizationId string `json:"authorizationId"`
	Links           struct {
		ScaRedirect struct {
			Href string `json:"href"`
		} `json:"scaRedirect"`
	} `json:"_links"`
	TransactionStatus string `json:"transactionStatus"`
}

var ChargeBearerValues = []string{
	"DEBT", "CRED",
	"SHAR", "SLEV",
}

var TransTypeValues = []string{
	models.PTYPE_SEPA,
	models.PTYPE_SEPA_INSTANT,
	models.PTYPE_CROSS_BORDER,
}

var InstantPaymentFlowValues = []string{
	"OPTIONAL", "MANDATORY",
}

func InitPayment(rt *kernel.RequestRuntime, dto *InitPaymentDto) (*PaymentInfo, error) {
	art := rt.AppRuntime
	tok := rt.Token

	rt.NewChildTracer("payment_init.real").Advance()

	jData := gin.H{
		"instructedAmount": &gin.H{
			"amount":   dto.Amount,
			"currency": dto.Currency,
		},
		"debtorAccount": &gin.H{
			"iban": dto.DebtorIban,
		},
		"creditorAccount": &gin.H{
			"iban": dto.CreditorIban,
		},
		"creditorName":                      dto.CreditorName,
		"remittanceInformationUnstructured": dto.Note,
	}

	if dto.Type == models.PTYPE_CROSS_BORDER {
		jData["chargeBearer"] = dto.ChargeBearer
	}

	j, err := json.Marshal(&jData)
	if err != nil {
		return nil, rt.MakeErrorf("could not marshal data: %v", err)
	}

	tbUrl := fmt.Sprintf("%s/v3/payments/%s", art.TbUrl, dto.Type)
	r, err := http.NewRequest(http.MethodPost, tbUrl, bytes.NewBuffer(j))
	if err != nil {
		return nil, rt.MakeErrorf("could not create request: %v", err)
	}

	requestId, err := kernel.UuidV7()
	if err != nil {
		return nil, rt.MakeErrorf("could not generate request id: %v", err)
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", "Bearer "+tok.AccessToken)
	r.Header.Add("X-Request-ID", requestId)
	rt.Span.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	if dto.Type == models.PTYPE_SEPA_INSTANT {
		r.Header.Add("Instant-Payment-Flow", dto.InstantPaymentFlow)
	}

	client := &http.Client{}
	rsp, err := client.Do(r)
	if err != nil {
		return nil, rt.MakeErrorf("could not execute request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("could not close response body: %v", err)
		}
	}(rsp.Body)

	if rsp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(rsp.Body)
		if err != nil {
			return nil, rt.MakeErrorf("failed to read response body: %v", err)
		}

		_ = rt.MakeErrorf("http request returned %d: %s", rsp.StatusCode, string(body))

		var e map[string]interface{}
		if err := json.Unmarshal(body, &e); err != nil {
			return nil, rt.MakeErrorf("could not unmarshal response body: %v", err)
		}

		if val, ok := e["transactionStatus"]; ok && val.(string) == "RJCT" {
			if val_, ok := e["errorCode"]; ok && val_.(string) == "FORMAT_ERROR" {
				return nil, rt.MakeErrorf("transaction data badly formatted: %s", e["additionalInformation"].(string))
			}
			if val_, ok := e["additionalInformation"]; ok {
				return nil, rt.MakeErrorf("transaction was rejected: %s", val_.(string))
			}
			return nil, rt.MakeErrorf("transaction was rejected")
		}

		return nil, rt.MakeErrorf("tatra banka returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rt.MakeErrorf("could not read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	var res PaymentInfo
	if err = json.Unmarshal(body, &res); err != nil {
		return nil, rt.MakeErrorf("could not unmarshal data: %v", err)
	}

	rt.EndBlock()
	return &res, nil
}

func InitializePayment(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.NewChildTracer("payment_init.handler").Advance()

	assert.NotNil(rt.Token, "token != nil")

	var dto InitPaymentDto
	if err := c.ShouldBindJSON(&dto); err != nil {
		rt.Ef(400, "bad request: %v", err)
		return
	}
	switch dto.Type {
	case models.PTYPE_SEPA:
	case models.PTYPE_CROSS_BORDER:
		if dto.ChargeBearer == "" {
			rt.Ef(400, "bad request: charge bearer required, options: %v", ChargeBearerValues)
			return
		}
		if !slices.Contains(ChargeBearerValues, dto.ChargeBearer) {
			rt.Ef(400, "bad request: invalid charge bearer value, options: %v", ChargeBearerValues)
			return
		}
	case models.PTYPE_SEPA_INSTANT:
		if dto.InstantPaymentFlow == "" {
			dto.InstantPaymentFlow = "OPTIONAL"
		} else if !slices.Contains(InstantPaymentFlowValues, dto.InstantPaymentFlow) {
			rt.Ef(http.StatusBadRequest, "bad request: invalid instant payment flow value, options: %v", InstantPaymentFlowValues)
			return
		}
	default:
		rt.Ef(400, "invalid payment type, options: %v", TransTypeValues)
		return
	}

	pi, err := InitPayment(rt, &dto)
	if err != nil {
		rt.Ef(500, "failed to initialize payment: %v", err.Error())
		return
	}

	m := &models.Payment{
		TokenID: rt.Token.ID,

		Amount:       dto.Amount,
		Currency:     dto.Currency,
		DebtorIban:   dto.DebtorIban,
		CreditorIban: dto.CreditorIban,
		CreditorName: dto.CreditorName,

		Note: dto.Note,
		Type: dto.Type,

		InstantPaymentFlow: dto.InstantPaymentFlow,
		ChargeBearer:       dto.ChargeBearer,

		PaymentID:       pi.PaymentId,
		AuthorizationID: pi.AuthorizationId,
	}

	result := rt.DB.Save(m)
	if result.Error != nil {
		rt.Ef(500, "failed to save payment: %v", result.Error)
		return
	}

	c.JSON(201, &gin.H{
		"paymentId": pi.PaymentId,
		//"authorizationId":   pi.AuthorizationId,
		"url":               pi.Links.ScaRedirect.Href,
		"transactionStatus": pi.TransactionStatus,
	})
	rt.EndBlock()
}

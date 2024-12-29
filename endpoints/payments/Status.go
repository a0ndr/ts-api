package payments

import (
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/assert"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"

	"git.sr.ht/~aondrejcak/ts-api/models"
)

func CheckPaymentAuthorization(rt *kernel.RequestRuntime, pmt *models.Payment) (string, error) {
	art := rt.AppRuntime

	rt.NewChildTracer("payment_status.authorization").Advance()

	tbUrl := fmt.Sprintf("%s/v1/payments/%s/%s/authorizations/%s",
		art.TbUrl, pmt.Type,
		pmt.PaymentID,
		pmt.AuthorizationID)

	client := &http.Client{}

	r, _ := http.NewRequest(http.MethodGet, tbUrl, nil)
	r.Header.Add("Authorization", "Bearer "+rt.Token.AccessToken)

	requestId, _ := kernel.UuidV7()
	r.Header.Add("X-Request-ID", requestId)
	rt.Span.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	rsp, err := client.Do(r)
	if err != nil {
		return "", rt.MakeErrorf("could not execute request: %v", err)
	}

	if rsp.StatusCode != http.StatusOK {
		return "", rt.MakeErrorfFromHttp(rsp, "tatra banka returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", rt.MakeErrorf("could not read body: %v", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", rt.MakeErrorf("could not unmarshal body: %v", err)
	}

	rt.EndBlock()
	return res["scaStatus"].(string), nil
}

func CheckPaymentStatus(rt *kernel.RequestRuntime, pmt *models.Payment) (string, error) {
	art := rt.AppRuntime

	rt.NewChildTracer("payment_status.status").Advance()

	tbUrl := fmt.Sprintf("%s/v3/payments/%s/%s/status",
		art.TbUrl, pmt.Type,
		pmt.PaymentID)

	client := &http.Client{}

	r, _ := http.NewRequest(http.MethodGet, tbUrl, nil)
	r.Header.Add("Authorization", "Bearer "+rt.Token.AccessToken)

	requestId, _ := kernel.UuidV7()
	r.Header.Add("X-Request-ID", requestId)
	rt.Span.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	rsp, err := client.Do(r)
	if err != nil {
		return "", rt.MakeErrorf("could not execute request: %v", err)
	}

	if rsp.StatusCode != http.StatusOK {
		return "", rt.MakeErrorfFromHttp(rsp, "tatra banka returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", rt.MakeErrorf("could not read body: %v", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(body, &res); err != nil {
		return "", rt.MakeErrorf("could not unmarshal body: %v", err)
	}

	rt.EndBlock()
	return res["transactionStatus"].(string), nil
}

func PaymentStatus(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.NewChildTracer("payment_status.handler").Advance()

	assert.NotNil(rt.Token, "token != nil")

	paymentId := c.Param("paymentId")

	var pmt models.Payment
	found, err := rt.First(&pmt, "payment_id = ?", paymentId)
	if !found {
		if err != nil {
			rt.Ef(500, "failed to query database: %v", err.Error())
			return
		}
		rt.Ef(404, "payment with ID '%s' not found", paymentId)
		return
	}

	if pmt.TokenID != rt.Token.ID {
		rt.Ef(404, "payment with ID '%s' not found", paymentId)
		return
	}

	authStatus, err := CheckPaymentAuthorization(rt, &pmt)
	if err != nil {
		rt.Ef(500, "failed to check authorization status: %v", err)
	}

	transStatus, err := CheckPaymentStatus(rt, &pmt)
	if err != nil {
		rt.Ef(500, "failed to check transaction status: %v", err)
	}

	c.JSON(200, gin.H{
		"authorizationStatus": authStatus,
		"transactionStatus":   transStatus,
	})
	rt.EndBlock()
}

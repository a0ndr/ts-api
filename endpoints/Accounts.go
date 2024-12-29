package endpoints

import (
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/assert"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"io"
	"log"
	"net/http"
)

type accounts struct {
	Accounts []struct {
		CashAccountType  string `json:"cashAccountType"`
		BankCode         string `json:"bankCode"`
		Product          string `json:"product"`
		AccountReference struct {
			Iban     string `json:"iban"`
			Currency string `json:"currency"`
		} `json:"accountReference"`
		//Links struct {
		//	Transactions struct {
		//		Href string `json:"href"`
		//	} `json:"transactions"`
		//} `json:"_links"`
		DisplayName           string `json:"displayName"`
		Usage                 string `json:"usage"`
		BankName              string `json:"bankName"`
		ConsentExpirationDate string `json:"consentExpirationDate"`
		DateUpdated           string `json:"dateUpdated"`
		AccountId             string `json:"accountId"`
		Balances              []struct {
			BalanceAmount struct {
				Currency string  `json:"currency"`
				Amount   float32 `json:"amount"`
			} `json:"balanceAmount"`
			BalanceType   string `json:"balanceType"`
			ReferenceDate string `json:"referenceDate"`
		} `json:"balances"`
		Name   string `json:"name"`
		Bic    string `json:"bic"`
		Status string `json:"status"`
	} `json:"accounts"`
}

func listAccounts(rt *kernel.RequestRuntime) (*accounts, error) {
	art := rt.AppRuntime

	rt.NewChildTracer("accounts.list").Advance()

	tbUrl := fmt.Sprintf("%s/v3/accounts?withBalance=true", art.TbUrl)

	client := &http.Client{}
	r, err := http.NewRequest(http.MethodGet, tbUrl, nil)
	if err != nil {
		return nil, rt.MakeErrorf("failed to create request: %v", err)
	}

	requestId, err := kernel.UuidV7()
	if err != nil {
		return nil, rt.MakeErrorf("failed to generate request id: %v", err)
	}
	rt.Span.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	r.Header.Add("X-Request-ID", requestId)
	r.Header.Add("Authorization", "Bearer "+rt.Token.AccessToken)

	rsp, err := client.Do(r)
	if err != nil {
		return nil, rt.MakeErrorf("failed to exec request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v\n", err)
		}
	}(rsp.Body)

	if rsp.StatusCode != http.StatusOK {
		return nil, rt.MakeErrorfFromHttp(rsp, "tatra banka api returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rt.MakeErrorf("failed to read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	accountLists := &accounts{}
	if err = json.Unmarshal(body, accountLists); err != nil {
		return nil, rt.MakeErrorf("failed to unmarshal response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("api.accounts", fmt.Sprintf("%+v", accountLists.Accounts)))
	rt.EndBlock()

	return accountLists, nil
}

func Accounts(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.NewChildTracer("accounts.handler").Advance()

	assert.NotNil(rt.Token, "token != nil")

	accountList, err := listAccounts(rt)
	if err != nil {
		if err.Error() == "tatra banka api returned a non-OK status code: 401" {
			rt.Ef(http.StatusForbidden, "failed to list accounts: invalid token")
			return
		}

		rt.Ef(http.StatusInternalServerError, "could not list accounts: %v", err)
		return
	}

	c.JSON(200, accountList)
	rt.EndBlock()
}

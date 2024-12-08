package endpoints

import (
	"context"
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/models"
	u "git.sr.ht/~aondrejcak/ts-api/utils"
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

func listAccounts(c *u.AppConfig, ctx context.Context, token *models.Token) (*accounts, error) {
	spanCtx, span := c.Tracer.Start(ctx, "accounts.list")
	defer span.End()

	tbUrl := fmt.Sprintf("%s/v3/accounts?withBalance=true", c.TbUrl)

	client := &http.Client{}

	_, tbSpan := c.Tracer.Start(spanCtx, "accounts.list.query")

	r, err := http.NewRequest(http.MethodGet, tbUrl, nil)
	if err != nil {
		return nil, u.SpanErrf(tbSpan, "failed to create request: %v", err)
	}

	requestId, err := u.UuidV4()
	if err != nil {
		return nil, u.SpanErrf(tbSpan, "failed to generate request id: %v", err)
	}
	tbSpan.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	r.Header.Add("X-Request-ID", requestId)
	r.Header.Add("Authorization", "Bearer "+token.AccessToken)

	rsp, err := client.Do(r)
	if err != nil {
		return nil, u.SpanErrf(tbSpan, "failed to exec request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close response body: %v\n", err)
		}
	}(rsp.Body)

	if rsp.StatusCode != http.StatusOK {
		return nil, u.SpanHttpErrf(tbSpan, rsp, "tatra banka api returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, u.SpanErrf(tbSpan, "failed to read response body: %v", err)
	}

	tbSpan.SetAttributes(attribute.KeyValue("tb.response", string(body)))
	tbSpan.End()

	_, parseSpan := c.Tracer.Start(spanCtx, "accounts.list.parse")
	defer parseSpan.End()

	accountLists := &accounts{}
	if err = json.Unmarshal(body, accountLists); err != nil {
		return nil, u.SpanErrf(tbSpan, "failed to unmarshal response body: %v", err)
	}

	parseSpan.SetAttributes(attribute.KeyValue("api.accounts", fmt.Sprintf("%+v", accountLists.Accounts)))
	return accountLists, nil
}

func Accounts(c *gin.Context) {
	config := u.LoadConfig()
	ctx, span := config.Tracer.Start(c.Request.Context(), "accounts.handler")
	defer span.End()

	tok, ok := c.Get("token")
	if !ok {
		span.RecordError(fmt.Errorf("failed to get token"))
		c.JSON(http.StatusUnauthorized, &gin.Error{
			Err: fmt.Errorf("unauthorized: could not get context token"),
		})
		return
	}

	token := tok.(models.Token)

	accountList, err := listAccounts(config, ctx, &token)
	if err != nil {
		span.RecordError(err)
		c.JSON(http.StatusInternalServerError, &gin.Error{
			Err: fmt.Errorf("could not list accounts: %w", err),
		})
		return
	}

	c.JSON(http.StatusOK, accountList)
}

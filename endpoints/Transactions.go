package endpoints

import (
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/assert"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"io"
	"net/http"
	"net/url"
)

type TransactionsModel struct {
	Iban      string `form:"iban"`
	AccountId string `form:"accountId"`

	// TB
	DateFrom             string `form:"dateFrom"`       //Je povinné v prípade, že nie je použité stránkovanie. ??? YYYY-MM-DD
	DateTo               string `form:"dateTo"`         // YYYY-MM-DD
	VariableSymbol       string `form:"variableSymbol"` // TB only
	ConstantSymbol       string `form:"constantSymbol"` // TB only
	SpecificSymbol       string `form:"specificSymbol"` // TB only
	E2eReference         string `form:"e2ereference"`   // TB only
	AmountFrom           string `form:"amountFrom"`
	AmountTo             string `form:"amountTo"`
	BookingStatus        string `form:"bookingStatus"`        // booked, pending
	TransactionDirection string `form:"transactionDirection"` // CRDT - credit, DBT - debit
	//EntryReferenceFrom string `form:"entryReferenceFrom"`
	Page     string `form:"page"`
	PageSize string `form:"pageSize"`
}

type transactions struct {
	Account struct {
		Iban     string `json:"iban"`
		Currency string `json:"currency"`
	} `json:"account"`
	Transactions []struct {
		TransactionId     string `json:"transactionId"`
		TransactionState  string `json:"transactionState"`
		EndToEndId        string `json:"endToEndId"`
		VariableSymbol    string `json:"variableSymbol"`
		BookingDate       string `json:"bookingDate"`
		ValueDate         string `json:"valueDate"`
		TransactionAmount struct {
			Amount   float32 `json:"amount"`
			Currency string  `json:"currency"`
		} `json:"transactionAmount"`
		CreditorName    string `json:"creditorName"`
		CreditorAccount struct {
			Iban string `json:"iban"`
		} `json:"creditorAccount"`
		CreditorAgent struct {
			Bic                 string `json:"bic"`
			Name                string `json:"name"`
			OtherIdentification string `json:"otherIdentification"`
		} `json:"creditorAgent"`
		DebtorName    string `json:"debtorName"`
		DebtorAccount struct {
			Iban     string `json:"iban"`
			Currency string `json:"currency"`
		} `json:"debtorAccount"`
		DebtorAgent struct {
			Bic                 string `json:"bic"`
			Name                string `json:"name"`
			OtherIdentification string `json:"otherIdentification"`
		} `json:"debtorAgent"`
		RemittanceInformationUnstructured string `json:"remittanceInformationUnstructured"`
		AdditionalInformation             string `json:"additionalInformation"`
		BankTransactionCode               string `json:"bankTransactionCode"`
		IsReversal                        bool   `json:"isReversal"`
	} `json:"transactions"`
	//Links struct {
	//	Account struct {
	//		Href string `json:"href"`
	//	} `json:"account"`
	//	Next struct {
	//		Href string `json:"href"`
	//	} `json:"next"`
	//} `json:"_links"`
}

func listTransactions(rt *kernel.RequestRuntime, t *TransactionsModel) (*transactions, error) {
	art := rt.AppRuntime

	rt.StepInto("transactions.list")

	parsedUrl, _ := url.Parse(fmt.Sprintf("%s/v5/accounts/%s/transactions", art.TbUrl, t.AccountId))
	q := parsedUrl.Query()
	if t.DateFrom != "" {
		q.Set("dateFrom", t.DateFrom)
	}
	if t.DateTo != "" {
		q.Set("dateTo", t.DateTo)
	}
	if t.VariableSymbol != "" {
		q.Set("variableSymbol", t.VariableSymbol)
	}
	if t.ConstantSymbol != "" {
		q.Set("constantSymbol", t.ConstantSymbol)
	}
	if t.SpecificSymbol != "" {
		q.Set("specificSymbol", t.SpecificSymbol)
	}
	if t.E2eReference != "" {
		q.Set("e2ereference", t.E2eReference)
	}
	if t.AmountFrom != "" {
		q.Set("amountFrom", t.AmountFrom)
	}
	if t.AmountTo != "" {
		q.Set("amountTo", t.AmountTo)
	}
	if t.BookingStatus != "" {
		q.Set("bookingStatus", t.BookingStatus)
	}
	if t.TransactionDirection != "" {
		q.Set("transactionDirection", t.TransactionDirection)
	}
	if t.Page != "" {
		q.Set("page", t.Page)
	}
	if t.PageSize != "" {
		q.Set("pageSize", t.PageSize)
	}

	parsedUrl.RawQuery = q.Encode()
	tbUrl := parsedUrl.String()

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
	if rsp.StatusCode != http.StatusOK {
		if rsp.StatusCode == http.StatusNotFound || rsp.StatusCode == http.StatusForbidden {
			return nil, rt.MakeErrorf("account not found")
		}
		return nil, rt.MakeErrorfFromHttp(rsp, "tatra banka api returned a non-OK status code: %d", rsp.StatusCode)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rt.MakeErrorf("failed to read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	transactionLists := &transactions{}
	if err = json.Unmarshal(body, transactionLists); err != nil {
		return nil, rt.MakeErrorf("failed to unmarshal response body: %v", err)
	}
	rt.Span.SetAttributes(attribute.KeyValue("api.accounts", fmt.Sprintf("%+v", transactionLists.Transactions)))
	rt.StepBack()

	return transactionLists, nil
}

func Transactions(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.StepInto("transactions.handler")

	assert.NotNil(rt.Token, "token != nil")

	var model TransactionsModel
	if err := c.BindQuery(&model); err != nil {
		rt.Ef(500, "failed to bind query body: %v", err)
		return
	}
	if model.Iban == "" && model.AccountId == "" {
		rt.Ef(400, "account id or iban is required")
		return
	} else if model.Iban != "" && model.AccountId != "" {
		rt.Ef(400, "only account id or iban can be provided")
		return
	}

	if model.AccountId == "" {
		accountList, err := listAccounts(rt)
		if err != nil {
			rt.Ef(500, "failed to list accounts: %v", err)
			return
		}

		for _, account := range accountList.Accounts {
			if account.AccountReference.Iban == model.Iban {
				model.AccountId = account.AccountId
				break
			}
		}
	}

	transactionList, err := listTransactions(rt, &model)
	if err != nil {
		if err.Error() == "account not found" {
			rt.Ef(404, err.Error())
			return
		}

		rt.Ef(500, "failed to list transactions: %v", err)
		return
	}

	c.JSON(200, transactionList)
	rt.StepBack()
}

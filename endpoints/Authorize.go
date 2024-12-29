package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	val "github.com/go-ozzo/ozzo-validation"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"

	"git.sr.ht/~aondrejcak/ts-api/models"
)

type AuthorizeDto struct {
	CompanyId string
}

func (dto AuthorizeDto) Validate() error {
	return val.ValidateStruct(&dto,
		val.Field(&dto.CompanyId, val.Required),
	)
}

func getClientCredentialsGrantToken(rt *kernel.RequestRuntime) (string, error) {
	art := rt.AppRuntime
	rt.NewChildTracer("authorize.grant_token").Advance()

	data := url.Values{}
	data.Set("client_id", art.ClientID)
	data.Set("client_secret", art.ClientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "PREMIUM_AIS")

	tbUrl := fmt.Sprintf("%s/auth/oauth/v2/token", art.TbUrl)
	client := &http.Client{}
	r, err := http.NewRequest(http.MethodPost, tbUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", rt.MakeErrorf("failed to make request: %v", err)
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(r)
	if err != nil {
		return "", rt.MakeErrorf("failed to make request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", rt.MakeErrorfFromHttp(resp, "tatra banka returned a non-Ok status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", rt.MakeErrorf("failed to read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", rt.MakeErrorf("failed to unmarshal response body: %v", err)
	}

	rt.EndBlock()
	return res["access_token"].(string), nil
}

func getConsentId(rt *kernel.RequestRuntime, grantToken string) (string, error) {
	art := rt.AppRuntime

	rt.NewChildTracer("authorize.consent_id").Advance()

	tbUrl := fmt.Sprintf("%s/v3/consents", art.TbUrl)

	j, err := json.Marshal(&gin.H{
		"combinedServiceIndicator": true,
	})
	if err != nil {
		return "", rt.MakeErrorf("failed to marshal request: %v", err)
	}

	r, err := http.NewRequest(http.MethodPost, tbUrl, bytes.NewBuffer(j))
	if err != nil {
		return "", rt.MakeErrorf("failed to create request: %v", err)
	}

	requestId, err := kernel.UuidV7()
	if err != nil {
		return "", rt.MakeErrorf("failed to generate request id: %v", err)
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", grantToken))
	r.Header.Add("X-Request-ID", requestId)
	rt.Span.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return "", rt.MakeErrorf("failed to make request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Printf("failed to close body: %v", err)
		}
	}(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", rt.MakeErrorfFromHttp(resp, "tatra banka returned a non-Ok status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", rt.MakeErrorf("failed to read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", rt.MakeErrorf("failed to unmarshal response body: %v", err)
	}

	rt.EndBlock()
	return res["consentId"].(string), nil
}

func createAuthUrl(rt *kernel.RequestRuntime, consentId string, state string) (string, error) {
	art := rt.AppRuntime

	rt.NewChildTracer("authorize.create_auth_url").Advance()

	parsedUrl, err := url.Parse(fmt.Sprintf("%s/auth/oauth/v2/authorize", art.TbUrl))
	if err != nil {
		return "", rt.MakeError(err)
	}

	q := parsedUrl.Query()
	q.Set("client_id", art.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", "PREMIUM_AIS:"+consentId)
	q.Set("redirect_uri", art.RedirectUri)
	q.Set("state", state)
	q.Set("code_challenge", art.CodeChallenge)
	q.Set("code_challenge_method", "S256")

	parsedUrl.RawQuery = q.Encode()
	finalUrl := parsedUrl.String()

	rt.Span.SetAttributes(attribute.KeyValue("tb.auth_url", finalUrl))
	rt.EndBlock()

	return finalUrl, nil
}

func Authorize(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.NewChildTracer("authorize.handler").Advance()

	var dto AuthorizeDto
	rt.BindJSON(&dto)
	if rt.Error != nil {
		rt.Ef(500, "could not bind body: %v", rt.Error)
		return
	}

	if err := dto.Validate(); err != nil {
		rt.E(http.StatusBadRequest, err)
		return
	}

	var company models.Company
	found, err := rt.First(&company, "id = ?", dto.CompanyId)
	if !found {
		if err != nil {
			rt.Ef(500, "could not find company: %v", err)
			return
		}
		rt.Ef(404, "company with ID '%v' does not exist", dto.CompanyId)
		return
	}

	// 1. client credentials grant
	grantToken, err := getClientCredentialsGrantToken(rt)
	if err != nil {
		rt.Ef(500, "failed to get client credentials grant token: %v", err)
		return
	}

	// 2. create consentId
	consentId, err := getConsentId(rt, grantToken)
	if err != nil {
		rt.Ef(500, "failed to get consent id: %v", err)
		return
	}

	// 3. (1.) redirect to portal
	state := kernel.RandStringBytesMaskImprSrcUnsafe(16)
	authUrl, err := createAuthUrl(rt, consentId, state)
	if err != nil {
		rt.Ef(500, "failed to create auth url: %v", err)
		return
	}

	token, err := kernel.UuidV7()
	if err != nil {
		rt.Ef(500, "failed to generate token: %v", err)
		return
	}

	hash := kernel.Sha512(token)

	t := time.Now()
	t = t.Add(time.Minute * 15)

	entity := models.Token{
		TokenHash:  hash,
		CompanyID:  dto.CompanyId,
		GrantToken: grantToken,
		ConsentId:  consentId,
		State:      state,
		ExpiresAt:  t,
	}
	// ^ TODO: add IP & User Agent verification
	//                  ~~~~~~~~~~ is needed?

	result := rt.DB.Create(&entity)
	if result.Error != nil {
		rt.Ef(500, "failed to save to database: %v", result.Error.Error())
		return
	}

	c.JSON(201, &gin.H{
		"token":     token,
		"url":       authUrl,
		"expiresAt": t.Format(time.RFC3339),
	})
	rt.EndBlock()
}

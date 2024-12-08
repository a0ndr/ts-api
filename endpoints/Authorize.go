package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.nhat.io/otelsql/attribute"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"git.sr.ht/~aondrejcak/ts-api/models"
	u "git.sr.ht/~aondrejcak/ts-api/utils"
)

type AuthorizeModel struct {
	CompanyId string `json:"companyId" binding:"required"`
}

func getClientCredentialsGrantToken(c *u.AppConfig, ctx context.Context) (string, error) {
	spanCtx, span := c.Tracer.Start(ctx, "authorize.grant_token")
	defer span.End()

	_, tbSpan := c.Tracer.Start(spanCtx, "authorize.grant_token.get")

	data := url.Values{}
	data.Set("client_id", c.ClientID)
	data.Set("client_secret", c.ClientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", "PREMIUM_AIS")

	tbUrl := fmt.Sprintf("%s/auth/oauth/v2/token", c.TbUrl)
	client := &http.Client{}
	r, err := http.NewRequest(http.MethodPost, tbUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to create request: %v", err)
	}
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(r)
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", u.SpanHttpErrf(tbSpan, resp, "tatra banka returned a non-Ok status code: %d", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to read response body: %v", err)
	}

	tbSpan.SetAttributes(attribute.KeyValue("tb.response", string(body)))
	tbSpan.End()

	_, parseSpan := c.Tracer.Start(spanCtx, "authorize.grant_token.parse")
	defer parseSpan.End()

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", u.SpanErrf(parseSpan, "failed to unmarshal response body: %v", err)
	}

	return res["access_token"].(string), nil
}

func getConsentId(c *u.AppConfig, ctx context.Context, grantToken string) (string, error) {
	spanCtx, span := c.Tracer.Start(ctx, "authorize.consent_id")
	defer span.End()

	tbUrl := fmt.Sprintf("%s/v3/consents", c.TbUrl)

	_, tbSpan := c.Tracer.Start(spanCtx, "authorize.consent_id.get")

	j, err := json.Marshal(&gin.H{
		"combinedServiceIndicator": true,
	})
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to marshal request: %v", err)
	}

	r, err := http.NewRequest(http.MethodPost, tbUrl, bytes.NewBuffer(j))
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to create request: %v", err)
	}

	requestId, err := u.UuidV4()
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to generate request id: %v", err)
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", grantToken))
	r.Header.Add("X-Request-ID", requestId)
	tbSpan.SetAttributes(attribute.KeyValue("tb.request_id", requestId))

	client := &http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", u.SpanHttpErrf(tbSpan, resp, "tatra banka returned a non-Ok status code: %d", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", u.SpanErrf(tbSpan, "failed to read response body: %v", err)
	}

	tbSpan.SetAttributes(attribute.KeyValue("tb.response", string(body)))
	tbSpan.End()

	_, parseSpan := c.Tracer.Start(spanCtx, "authorize.consent_id.parse")
	defer parseSpan.End()

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", u.SpanErrf(parseSpan, "failed to unmarshal response body: %v", err)
	}

	return res["consentId"].(string), nil
}

func createAuthUrl(c *u.AppConfig, ctx context.Context, consentId string, state string) (string, error) {
	_, span := c.Tracer.Start(ctx, "authorize.create_auth_url")
	defer span.End()

	parsedUrl, err := url.Parse(fmt.Sprintf("%s/auth/oauth/v2/authorize", c.TbUrl))
	if err != nil {
		return "", u.SpanErr(span, err)
	}

	q := parsedUrl.Query()
	q.Set("client_id", c.ClientID)
	q.Set("response_type", "code")
	q.Set("scope", "PREMIUM_AIS:"+consentId)
	q.Set("redirect_uri", c.RedirectUri)
	q.Set("state", state)
	q.Set("code_challenge", c.CodeChallenge)
	q.Set("code_challenge_method", "S256")

	parsedUrl.RawQuery = q.Encode()
	finalUrl := parsedUrl.String()

	span.SetAttributes(attribute.KeyValue("tb.auth_url", finalUrl))
	return finalUrl, nil
}

func Authorize(c *gin.Context) {
	config := u.LoadConfig()

	ctx, span := config.Tracer.Start(c.Request.Context(), "authorize.handler")
	defer span.End()

	var req AuthorizeModel
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Println(c.ShouldBindJSON(&req) != nil)
		u.SpanGinErrf(span, c, 500, "invalid request body")
		return
	}

	_, companySpan := config.Tracer.Start(ctx, "authorize.company")
	var company models.Company
	result := config.DatabaseClient.First(&company, req.CompanyId)
	if err := result.Error; err != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			u.SpanGinErrf(companySpan, c, 500, "company with ID '%s' not found", req.CompanyId)
			return
		}
	}
	companySpan.End()

	// 1. client credentials grant
	grantToken, err := getClientCredentialsGrantToken(config, ctx)
	if err != nil {
		u.SpanGinErrf(span, c, 500, "failed to get client credentials grant token: %v", err)
		return
	}

	// 2. create consentId
	consentId, err := getConsentId(config, ctx, grantToken)
	if err != nil {
		u.SpanGinErrf(span, c, 500, "failed to get consent id: %v", err)
		return
	}

	// 3. (1.) redirect to portal
	state := u.RandStringBytesMaskImprSrcUnsafe(16)
	authUrl, err := createAuthUrl(config, ctx, consentId, state)
	if err != nil {
		u.SpanGinErrf(span, c, 500, "failed to create auth url: %v", err)
		return
	}

	_, saveSpan := config.Tracer.Start(ctx, "authorize.create_token")
	defer saveSpan.End()

	token, err := u.UuidV4()
	if err != nil {
		u.SpanGinErrf(saveSpan, c, 500, "failed to generate token: %v", err)
		return
	}

	hash := u.Sha512(token)

	t := time.Now()
	t = t.Add(time.Minute * 15)

	entity := models.Token{
		TokenHash:  hash,
		CompanyId:  req.CompanyId,
		GrantToken: grantToken,
		ConsentId:  consentId,
		State:      state,
		ExpiresAt:  t,
	}
	// ^ TODO: add IP & User Agent verification
	//                  ~~~~~~~~~~ is needed?

	result = config.DatabaseClient.Create(&entity)
	if result.Error != nil {
		u.SpanGinErrf(saveSpan, c, 500, "failed to save to database: %v", result.Error)
		return
	}

	c.JSON(http.StatusOK, &gin.H{
		"token":     token,
		"url":       authUrl,
		"expiresAt": t.Format(time.RFC3339),
	})
}

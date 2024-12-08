package endpoints

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/models"
	u "git.sr.ht/~aondrejcak/ts-api/utils"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type CallbackModel struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

func getAccessToken(c *u.AppConfig, ctx context.Context, code string) (string, string, error) {
	spanCtx, span := c.Tracer.Start(ctx, "callback.get")
	defer span.End()

	tbUrl := fmt.Sprintf("%s/auth/oauth/v2/token", c.TbUrl)

	client := &http.Client{}

	_, tbSpan := c.Tracer.Start(spanCtx, "callback.get.request")

	bodyParams := url.Values{}
	bodyParams.Set("code", code)
	bodyParams.Set("grant_type", "authorization_code")
	bodyParams.Set("redirect_uri", c.RedirectUri)
	bodyParams.Set("scope", "PREMIUM_AIS")
	bodyParams.Set("code_verifier", c.CodeChallengeVerifier)

	r, err := http.NewRequest(http.MethodPost, tbUrl, strings.NewReader(bodyParams.Encode()))
	if err != nil {
		return "", "", u.SpanErrf(tbSpan, "failed to create request: %v", err)
	}

	authHeader := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.ClientID, c.ClientSecret)))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Authorization", "Basic "+authHeader)

	rsp, err := client.Do(r)
	if err != nil {
		return "", "", u.SpanErrf(tbSpan, "failed to exec request: %v", err)
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return "", "", u.SpanHttpErrf(tbSpan, rsp, "tatra banka api returned a non-OK status code: %s", rsp.Status)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", "", u.SpanErrf(tbSpan, "failed to read response body: %v", err)
	}

	tbSpan.SetAttributes(attribute.KeyValue("tb.response", string(body)))
	tbSpan.End()

	_, parseSpan := c.Tracer.Start(spanCtx, "callback.get.parse")
	defer parseSpan.End()

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", "", u.SpanErrf(parseSpan, "failed to unmarshal response body: %v", err)
	}

	return res["access_token"].(string), res["refresh_token"].(string), nil
	// ^ ACCESS TOKEN - 180 DAYS; REFRESH TOKEN - 360 DAYS
}

func Callback_(c *gin.Context) {
	config := u.LoadConfig()
	spanCtx, span := config.Tracer.Start(c.Request.Context(), "callback.handler")
	defer span.End()

	var req CallbackModel
	if err := c.ShouldBindJSON(&req); err != nil {
		u.SpanGinErrf(span, c, 400, "invalid request body")
		return
	}

	if req.Code == "" || req.State == "" {
		u.SpanGinErrf(span, c, 400, "code and state are required")
		return
	}

	_, querySpan := config.Tracer.Start(spanCtx, "callback.handler.query")
	var token models.Token
	result := config.DatabaseClient.First(&token, "state = ?", req.State)
	if err := result.Error; err != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			u.SpanGinErrf(querySpan, c, 404, "token with state '%s' not found", req.State)
			return
		}

		// todo: other error cases
	}
	querySpan.End()

	accessToken, refreshToken, err := getAccessToken(config, spanCtx, req.Code)
	if err != nil {
		u.SpanGinErrf(span, c, 500, "failed to get access token: %v", err)
		return
	}

	_, saveSpan := config.Tracer.Start(spanCtx, "callback.handler.save")
	defer saveSpan.End()

	token.AccessToken = accessToken
	token.RefreshToken = refreshToken

	result = config.DatabaseClient.Save(&token)
	if err = result.Error; err != nil {
		u.SpanGinErrf(saveSpan, c, 500, "failed to save token: %v", err)
		return
	}

	c.Status(http.StatusOK)
}

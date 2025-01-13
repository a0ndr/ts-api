package endpoints

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"git.sr.ht/~aondrejcak/ts-api/kernel"
	"git.sr.ht/~aondrejcak/ts-api/models"
	"github.com/gin-gonic/gin"
	"go.nhat.io/otelsql/attribute"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type CallbackModel struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

func getAccessToken(rt *kernel.RequestRuntime, code string) (string, string, error) {
	art := rt.AppRuntime

	rt.StepInto("callback.exchange")

	tbUrl := fmt.Sprintf("%s/auth/oauth/v2/token", art.TbUrl)

	client := &http.Client{}

	bodyParams := url.Values{}
	bodyParams.Set("code", code)
	bodyParams.Set("grant_type", "authorization_code")
	bodyParams.Set("redirect_uri", art.RedirectUri)
	bodyParams.Set("scope", "PREMIUM_AIS")
	bodyParams.Set("code_verifier", art.CodeChallengeVerifier)

	r, err := http.NewRequest(http.MethodPost, tbUrl, strings.NewReader(bodyParams.Encode()))
	if err != nil {
		return "", "", rt.MakeErrorf("failed to create request: %v", err)
	}

	authHeader := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", art.ClientID, art.ClientSecret)))
	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Add("Authorization", "Basic "+authHeader)

	rsp, err := client.Do(r)
	if err != nil {
		return "", "", rt.MakeErrorf("failed to exec request: %v", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(rsp.Body)

	if rsp.StatusCode != http.StatusOK {
		return "", "", rt.MakeErrorfFromHttp(rsp, "tatra banka api returned a non-OK status code: %s", rsp.Status)
	}

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return "", "", rt.MakeErrorf("failed to read response body: %v", err)
	}

	rt.Span.SetAttributes(attribute.KeyValue("tb.response", string(body)))

	var res map[string]interface{}
	if err = json.Unmarshal(body, &res); err != nil {
		return "", "", rt.MakeErrorf("failed to unmarshal response body: %v", err)
	}

	rt.StepBack()
	return res["access_token"].(string), res["refresh_token"].(string), nil
	// ^ ACCESS TOKEN - 180 DAYS; REFRESH TOKEN - 360 DAYS
}

func Callback_(c *gin.Context) {
	rt := c.MustGet("rt").(*kernel.RequestRuntime)
	rt.StepInto("callback.handler")

	var req CallbackModel
	if err := c.ShouldBindJSON(&req); err != nil {
		rt.Ef(http.StatusUnprocessableEntity, "invalid request body")
		return
	}

	if req.Code == "" || req.State == "" {
		rt.Ef(400, "code and state are required")
		return
	}

	var tkn models.Token
	found, err := rt.First(&tkn, "state = ?", req.State)
	if !found {
		if err != nil {
			rt.Ef(500, "failed to query database: %v", err)
			return
		}
		rt.Ef(404, "token not found")
		return
	}

	accessToken, refreshToken, err := getAccessToken(rt, req.Code)
	if err != nil {
		rt.Ef(500, "failed to get access token: %v", err)
		return
	}

	tkn.AccessToken = accessToken
	tkn.RefreshToken = refreshToken

	result := rt.DB.Save(&tkn)
	if err = result.Error; err != nil {
		rt.Ef(500, "failed to save token: %v", err)
		return
	}

	c.Status(http.StatusNoContent)
	rt.StepBack()
}

// Package zhipu implements the Zhipu AI provider adapter.
package zhipu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/blueberrycongee/llmux/pkg/provider"
	"github.com/blueberrycongee/llmux/pkg/types"
	"github.com/blueberrycongee/llmux/providers/openailike"
)

const (
	ProviderName   = "zhipu"
	DefaultBaseURL = "https://open.bigmodel.cn/api/paas/v4"
	TokenTTL       = 30 * time.Minute
)

type Provider struct {
	*openailike.Provider
	apiKey     string
	tokenCache struct {
		sync.Mutex
		token string
		exp   time.Time
	}
}

var providerInfo = openailike.Info{
	Name:              ProviderName,
	DefaultBaseURL:    DefaultBaseURL,
	SupportsStreaming: true,
	ModelPrefixes:     []string{"glm-", "chatglm", "cogview"},
}

var DefaultModels = []string{"glm-4", "glm-4v", "glm-3-turbo"}

func New(opts ...openailike.Option) *Provider {
	base := openailike.New(providerInfo, opts...)
	return &Provider{
		Provider: base,
		apiKey:   base.GetAPIKey(),
	}
}

func NewFromConfig(cfg provider.Config) (provider.Provider, error) {
	base, err := openailike.NewFromConfig(providerInfo, cfg)
	if err != nil {
		return nil, err
	}
	p, ok := base.(*openailike.Provider)
	if !ok {
		return nil, fmt.Errorf("unexpected provider type: %T", base)
	}
	return &Provider{
		Provider: p,
		apiKey:   p.GetAPIKey(),
	}, nil
}

func (p *Provider) BuildRequest(ctx context.Context, req *types.ChatRequest) (*http.Request, error) {
	// Zhipu V4 is mostly OpenAI compatible, but needs JWT token

	// 1. Generate/Get JWT
	token, err := p.getJWT()
	if err != nil {
		return nil, fmt.Errorf("generate jwt: %w", err)
	}

	// 2. Use openailike to build base request
	// We temporarily set API Key to the JWT so openailike uses it?
	// openailike uses "Authorization: Bearer <apiKey>".
	// So if we swap apiKey with JWT, it works.
	// But p.Provider is shared? No, it's a struct embedding *openailike.Provider.
	// But we shouldn't mutate the base provider's state if it's shared (it shouldn't be).
	// However, openailike.BuildRequest uses p.apiKey.

	// Better: Build request manually or use openailike and override header.

	httpReq, err := p.Provider.BuildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Override Authorization header
	httpReq.Header.Set("Authorization", "Bearer "+token)

	return httpReq, nil
}

func (p *Provider) getJWT() (string, error) {
	p.tokenCache.Lock()
	defer p.tokenCache.Unlock()

	if p.tokenCache.token != "" && time.Now().Before(p.tokenCache.exp) {
		return p.tokenCache.token, nil
	}

	parts := strings.Split(p.apiKey, ".")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid api key format")
	}
	id, secret := parts[0], parts[1]

	now := time.Now()
	exp := now.Add(TokenTTL)

	payload := jwt.MapClaims{
		"api_key":   id,
		"exp":       exp.UnixMilli(),
		"timestamp": now.UnixMilli(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	token.Header["sign_type"] = "SIGN"

	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	p.tokenCache.token = signedToken
	p.tokenCache.exp = exp.Add(-1 * time.Minute) // Buffer
	return signedToken, nil
}

// ParseResponse and ParseStreamChunk are handled by openailike as Zhipu V4 is compatible.
// Except if Zhipu has specific quirks.
// Zhipu V4 is OpenAI compatible.

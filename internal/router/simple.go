package router

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/blueberrycongee/llmux/internal/provider"
	llmerrors "github.com/blueberrycongee/llmux/pkg/errors"
)

// ErrNoAvailableDeployment is returned when no healthy deployment is available.
var ErrNoAvailableDeployment = errors.New("no available deployment for model")

// SimpleRouter implements a basic routing strategy with cooldown support.
// It uses random selection among healthy deployments.
type SimpleRouter struct {
	mu             sync.RWMutex
	deployments    map[string][]*provider.Deployment // model -> deployments
	stats          map[string]*DeploymentStats       // deploymentID -> stats
	cooldownPeriod time.Duration
	rng            *rand.Rand
}

// NewSimpleRouter creates a new simple router with the given cooldown period.
func NewSimpleRouter(cooldownPeriod time.Duration) *SimpleRouter {
	return &SimpleRouter{
		deployments:    make(map[string][]*provider.Deployment),
		stats:          make(map[string]*DeploymentStats),
		cooldownPeriod: cooldownPeriod,
		rng:            rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Pick selects a random healthy deployment for the given model.
func (r *SimpleRouter) Pick(ctx context.Context, model string) (*provider.Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	deployments, ok := r.deployments[model]
	if !ok || len(deployments) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Filter healthy deployments
	now := time.Now()
	healthy := make([]*provider.Deployment, 0, len(deployments))
	for _, d := range deployments {
		stats := r.stats[d.ID]
		if stats == nil || now.After(stats.CooldownUntil) {
			healthy = append(healthy, d)
		}
	}

	if len(healthy) == 0 {
		return nil, ErrNoAvailableDeployment
	}

	// Random selection
	return healthy[r.rng.Intn(len(healthy))], nil
}

// ReportSuccess records a successful request.
func (r *SimpleRouter) ReportSuccess(deployment *provider.Deployment, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.SuccessCount++
	stats.LastRequestTime = time.Now()

	// Update rolling average latency
	latencyMs := float64(latency.Milliseconds())
	if stats.AvgLatencyMs == 0 {
		stats.AvgLatencyMs = latencyMs
	} else {
		// Exponential moving average with alpha = 0.1
		stats.AvgLatencyMs = stats.AvgLatencyMs*0.9 + latencyMs*0.1
	}
}

// ReportFailure records a failed request and triggers cooldown if needed.
func (r *SimpleRouter) ReportFailure(deployment *provider.Deployment, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.getOrCreateStats(deployment.ID)
	stats.TotalRequests++
	stats.FailureCount++
	stats.LastRequestTime = time.Now()

	// Check if cooldown is required based on error type
	var llmErr *llmerrors.LLMError
	if errors.As(err, &llmErr) && llmerrors.IsCooldownRequired(llmErr.StatusCode) {
		stats.CooldownUntil = time.Now().Add(r.cooldownPeriod)
	}
}

// IsCircuitOpen checks if the deployment is in cooldown.
func (r *SimpleRouter) IsCircuitOpen(deployment *provider.Deployment) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats, ok := r.stats[deployment.ID]
	if !ok {
		return false
	}
	return time.Now().Before(stats.CooldownUntil)
}

// AddDeployment registers a new deployment.
func (r *SimpleRouter) AddDeployment(deployment *provider.Deployment) {
	r.mu.Lock()
	defer r.mu.Unlock()

	model := deployment.ModelName
	if deployment.ModelAlias != "" {
		model = deployment.ModelAlias
	}

	r.deployments[model] = append(r.deployments[model], deployment)
	r.stats[deployment.ID] = &DeploymentStats{}
}

// RemoveDeployment removes a deployment from the router.
func (r *SimpleRouter) RemoveDeployment(deploymentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for model, deps := range r.deployments {
		for i, d := range deps {
			if d.ID == deploymentID {
				r.deployments[model] = append(deps[:i], deps[i+1:]...)
				break
			}
		}
	}
	delete(r.stats, deploymentID)
}

// GetDeployments returns all deployments for a model.
func (r *SimpleRouter) GetDeployments(model string) []*provider.Deployment {
	r.mu.RLock()
	defer r.mu.RUnlock()

	deps := r.deployments[model]
	result := make([]*provider.Deployment, len(deps))
	copy(result, deps)
	return result
}

func (r *SimpleRouter) getOrCreateStats(deploymentID string) *DeploymentStats {
	stats, ok := r.stats[deploymentID]
	if !ok {
		stats = &DeploymentStats{}
		r.stats[deploymentID] = stats
	}
	return stats
}

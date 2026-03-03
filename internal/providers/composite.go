package providers

import (
	"fmt"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/tasks"
)

// CompositeProvider merges tasks from multiple providers into a single list.
// Errors from individual providers are collected and returned as a combined
// error rather than halting the entire fetch, so a single broken provider
// does not prevent the others from loading.
type CompositeProvider struct {
	providers []tasks.Provider
}

// NewComposite wraps multiple providers. If only one provider is given,
// it is returned directly to avoid unnecessary wrapping.
func NewComposite(ps []tasks.Provider) tasks.Provider {
	if len(ps) == 1 {
		return ps[0]
	}
	return &CompositeProvider{providers: ps}
}

// Name returns a slash-joined list of the underlying provider names.
func (c *CompositeProvider) Name() string {
	names := make([]string, len(c.providers))
	for i, p := range c.providers {
		names[i] = p.Name()
	}
	return strings.Join(names, "/")
}

// GetTasks fetches tasks from all providers concurrently and merges the results.
// If any provider fails, its error is collected; the remaining tasks are still
// returned so the dashboard stays useful even when one source is unavailable.
func (c *CompositeProvider) GetTasks() ([]tasks.Task, error) {
	type result struct {
		tasks []tasks.Task
		err   error
	}

	ch := make(chan result, len(c.providers))
	for _, p := range c.providers {
		p := p
		go func() {
			t, err := p.GetTasks()
			ch <- result{t, err}
		}()
	}

	var all []tasks.Task
	var errs []string
	for range c.providers {
		r := <-ch
		if r.err != nil {
			errs = append(errs, r.err.Error())
		} else {
			all = append(all, r.tasks...)
		}
	}

	if len(errs) > 0 {
		return all, fmt.Errorf("provider error(s): %s", strings.Join(errs, "; "))
	}
	return all, nil
}

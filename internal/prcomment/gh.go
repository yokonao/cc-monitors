package prcomment

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/yokonao/cc-monitors/internal/gh"
)

func fetchSelf(ctx context.Context) (string, error) {
	out, err := gh.Output(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ghAPIList pages through a REST list endpoint and flattens the pages.
func ghAPIList[T any](ctx context.Context, host, path string) ([]T, error) {
	out, err := gh.Output(ctx, "api", "--hostname", host, "--paginate", "--slurp", path)
	if err != nil {
		return nil, err
	}
	var pages [][]T
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, fmt.Errorf("unparseable gh output: %w", err)
	}
	return slices.Concat(pages...), nil
}

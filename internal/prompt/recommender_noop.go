package prompt

import "context"

// NoopRecommender is the default recommender; it never triggers network/model calls.
type NoopRecommender struct{}

func (n *NoopRecommender) Recommend(ctx context.Context, in RecommendInput) ([]Recommendation, error) {
	return nil, nil
}

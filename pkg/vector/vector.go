package vector

import (
	"math"
	"sort"
)

type Vector []float32

type Document struct {
	ID        string
	Content   string
	Embedding Vector
	Metadata  map[string]string
}

type SearchResult struct {
	DocumentID string
	Content    string
	Score      float32
	Metadata   map[string]string
}

func CosineSimilarity(a, b Vector) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

type scoredDoc struct {
	doc   Document
	score float32
}

func Search(query Vector, documents []Document, topK int) []SearchResult {
	if topK <= 0 {
		topK = len(documents)
	}

	scored := make([]scoredDoc, 0, len(documents))
	for _, doc := range documents {
		score := CosineSimilarity(query, doc.Embedding)
		scored = append(scored, scoredDoc{doc, score})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	results := make([]SearchResult, 0, topK)
	for i := 0; i < topK && i < len(scored); i++ {
		results = append(results, SearchResult{
			DocumentID: scored[i].doc.ID,
			Content:    scored[i].doc.Content,
			Score:      scored[i].score,
			Metadata:   scored[i].doc.Metadata,
		})
	}

	return results
}

func Normalize(v Vector) Vector {
	var norm float64
	for _, x := range v {
		norm += float64(x * x)
	}

	if norm == 0 {
		return v
	}

	norm = math.Sqrt(norm)
	result := make(Vector, len(v))
	for i, x := range v {
		result[i] = float32(float64(x) / norm)
	}

	return result
}

package vector

import "errors"

var (
	ErrEmptyEmbedding    = errors.New("embedding cannot be empty")
	ErrDimensionMismatch = errors.New("vector dimension mismatch")
	ErrNotFound          = errors.New("document not found")
)

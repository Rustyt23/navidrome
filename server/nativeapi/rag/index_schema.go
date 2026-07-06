package rag

import (
	"errors"
	"fmt"
	"strings"
)

const CurrentIndexSchemaVersion = 2

const indexMetadataPointID = "_rag:index-metadata"

var ErrIndexSchemaDrift = errors.New("RAG index schema drift")

// IndexSchema identifies the vector space and payload contract shared by every
// point in a collection. A model tag change is incompatible even when the
// vector dimensions happen to be the same.
type IndexSchema struct {
	Version        int    `json:"indexVersion"`
	EmbeddingModel string `json:"embeddingModel"`
	Dimensions     int    `json:"dimensions"`
}

func ExpectedIndexSchema(embeddingModel string) IndexSchema {
	return IndexSchema{
		Version:        CurrentIndexSchemaVersion,
		EmbeddingModel: strings.TrimSpace(embeddingModel),
		Dimensions:     GeminiEmbeddingDimensions,
	}
}

func (schema IndexSchema) normalized() IndexSchema {
	if schema.Version <= 0 {
		schema.Version = CurrentIndexSchemaVersion
	}
	if schema.Dimensions <= 0 {
		schema.Dimensions = GeminiEmbeddingDimensions
	}
	schema.EmbeddingModel = strings.TrimSpace(schema.EmbeddingModel)
	return schema
}

func (schema IndexSchema) compatibilityError(actual IndexSchema) error {
	expected := schema.normalized()
	actual.EmbeddingModel = strings.TrimSpace(actual.EmbeddingModel)
	if actual.Version != expected.Version || actual.Dimensions != expected.Dimensions || actual.EmbeddingModel != expected.EmbeddingModel {
		return fmt.Errorf(
			"stored version=%d model=%q dimensions=%d; expected version=%d model=%q dimensions=%d",
			actual.Version,
			actual.EmbeddingModel,
			actual.Dimensions,
			expected.Version,
			expected.EmbeddingModel,
			expected.Dimensions,
		)
	}
	return nil
}

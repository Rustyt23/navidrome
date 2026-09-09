package aitools_test

import (
	"encoding/json"
	"time"

	"github.com/navidrome/navidrome/server/nativeapi/aitools"
	"github.com/navidrome/navidrome/server/nativeapi/rag"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type searchArgs struct {
	Query   string            `json:"query" jsonschema:"description=What to search for, in plain language"`
	TopK    int               `json:"topK,omitempty"`
	Filters rag.SearchFilters `json:"filters,omitempty"`
}

type operation struct {
	Kind        string `json:"kind" jsonschema:"enum=add|remove|move|replace"`
	MediaFileID string `json:"mediaFileId"`
	ToPosition  *int   `json:"toPosition"`
	Secret      string `json:"-"`
	unexported  string //nolint:unused
}

type proposeArgs struct {
	PlaylistID string      `json:"playlistId"`
	Operations []operation `json:"operations"`
}

type node struct {
	Name     string  `json:"name"`
	Children []*node `json:"children,omitempty"`
}

type base struct {
	ID string `json:"id"`
}

type embedding struct {
	base
	Label string `json:"label,omitempty"`
}

type scalars struct {
	When time.Time       `json:"when"`
	Raw  json.RawMessage `json:"raw,omitempty"`
	Blob []byte          `json:"blob,omitempty"`
	Any  any             `json:"any,omitempty"`
	Rate float64         `json:"rate"`
	Flag bool            `json:"flag"`
}

var _ = Describe("SchemaOf", func() {
	It("marks omitempty and pointer fields optional, everything else required", func() {
		s := aitools.SchemaOf[searchArgs]()

		Expect(s.Type).To(Equal(aitools.TypeObject))
		Expect(s.Required).To(ConsistOf("query"))
		Expect(s.Properties).To(HaveKey("topK"))
		Expect(s.Properties["query"].Type).To(Equal(aitools.TypeString))
		Expect(s.Properties["topK"].Type).To(Equal(aitools.TypeInteger))
	})

	It("reads descriptions and enums from the jsonschema tag", func() {
		Expect(aitools.SchemaOf[searchArgs]().Properties["query"].Description).
			To(Equal("What to search for, in plain language"))

		kind := aitools.SchemaOf[proposeArgs]().Properties["operations"].Items.Properties["kind"]
		Expect(kind.Enum).To(Equal([]string{"add", "remove", "move", "replace"}))
	})

	It("derives the real SearchFilters struct", func() {
		filters := aitools.SchemaOf[searchArgs]().Properties["filters"]

		// Every filter is optional, so nothing in it is required.
		Expect(filters.Required).To(BeEmpty())
		Expect(filters.Properties).To(HaveKey("playCountMin"))
		Expect(filters.Properties["playCountMin"].Type).To(Equal(aitools.TypeInteger))
		Expect(filters.Properties["playCountMin"].Nullable).To(BeTrue())
		Expect(filters.Properties["bpmMin"].Type).To(Equal(aitools.TypeNumber))
		Expect(filters.Properties["hasLyrics"].Type).To(Equal(aitools.TypeBoolean))
		Expect(filters.Properties["genre"].Type).To(Equal(aitools.TypeString))
		Expect(filters.Properties["genre"].Nullable).To(BeFalse())
	})

	It("describes slices of structs through Items", func() {
		s := aitools.SchemaOf[proposeArgs]()

		ops := s.Properties["operations"]
		Expect(ops.Type).To(Equal(aitools.TypeArray))
		Expect(ops.Items.Type).To(Equal(aitools.TypeObject))
		Expect(ops.Items.Required).To(ConsistOf("kind", "mediaFileId"))
	})

	It("skips json:\"-\" and unexported fields", func() {
		item := aitools.SchemaOf[proposeArgs]().Properties["operations"].Items
		Expect(item.Properties).NotTo(HaveKey("Secret"))
		Expect(item.Properties).NotTo(HaveKey("unexported"))
	})

	It("flattens embedded structs the way encoding/json does", func() {
		s := aitools.SchemaOf[embedding]()
		Expect(s.Properties).To(HaveKey("id"))
		Expect(s.Properties).To(HaveKey("label"))
		Expect(s.Required).To(ConsistOf("id"))
	})

	It("terminates on a self-referential type", func() {
		s := aitools.SchemaOf[node]()
		// The recursive branch is described as an open object rather than
		// expanded forever.
		Expect(s.Properties["children"].Items.Type).To(Equal(aitools.TypeObject))
		Expect(s.Properties["children"].Items.Properties).To(BeEmpty())
	})

	It("special-cases time, raw JSON, bytes and any", func() {
		s := aitools.SchemaOf[scalars]()
		Expect(s.Properties["when"].Type).To(Equal(aitools.TypeString))
		Expect(s.Properties["when"].Format).To(Equal("date-time"))
		Expect(s.Properties["raw"].Type).To(BeEmpty())
		Expect(s.Properties["blob"].Type).To(Equal(aitools.TypeString))
		Expect(s.Properties["any"].Type).To(BeEmpty())
		Expect(s.Properties["rate"].Type).To(Equal(aitools.TypeNumber))
		Expect(s.Properties["flag"].Type).To(Equal(aitools.TypeBoolean))
	})

	It("does not leak the strict flag onto the wire", func() {
		encoded, err := json.Marshal(aitools.SchemaOf[searchArgs]())
		Expect(err).ToNot(HaveOccurred())
		Expect(string(encoded)).ToNot(ContainSubstring("strict"))
		Expect(string(encoded)).ToNot(ContainSubstring("additionalProperties"))
		Expect(string(encoded)).To(ContainSubstring(`"type":"object"`))
	})
})

var _ = Describe("Schema.Validate", func() {
	var schema aitools.Schema

	BeforeEach(func() {
		schema = aitools.SchemaOf[searchArgs]()
	})

	It("accepts a well-formed call", func() {
		Expect(schema.Validate([]byte(`{"query":"sad songs","topK":5}`))).To(Succeed())
	})

	It("accepts nested filters", func() {
		Expect(schema.Validate([]byte(
			`{"query":"loud","filters":{"playCountMin":10,"bpmMin":128.5,"hasLyrics":true}}`,
		))).To(Succeed())
	})

	It("reports a missing required argument by name", func() {
		err := schema.Validate([]byte(`{"topK":5}`))
		Expect(err).To(MatchError(ContainSubstring("query is required")))
	})

	It("rejects a hallucinated argument name and lists the real ones", func() {
		err := schema.Validate([]byte(`{"query":"x","play_count_min":3}`))
		Expect(err).To(MatchError(ContainSubstring("play_count_min is not a valid argument")))
		Expect(err).To(MatchError(ContainSubstring("query")))
	})

	It("rejects a hallucinated name inside a nested object, with its path", func() {
		err := schema.Validate([]byte(`{"query":"x","filters":{"yearMinimum":1999}}`))
		Expect(err).To(MatchError(ContainSubstring("filters.yearMinimum is not a valid argument")))
	})

	It("rejects a number sent as a string", func() {
		err := schema.Validate([]byte(`{"query":"x","topK":"5"}`))
		Expect(err).To(MatchError(ContainSubstring("topK must be integer, got string")))
	})

	It("rejects a fractional value for an integer field", func() {
		err := schema.Validate([]byte(`{"query":"x","filters":{"playCountMin":3.5}}`))
		Expect(err).To(MatchError(ContainSubstring("filters.playCountMin must be integer")))
	})

	It("accepts an integer for a number field", func() {
		Expect(schema.Validate([]byte(`{"query":"x","filters":{"bpmMin":128}}`))).To(Succeed())
	})

	It("accepts an explicit null for a pointer field", func() {
		Expect(schema.Validate([]byte(`{"query":"x","filters":{"yearMin":null}}`))).To(Succeed())
	})

	It("rejects an explicit null for a required field", func() {
		err := schema.Validate([]byte(`{"query":null}`))
		Expect(err).To(MatchError(ContainSubstring("query is required")))
	})

	It("treats absent arguments as an empty object", func() {
		type noArgs struct{}
		Expect(aitools.SchemaOf[noArgs]().Validate(nil)).To(Succeed())
		Expect(aitools.SchemaOf[noArgs]().Validate([]byte(`null`))).To(Succeed())
	})

	It("reports malformed JSON rather than panicking", func() {
		err := schema.Validate([]byte(`{"query":`))
		Expect(err).To(MatchError(ContainSubstring("not valid JSON")))
	})

	It("validates array items and reports the index", func() {
		s := aitools.SchemaOf[proposeArgs]()
		Expect(s.Validate([]byte(
			`{"playlistId":"p1","operations":[{"kind":"add","mediaFileId":"m1","toPosition":3}]}`,
		))).To(Succeed())

		err := s.Validate([]byte(
			`{"playlistId":"p1","operations":[{"kind":"add","mediaFileId":"m1"},{"kind":"add"}]}`,
		))
		Expect(err).To(MatchError(ContainSubstring("operations[1].mediaFileId is required")))
	})

	It("enforces enums", func() {
		s := aitools.SchemaOf[proposeArgs]()
		err := s.Validate([]byte(`{"playlistId":"p1","operations":[{"kind":"delete","mediaFileId":"m1"}]}`))
		Expect(err).To(MatchError(ContainSubstring("must be one of add, remove, move, replace")))
	})
})

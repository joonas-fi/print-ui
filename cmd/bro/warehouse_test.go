package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	. "github.com/function61/gokit/builtin"
	"github.com/function61/gokit/testing/assert"
)

func TestRendering(t *testing.T) {
	elements := Must(defineContent("Foo", "1234567890", barcodeKindQR))
	buf := &bytes.Buffer{}
	assert.Ok(t, render(elements, buf))
	// we want to catch during refactoring if any pixels change in the output
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), "38ead797d3a79e431cdfa6f2639bd2a6d4c76cec72a80e942a0c41dbed05c664")

	// now without barcode
	elements = Must(defineContent("Foo", "", barcodeKindQR))
	buf = &bytes.Buffer{}
	assert.Ok(t, render(elements, buf))
	// we want to catch during refactoring if any pixels change in the output
	assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256(buf.Bytes())), "84b4eb182b62f072f294d4a570921752c1d17ec6e0f8be239fb262a49121a8ee")
}

func TestPrintJobs(t *testing.T) {
	for _, tc := range []struct {
		description    string
		expectedOutput string
		uniqueLabels   int
		copiesOfEach   int
		reprint        string
		justText       bool
	}{
		{
			description:  "two labels",
			uniqueLabels: 2,
			copiesOfEach: 1,
			expectedOutput: `[
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": "https://xs.fi/9/id1"
  },
  {
    "LabelHumanReadable": "id2",
    "CodeForBarcode": "https://xs.fi/9/id2"
  }
]`,
		},
		{
			description:  "one label, two copies",
			uniqueLabels: 1,
			copiesOfEach: 2,
			expectedOutput: `[
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": "https://xs.fi/9/id1"
  },
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": "https://xs.fi/9/id1"
  }
]`,
		},
		{
			description:  "two labels, two copies each",
			uniqueLabels: 2,
			copiesOfEach: 2,
			expectedOutput: `[
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": "https://xs.fi/9/id1"
  },
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": "https://xs.fi/9/id1"
  },
  {
    "LabelHumanReadable": "id2",
    "CodeForBarcode": "https://xs.fi/9/id2"
  },
  {
    "LabelHumanReadable": "id2",
    "CodeForBarcode": "https://xs.fi/9/id2"
  }
]`,
		},
		{
			description:  "just text, no barcode",
			uniqueLabels: 1,
			copiesOfEach: 1,
			justText:     true,
			expectedOutput: `[
  {
    "LabelHumanReadable": "id1",
    "CodeForBarcode": ""
  }
]`,
		},
		{
			description:  "re-print an existing label",
			uniqueLabels: 1,
			copiesOfEach: 1,
			reprint:      "old-id",
			expectedOutput: `[
  {
    "LabelHumanReadable": "old-id",
    "CodeForBarcode": "https://xs.fi/9/old-id"
  }
]`,
		},
		/* template for new test:
		{
			description: "twoLabels",
			uniqueLabels:2,
			copiesOfEach:1,
			expectedOutput: `[]`,
		},
		*/
	} {
		t.Run(tc.description, func(t *testing.T) {
			output := makePrintJobs(tc.uniqueLabels, tc.copiesOfEach, tc.reprint, !tc.justText, true, newTestIDBuilder())
			assert.EqualJSON(t, output, tc.expectedOutput)
		})
	}
}

// creates IDs for testing purposes
func newTestIDBuilder() identifierBuilder {
	nextID := 1 // start from this, increment on each build call
	return func() string {
		defer func() { nextID++ }()

		return fmt.Sprintf("id%d", nextID)
	}
}

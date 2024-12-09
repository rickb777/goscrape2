package document

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestSVG(t *testing.T) {
	sample := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.w3.org/1999/xlink" 
height="30" width="200">
  <a href="link1.svg"><text>Link1</text></a>
  <a href="/here/link2.svg"><text>Link2</text></a>
  <a href="http://example.com/there/link3.svg"><text>Link3</text></a>
</svg>`

	u1 := mustParseURL("http://example.com/dir/page.svg")
	u2 := mustParseURL("http://example.com/")

	doc, err := ParseSVG(u1, u2, strings.NewReader(sample))
	require.NoError(t, err)

	_, fixed, _, err := doc.FixURLReferences()
	require.NoError(t, err)
	assert.True(t, fixed)

	//	expected := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://www.w3.org/1999/xlink" height="30" width="200">
	//  <a href="link1.svg"><text>Link1</text></a>
	//  <a href="../here/link2.svg"><text>Link2</text></a>
	//  <a href="../there/link3.svg"><text>Link3</text></a>
	//</svg>`
	//
	//	assert.Equal(t, expected, string(data))
}

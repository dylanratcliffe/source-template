package sources

import (
	"regexp"
	"testing"

	"github.com/overmindtech/sdp-go"
)

// This file contains tests for the ColourNameSource source. It is a good idea
// to write as many exhaustive tests as possible at this level to ensure that
// your source responds correctly to certain requests.
func TestGet(t *testing.T) {
	tests := []SourceTest{
		{
			Name:          "Getting a known colour",
			ItemScope:     "global",
			Query:         "GreenYellow",
			Method:        sdp.QueryMethod_GET,
			ExpectedError: nil,
			ExpectedItems: &ExpectedItems{
				NumItems: 1,
				ExpectedAttributes: []map[string]interface{}{
					{
						"name": "GreenYellow",
					},
				},
			},
		},
		{
			Name:      "Getting an unknown colour",
			ItemScope: "global",
			Query:     "UpsideDownBlack",
			Method:    sdp.QueryMethod_GET,
			ExpectedError: &ExpectedError{
				Type:             sdp.QueryError_NOTFOUND,
				ErrorStringRegex: regexp.MustCompile("not recognized"),
				Scope:            "global",
			},
			ExpectedItems: nil,
		},
		{
			Name:      "Getting an unknown scope",
			ItemScope: "wonkySpace",
			Query:     "Red",
			Method:    sdp.QueryMethod_GET,
			ExpectedError: &ExpectedError{
				Type:             sdp.QueryError_NOSCOPE,
				ErrorStringRegex: regexp.MustCompile("colours are only supported"),
				Scope:            "wonkySpace",
			},
			ExpectedItems: nil,
		},
	}

	RunSourceTests(t, tests, &ColourNameSource{})
}

func TestList(t *testing.T) {
	tests := []SourceTest{
		{
			Name:          "Using correct scope",
			ItemScope:     "global",
			Method:        sdp.QueryMethod_LIST,
			ExpectedError: nil,
			ExpectedItems: &ExpectedItems{
				NumItems: 147,
			},
		},
		{
			Name:      "Using incorrect scope",
			ItemScope: "somethingElse",
			Method:    sdp.QueryMethod_LIST,
			ExpectedError: &ExpectedError{
				Type:             sdp.QueryError_NOSCOPE,
				ErrorStringRegex: regexp.MustCompile("colours are only supported"),
				Scope:            "somethingElse",
			},
			ExpectedItems: nil,
		},
	}

	RunSourceTests(t, tests, &ColourNameSource{})
}

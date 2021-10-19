package sources

import (
	"regexp"
	"testing"

	"github.com/dylanratcliffe/discovery"
	"github.com/dylanratcliffe/sdp-go"
)

// This file contains shared testing libraries to make testing sources easier

type ExpectedError struct {
	// The expected type of the error
	Type sdp.ItemRequestError_ErrorType

	// A pointer to a regex that will be used to validate the error message,
	// leave as `nil`if you don't want to check this
	ErrorStringRegex *regexp.Regexp

	// The context that the error should come from. Leave as "" if you don't
	// want to check this
	Context string
}

type ExpectedItems struct {
	// The expected number of items
	NumItems int
}

type SourceTest struct {
	// Name of the test for logging
	Name string
	// The context to be passed to the Get() request
	ItemContext string
	// The query to be passed
	Query string
	// The method that should be used
	Method sdp.RequestMethod
	// Details of the expected error, `nil` means no error
	ExpectedError *ExpectedError
	// The expected items
	ExpectedItems *ExpectedItems
}

func RunSourceTests(t *testing.T, tests []SourceTest, source discovery.Source) {
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var item *sdp.Item
			var items []*sdp.Item
			var err error

			switch test.Method {
			case sdp.RequestMethod_FIND:
				items, err = source.Find(test.ItemContext)
			case sdp.RequestMethod_SEARCH:
				searchable, ok := source.(discovery.SearchableSource)

				if !ok {
					t.Fatal("Supplied source did not fulfill discovery.SearchableSource interface. Cannot execute search tests against this source")
				}

				items, err = searchable.Search(test.ItemContext, test.Query)
			case sdp.RequestMethod_GET:
				item, err = source.Get(test.ItemContext, test.Query)
				items = []*sdp.Item{item}
			default:
				t.Fatalf("Test Method invalid: %v. Should be one of: sdp.RequestMethod_FIND, sdp.RequestMethod_SEARCH, sdp.RequestMethod_GET", test.Method)
			}

			// If an error was expected then validate that it was found
			if ee := test.ExpectedError; ee != nil {
				if err == nil {
					t.Error("expected error but got nil")
				}

				ire, ok := err.(*sdp.ItemRequestError)

				if !ok {
					t.Fatalf("error returned was type %T, expected *sdp.ItemRequestError", err)
				}

				if ee.Type != ire.ErrorType {
					t.Fatalf("error type was %v, expected %v", ire.ErrorType, ee.Type)
				}

				if ee.Context != "" {
					if ee.Context != ire.Context {
						t.Fatalf("error context was %v, expected %v", ire.Context, ee.Context)
					}
				}

				if ee.ErrorStringRegex != nil {
					if !ee.ErrorStringRegex.MatchString(ire.ErrorString) {
						t.Fatalf("error string did not match regex %v, raw value: %v", ee.ErrorStringRegex, ire.ErrorString)
					}
				}
			}

			if ei := test.ExpectedItems; ei != nil {
				if len(items) != ei.NumItems {
					t.Fatalf("expected %v items, got %v", ei.NumItems, len(items))
				}
			}
		})
	}
}

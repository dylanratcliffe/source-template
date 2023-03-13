package sources

import (
	"context"
	"regexp"
	"testing"

	"github.com/overmindtech/discovery"
	"github.com/overmindtech/sdp-go"
)

// This file contains shared testing libraries to make testing sources easier

type ExpectedError struct {
	// The expected type of the error
	Type sdp.QueryError_ErrorType

	// A pointer to a regex that will be used to validate the error message,
	// leave as `nil`if you don't want to check this
	ErrorStringRegex *regexp.Regexp

	// The scope that the error should come from. Leave as "" if you don't
	// want to check this
	Scope string
}

type ExpectedItems struct {
	// The expected number of items
	NumItems int

	// A list of expected attributes for the items, will be checked in order
	// with the first set of attributes neeing to match those of the first item
	// etc. Note that this doesn't need to have the same number of entries as
	// there are items
	ExpectedAttributes []map[string]interface{}
}

type SourceTest struct {
	// Name of the test for logging
	Name string
	// The scope to be passed to the Get() request
	ItemScope string
	// The query to be passed
	Query string
	// The method that should be used
	Method sdp.QueryMethod
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
			case sdp.QueryMethod_LIST:
				items, err = source.List(context.Background(), test.ItemScope)
			case sdp.QueryMethod_SEARCH:
				searchable, ok := source.(discovery.SearchableSource)

				if !ok {
					t.Fatal("Supplied source did not fulfill discovery.SearchableSource interface. Cannot execute search tests against this source")
				}

				items, err = searchable.Search(context.Background(), test.ItemScope, test.Query)
			case sdp.QueryMethod_GET:
				item, err = source.Get(context.Background(), test.ItemScope, test.Query)
				items = []*sdp.Item{item}
			default:
				t.Fatalf("Test Method invalid: %v. Should be one of: sdp.QueryMethod_LIST, sdp.QueryMethod_SEARCH, sdp.QueryMethod_GET", test.Method)
			}

			// If an error was expected then validate that it was found
			if ee := test.ExpectedError; ee != nil {
				if err == nil {
					t.Error("expected error but got nil")
				}

				ire, ok := err.(*sdp.QueryError)

				if !ok {
					t.Fatalf("error returned was type %T, expected *sdp.QueryError", err)
				}

				if ee.Type != ire.ErrorType {
					t.Fatalf("error type was %v, expected %v", ire.ErrorType, ee.Type)
				}

				if ee.Scope != "" {
					if ee.Scope != ire.Scope {
						t.Fatalf("error scope was %v, expected %v", ire.Scope, ee.Scope)
					}
				}

				if ee.ErrorStringRegex != nil {
					if !ee.ErrorStringRegex.MatchString(ire.ErrorString) {
						t.Fatalf("error string did not match regex %v, raw value: %v", ee.ErrorStringRegex, ire.ErrorString)
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
			}

			if ei := test.ExpectedItems; ei != nil {
				if len(items) != ei.NumItems {
					t.Fatalf("expected %v items, got %v", ei.NumItems, len(items))
				}

				discovery.TestValidateItems(t, items)

				// Loop over the expected attributes and check
				for i, expectedAttributes := range ei.ExpectedAttributes {
					relevantItem := items[i]

					for key, expectedValue := range expectedAttributes {
						value, err := relevantItem.Attributes.Get(key)

						if err != nil {
							t.Error(err)
						}

						// Deal with comparing slices
						if expectedStringSlice, ok := expectedValue.([]interface{}); ok {
							if !interfaceSliceEqual(expectedStringSlice, value.([]interface{})) {
								t.Errorf("expected attribute %v to be %v, got %v", key, expectedValue, value)
							}
						} else {
							if value != expectedValue {
								t.Errorf("expected attribute %v to be %v, got %v", key, expectedValue, value)
							}
						}
					}
				}
			}
		})
	}
}

func interfaceSliceEqual(a, b []interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

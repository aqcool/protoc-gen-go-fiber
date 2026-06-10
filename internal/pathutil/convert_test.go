package pathutil_test

import (
	"testing"

	"github.com/aqcool/protoc-gen-go-fiber/internal/pathutil"
)

func TestConvertSimple(t *testing.T) {
	route, params := pathutil.Convert("/v1/books/{id}")
	if route != "/v1/books/:id" {
		t.Fatalf("route = %q", route)
	}
	if len(params) != 1 || params[0].FieldPath != "id" {
		t.Fatalf("params = %+v", params)
	}
}

func TestConvertNested(t *testing.T) {
	route, params := pathutil.Convert("/v1/books/{book.id}")
	if route != "/v1/books/:book_id" {
		t.Fatalf("route = %q", route)
	}
	if len(params) != 1 || params[0].ParamName != "book_id" {
		t.Fatalf("params = %+v", params)
	}
}

func TestConvertPattern(t *testing.T) {
	route, _ := pathutil.Convert("/v1/{name=projects/*}/members")
	if route != "/v1/:name<regex(^projects/[^/]+$)>/members" {
		t.Fatalf("route = %q", route)
	}
}

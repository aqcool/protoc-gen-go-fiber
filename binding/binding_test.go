package binding_test

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/aqcool/protoc-gen-go-fiber/binding"
)

func TestPathSubstitutesTemplateParams(t *testing.T) {
	msg := &wrapperspb.StringValue{Value: "item-1"}
	got := binding.Path("/v1/items/{value}", msg)
	if got != "/v1/items/item-1" {
		t.Fatalf("got %q", got)
	}
}

func TestPathWithQueryParams(t *testing.T) {
	msg := &wrapperspb.StringValue{Value: "item-1"}
	got := binding.Path("/v1/items/{value}", msg, binding.WithQueryParams())
	if got != "/v1/items/item-1" {
		t.Fatalf("got %q", got)
	}
}

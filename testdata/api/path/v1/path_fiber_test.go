package pathv1_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	pathv1 "github.com/aqcool/protoc-gen-go-fiber/testdata/api/path/v1"
)

type testBooks struct{}

func (testBooks) GetBook(_ context.Context, req *pathv1.GetBookRequest) (*pathv1.Book, error) {
	return &pathv1.Book{
		Id:    req.GetBook().GetId(),
		Title: "book-" + req.GetBook().GetId(),
	}, nil
}

func (testBooks) ListMembers(_ context.Context, req *pathv1.ListMembersRequest) (*pathv1.ListMembersReply, error) {
	return &pathv1.ListMembersReply{Members: []string{req.GetName() + "/alice"}}, nil
}

func TestBooksNestedPathBinding(t *testing.T) {
	app := fiber.New()
	pathv1.RegisterBooksHTTPServer(app, testBooks{})

	t.Run("GetBook", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/books/b42", nil)
		resp, err := app.Test(req, fiber.TestConfig{})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "b42") {
			t.Fatalf("body=%s", body)
		}
	})

	t.Run("ListMembers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/members/demo", nil)
		resp, err := app.Test(req, fiber.TestConfig{})
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, body)
		}
		if !strings.Contains(string(body), "demo") {
			t.Fatalf("body=%s", body)
		}
	})
}

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"paint-canvas-go/internal/model"
)

func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Document{}); err != nil {
		t.Fatal(err)
	}
	return New(db, zap.NewNop(), []string{"http://localhost:5173"}, "test-secret-that-is-long-enough").(*gin.Engine)
}

func perform(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	return performWithToken(router, method, path, body, "")
}

func performWithToken(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func registerTestUser(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	response := perform(router, http.MethodPost, "/api/v1/auth/register",
		`{"email":"`+email+`","password":"correct-horse-battery"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", response.Code, response.Body.String())
	}
	var session struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return session.Token
}

func TestDocumentCRUDAndOptimisticRevision(t *testing.T) {
	router := testRouter(t)
	token := registerTestUser(t, router, "owner@example.com")
	payload := `{"name":"Demo","data":{"activePageId":"p1","pageIds":["p1"],"pages":{"p1":{"id":"p1"}}}}`

	created := performWithToken(router, http.MethodPost, "/api/v1/documents", payload, token)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var createdDoc model.Document
	if err := json.Unmarshal(created.Body.Bytes(), &createdDoc); err != nil {
		t.Fatal(err)
	}
	if createdDoc.ID == "" || createdDoc.Revision != 1 {
		t.Fatalf("unexpected created document: %+v", createdDoc)
	}

	updated := performWithToken(router, http.MethodPut, "/api/v1/documents/"+createdDoc.ID,
		`{"name":"Changed","revision":1,"data":{"activePageId":"p1","pageIds":["p1"],"pages":{"p1":{"id":"p1"}}}}`, token)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.Code, updated.Body.String())
	}
	var updatedDoc model.Document
	if err := json.Unmarshal(updated.Body.Bytes(), &updatedDoc); err != nil {
		t.Fatal(err)
	}
	if updatedDoc.Revision != 2 || updatedDoc.Name != "Changed" {
		t.Fatalf("unexpected updated document: %+v", updatedDoc)
	}

	conflict := performWithToken(router, http.MethodPut, "/api/v1/documents/"+createdDoc.ID,
		`{"name":"Stale","revision":1,"data":{"activePageId":"p1","pageIds":["p1"],"pages":{"p1":{"id":"p1"}}}}`, token)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("stale update status = %d, want %d", conflict.Code, http.StatusConflict)
	}

	got := performWithToken(router, http.MethodGet, "/api/v1/documents/"+createdDoc.ID, "", token)
	if got.Code != http.StatusOK {
		t.Fatalf("get status = %d", got.Code)
	}
	deleted := performWithToken(router, http.MethodDelete, "/api/v1/documents/"+createdDoc.ID, "", token)
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", deleted.Code)
	}
	missing := performWithToken(router, http.MethodGet, "/api/v1/documents/"+createdDoc.ID, "", token)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want %d", missing.Code, http.StatusNotFound)
	}
}

func TestCreateRejectsInvalidCanvasDocument(t *testing.T) {
	router := testRouter(t)
	token := registerTestUser(t, router, "owner@example.com")
	response := performWithToken(router, http.MethodPost, "/api/v1/documents", `{"name":"Demo","data":{}}`, token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestListRejectsInvalidLimit(t *testing.T) {
	router := testRouter(t)
	token := registerTestUser(t, router, "owner@example.com")
	response := performWithToken(router, http.MethodGet, "/api/v1/documents?limit=1000", "", token)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestDocumentsRequireAuthentication(t *testing.T) {
	response := perform(testRouter(t), http.MethodGet, "/api/v1/documents", "")
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestUsersCannotReadOtherUsersDocuments(t *testing.T) {
	router := testRouter(t)
	ownerToken := registerTestUser(t, router, "owner@example.com")
	otherToken := registerTestUser(t, router, "other@example.com")
	created := performWithToken(router, http.MethodPost, "/api/v1/documents",
		`{"name":"Demo","data":{"activePageId":"p1","pageIds":["p1"],"pages":{"p1":{"id":"p1"}}}}`, ownerToken)
	var document model.Document
	if err := json.Unmarshal(created.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	response := performWithToken(router, http.MethodGet, "/api/v1/documents/"+document.ID, "", otherToken)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestRegistrationRejectsDuplicateEmailAndLoginIssuesToken(t *testing.T) {
	router := testRouter(t)
	registerTestUser(t, router, "owner@example.com")
	duplicate := perform(router, http.MethodPost, "/api/v1/auth/register",
		`{"email":"OWNER@example.com","password":"correct-horse-battery"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate registration status = %d, want %d", duplicate.Code, http.StatusConflict)
	}
	login := perform(router, http.MethodPost, "/api/v1/auth/login",
		`{"email":"owner@example.com","password":"correct-horse-battery"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
}

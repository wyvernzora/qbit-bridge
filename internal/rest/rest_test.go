package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qbt "github.com/autobrr/go-qbittorrent"

	"github.com/wyvernzora/qbit-bridge/internal/downloads"
	"github.com/wyvernzora/qbit-bridge/internal/savepath"
)

const (
	validMagnet       = "magnet:?xt=urn:btih:AABBCCDDEEFFAABBCCDDEEFFAABBCCDDEEFFAABB&dn=show"
	validMagnetHash   = "aabbccddeeffaabbccddeeffaabbccddeeffaabb"
	fixture6Downloads = `[
  {"hash":"aaa","name":"anime-show-01","state":"downloading","progress":0.42,"size":100,"dlspeed":500,"upspeed":0,"eta":1234,"ratio":0.0,"tags":"tvdb:12345,weekly","added_on":1000,"save_path":"/data/anime","completion_on":0,"uploaded":0,"downloaded":42,"num_complete":50,"num_seeds":5,"num_leechs":3,"trackers_count":2},
  {"hash":"bbb","name":"anime-show-02","state":"uploading","progress":1.0,"size":200,"dlspeed":0,"upspeed":1000,"eta":0,"ratio":2.5,"tags":"tvdb:67890,complete","added_on":900,"save_path":"/data/anime","completion_on":950,"uploaded":500,"downloaded":200,"num_complete":100,"num_seeds":10,"num_leechs":0,"trackers_count":3},
  {"hash":"eee","name":"just-added","state":"metaDL","progress":0.0,"size":0,"dlspeed":0,"upspeed":0,"eta":0,"ratio":0.0,"tags":"","added_on":600},
  {"hash":"fff","name":"other-anime","state":"downloading","progress":0.1,"size":150,"dlspeed":100,"upspeed":0,"eta":1500,"ratio":0.0,"tags":"tvdb:11111","added_on":500}
]`
	fixture1Download = `[{
  "hash":"aaa","name":"anime-show-01","state":"downloading","progress":0.42,
  "size":100,"total_size":120,"dlspeed":500,"upspeed":0,"eta":1234,"ratio":0.5,
  "tags":"tvdb:12345,weekly","added_on":1000,"last_activity":1500,
  "save_path":"/data/anime","content_path":"/data/anime/show.mkv","magnet_uri":"magnet:?xt=urn:btih:aaa"
}]`
)

type capturedReq struct {
	Method   string
	Path     string
	Query    url.Values
	PostForm url.Values
}

type mockRoute struct {
	status      int
	body        string
	contentType string
}

func startRESTTestServer(t *testing.T, routes map[string]mockRoute, resolverSpec string) (server *httptest.Server, captured map[string]*capturedReq) {
	t.Helper()
	client, captured := newQbitMockRoutes(t, routes)
	resolver, err := savepath.Parse(resolverSpec)
	if err != nil {
		t.Fatalf("savepath.Parse(%q): %v", resolverSpec, err)
	}
	mux := http.NewServeMux()
	Register(mux, downloads.New(client, resolver, discardLogger()))
	return httptest.NewServer(mux), captured
}

func newQbitMockRoutes(t *testing.T, routes map[string]mockRoute) (client *qbt.Client, captured map[string]*capturedReq) {
	t.Helper()
	captured = make(map[string]*capturedReq, len(routes))
	for k := range routes {
		captured[k] = &capturedReq{}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			t.Errorf("unrouted request to %q", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		cap := captured[r.URL.Path]
		cap.Method = r.Method
		cap.Path = r.URL.Path
		cap.Query = r.URL.Query()
		if r.Method == http.MethodPost {
			_ = r.ParseForm()
			cap.PostForm = r.PostForm
		}
		if route.status != 0 && route.status != http.StatusOK {
			w.WriteHeader(route.status)
			return
		}
		contentType := route.contentType
		if contentType == "" {
			contentType = "application/json"
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(route.body))
	}))
	t.Cleanup(srv.Close)
	return qbt.NewClient(qbt.Config{Host: srv.URL, Timeout: 2, RetryAttempts: 1, RetryDelay: 1}), captured
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func addRouteOK() map[string]mockRoute {
	return map[string]mockRoute{
		"/api/v2/torrents/info": {body: `[]`},
		"/api/v2/torrents/add":  {body: "Ok.", contentType: "text/plain"},
	}
}

func removeRoutes() map[string]mockRoute {
	return map[string]mockRoute{
		"/api/v2/torrents/info":   {body: fixture6Downloads},
		"/api/v2/torrents/delete": {body: ""},
	}
}

func updateTagRoutes() map[string]mockRoute {
	return map[string]mockRoute{
		"/api/v2/torrents/addTags":    {body: ""},
		"/api/v2/torrents/removeTags": {body: ""},
	}
}

func decodeREST[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}

func TestAddDownload(t *testing.T) {
	hs, captured := startRESTTestServer(t, addRouteOK(), "kura-inbox=/mnt/kura")
	defer hs.Close()

	body := []byte(`{"magnet":"` + validMagnet + `","tags":["tvdb:12345"],"destination":"kura-inbox","rename":"Show"}`)
	resp, err := http.Post(hs.URL+DownloadsPath, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.AddDownloadOutput](t, resp)
	if out.Hash != validMagnetHash || !out.Accepted || out.AlreadyExisted {
		t.Errorf("out = %+v", out)
	}
	if captured["/api/v2/torrents/add"].PostForm.Get("savepath") != "/mnt/kura" {
		t.Errorf("savepath = %q", captured["/api/v2/torrents/add"].PostForm.Get("savepath"))
	}
}

func TestAddDownloadRejectsUnknownField(t *testing.T) {
	hs, captured := startRESTTestServer(t, addRouteOK(), "")
	defer hs.Close()

	resp, err := http.Post(hs.URL+DownloadsPath, "application/json", bytes.NewReader([]byte(`{"magnet":"`+validMagnet+`","delete_files":true}`)))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	out := decodeREST[downloads.ToolError](t, resp)
	if out.Code != downloads.CodeInvalidArgument {
		t.Errorf("code = %q, want %q", out.Code, downloads.CodeInvalidArgument)
	}
	if captured["/api/v2/torrents/add"].Method != "" {
		t.Error("upstream add should not be called on bad JSON")
	}
}

func TestListDownloads(t *testing.T) {
	hs, _ := startRESTTestServer(t, map[string]mockRoute{
		"/api/v2/torrents/info": {body: fixture6Downloads},
	}, "anime=/data/anime")
	defer hs.Close()

	resp, err := http.Get(hs.URL + DownloadsPath + "?states=downloading&tags=tvdb:*&include_fields=save_path&limit=2")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.SearchDownloadsOutput](t, resp)
	if out.Count != 2 {
		t.Fatalf("count = %d, want 2", out.Count)
	}
	got := map[string]downloads.Download{}
	for _, d := range out.Downloads {
		got[d.Hash] = d
	}
	if got["aaa"].SavePath != "anime" {
		t.Errorf("aaa save_path = %q, want anime", got["aaa"].SavePath)
	}
	if _, ok := got["bbb"]; ok {
		t.Error("seeding download bbb should be filtered out")
	}
}

func TestListDownloads_NotTags(t *testing.T) {
	hs, _ := startRESTTestServer(t, map[string]mockRoute{
		"/api/v2/torrents/info": {body: fixture6Downloads},
	}, "")
	defer hs.Close()

	resp, err := http.Get(hs.URL + DownloadsPath + "?states=downloading&tags=tvdb:*&not_tags=weekly")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.SearchDownloadsOutput](t, resp)
	if out.Count != 1 {
		t.Fatalf("count = %d, want 1", out.Count)
	}
	got := map[string]downloads.Download{}
	for _, d := range out.Downloads {
		got[d.Hash] = d
	}
	if got["fff"].Hash != "fff" {
		t.Errorf("downloads = %+v, want fff", got)
	}
	if _, ok := got["aaa"]; ok {
		t.Error("weekly download aaa should be filtered out by not_tags")
	}
}

func TestGetDownload(t *testing.T) {
	hs, captured := startRESTTestServer(t, map[string]mockRoute{
		"/api/v2/torrents/info": {body: fixture1Download},
	}, "anime=/data/anime")
	defer hs.Close()

	resp, err := http.Get(hs.URL + downloadsPathSlash + "aaa?include_fields=content_path")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.Download](t, resp)
	if out.Hash != "aaa" {
		t.Errorf("hash = %q, want aaa", out.Hash)
	}
	if out.ContentPath != "anime:show.mkv" {
		t.Errorf("content_path = %q, want anime:show.mkv", out.ContentPath)
	}
	if captured["/api/v2/torrents/info"].Query.Get("hashes") != "aaa" {
		t.Errorf("upstream hashes = %q, want aaa", captured["/api/v2/torrents/info"].Query.Get("hashes"))
	}
}

func TestGetDownloadMissing(t *testing.T) {
	hs, _ := startRESTTestServer(t, map[string]mockRoute{
		"/api/v2/torrents/info": {body: `[]`},
	}, "")
	defer hs.Close()

	resp, err := http.Get(hs.URL + downloadsPathSlash + "missing")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	out := decodeREST[downloads.ToolError](t, resp)
	if out.Code != downloads.CodeUpstreamNotFound {
		t.Errorf("code = %q, want %q", out.Code, downloads.CodeUpstreamNotFound)
	}
}

func TestRemoveDownload(t *testing.T) {
	hs, captured := startRESTTestServer(t, removeRoutes(), "")
	defer hs.Close()

	req, err := http.NewRequest(http.MethodDelete, hs.URL+downloadsPathSlash+"aaa", http.NoBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.AffectedOutput](t, resp)
	if out.AffectedCount != 1 || len(out.AffectedHashes) != 1 || out.AffectedHashes[0] != "aaa" {
		t.Errorf("out = %+v", out)
	}
	if captured["/api/v2/torrents/delete"].PostForm.Get("hashes") != "aaa" {
		t.Errorf("upstream hashes = %q, want aaa", captured["/api/v2/torrents/delete"].PostForm.Get("hashes"))
	}
	if captured["/api/v2/torrents/delete"].PostForm.Get("deleteFiles") != "false" {
		t.Errorf("deleteFiles = %q, want false", captured["/api/v2/torrents/delete"].PostForm.Get("deleteFiles"))
	}
}

func TestUpdateDownloadTags(t *testing.T) {
	hs, captured := startRESTTestServer(t, updateTagRoutes(), "")
	defer hs.Close()

	body := []byte(`{"add":["require-review"],"remove":["auto-adopt"]}`)
	req, err := http.NewRequest(http.MethodPut, hs.URL+downloadsPathSlash+"aaa/tags", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	out := decodeREST[downloads.AffectedOutput](t, resp)
	if out.AffectedCount != 1 || len(out.AffectedHashes) != 1 || out.AffectedHashes[0] != "aaa" {
		t.Errorf("out = %+v", out)
	}
	if captured["/api/v2/torrents/addTags"].PostForm.Get("hashes") != "aaa" {
		t.Errorf("add hashes = %q, want aaa", captured["/api/v2/torrents/addTags"].PostForm.Get("hashes"))
	}
	if captured["/api/v2/torrents/addTags"].PostForm.Get("tags") != "require-review" {
		t.Errorf("add tags = %q, want require-review", captured["/api/v2/torrents/addTags"].PostForm.Get("tags"))
	}
	if captured["/api/v2/torrents/removeTags"].PostForm.Get("hashes") != "aaa" {
		t.Errorf("remove hashes = %q, want aaa", captured["/api/v2/torrents/removeTags"].PostForm.Get("hashes"))
	}
	if captured["/api/v2/torrents/removeTags"].PostForm.Get("tags") != "auto-adopt" {
		t.Errorf("remove tags = %q, want auto-adopt", captured["/api/v2/torrents/removeTags"].PostForm.Get("tags"))
	}
}

func TestUpdateDownloadTagsRejectsUnknownField(t *testing.T) {
	hs, captured := startRESTTestServer(t, updateTagRoutes(), "")
	defer hs.Close()

	req, err := http.NewRequest(http.MethodPut, hs.URL+downloadsPathSlash+"aaa/tags", bytes.NewReader([]byte(`{"hashes":["bbb"],"add":["tag"]}`)))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	out := decodeREST[downloads.ToolError](t, resp)
	if out.Code != downloads.CodeInvalidArgument {
		t.Errorf("code = %q, want %q", out.Code, downloads.CodeInvalidArgument)
	}
	if captured["/api/v2/torrents/addTags"].Method != "" || captured["/api/v2/torrents/removeTags"].Method != "" {
		t.Error("upstream tag update should not be called on bad JSON")
	}
}

func TestUpdateDownloadTagsMethodNotAllowed(t *testing.T) {
	hs, captured := startRESTTestServer(t, updateTagRoutes(), "")
	defer hs.Close()

	resp, err := http.Get(hs.URL + downloadsPathSlash + "aaa/tags")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if resp.Header.Get("Allow") != "PUT" {
		t.Errorf("allow = %q, want PUT", resp.Header.Get("Allow"))
	}
	if captured["/api/v2/torrents/addTags"].Method != "" || captured["/api/v2/torrents/removeTags"].Method != "" {
		t.Error("upstream tag update should not be called on wrong method")
	}
}

func TestStatusMapping(t *testing.T) {
	hs, _ := startRESTTestServer(t, map[string]mockRoute{
		"/api/v2/torrents/info": {status: http.StatusInternalServerError},
	}, "")
	defer hs.Close()

	resp, err := http.Get(hs.URL + DownloadsPath)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	out := decodeREST[downloads.ToolError](t, resp)
	if out.Code != downloads.CodeUpstreamUnavailable {
		t.Errorf("code = %q, want %q", out.Code, downloads.CodeUpstreamUnavailable)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	hs, _ := startRESTTestServer(t, addRouteOK(), "")
	defer hs.Close()

	req, err := http.NewRequest(http.MethodPut, hs.URL+DownloadsPath, http.NoBody)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", resp.StatusCode)
	}
	if resp.Header.Get("Allow") != "GET, POST" {
		t.Errorf("allow = %q, want GET, POST", resp.Header.Get("Allow"))
	}
}

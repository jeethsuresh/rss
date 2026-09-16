package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/jeeth/rss-reader/backend/internal/application"
	"github.com/jeeth/rss-reader/backend/internal/httpapi"
	"github.com/jeeth/rss-reader/backend/internal/serverstore"
	"github.com/jeeth/rss-reader/backend/internal/storage/sqlite"
	"github.com/jeeth/rss-reader/backend/internal/syncclient"
)

type blockingCrawler struct{ release <-chan struct{} }

func (c blockingCrawler) EnqueueAndKick(context.Context)         {}
func (c blockingCrawler) CrawlOne(context.Context, string) error { return nil }
func (c blockingCrawler) BackfillExtracts(context.Context)       {}
func (c blockingCrawler) FetchLive(context.Context, string) (string, error) {
	<-c.release
	return "", errors.New("released")
}

func TestServeDoesNotLetNetworkWorkBlockOtherRequests(t *testing.T) {
	release := make(chan struct{})
	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	service := &application.Service{
		Crawler: blockingCrawler{release: release},
		Version: "test",
	}
	server := NewServer(service, slog.New(slog.NewTextHandler(io.Discard, nil)), outputWriter)
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(context.Background(), inputReader) }()

	if _, err := io.WriteString(inputWriter,
		`{"id":"slow","method":"articles.fetchLive","params":{"id":"article-1"}}`+"\n"+
			`{"id":"ping","method":"system.ping","params":{}}`+"\n"); err != nil {
		t.Fatal(err)
	}

	responses := json.NewDecoder(outputReader)
	first := Response{}
	firstDone := make(chan error, 1)
	go func() { firstDone <- responses.Decode(&first) }()
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ping was blocked behind the slow network request")
	}
	if first.ID != "ping" || first.Error != nil {
		t.Fatalf("expected ping response first, got %#v", first)
	}

	close(release)
	_ = inputWriter.Close()
	second := Response{}
	if err := responses.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if second.ID != "slow" || second.Error == nil {
		t.Fatalf("expected released slow response, got %#v", second)
	}
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
	_ = outputWriter.Close()
}

type gatedHandlerTransport struct {
	handler http.Handler
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (t *gatedHandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.once.Do(func() {
		close(t.entered)
		<-t.release
	})
	recorder := httptest.NewRecorder()
	t.handler.ServeHTTP(recorder, req)
	response := recorder.Result()
	response.Request = req
	return response, nil
}

func TestServeQueuesRequestsBehindSessionRestore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	serverDB, err := sqlite.Open(filepath.Join(t.TempDir(), "server.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer serverDB.Close()
	remote := httpapi.New(
		serverstore.New(serverDB), &application.Service{}, log,
		httpapi.Config{RegistrationEnabled: true},
	)
	localDB, err := sqlite.Open(filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()

	entered := make(chan struct{})
	release := make(chan struct{})
	manager := syncclient.NewManager(localDB, log, time.Hour)
	manager.HTTP = &http.Client{Transport: &gatedHandlerTransport{
		handler: remote, entered: entered, release: release,
	}}
	manager.Start(ctx)

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	server := NewServer(&application.Service{Version: "test"}, log, outputWriter)
	server.Sync = manager
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(ctx, inputReader) }()

	if _, err := io.WriteString(inputWriter,
		`{"id":"connect","method":"sync.connect","params":{"serverUrl":"http://rss.test","username":"reader","password":"correct horse battery staple","register":true}}`+"\n"+
			`{"id":"ping","method":"system.ping","params":{}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("session restore did not reach the server")
	}

	responses := json.NewDecoder(outputReader)
	first := Response{}
	firstDone := make(chan error, 1)
	go func() { firstDone <- responses.Decode(&first) }()
	select {
	case err := <-firstDone:
		t.Fatalf("request crossed the session restore barrier: response=%#v err=%v", first, err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("session restore did not finish")
	}
	if first.ID != "connect" || first.Error != nil {
		t.Fatalf("expected connect response first, got %#v", first)
	}
	second := Response{}
	if err := responses.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if second.ID != "ping" || second.Error != nil {
		t.Fatalf("expected ping after connect, got %#v", second)
	}

	_ = inputWriter.Close()
	if err := <-serveDone; err != nil {
		t.Fatal(err)
	}
	_ = outputWriter.Close()
}

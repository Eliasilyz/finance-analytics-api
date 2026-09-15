package service

import (
	"context"
	"errors"
	"testing"

	"github.com/eliasilyz/finance-analytics-api/test/mocks"
)

func TestIngestInvalidatesCacheOnSuccess(t *testing.T) {
	store := newFakeStore()
	store.companyID = 42
	store.inserted["prices"] = 2
	store.inserted["income"] = 1
	store.inserted["balance"] = 1
	store.inserted["cashflow"] = 1

	cache := newFakeCache()
	cache.entries["analytics:AAPL"] = []byte(`{"symbol":"AAPL"}`)
	cache.entries["technical:AAPL"] = []byte(`{"symbol":"AAPL"}`)
	svc := NewIngestionService(&mocks.MockProvider{Snapshot: validSnapshot()}, store, testLogger(), cache)

	if _, err := svc.Ingest(context.Background(), "aapl"); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if _, ok := cache.entries["analytics:AAPL"]; ok {
		t.Error("analytics cache entry should be deleted after successful ingest")
	}
	if _, ok := cache.entries["technical:AAPL"]; ok {
		t.Error("technical cache entry should be deleted after successful ingest")
	}
}

func TestIngestFailureKeepsCache(t *testing.T) {
	store := newFakeStore()
	store.recordRunErr = errors.New("boom")

	cache := newFakeCache()
	cache.entries["analytics:AAPL"] = []byte(`{"symbol":"AAPL"}`)
	svc := NewIngestionService(&mocks.MockProvider{Snapshot: validSnapshot()}, store, testLogger(), cache)

	if _, err := svc.Ingest(context.Background(), "aapl"); err == nil {
		t.Fatal("expected ingest failure")
	}
	if _, ok := cache.entries["analytics:AAPL"]; !ok {
		t.Error("cache must not be invalidated when ingestion fails")
	}
}

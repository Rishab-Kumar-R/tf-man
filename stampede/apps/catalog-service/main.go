package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Rishab-Kumar-R/tf-man/stampede/catalog-service/internal/sqlcgen"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/logging"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/secrets"
	"github.com/Rishab-Kumar-R/tf-man/stampede/internal/shutdown"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opensearch-project/opensearch-go/v5"
	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	requestsigner "github.com/opensearch-project/opensearch-go/v5/signer/awsv2"
	"golang.org/x/sync/errgroup"
)

//go:embed db/schema.sql
var schemaFS embed.FS

const productsIndex = "products"

type server struct {
	pool         *pgxpool.Pool
	queries      *sqlcgen.Queries
	relayPool    *pgxpool.Pool
	relayQueries *sqlcgen.Queries
	os           *opensearchapi.Client
	logger       *slog.Logger
}

type productDoc struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	PriceCents  int32  `json:"price_cents"`
}

type createProductRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceCents     int    `json:"price_cents"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func (s *server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.PriceCents <= 0 {
		http.Error(w, "name and a positive price_cents are required", http.StatusBadRequest)
		return
	}

	productID := uuid.New()
	if req.IdempotencyKey != "" {
		parsed, err := uuid.Parse(req.IdempotencyKey)
		if err != nil {
			http.Error(w, "idempotency_key must be a valid UUID", http.StatusBadRequest)
			return
		}
		productID = parsed
	}

	doc := productDoc{
		ID:          productID.String(),
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  int32(req.PriceCents),
	}

	payload, err := json.Marshal(doc)
	if err != nil {
		s.logger.Error("marshal outbox event failed", "error", err, "product_id", productID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		s.logger.Error("begin transaction failed", "error", err, "product_id", productID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.queries.WithTx(tx)

	product, err := qtx.CreateProduct(ctx, sqlcgen.CreateProductParams{
		ID:          productID,
		Name:        req.Name,
		Description: req.Description,
		PriceCents:  int32(req.PriceCents),
	})
	if err != nil {
		s.logger.Error("insert product failed", "error", err, "product_id", productID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := qtx.InsertOutboxEvent(ctx, sqlcgen.InsertOutboxEventParams{
		ID:        productID,
		EventType: "ProductIndexed",
		Payload:   payload,
	}); err != nil {
		s.logger.Error("insert outbox event failed", "error", err, "product_id", productID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("commit transaction failed", "error", err, "product_id", productID)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(productDoc{
		ID:          product.ID.String(),
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
	}); err != nil {
		s.logger.Error("encode response failed", "error", err)
	}
}

func (s *server) runOutboxRelay(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.indexOutboxBatch(ctx)
		}
	}
}

func (s *server) indexOutboxBatch(ctx context.Context) {
	for range 20 {
		hadWork, err := s.indexNextOutboxEvent(ctx)
		if err != nil {
			s.logger.Error("process outbox event failed", "error", err)
		}
		if !hadWork {
			return
		}
	}
}

func (s *server) indexNextOutboxEvent(ctx context.Context) (bool, error) {
	tx, err := s.relayPool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.relayQueries.WithTx(tx)

	rows, err := qtx.ListUnpublishedOutboxEvents(ctx, 1)
	if err != nil {
		return false, fmt.Errorf("list unpublished outbox events: %w", err)
	}
	if len(rows) == 0 {
		return false, nil
	}
	row := rows[0]

	indexCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = s.os.Doc.Index(indexCtx, opensearchapi.IndexReq{
		Index: productsIndex,
		ID:    row.ID.String(),
		Body:  bytes.NewReader(row.Payload),
	})
	if err != nil {
		return true, fmt.Errorf("index product %s in opensearch: %w", row.ID, err)
	}

	if err := qtx.MarkOutboxEventPublished(ctx, row.ID); err != nil {
		return true, fmt.Errorf("mark outbox event %s published: %w", row.ID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return true, fmt.Errorf("commit transaction for %s: %w", row.ID, err)
	}

	return true, nil
}

func (s *server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := s.queries.GetProduct(r.Context(), id)
	if err != nil {
		http.Error(w, "product not found", http.StatusNotFound)
		return
	}

	doc := productDoc{
		ID:          product.ID.String(),
		Name:        product.Name,
		Description: product.Description,
		PriceCents:  product.PriceCents,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(doc); err != nil {
		s.logger.Error("encode response failed", "error", err)
	}
}

func (s *server) handleSearchProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	query := strings.NewReader(fmt.Sprintf(`{
		"query": {
			"multi_match": {
				"query": %q,
				"fields": ["name^2", "description"]
			}
		}
	}`, q))

	resp, err := s.os.Search(ctx, &opensearchapi.SearchReq{
		Indices:    []string{productsIndex},
		BodyReader: query,
	})
	if err != nil {
		s.logger.Error("search failed", "error", err, "query", q)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	results := make([]productDoc, 0, len(resp.Hits.Hits))
	for _, hit := range resp.Hits.Hits {
		var doc productDoc

		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			s.logger.Error("unmarshal search hit failed", "error", err)
			continue
		}

		results = append(results, doc)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(results); err != nil {
		s.logger.Error("encode response failed", "error", err)
	}
}

func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	logger := logging.New("catalog-service")

	ctx, cancel := shutdown.Context()
	defer cancel()

	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		logger.Error("load aws config failed", "error", err)
		os.Exit(1)
	}

	creds, err := secrets.FetchDBCredentials(ctx, secretsmanager.NewFromConfig(awsCfg), os.Getenv("DB_SECRET_ARN"))
	if err != nil {
		logger.Error("fetch db credentials failed", "error", err)
		os.Exit(1)
	}

	dbPoolMaxConns := os.Getenv("DB_POOL_MAX_CONNS")
	if dbPoolMaxConns == "" {
		dbPoolMaxConns = "20"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=%s&pool_min_conns=2",
		creds.Username, creds.Password, creds.Host, creds.Port, creds.DBName, dbPoolMaxConns)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Error("connect to database failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	relayDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require&pool_max_conns=5&pool_min_conns=1",
		creds.Username, creds.Password, creds.Host, creds.Port, creds.DBName)

	relayPool, err := pgxpool.New(ctx, relayDSN)
	if err != nil {
		logger.Error("connect relay pool to database failed", "error", err)
		os.Exit(1)
	}
	defer relayPool.Close()

	schemaSQL, err := schemaFS.ReadFile("db/schema.sql")
	if err != nil {
		logger.Error("read embedded schema failed", "error", err)
		os.Exit(1)
	}

	if _, err := pool.Exec(ctx, string(schemaSQL)); err != nil {
		logger.Error("ensure schema failed", "error", err)
		os.Exit(1)
	}

	signer, err := requestsigner.NewSignerWithService(awsCfg, "es")
	if err != nil {
		logger.Error("create opensearch signer failed", "error", err)
		os.Exit(1)
	}

	osClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{os.Getenv("OPENSEARCH_ENDPOINT")},
			Signer:    signer,
		},
	})
	if err != nil {
		logger.Error("create opensearch client failed", "error", err)
		os.Exit(1)
	}

	_, err = osClient.Indices.Create(ctx, opensearchapi.IndicesCreateReq{Index: productsIndex})
	if err != nil {
		var osErr *opensearch.StructError
		if !errors.As(err, &osErr) || osErr.Err.Type != "resource_already_exists_exception" {
			logger.Error("create opensearch index failed", "error", err)
			os.Exit(1)
		}
	}

	s := &server{
		pool:         pool,
		queries:      sqlcgen.New(pool),
		relayPool:    relayPool,
		relayQueries: sqlcgen.New(relayPool),
		os:           osClient,
		logger:       logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /products", s.handleCreateProduct)
	mux.HandleFunc("GET /products/search", s.handleSearchProducts)
	mux.HandleFunc("GET /products/{id}", s.handleGetProduct)
	mux.HandleFunc("GET /healthz", s.handleHealthz)

	httpServer := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s.runOutboxRelay(gCtx)
		return nil
	})

	g.Go(func() error {
		logger.Info("catalog-service listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	})

	if err := g.Wait(); err != nil {
		logger.Error("service exited with error", "error", err)
		os.Exit(1)
	}

	logger.Info("catalog-service shut down cleanly")
}

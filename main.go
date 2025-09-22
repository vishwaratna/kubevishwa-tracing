package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Product struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type Order struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
}

// Mock data
var users = []User{
	{ID: 1, Name: "Alice Johnson", Email: "alice@example.com"},
	{ID: 2, Name: "Bob Smith", Email: "bob@example.com"},
	{ID: 3, Name: "Charlie Brown", Email: "charlie@example.com"},
}

var products = []Product{
	{ID: 1, Name: "Laptop", Price: 999.99},
	{ID: 2, Name: "Mouse", Price: 29.99},
	{ID: 3, Name: "Keyboard", Price: 79.99},
}

var orders = []Order{}

func initTracer() func() {
	ctx := context.Background()

	// Get OTLP endpoint from environment variable
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "otel-collector.observability.svc.cluster.local:4317"
	}

	// Get timeout from environment variable
	timeoutStr := os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT")
	timeout := 30 * time.Second // Default timeout
	if timeoutStr != "" {
		if timeoutMs, err := strconv.Atoi(timeoutStr); err == nil {
			timeout = time.Duration(timeoutMs) * time.Millisecond
		}
	}

	// Create OTLP exporter
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithTimeout(timeout),
	)
	if err != nil {
		log.Fatalf("Failed to create OTLP exporter: %v", err)
	}

	// Get service name from environment variable
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "kubevishwa-api"
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		log.Fatalf("Failed to create resource: %v", err)
	}

	// Get sampling configuration from environment
	sampler := sdktrace.AlwaysSample() // Default to always sample
	if samplerType := os.Getenv("OTEL_TRACES_SAMPLER"); samplerType == "traceidratio" {
		if samplerArg := os.Getenv("OTEL_TRACES_SAMPLER_ARG"); samplerArg != "" {
			if ratio, err := strconv.ParseFloat(samplerArg, 64); err == nil {
				sampler = sdktrace.TraceIDRatioBased(ratio)
			}
		}
	}

	// Get batch configuration from environment variables
	batchOptions := []sdktrace.BatchSpanProcessorOption{}

	if maxBatchSizeStr := os.Getenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE"); maxBatchSizeStr != "" {
		if maxBatchSize, err := strconv.Atoi(maxBatchSizeStr); err == nil {
			batchOptions = append(batchOptions, sdktrace.WithMaxExportBatchSize(maxBatchSize))
		}
	}

	if scheduleDelayStr := os.Getenv("OTEL_BSP_SCHEDULE_DELAY"); scheduleDelayStr != "" {
		if scheduleDelayMs, err := strconv.Atoi(scheduleDelayStr); err == nil {
			scheduleDelay := time.Duration(scheduleDelayMs) * time.Millisecond
			batchOptions = append(batchOptions, sdktrace.WithBatchTimeout(scheduleDelay))
		}
	}

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, batchOptions...),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer = otel.Tracer(serviceName)

	log.Printf("OpenTelemetry initialized successfully")
	log.Printf("Service name: %s", serviceName)
	log.Printf("OTLP endpoint: %s", otlpEndpoint)
	log.Printf("Sampling rate: %s", os.Getenv("OTEL_TRACES_SAMPLER_ARG"))

	return func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "get_users")
	defer span.End()

	log.Printf("Created span for get_users: %s", span.SpanContext().TraceID())

	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.Int("users.count", len(users)),
	)

	// Simulate some processing time
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)

	span.SetAttributes(attribute.String("http.status_code", "200"))
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "get_user")
	defer span.End()

	userIDStr := r.URL.Query().Get("id")
	if userIDStr == "" {
		span.SetAttributes(attribute.String("error", "missing user ID"))
		http.Error(w, "Missing user ID", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		span.SetAttributes(attribute.String("error", "invalid user ID"))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int("user.id", userID))

	// Simulate database lookup
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

	for _, user := range users {
		if user.ID == userID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)
			span.SetAttributes(attribute.String("user.name", user.Name))
			return
		}
	}

	span.SetAttributes(attribute.String("error", "user not found"))
	http.Error(w, "User not found", http.StatusNotFound)
}

func getProductsHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "get_products")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", r.Method),
		attribute.String("http.url", r.URL.String()),
		attribute.Int("products.count", len(products)),
	)

	// Simulate some processing time
	time.Sleep(time.Duration(rand.Intn(80)) * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func createOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := tracer.Start(r.Context(), "create_order")
	defer span.End()

	if r.Method != http.MethodPost {
		span.SetAttributes(attribute.String("error", "method not allowed"))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		span.SetAttributes(attribute.String("error", "invalid JSON"))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Simulate order processing with child spans
	ctx, validateSpan := tracer.Start(ctx, "validate_order")
	time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
	validateSpan.SetAttributes(
		attribute.Int("order.user_id", order.UserID),
		attribute.Int("order.product_id", order.ProductID),
		attribute.Int("order.quantity", order.Quantity),
	)
	validateSpan.End()

	ctx, calculateSpan := tracer.Start(ctx, "calculate_total")
	time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)

	// Find product price
	var productPrice float64
	for _, product := range products {
		if product.ID == order.ProductID {
			productPrice = product.Price
			break
		}
	}

	order.Total = productPrice * float64(order.Quantity)
	calculateSpan.SetAttributes(attribute.Float64("order.total", order.Total))
	calculateSpan.End()

	ctx, saveSpan := tracer.Start(ctx, "save_order")
	time.Sleep(time.Duration(rand.Intn(40)) * time.Millisecond)
	order.ID = len(orders) + 1
	orders = append(orders, order)
	saveSpan.SetAttributes(attribute.Int("order.id", order.ID))
	saveSpan.End()

	span.SetAttributes(
		attribute.Int("order.id", order.ID),
		attribute.Float64("order.total", order.Total),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	_, span := tracer.Start(r.Context(), "health_check")
	defer span.End()

	response := map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Initialize tracing
	shutdown := initTracer()
	defer shutdown()

	// Create HTTP handlers with OpenTelemetry instrumentation
	mux := http.NewServeMux()
	mux.HandleFunc("/users", getUsersHandler)
	mux.HandleFunc("/user", getUserHandler)
	mux.HandleFunc("/products", getProductsHandler)
	mux.HandleFunc("/orders", createOrderHandler)
	mux.HandleFunc("/health", healthHandler)

	// Wrap the mux with OpenTelemetry HTTP instrumentation
	handler := otelhttp.NewHandler(mux, "kubeVishwa-api")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	log.Printf("OTLP endpoint: %s", os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

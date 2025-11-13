package aichat

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	metric "go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
)

// variables
var apiBaseURL = fmt.Sprintf("https://%s/v1", os.Getenv("OPENAI_HOSTNAME"))
var apiKey = os.Getenv("OPENAI_API_KEY")

func AIChat(model string, role string, message string) string {

	fmt.Printf("AIChat: Called:\n  model: %s\n  role: %s\n  message: %s\n", model, role, message)

	ctx := context.Background()

	// Setup OpenTelemetry Metrics Provider
	mp, err := initOTelMetricsProvider(ctx)
	if err != nil {
		log.Fatalf("failed to initialize metrics provider: %v", err)
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			log.Fatalf("error shutting down metrics provider: %v", err)
		}
	}()
	otel.SetMeterProvider(mp)

	// Initialise Metrics
	err = initOTelMetrics()
	if err != nil {
		log.Fatalf("Failed to init metrics: %v", err)
	}

	// Otel tracer
	// Set up OpenTelemetry trace provider
	tp, err := initOTelTraceProvider(ctx)
	if err != nil {
		log.Fatalf("failed to initialize trace provider: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Fatalf("error shutting down tracer provider: %v", err)
		}
	}()
	otel.SetTracerProvider(tp)
	tracer := otel.Tracer("vllm-client-tracer")

	ctx, span := tracer.Start(ctx,
		"vllm-client-session",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.ServiceNamespace("openai"),
			attribute.String("ai.model", model),
			attribute.String("user.input", message),
			attribute.String("ai.request.role", role),
			attribute.Int("ai.request.message.length", len(message)),
			attribute.String("telemetry.sdk.name", "openlit"),
		),
	)
	defer span.End()

	start := time.Now()

	// Create openapi client with config for custom baseurl and selfsigned certs
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	insecureClient := &http.Client{
		Transport: tr,
		Timeout:   120 * time.Second,
	}
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = apiBaseURL
	config.HTTPClient = insecureClient

	// new openai client with config
	client := openai.NewClientWithConfig(config)

	// call local vLLM AI Server Chat Completion API..
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: role,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: message,
			},
		},
	}

	// using ctx created from otel span
	resp, err := client.CreateChatCompletion(ctx, req)

	latency := time.Since(start).Seconds()

	// Process response
	if err != nil {
		log.Fatalf("AIChat: ChatCompletion error: %v", err)
		span.SetStatus(codes.Error, "ChatCompletion error")
		span.RecordError(err)
	}

	// Record Metrics, Success Counter, API call latency
	mOpts := metric.WithAttributes(
		attribute.String("api.status", "success"),
		attribute.String("api.target", "chat_completion"),
		attribute.String("openai.model", model),
		attribute.String("telemetry.sdk.name", "openlit"),
	)
	successCounter.Add(ctx, 1, mOpts)
	apiLatency.Record(ctx, latency, mOpts)

	// if response recieved
	if len(resp.Choices) > 0 {
		if resp.Usage.TotalTokens > 0 {
			span.SetAttributes(
				attribute.String("assistant.response", resp.Choices[0].Message.Content),
				attribute.Int("ai.usage.prompt_tokens", resp.Usage.PromptTokens),
				attribute.Int("ai.usage.completion_tokens", resp.Usage.CompletionTokens),
				attribute.Int("ai.usage.total_tokens", resp.Usage.TotalTokens),
			)
		}
		span.SetStatus(codes.Ok, "Success")
		return resp.Choices[0].Message.Content
	}
	span.SetStatus(codes.Error, "No response received")
	return "No response received."
}

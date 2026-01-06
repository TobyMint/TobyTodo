package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

type SummaryResponse struct {
	Summary string `json:"summary"`
}

func getAPIKey() string {
	data, err := os.ReadFile(".env.yaml")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ARK_API_KEY:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
	}
	return os.Getenv("ARK_API_KEY")
}

func SummaryHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		http.Error(w, "Missing period parameter", http.StatusBadRequest)
		return
	}

	todos := store.GetCompletedTodosByPeriod(period)
	if len(todos) == 0 {
		json.NewEncoder(w).Encode(SummaryResponse{Summary: "No completed tasks found for this period."})
		return
	}

	// Prepare prompt
	var taskList strings.Builder
	for _, t := range todos {
		taskList.WriteString(fmt.Sprintf("- %s (Completed at: %s)\n", t.Content, t.CompletedAt.Format("2006-01-02 15:04")))
	}

	apiKey := getAPIKey()
	if apiKey == "" {
		http.Error(w, "API Key not found. Please check .env.yaml", http.StatusInternalServerError)
		return
	}

	client := arkruntime.NewClientWithApiKey(
		apiKey,
		arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"),
	)
	ctx := context.Background()

	prompt := fmt.Sprintf(`You are a helpful productivity assistant. 
Below is a list of tasks completed by the user during the period: %s.
Please provide a concise, encouraging, and professional summary of their achievements. 
Group similar tasks if possible and highlight key accomplishments.

Tasks:
%s`, period, taskList.String())

	req := model.CreateChatCompletionRequest{
		Model: "doubao-seed-1-8-251228",
		Messages: []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					ListValue: []*model.ChatCompletionMessageContentPart{
						{
							Type: model.ChatCompletionMessageContentPartTypeText,
							Text: prompt,
						},
					},
				},
			},
		},
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		// Log error for debugging
		fmt.Printf("AI Error: %v\n", err)
		http.Error(w, fmt.Sprintf("AI Service Error: %v", err), http.StatusInternalServerError)
		return
	}

	summary := *resp.Choices[0].Message.Content.StringValue
	json.NewEncoder(w).Encode(SummaryResponse{Summary: summary})
}

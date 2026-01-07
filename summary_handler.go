package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
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

func GetSummary(c *gin.Context) {
	period := c.Query("period")
	if period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing period parameter"})
		return
	}

	store, err := getUserStorage(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	todos := store.GetCompletedTodosByPeriod(period)
	if len(todos) == 0 {
		c.JSON(http.StatusOK, SummaryResponse{Summary: "No completed tasks found for this period."})
		return
	}

	// Prepare prompt
	var taskList strings.Builder
	for _, t := range todos {
		taskList.WriteString(fmt.Sprintf("- %s (Completed at: %s)\n", t.Content, t.CompletedAt.Format("2006-01-02 15:04")))
	}

	apiKey := getAPIKey()
	if apiKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "API Key not found. Please check .env.yaml"})
		return
	}

	client := arkruntime.NewClientWithApiKey(
		apiKey,
		arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"),
	)
	ctx := context.Background()

	prompt := fmt.Sprintf(`你是一个专业的生产力助手。
请对用户在以下时间段完成的任务进行总结： %s。
要求：
1. 使用中文回答。
2. 语言风格要专业、鼓励且简洁。
3. 使用 Markdown 格式（例如使用加粗、列表、小标题等）。
4. 归纳相似的任务，并突出关键成就。

任务列表：
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AI Service Error: %v", err)})
		return
	}

	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content != nil {
		if resp.Choices[0].Message.Content.StringValue != nil {
			c.JSON(http.StatusOK, SummaryResponse{Summary: *resp.Choices[0].Message.Content.StringValue})
			return
		}
		if len(resp.Choices[0].Message.Content.ListValue) > 0 {
			c.JSON(http.StatusOK, SummaryResponse{Summary: resp.Choices[0].Message.Content.ListValue[0].Text})
			return
		}
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "No response from AI"})
}

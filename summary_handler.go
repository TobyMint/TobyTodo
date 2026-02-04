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
请根据用户在以下时间段完成的任务，总结并整理出每天的学习 / 训练打卡记录：%s。
请严格按照下面的要求输出：
1. 使用中文回答，语言风格专业且简洁。
2. 使用 Markdown 格式，可以使用日期等小标题和有序列表。
3. 请根据任务内容，尝试归类到以下几类（如果没有匹配的，那你就自由发挥啦），并用一句话概括：
   - 学习了什么课程的什么知识点
   - 学习了什么技术的哪一部分
   - 干了什么样的杂事儿
   - 刷了哪些八股文或者算法题
   - 做了哪些锻炼
4. 每条打卡记录使用有序列表（1. 2. 3. ...）的形式输出，每条一句话。
5. 建议按照日期分组（从最近一天开始），每一天下面是该日的有序列表。

打卡格式示例（仅作参考，请根据实际任务内容生成）：
1. 学习了 [Go项目开发中级实战课] 的第3节课
2. 算法：练习了排序算法
3. 八股文：深入学习了 vLLM 的 PageAttention 原理
4. 做了 3 组俯卧撑

下面是原始任务列表（可能包含上述类别以外的任务，你可以智能归类或归入“其他”）：
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

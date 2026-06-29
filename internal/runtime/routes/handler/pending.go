package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/agents"
	"github.com/pardnchiu/agenvoy/internal/agents/exec"
	agentTypes "github.com/pardnchiu/agenvoy/internal/agents/types"
	"github.com/pardnchiu/agenvoy/internal/runtime/pubsub"
	"github.com/pardnchiu/agenvoy/internal/tools/interactive"
)

type pendingTaskInfo struct {
	TaskHash     string `json:"task_hash"`
	Objective    string `json:"objective"`
	HasQuestions bool   `json:"has_questions"`
}

func ListSessionPending() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid := strings.TrimSpace(c.Param("session_id"))
		if sid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
			return
		}

		hashes := interactive.ListPendingTasks(sid)
		list := make([]pendingTaskInfo, 0, len(hashes))
		for _, h := range hashes {
			info, ok := interactive.LoadPendingInfo(sid, h)
			if !ok {
				continue
			}
			list = append(list, pendingTaskInfo{
				TaskHash:     info.TaskHash,
				Objective:    info.Objective,
				HasQuestions: info.HasQuestions,
			})
		}
		c.JSON(http.StatusOK, gin.H{"pending": list})
	}
}

type pendingQuestionInfo struct {
	Question    string   `json:"question"`
	Detail      string   `json:"detail,omitempty"`
	Options     []string `json:"options,omitempty"`
	MultiSelect bool     `json:"multi_select,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
}

func GetSessionPendingQuestions() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid := strings.TrimSpace(c.Param("session_id"))
		taskHash := strings.TrimSpace(c.Param("task_hash"))
		if sid == "" || taskHash == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and task_hash are required"})
			return
		}

		questions, err := interactive.LoadPendingQuestions(sid, taskHash)
		if err != nil {
			c.JSON(http.StatusGone, gin.H{"error": "pending task already resolved in another session"})
			return
		}

		list := make([]pendingQuestionInfo, 0, len(questions))
		for _, q := range questions {
			list = append(list, pendingQuestionInfo{
				Question:    q.Question,
				Detail:      q.Detail,
				Options:     q.Options,
				MultiSelect: q.MultiSelect,
				Secret:      q.Secret,
			})
		}
		c.JSON(http.StatusOK, gin.H{"questions": list})
	}
}

func ResumeSessionPending() gin.HandlerFunc {
	return func(c *gin.Context) {
		sid := strings.TrimSpace(c.Param("session_id"))
		taskHash := strings.TrimSpace(c.Param("task_hash"))
		if sid == "" || taskHash == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and task_hash are required"})
			return
		}

		var body struct {
			Answers []any `json:"answers"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		info, ok := interactive.LoadPendingInfo(sid, taskHash)
		if !ok {
			c.JSON(http.StatusGone, gin.H{"error": "pending task already resolved in another session"})
			return
		}

		var content string
		if info.HasQuestions && len(body.Answers) > 0 {
			msg, err := interactive.LoadResumeMessage(sid, taskHash, body.Answers)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("load resume: %v", err)})
				return
			}
			content = msg
		} else {
			msg, err := interactive.LoadResumeMessage(sid, taskHash, nil)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("load resume: %v", err)})
				return
			}
			content = msg
		}

		slog.Info("pending resume via web",
			slog.String("session", sid),
			slog.String("task_hash", taskHash),
			slog.Int("answers", len(body.Answers)))

		events := make(chan agentTypes.Event, 64)
		ctx := c.Request.Context()
		wrapped := pubsub.Wrap(ctx, sid, events, 64)

		go func() {
			defer close(wrapped)

			workDir, _ := os.UserHomeDir()
			scanner := agents.Scanner()
			if scanner != nil {
				scanner.Scan()
			}
			err := exec.Run(
				ctx,
				agents.DispatcherBot(),
				agents.Registry(),
				scanner,
				content,
				nil, nil,
				wrapped,
				true,
				workDir,
				sid,
				taskHash,
			)
			if err != nil {
				wrapped <- agentTypes.Event{Type: agentTypes.EventError, Text: err.Error()}
			}
		}()

		result := collectResult(content, events)
		if result.Error != "" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      result.Error,
				"session_id": sid,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"session_id": sid,
			"text":       result.Text,
		})
	}
}

type resumeResult struct {
	Text  string
	Error string
}

func collectResult(_ string, events <-chan agentTypes.Event) resumeResult {
	var text strings.Builder
	var lastErr string
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return resumeResult{Text: text.String(), Error: lastErr}
			}
			switch ev.Type {
			case agentTypes.EventText, agentTypes.EventTextDone:
				if ev.Text != "" {
					text.WriteString(ev.Text)
				}
			case agentTypes.EventError, agentTypes.EventExecError:
				lastErr = ev.Text
				if ev.Err != nil {
					lastErr = ev.Err.Error()
				}
			}
		case <-timeout:
			return resumeResult{Error: "execution timeout"}
		}
	}
}


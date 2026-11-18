package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	"github.com/pardnchiu/agenvoy/internal/filesystem/skill"
	"github.com/pardnchiu/agenvoy/internal/runtime"
	historystore "github.com/pardnchiu/agenvoy/internal/runtime/store"
	sessionmanager "github.com/pardnchiu/agenvoy/internal/session"
	toolcron "github.com/pardnchiu/agenvoy/internal/tools/cron"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var scheduleName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type scheduleItem struct {
	Type       string     `json:"type"`
	Skill      string     `json:"skill"`
	SessionID  string     `json:"session_id"`
	Expression string     `json:"expression,omitempty"`
	At         *time.Time `json:"at,omitempty"`
}

func loadSchedules(kind string) ([]scheduleItem, error) {
	list := make([]scheduleItem, 0)
	if kind != "task" {
		crons, err := runtime.LoadCrons()
		if err != nil {
			return nil, err
		}
		for _, one := range crons {
			list = append(list, scheduleItem{
				Type:       "cron",
				Skill:      one.Skill,
				SessionID:  one.SessionID,
				Expression: one.Expression,
			})
		}
	}
	if kind != "cron" {
		tasks, err := runtime.LoadTasks()
		if err != nil {
			return nil, err
		}
		for _, one := range tasks {
			at := one.At
			list = append(list, scheduleItem{
				Type:      "task",
				Skill:     one.Skill,
				SessionID: one.SessionID,
				At:        &at,
			})
		}
	}
	return list, nil
}

func GetScheduleSkill() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimPrefix(c.Param("skill"), "/")
		name = strings.TrimSpace(name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "skill is required"})
			return
		}
		raw, err := skill.ReadSchedule(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"skill": name,
			"name":  name,
			"body":  raw,
			"files": skillFiles(filesystem.ScheduleSkillPath(name)),
		})
	}
}

func ListSchedules() gin.HandlerFunc {
	return func(c *gin.Context) {
		kind := strings.ToLower(strings.TrimSpace(c.Query("type")))
		if kind != "" && kind != "cron" && kind != "task" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'cron' or 'task'"})
			return
		}
		list, err := loadSchedules(kind)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"schedules": list})
	}
}

type scheduleBody struct {
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Content     string   `json:"content"`
	SessionID   string   `json:"session_id"`
	Expressions []string `json:"expressions"`
}

func (b scheduleBody) values() []string {
	list := make([]string, 0, len(b.Expressions))
	for _, one := range b.Expressions {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}
		list = append(list, one)
	}
	return list
}

func scheduleTimes(list []string) ([]time.Time, error) {
	out := make([]time.Time, 0, len(list))
	for _, one := range list {
		at, err := toolcron.ParseTime(one)
		if err != nil {
			return nil, err
		}
		if !at.After(time.Now()) {
			return nil, utilsmsg.New(utilsmsg.Invalid, "schedule time", "already gone: %s", one)
		}
		out = append(out, at)
	}
	return out, nil
}

func scheduleExpressions(list []string) error {
	for _, one := range list {
		if err := toolcron.ValidateCron(one); err != nil {
			return err
		}
	}
	return nil
}

func scheduleSession(name, given string) (string, error) {
	if given = strings.TrimSpace(given); given != "" {
		return given, nil
	}
	list, err := loadSchedules("")
	if err != nil {
		return "", err
	}
	for _, one := range list {
		if one.Skill == name && strings.TrimSpace(one.SessionID) != "" {
			return one.SessionID, nil
		}
	}
	return sessionmanager.New("chat-")
}

func prepareSchedule(kind string, list []string) (func(name, sessionID string) error, error) {
	if kind == "cron" {
		if err := scheduleExpressions(list); err != nil {
			return nil, err
		}
		return func(name, sessionID string) error {
			if _, err := runtime.RemoveTask(name); err != nil {
				return err
			}
			return runtime.SetCrons(name, sessionID, list)
		}, nil
	}
	times, err := scheduleTimes(list)
	if err != nil {
		return nil, err
	}
	return func(name, sessionID string) error {
		if _, err := runtime.RemoveCron(name); err != nil {
			return err
		}
		return runtime.SetTasks(name, sessionID, times)
	}, nil
}

func readScheduleBody(c *gin.Context) (scheduleBody, []string, bool) {
	var body scheduleBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return body, nil, false
	}

	body.Type = strings.ToLower(strings.TrimSpace(body.Type))
	if body.Type != "cron" && body.Type != "task" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'cron' or 'task'"})
		return body, nil, false
	}

	body.Name = strings.TrimSpace(body.Name)
	if !scheduleName.MatchString(body.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name allows only A-Z a-z 0-9 _ - and must be 1-64 characters, no spaces"})
		return body, nil, false
	}

	if strings.TrimSpace(body.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return body, nil, false
	}

	list := body.values()
	if len(list) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one " + body.Type + " entry is required"})
		return body, nil, false
	}
	return body, list, true
}

func writeSchedule(c *gin.Context, exists bool) {
	body, list, ok := readScheduleBody(c)
	if !ok {
		return
	}
	if exists && !skill.HasSchedule(body.Name) {
		c.JSON(http.StatusNotFound, gin.H{"error": "schedule skill not found: " + body.Name})
		return
	}
	if !exists && skill.HasSchedule(body.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": "name already taken: " + body.Name})
		return
	}

	commit, err := prepareSchedule(body.Type, list)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessionID, err := scheduleSession(body.Name, body.SessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := skill.WriteSchedule(body.Name, body.Content); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := commit(body.Name, sessionID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"type": body.Type, "name": body.Name, "session_id": sessionID})
}

func CreateSchedule() gin.HandlerFunc {
	return func(c *gin.Context) {
		writeSchedule(c, false)
	}
}

func UpdateSchedule() gin.HandlerFunc {
	return func(c *gin.Context) {
		writeSchedule(c, true)
	}
}

func DeleteSchedule() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Type  string `json:"type"`
			Skill string `json:"skill"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		kind := strings.ToLower(strings.TrimSpace(body.Type))
		if kind != "" && kind != "cron" && kind != "task" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'cron' or 'task'"})
			return
		}
		name := strings.TrimSpace(body.Skill)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "skill is required"})
			return
		}

		sessionID, _ := scheduleSessionOf(name)

		removed := 0
		if kind != "task" {
			count, err := runtime.RemoveCron(name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			removed += count
		}
		if kind != "cron" {
			count, err := runtime.RemoveTask(name)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			removed += count
		}
		if removed == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "schedule not found: " + name})
			return
		}

		bound, err := scheduleBound(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		trashed := false
		if !bound {
			if err := skill.TrashSchedule(c.Request.Context(), name, historystore.Meta{SessionID: sessionID}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			trashed = true
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "removed": removed, "trashed": trashed})
	}
}

func scheduleSessionOf(name string) (string, error) {
	list, err := loadSchedules("")
	if err != nil {
		return "", err
	}
	for _, one := range list {
		if one.Skill == name {
			return one.SessionID, nil
		}
	}
	return "", nil
}

func scheduleBound(name string) (bool, error) {
	list, err := loadSchedules("")
	if err != nil {
		return false, err
	}
	for _, one := range list {
		if one.Skill == name {
			return true, nil
		}
	}
	return false, nil
}

func RunSchedule() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			SessionID string `json:"session_id"`
			Skill     string `json:"skill"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sessionID := strings.TrimSpace(body.SessionID)
		name := strings.TrimSpace(body.Skill)
		if sessionID == "" || name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and skill are required"})
			return
		}
		go runtime.Fire(sessionID, name)
		c.JSON(http.StatusAccepted, gin.H{"ok": true, "started": true})
	}
}

package interactive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

var ErrBackgroundCanceled = errors.New("[Canceled] background task: the owning task was canceled")

var (
	backgroundCancelMu sync.Mutex
	backgroundCancels  = map[string]map[string]context.CancelFunc{}
	backgroundCanceled = map[string]bool{}
	canceledLegs       = map[string]bool{}
)

func backgroundKey(sessionID, taskHash string) string {
	return sessionID + "/" + taskHash
}

func RegisterBackgroundCancel(sessionID, taskHash, id string, cancel context.CancelFunc) {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	if backgroundCancels[key] == nil {
		backgroundCancels[key] = map[string]context.CancelFunc{}
	}
	backgroundCancels[key][id] = cancel
}

func unregisterBackgroundCancel(sessionID, taskHash, id string) {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	delete(backgroundCancels[key], id)
	if len(backgroundCancels[key]) == 0 {
		delete(backgroundCancels, key)
	}
}

func CancelTask(sessionID, taskHash string) {
	StopBackground(sessionID, taskHash)
	if IsOnline(sessionID, taskHash) {
		return
	}
	CancelPending(sessionID, taskHash)
}

func PauseTask(sessionID, taskHash string) {
	StopBackground(sessionID, taskHash)
}

func StopBackground(sessionID, taskHash string) {
	if taskHash == "" {
		return
	}

	backgroundCancelMu.Lock()
	key := backgroundKey(sessionID, taskHash)
	list := make([]context.CancelFunc, 0, len(backgroundCancels[key]))
	for _, cancel := range backgroundCancels[key] {
		list = append(list, cancel)
	}
	if len(list) > 0 {
		backgroundCanceled[key] = true
	}
	backgroundCancelMu.Unlock()

	for _, cancel := range list {
		cancel()
	}

	jobMu.Lock()
	defer jobMu.Unlock()
	for _, job := range parentJobs(sessionID, taskHash) {
		os.Remove(filesystem.BackgroundJobPath(sessionID, job.ID))
	}
}

func takeBackgroundCanceled(sessionID, taskHash string) bool {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()

	key := backgroundKey(sessionID, taskHash)
	if !backgroundCanceled[key] {
		return false
	}
	if len(backgroundCancels[key]) == 0 {
		delete(backgroundCanceled, key)
	}
	return true
}

func CancelBackgroundLeg(sessionID, jobID, id string) error {
	jobMu.Lock()
	job, err := readJob(filesystem.BackgroundJobPath(sessionID, jobID))
	jobMu.Unlock()
	if err != nil {
		return utilsmsg.Trace("readJob", jobID, err)
	}

	backgroundCancelMu.Lock()
	key := backgroundKey(sessionID, job.ParentTask)
	cancel, ok := backgroundCancels[key][id]
	if ok {
		delete(backgroundCancels[key], id)
		if len(backgroundCancels[key]) == 0 {
			delete(backgroundCancels, key)
		}
		canceledLegs[id] = true
	}
	backgroundCancelMu.Unlock()
	if !ok {
		return utilsmsg.New(utilsmsg.NotFound, "background task", "%q is not running", id)
	}
	cancel()

	jobMu.Lock()
	job, err = readJob(filesystem.BackgroundJobPath(sessionID, jobID))
	if err == nil {
		job.Legs = slices.DeleteFunc(job.Legs, func(leg BackgroundTask) bool { return leg.ID == id })
		if len(job.Legs) == 0 {
			os.Remove(filesystem.BackgroundJobPath(sessionID, jobID))
		} else {
			err = writeJob(&job)
		}
	}
	jobMu.Unlock()
	if err != nil {
		return err
	}
	SettleBackground(sessionID, job.ParentTask)
	return nil
}

func takeLegCanceled(id string) bool {
	backgroundCancelMu.Lock()
	defer backgroundCancelMu.Unlock()
	if !canceledLegs[id] {
		return false
	}
	delete(canceledLegs, id)
	return true
}

type RunningLeg struct {
	JobID       string
	ID          string
	Description string
}

func ListRunningBackground(sessionID string) []RunningLeg {
	paths, err := filepath.Glob(filepath.Join(filesystem.BackgroundDir(sessionID), "job_*.json"))
	if err != nil {
		return nil
	}
	var list []RunningLeg
	for _, path := range paths {
		job, err := readJob(path)
		if err != nil {
			continue
		}
		for _, leg := range job.Legs {
			if !leg.Done {
				list = append(list, RunningLeg{JobID: job.ID, ID: leg.ID, Description: leg.Description})
			}
		}
	}
	return list
}

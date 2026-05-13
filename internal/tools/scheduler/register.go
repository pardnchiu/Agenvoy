package scheduler

func Register() {
	registAddTask()
	registAddCron()
	registPatchTask()
	registPatchCron()
	registRemoveTask()
	registRemoveCron()
}

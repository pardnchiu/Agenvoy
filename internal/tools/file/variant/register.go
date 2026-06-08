package variant

const maxReadSize = 1 << 20

func Register() {
	registGenerateTool()
	registPatchTool()
	registRemoveTool()
	registWriteSkill()
	registPatchSkill()
	registRemoveSkill()
}
